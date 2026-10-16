#!/usr/bin/env python3
"""First-boot check for a DeskOS disk image (Tier A, instrumented).

Boots the disk through a throwaway overlay and asks **the guest**, not a screenshot:
that the first-boot wizard is the session that came up, that no account could log in,
and that no service unit failed. No account and no session are added: the point is
the machine as it ships, at its first boot.

This exists because `bootcheck.py` classifies pixels. It stops at the first stable
graphical screen, which on an Ubuntu 26.04 image is GDM, not the wizard — so it
reports the same PASS with the wizard suppressed, and suppressing it is exactly the
defect the base cleanup fixed. The verdict here is the guest's.

The instrumentation is the mechanism `sessioncheck.py` uses: systemd credentials
through SMBIOS type 11 install a script and a unit (`tmpfiles.extra`,
`systemd.extra-unit.*`, `systemd.unit-dropin.*`) that wait for the wizard and report
over the serial port, then power the machine off. The disk is never written, but the
boot is instrumented, not the artifact as shipped: report it as such. The serial
protocol marks are the session check's, since it is the same harness.
"""

import argparse
import json
import os
import sys

import bootcheck
import sessioncheck

MARK_BEGIN = sessioncheck.MARK_BEGIN
MARK_END = sessioncheck.MARK_END
WIZARD = "gnome-initial-setup"
REPORT = "/var/lib/deskos-qa/firstboot-report.sh"
# How long the guest waits for the wizard before reporting what it has. The wizard
# follows the display manager; on the lab Ubuntu image it appears well inside this.
WIZARD_WAIT = 300

REPORT_SCRIPT = f"""#!/bin/bash
# Wait for the first-boot wizard, then report what this guest can see.
#
# Two ways to see the wizard, because which process runs it depends on the GNOME
# version: `gnome-initial-setup` as its own program, or gnome-shell in the
# initial-setup mode. The name is also truncated in /proc/PID/comm (15 characters,
# "gnome-initial-s"), so match command lines, and keep the patterns from matching
# this pgrep itself with the bracket trick.
GIS_RE='[g]nome-initial-setup'
MODE_RE='[i]nitial-setup'
for i in $(seq 1 {WIZARD_WAIT // 5}); do
  if pgrep -f "$GIS_RE" >/dev/null 2>&1 || ps -eo args= | grep -q -- "$MODE_RE"; then break; fi
  sleep 5
done
stty -F /dev/ttyS0 115200 2>/dev/null
{{
echo {MARK_BEGIN}
echo "gis_processes=$(pgrep -c -f "$GIS_RE" 2>/dev/null || true)"
echo "gis_comm=$(ps -eo comm= | grep -i 'gnome-initial' | sort -u | paste -sd, -)"
echo "gis_args=$(ps -eo args= | grep "$GIS_RE" | head -2 | paste -sd'|' -)"
echo "initial_setup_mode=$(ps -eo args= | grep -c -- "$MODE_RE" || true)"
echo "shell_modes_in_use=$(ps -eo args= | grep -o -- '--mode=[a-z-]*' | sort -u | paste -sd, -)"
echo "shell_args=$(ps -eo args= | grep '[g]nome-shell' | head -3 | paste -sd'|' -)"
echo "shell_modes_installed=$(ls /usr/share/gnome-shell/modes/ 2>/dev/null | paste -sd, -)"
echo "gdm_active=$(systemctl is-active gdm.service 2>/dev/null || echo unknown)"
echo "user_sessions=$(loginctl list-sessions --no-legend 2>/dev/null | awk '{{print $3}}' | paste -sd, -)"
echo "sessions_raw=$(loginctl list-sessions --no-legend 2>/dev/null | tr -s ' ' | cut -d' ' -f1-4 | paste -sd'|' -)"
echo "system_failed_services=$(systemctl --failed --type=service --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
echo "system_failed_units=$(systemctl --failed --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
# An account that could log in with a password: uid >= 1000, a usable shell, and a
# shadow field that is neither locked (starts with ! or *) nor empty (no password
# required). What the upstream image ships may be a locked account with no home;
# what must not exist is one somebody could log in as.
echo "loginable_accounts=$(awk -F: 'NR==FNR {{ if ($3 >= 1000 && $7 !~ /(nologin|false)$/) shell[$1] = 1; next }} ($1 in shell) {{ if ($2 !~ /^[!*]/) print $1 }}' /etc/passwd /etc/shadow | paste -sd, -)"
echo "homes=$(ls -A /home 2>/dev/null | paste -sd, -)"
# Why the wizard did or did not start is GDM's to say, not a screenshot's.
echo "gis_marker=$(ls -A /var/lib/gnome-initial-setup/ 2>/dev/null | paste -sd, -)"
echo "gis_user=$(getent passwd gnome-initial-setup | cut -d: -f1,3,7)"
echo "session_detail=$(for s in $(loginctl list-sessions --no-legend 2>/dev/null | awk '{{print $1}}'); do loginctl show-session "$s" -p Name -p User -p Type -p Class -p Service --value 2>/dev/null | paste -sd/ -; done | paste -sd'|' -)"
echo "accounts_cached=$(ls -A /var/lib/AccountsService/users/ 2>/dev/null | paste -sd, -)"
echo "gdm_journal=$(journalctl -b -u gdm.service --no-pager 2>/dev/null | grep -iE 'initial|greeter|no user|session' | tail -6 | tr -s ' ' | cut -c1-170 | paste -sd'|' -)"
echo {MARK_END}
}} > /dev/ttyS0
systemctl poweroff
"""


