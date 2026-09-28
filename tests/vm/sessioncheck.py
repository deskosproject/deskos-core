#!/usr/bin/env python3
"""Instrumented GNOME session check for a DeskOS disk image (Tier B).

The disk is never written: the machine boots a throwaway overlay, and the
instrumentation arrives as systemd credentials through SMBIOS type 11
(sysusers.extra, tmpfiles.extra, systemd.extra-unit.*, systemd.unit-dropin.*).
They add a test user with GDM autologin, a session script that records the
GNOME state, and a unit that writes the results and the kernel and Plymouth
journal to the serial port. Kernel arguments and Plymouth are unchanged, but
this is an instrumented boot, not the artifact as shipped: report it as such.

Expected values come from the typed Plan (plan.json) of the tested image.

With --mode journal, no user or session is added: the unit writes the
journal of the previous boot of the same disk, for diagnosing a failed
unmodified boot whose overlay was kept (bootcheck.py --keep-overlay).
"""

import argparse
import base64
import json
import os
import sys

import bootcheck

USER, UID = "deskos-qa", 1500
HOME = f"/var/home/{USER}"
RESULTS = "/var/tmp/deskos-qa/results.txt"
MARK_BEGIN, MARK_JOURNAL, MARK_END = "DESKOS-QA-BEGIN", "DESKOS-QA-JOURNAL-BEGIN", "DESKOS-QA-END"
DOCK_UUID = "dash-to-dock@micxgx.gmail.com"

SESSION_SCRIPT = f"""#!/bin/bash
# Runs inside the autologin GNOME session; records what the session sees.
sleep 20
out={RESULTS}
{{
echo "session_type=$XDG_SESSION_TYPE"
echo "gnome_shell=$(gnome-shell --version)"
echo "disable_user_extensions=$(gsettings get org.gnome.shell disable-user-extensions)"
echo "enabled_extensions=$(gsettings get org.gnome.shell enabled-extensions)"
echo "favorite_apps=$(gsettings get org.gnome.shell favorite-apps)"
echo "picture_uri=$(gsettings get org.gnome.desktop.background picture-uri)"
gnome-extensions info {DOCK_UUID} | awk -F': ' '/Enabled:/{{print "dock_enabled="$2}} /State:/{{print "dock_state="$2}}'
echo "user_failed_units=$(systemctl --user --failed --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
timeout 120 firefox --headless --screenshot /var/tmp/deskos-qa/firefox.png about:blank >/dev/null 2>&1
echo "firefox_headless_exit=$?"
test -s /var/tmp/deskos-qa/firefox.png && echo "firefox_screenshot=yes" || echo "firefox_screenshot=no"
}} > "$out.tmp"
mv "$out.tmp" "$out"
"""

GDM_CONF = f"[daemon]\nAutomaticLoginEnable=True\nAutomaticLogin={USER}\n"

AUTOSTART = f"""[Desktop Entry]
Type=Application
Name=DeskOS QA session check
Exec=/bin/bash {HOME}/deskos-qa-check.sh
NoDisplay=true
X-GNOME-Autostart-enabled=true
"""

REPORT_SESSION = f"""#!/bin/bash
# Waits for the session results, then writes them and the journal to ttyS0.
for i in $(seq 1 240); do test -s {RESULTS} && break; sleep 2; done
stty -F /dev/ttyS0 115200 2>/dev/null
{{
echo {MARK_BEGIN}
test -s {RESULTS} && cat {RESULTS} || echo "session_results=missing"
echo "system_failed_units=$(systemctl --failed --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
echo "flatpak_preinstall=$(systemctl show deskos-flatpak-preinstall.service -p ActiveState -p Result --value | paste -sd/ -)"
echo "plymouth_theme=$(plymouth-set-default-theme)"
echo "cmdline=$(cat /proc/cmdline)"
echo "avc_denials=$(grep -c 'avc:  denied' /var/log/audit/audit.log 2>/dev/null || echo 0)"
echo {MARK_JOURNAL}
journalctl -b -k -o short-monotonic --no-pager | tail -n 300
journalctl -b -o short-monotonic --no-pager -u plymouth-start.service -u plymouth-quit.service -u plymouth-quit-wait.service -u gdm.service | tail -n 150
for u in $(systemctl --failed --no-legend --plain | awk '{{print $1}}'); do echo "--- failed unit $u"; journalctl -b -u "$u" -o short-monotonic --no-pager | tail -n 30; done
echo "--- SELinux AVC denials"
grep 'avc:  denied' /var/log/audit/audit.log 2>/dev/null | tail -n 30
echo {MARK_END}
}} > /dev/ttyS0 2>&1
systemctl --no-block poweroff
"""

