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
MARK_MCE, MARK_AVC = "--- kernel mce/edac lines", "--- SELinux AVC denials"
MCE_LINES = 100
MCELOG_PROPS = ("LoadState", "ActiveState", "SubState", "Result", "ConditionResult", "ExecMainStatus")
# The only failed unit tolerated: mcelog 210 on an AMD guest without edac_mce_amd (docs/research-notes.md).
MCELOG_UNIT = "mcelog.service"
MCELOG_FAILED_PROPS = {"LoadState": "loaded", "ActiveState": "failed", "SubState": "failed",
                       "Result": "exit-code", "ExecMainStatus": "1"}
MCELOG_JOURNAL_SIGNATURE = ("does not support this processor", "edac_mce_amd")
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
echo "diag_cpu_vendor=$(awk -F': *' '/^vendor_id/{{print $2; exit}}' /proc/cpuinfo)"
echo "diag_cpu_family=$(awk -F': *' '/^cpu family/{{print $2; exit}}' /proc/cpuinfo)"
echo "diag_cpu_model_name=$(awk -F': *' '/^model name/{{print $2; exit}}' /proc/cpuinfo)"
if test -e /sys/module/edac_mce_amd/initstate; then echo "diag_edac_mce_amd=$(cat /sys/module/edac_mce_amd/initstate)"; elif test -d /sys/module/edac_mce_amd; then echo "diag_edac_mce_amd=present-no-initstate"; else echo "diag_edac_mce_amd=absent"; fi
test -e /dev/mcelog && echo "diag_dev_mcelog=present" || echo "diag_dev_mcelog=absent"
echo "diag_mcelog_service=$(systemctl show mcelog.service {' '.join('-p ' + p for p in MCELOG_PROPS)} 2>/dev/null | paste -sd';' -)"
echo {MARK_JOURNAL}
journalctl -b -k -o short-monotonic --no-pager | tail -n 300
journalctl -b -o short-monotonic --no-pager -u plymouth-start.service -u plymouth-quit.service -u plymouth-quit-wait.service -u gdm.service | tail -n 150
for u in $(systemctl --failed --no-legend --plain | awk '{{print $1}}'); do echo "--- failed unit $u"; journalctl -b -u "$u" -o short-monotonic --no-pager | tail -n 30; done
echo "{MARK_MCE}"
journalctl -b -k -o short-monotonic --no-pager | grep -iE '(^|[^a-z])mce|edac' | head -n {MCE_LINES}
echo "{MARK_AVC}"
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


def unit_properties(text):
    """Parse `systemctl show -p ...` output joined with ';' into a dict."""
    props = {}
    for item in (text or "").split(";"):
        key, sep, value = item.strip().partition("=")
        if sep and key:
            props[key] = value
    return props


def mce_lines(journal):
    """Kernel mce/edac lines from the report's bounded section, or None if the section is missing."""
    if MARK_MCE not in journal:
        return None
    section = journal.split(MARK_MCE, 1)[1].split(MARK_AVC, 1)[0]
    return [l.strip("\r") for l in section.strip("\r\n").splitlines() if l.strip()][:MCE_LINES]


def failed_units(values):
    """Failed system units as reported, or None when the report lacks them."""
    raw = values.get("system_failed_units")
    if raw is None:
        return None
    return [u for u in raw.split(",") if u]


def unit_journal(journal, unit):
    """Journal lines the report recorded for a failed unit, or None if it recorded none."""
    header = f"--- failed unit {unit}"
    if header not in journal:
        return None
    section = journal.split(header, 1)[1]
    lines = []
    for line in section.splitlines()[1:]:
        if line.startswith("--- "):
            break
        lines.append(line.strip("\r"))
    return "\n".join(lines)


def mcelog_amd_mismatches(values, journal):
    """Unmet conditions for the known AMD guest mcelog failure; empty means every condition holds."""
    unmet = []
    if failed_units(values) != [MCELOG_UNIT]:
        unmet.append(f"failed units are {values.get('system_failed_units')!r}, not exactly {MCELOG_UNIT}")
    if values.get("diag_cpu_vendor") != "AuthenticAMD":
        unmet.append(f"cpu vendor {values.get('diag_cpu_vendor')!r}")
    if values.get("diag_dev_mcelog") != "present":
        unmet.append(f"/dev/mcelog {values.get('diag_dev_mcelog')!r}")
    if values.get("diag_edac_mce_amd") not in ("absent", "present-no-initstate"):
        unmet.append(f"edac_mce_amd {values.get('diag_edac_mce_amd')!r}")
    props = unit_properties(values.get("diag_mcelog_service"))
    for key, want in MCELOG_FAILED_PROPS.items():
        if props.get(key) != want:
            unmet.append(f"{key}={props.get(key)!r}, want {want!r}")
    text = (unit_journal(journal, MCELOG_UNIT) or "").lower()
    for phrase in MCELOG_JOURNAL_SIGNATURE:
        if phrase not in text:
            unmet.append(f"unit journal lacks {phrase!r}")
    return unmet