def credentials():
    """systemd credentials that install the report without touching the disk."""
    return {
        # Directories for the report, then the report itself. `f` truncates, as
        # tmpfiles.d documents for a file it owns.
        "tmpfiles.extra": "\n".join([
            "d /var/lib/deskos-qa 0755 root root -",
            f"f {REPORT} 0755 root root - {sessioncheck.tmpfiles_arg(REPORT_SCRIPT)}",
        ]) + "\n",
        "systemd.extra-unit.deskos-qa-firstboot.service":
            "[Unit]\nDescription=DeskOS QA first-boot report to the serial port\n"
            "ConditionPathExists=!/etc/initrd-release\nAfter=graphical.target\n\n"
            "[Service]\nType=oneshot\nExecStart=/bin/bash " + REPORT + "\n",
        "systemd.unit-dropin.graphical.target": "[Unit]\nWants=deskos-qa-firstboot.service\n",
    }


def check(checks, name, ok, detail):
    checks.append({"check": name, "pass": bool(ok), "detail": detail})


def evaluate(values):
    """The three things a first boot must show. Every value that can differ between
    images is quoted in the detail, and the full failed-unit list is reported even
    where it is not asserted, so a run cannot hide what it did not judge.
    """
    checks = []
    gis = values.get("gis_processes", "")
    check(checks, f"the first-boot wizard ({WIZARD}) is running",
          gis.isdigit() and int(gis) > 0,
          f"gis_processes={gis!r} gis_comm={values.get('gis_comm')!r} "
          f"gis_args={values.get('gis_args')!r} gdm_active={values.get('gdm_active')!r} "
          f"sessions={values.get('user_sessions')!r} sessions_raw={values.get('sessions_raw')!r}")

    loginable = values.get("loginable_accounts")
    check(checks, "no account can log in with a password",
          loginable == "", f"loginable_accounts={loginable!r} homes={values.get('homes')!r}")

    # Service units only. On the Ubuntu image the sssd responder sockets report
    # `dependency` because sssd.service is skipped by its own condition (sssd is
    # installed unconfigured); that is a socket, not a service, and the full list is
    # reported below either way.
    failed = values.get("system_failed_services")
    check(checks, "no failed system service units",
          failed == "", f"failed_services={failed!r} all_failed={values.get('system_failed_units')!r}")
    return checks


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--disk", required=True, help="disk to boot (never written): qcow2 or raw")
    p.add_argument("--out", required=True, help="new output directory")
    p.add_argument("--memory", type=int, default=4096)
    p.add_argument("--cpus", type=int, default=4)
    p.add_argument("--boot-timeout", type=int, default=900,
                   help="seconds; must cover the wait for the wizard")
    p.add_argument("--shutdown-timeout", type=int, default=120)
    p.add_argument("--interval", type=float, default=3.0)
    p.add_argument("--qemu")
    p.add_argument("--ovmf-code")
    p.add_argument("--ovmf-vars")
    a = p.parse_args()

    run_args = argparse.Namespace(
        disk=a.disk, out=a.out, qemu=a.qemu, ovmf_code=a.ovmf_code, ovmf_vars=a.ovmf_vars,
        memory=a.memory, cpus=a.cpus, boot_timeout=a.boot_timeout, shutdown_timeout=a.shutdown_timeout,
        interval=a.interval, stable_frames=5, expect_splash=False, hash_disk=True, keep_overlay=False,
        extra_qemu=sessioncheck.smbios_args(credentials()),
        until_serial=MARK_END, guest_powers_off=True,
    )
    try:
        summary = bootcheck.run(run_args)
    except bootcheck.HarnessError as e:
        print(f"firstbootcheck: {e}", file=sys.stderr)
        return 2

    values, _ = sessioncheck.parse_serial(os.path.join(a.out, "serial.log"))
    checks = evaluate(values)
    ok = summary["result"] == "pass" and all(c["pass"] for c in checks)
    result = {"mode": "firstboot", "instrumented": True,
              "boot": summary["result"], "boot_failures": summary["failures"],
              "values": values, "checks": checks,
              "result": "pass" if ok else "fail"}
    with open(os.path.join(a.out, "firstboot.json"), "w") as f:
        json.dump(result, f, indent=2)
        f.write("\n")
    lines = [f"# First-boot check (instrumented boot): {result['result'].upper()}", "",
             f"- Boot: {summary['result']} {summary['failures']}"]
    lines += [f"- {'PASS' if c['pass'] else 'FAIL'}: {c['check']} ({c['detail']})" for c in checks]
    lines += ["", "Frames: summary.md. Serial: serial.log."]
    with open(os.path.join(a.out, "firstboot.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"firstbootcheck: {result['result']} ({a.out}/firstboot.md)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