REPORT_JOURNAL = f"""#!/bin/bash
# Writes the journal of the previous boot of this disk to ttyS0.
stty -F /dev/ttyS0 115200 2>/dev/null
{{
echo {MARK_BEGIN}
echo "previous_boot=$(journalctl --list-boots --no-pager | tail -n 2 | head -n 1)"
echo {MARK_JOURNAL}
journalctl -b -1 -k -o short-monotonic --no-pager | tail -n 400
journalctl -b -1 -o short-monotonic --no-pager -u plymouth-start.service -u plymouth-quit.service -u plymouth-quit-wait.service -u gdm.service | tail -n 200
echo {MARK_END}
}} > /dev/ttyS0 2>&1
systemctl --no-block poweroff
"""


def tmpfiles_arg(text):
    """Escape file content for a tmpfiles.d argument field."""
    return text.replace("\\", "\\\\").replace("%", "%%").replace("\n", "\\n")


def credentials(mode):
    report = REPORT_SESSION if mode == "session" else REPORT_JOURNAL
    target = "graphical.target" if mode == "session" else "multi-user.target"
    tmpfiles = [
        "d /var/lib/deskos-qa 0755 root root -",
        f"f /var/lib/deskos-qa/report.sh 0755 root root - {tmpfiles_arg(report)}",
    ]
    creds = {}
    if mode == "session":
        creds["sysusers.extra"] = f'u {USER} {UID} "DeskOS QA" {HOME} /bin/bash\n'
        tmpfiles += [
            "d /var/tmp/deskos-qa 1777 root root -",
            f"d {HOME} 0700 {USER} {USER} -",
            f"d {HOME}/.config 0755 {USER} {USER} -",
            f"f {HOME}/.config/gnome-initial-setup-done 0644 {USER} {USER} - yes",
            f"d {HOME}/.config/autostart 0755 {USER} {USER} -",
            f"f {HOME}/.config/autostart/deskos-qa-check.desktop 0644 {USER} {USER} - {tmpfiles_arg(AUTOSTART)}",
            f"f {HOME}/deskos-qa-check.sh 0755 {USER} {USER} - {tmpfiles_arg(SESSION_SCRIPT)}",
            f"f+ /etc/gdm/custom.conf 0644 root root - {tmpfiles_arg(GDM_CONF)}",
        ]
    creds["tmpfiles.extra"] = "\n".join(tmpfiles) + "\n"
    creds["systemd.extra-unit.deskos-qa-report.service"] = (
        "[Unit]\nDescription=DeskOS QA report to the serial port\n"
        f"ConditionPathExists=!/etc/initrd-release\nAfter={target}\n\n"
        "[Service]\nType=oneshot\nExecStart=/bin/bash /var/lib/deskos-qa/report.sh\n"
    )
    creds[f"systemd.unit-dropin.{target}"] = "[Unit]\nWants=deskos-qa-report.service\n"
    return creds


def smbios_args(creds):
    args = []
    for name, value in creds.items():
        b64 = base64.b64encode(value.encode()).decode()
        args += ["-smbios", f"type=11,value=io.systemd.credential.binary:{name}={b64}"]
    return args


def parse_serial(path):
    with open(path, "rb") as f:
        text = f.read().decode("utf-8", "replace")
    if MARK_BEGIN not in text:
        return {}, ""
    body = text.split(MARK_BEGIN, 1)[1].split(MARK_END, 1)[0]
    head, _, journal = body.partition(MARK_JOURNAL)
    values = {}
    for line in head.splitlines():
        key, sep, value = line.strip().partition("=")
        if sep and key.replace("_", "").isalnum():
            values[key] = value
    return values, journal.strip("\r\n")


def dconf_value(plan, key):
    for d in (plan.get("artifact", {}).get("dconf") or {}).get("defaults", []):
        if d["key"] == key:
            return d["value"]
    return None