def known_failures(values, journal):
    """Failed units matching a known non-blocking signature; any mismatch leaves them unexpected."""
    return [MCELOG_UNIT] if not mcelog_amd_mismatches(values, journal) else []


def platform_diagnostics(values, journal):
    """Guest CPU and EDAC context for mcelog.service; it affects pass/fail only through the known AMD guest signature."""
    unknown = "unknown"
    props = unit_properties(values.get("diag_mcelog_service"))
    failed = values.get("system_failed_units")
    return {
        "cpu_vendor": values.get("diag_cpu_vendor") or unknown,
        "cpu_family": values.get("diag_cpu_family") or unknown,
        "cpu_model_name": values.get("diag_cpu_model_name") or unknown,
        "edac_mce_amd": values.get("diag_edac_mce_amd") or unknown,
        "dev_mcelog": values.get("diag_dev_mcelog") or unknown,
        "mcelog_service": {p: props.get(p) or unknown for p in MCELOG_PROPS},
        "mcelog_failed": unknown if failed is None else "mcelog.service" in failed.split(","),
        "kernel_mce_edac_lines": mce_lines(journal),
        "mcelog_known_amd_guest": not mcelog_amd_mismatches(values, journal),
        "mcelog_signature_unmet": mcelog_amd_mismatches(values, journal),
    }


def diagnostics_md(diag):
    svc = diag["mcelog_service"]
    status = {True: "FAIL: mcelog.service", False: "mcelog.service not failed", "unknown": "mcelog.service state unknown"}[diag["mcelog_failed"]]
    if diag["mcelog_failed"] is True and diag["mcelog_known_amd_guest"]:
        status = "FAIL (known, non-blocking): mcelog.service, AMD guest without edac_mce_amd"
    lines = ["", "## Platform diagnostics", "",
             f"- {status} ({' '.join(f'{p}={svc[p]}' for p in MCELOG_PROPS)})",
             f"- CPU: vendor={diag['cpu_vendor']} family={diag['cpu_family']} model={diag['cpu_model_name']}",
             f"- edac_mce_amd={diag['edac_mce_amd']} /dev/mcelog={diag['dev_mcelog']}"]
    if diag["mcelog_failed"] is True and not diag["mcelog_known_amd_guest"]:
        lines.append(f"- not the known AMD guest signature: {'; '.join(diag['mcelog_signature_unmet'])}")
    k = diag["kernel_mce_edac_lines"]
    if k is None:
        lines.append("- kernel mce/edac lines: unknown (not reported)")
    else:
        lines.append(f"- kernel mce/edac lines: {len(k)} (first {MCE_LINES} at most)")
        lines += ["", "```"] + (k or ["(none)"]) + ["```"]
    return lines


def dconf_value(plan, key):
    for d in (plan.get("artifact", {}).get("dconf") or {}).get("defaults", []):
        if d["key"] == key:
            return d["value"]
    return None


def evaluate(values, plan, journal=""):
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
    failed = failed_units(values)
    known = known_failures(values, journal)
    unexpected = None if failed is None else [u for u in failed if u not in known]
    detail = f"failed={values.get('system_failed_units')}"
    if known:
        detail += f"; known non-blocking: {','.join(known)} (see Platform diagnostics)"
    check("no unexpected failed system units", unexpected == [], detail)
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
        result["checks"] = evaluate(values, plan, journal)
        result["platform_diagnostics"] = platform_diagnostics(values, journal)
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
    if "platform_diagnostics" in result:
        lines += diagnostics_md(result["platform_diagnostics"])
    lines += ["", "Journal excerpt: journal.txt. Frames: frames/ and summary.md."]
    with open(os.path.join(a.out, "session.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"sessioncheck: {result['result']} ({a.out}/session.md)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