def evaluate(values, plan):
    checks = []

    def check(name, ok, detail):
        checks.append({"check": name, "pass": bool(ok), "detail": detail})

    check("session results reported", values.get("session_results") != "missing" and "session_type" in values,
          f"session_type={values.get('session_type')}")
    check("user extensions allowed", values.get("disable_user_extensions") == "false",
          f"disable-user-extensions={values.get('disable_user_extensions')}")
    exts = dconf_value(plan, "/org/gnome/shell/enabled-extensions") or ""
    if DOCK_UUID in exts:
        check("Dash to Dock active", values.get("dock_enabled") == "Yes" and values.get("dock_state") in ("ACTIVE", "ENABLED"),
              f"Enabled={values.get('dock_enabled')} State={values.get('dock_state')}")
    fav = dconf_value(plan, "/org/gnome/shell/favorite-apps")
    if fav:
        check("favorites as planned", values.get("favorite_apps") == fav, f"got {values.get('favorite_apps')} want {fav}")
    wp = dconf_value(plan, "/org/gnome/desktop/background/picture-uri")
    if wp:
        check("wallpaper as planned", values.get("picture_uri") == wp, f"got {values.get('picture_uri')} want {wp}")
    if any(p["name"] == "firefox" for p in plan.get("artifact", {}).get("rpmPackages", [])):
        check("Firefox runs headless", values.get("firefox_headless_exit") == "0" and values.get("firefox_screenshot") == "yes",
              f"exit={values.get('firefox_headless_exit')} screenshot={values.get('firefox_screenshot')}")
    check("no failed system units", values.get("system_failed_units", "?") == "", f"failed={values.get('system_failed_units')}")
    check("no failed user units", values.get("user_failed_units", "?") == "", f"failed={values.get('user_failed_units')}")
    return checks


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--disk", required=True, help="qcow2 disk to boot (never written)")
    p.add_argument("--out", required=True, help="new output directory")
    p.add_argument("--plan", help="plan.json of the tested image (required for --mode session)")
    p.add_argument("--mode", choices=["session", "journal"], default="session")
    p.add_argument("--memory", type=int, default=4096)
    p.add_argument("--cpus", type=int, default=4)
    p.add_argument("--boot-timeout", type=int, default=900)
    p.add_argument("--shutdown-timeout", type=int, default=120)
    p.add_argument("--interval", type=float, default=3.0)
    p.add_argument("--qemu")
    p.add_argument("--ovmf-code")
    p.add_argument("--ovmf-vars")
    a = p.parse_args()
    if a.mode == "session" and not a.plan:
        p.error("--plan is required for --mode session")
    plan = json.load(open(a.plan)) if a.plan else {}

    run_args = argparse.Namespace(
        disk=a.disk, out=a.out, qemu=a.qemu, ovmf_code=a.ovmf_code, ovmf_vars=a.ovmf_vars,
        memory=a.memory, cpus=a.cpus, boot_timeout=a.boot_timeout, shutdown_timeout=a.shutdown_timeout,
        interval=a.interval, stable_frames=5, expect_splash=False, hash_disk=True, keep_overlay=False,
        extra_qemu=smbios_args(credentials(a.mode)), until_serial=MARK_END, guest_powers_off=True,
    )
    try:
        summary = bootcheck.run(run_args)
    except bootcheck.HarnessError as e:
        print(f"sessioncheck: {e}", file=sys.stderr)
        return 2

    values, journal = parse_serial(os.path.join(a.out, "serial.log"))
    with open(os.path.join(a.out, "journal.txt"), "w") as f:
        f.write(journal + "\n")
    result = {"mode": a.mode, "instrumented": True, "boot": summary["result"], "boot_failures": summary["failures"],
              "values": values, "plan_workstation": plan.get("workstation", {}).get("name")}
    if a.mode == "session":
        result["checks"] = evaluate(values, plan)
        ok = summary["result"] == "pass" and all(c["pass"] for c in result["checks"])
    else:
        ok = summary["result"] == "pass" and bool(journal)
    result["result"] = "pass" if ok else "fail"
    with open(os.path.join(a.out, "session.json"), "w") as f:
        json.dump(result, f, indent=2)
        f.write("\n")
    lines = [f"# Session check ({a.mode}, instrumented boot): {result['result'].upper()}", ""]
    lines += [f"- Boot: {summary['result']} {summary['failures']}"]
    for c in result.get("checks", []):
        lines.append(f"- {'PASS' if c['pass'] else 'FAIL'}: {c['check']} ({c['detail']})")
    lines += ["", "Journal excerpt: journal.txt. Frames: frames/ and summary.md."]
    with open(os.path.join(a.out, "session.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"sessioncheck: {result['result']} ({a.out}/session.md)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
