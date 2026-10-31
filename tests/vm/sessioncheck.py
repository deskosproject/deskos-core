#!/usr/bin/env python3
"""Instrumented GNOME session check for a DeskOS disk image (Tier B).

The disk is never written: the machine boots a throwaway overlay, and the
instrumentation arrives as systemd credentials through SMBIOS type 11
(sysusers.extra, tmpfiles.extra, systemd.extra-unit.*, systemd.unit-dropin.*).
They add a test user with GDM autologin, a session script that records the
GNOME state, and a unit that writes the results and the kernel and Plymouth
journal to the serial port. Kernel arguments and Plymouth are unchanged, but
this is an instrumented boot, not the artifact as shipped: report it as such.

Expected values come from the typed Plan (plan.json) of the tested image. Every
dconf default the plan declares is read back from the session and compared, and
the list of defaults comes from the plan, not from a constant here: a default
declared later is asserted without editing the harness.

With --mode journal, no user or session is added: the unit writes the
journal of the previous boot of the same disk, for diagnosing a failed
unmodified boot whose overlay was kept (bootcheck.py --keep-overlay).
"""

import argparse
import base64
import hashlib
import http.server
import json
import os
import re
import shlex
import sys
import threading

import bootcheck

USER, UID = "deskos-qa", 1500
HOME = f"/var/home/{USER}"
RESULTS = "/var/tmp/deskos-qa/results.txt"
MARK_BEGIN, MARK_JOURNAL, MARK_END = "DESKOS-QA-BEGIN", "DESKOS-QA-JOURNAL-BEGIN", "DESKOS-QA-END"
# The page the guest's browser must fetch from the host side of QEMU's user
# network. A headless Firefox on about:blank proves the binary starts; this
# marker proves the network path and the renderer did something with real bytes.
BROWSER_MARKER = "DESKOS-QA-BROWSER-OK"
BROWSER_HOST = "10.0.2.2"
MARK_MCE, MARK_AVC = "--- kernel mce/edac lines", "--- SELinux AVC denials"
MCE_LINES = 100
MCELOG_PROPS = ("LoadState", "ActiveState", "SubState", "Result", "ConditionResult", "ExecMainStatus")
# The only failed unit tolerated: mcelog 210 on an AMD guest without edac_mce_amd (docs/research-notes.md).
MCELOG_UNIT = "mcelog.service"
MCELOG_FAILED_PROPS = {"LoadState": "loaded", "ActiveState": "failed", "SubState": "failed",
                       "Result": "exit-code", "ExecMainStatus": "1"}
MCELOG_JOURNAL_SIGNATURE = ("does not support this processor", "edac_mce_amd")
# Every extension the composition turned on is asked about inside the session: a
# decision that does not really load is a defect, not a silent no-op (docs/adr/0017).
ENABLED_EXTENSIONS_PATH = "/var/lib/deskos-qa/enabled-extensions.keys"
# The binaries the plan installs, one destination per line. What is asserted about them
# is not that they exist but that their bytes are the ones the plan pinned: a
# checksum-verified download is the whole claim, and the chain should be closed on a
# booted machine and not only in the build (docs/adr/0018).
BINARY_PATHS_PATH = "/var/lib/deskos-qa/binaries.keys"
# The plan's dconf defaults are read back inside the session. `dconf read` reports
# the effective value in the same GVariant text the plan stores (measured: uint32 0,
# true, ['a', 'b'], @as [], 'a b'), so the plan's value is compared with what the
# session sees. The key list travels as a file of its own so the script that reads
# it stays inside the tmpfiles.d argument cap (tests/vm/README.md) as plans grow.
DCONF_KEYS_PATH = "/var/lib/deskos-qa/dconf-defaults.keys"
# The dconf keys the plan locks. The session tries to write each one and must
# fail: a rendered lock file is not evidence that the session enforces it.
DCONF_LOCKS_PATH = "/var/lib/deskos-qa/dconf-locks.keys"
# The lock probe is its own script: the session report is close to systemd's
# 4095 B tmpfiles argument limit, and adding the probe inline pushed it over.
LOCK_SCRIPT_PATH = "/var/lib/deskos-qa/lock-check.sh"
# The tray probe is its own script for the same reason; its retry loop pushed
# the report over the limit too.
TRAY_SCRIPT_PATH = "/var/lib/deskos-qa/tray-check.sh"
# The domain join runs before GDM, from runtime credentials; the session report
# reads the status file it leaves behind.
JOIN_SCRIPT_PATH = "/var/lib/deskos-qa/join.sh"
JOIN_STATUS_SCRIPT_PATH = "/var/lib/deskos-qa/join-status.sh"
JOIN_CLIENT_SCRIPT_PATH = "/var/lib/deskos-qa/join-client.sh"
WATCHDOG_SCRIPT_PATH = "/var/lib/deskos-qa/watchdog.sh"
JOIN_ENV_PATH = "/var/lib/deskos-qa/join.env"
JOIN_STATUS_PATH = "/var/lib/deskos-qa/join.status"
# The probes live outside the home: with a directory user the home does not
# exist until first login, so nothing can be seeded there in advance. A
# system-wide autostart runs them in whichever account GDM logs in.
QA_SCRIPT_PATH = "/var/lib/deskos-qa/deskos-qa-check.sh"
# The reads that need more than a line live in their own file, because one tmpfiles
# argument is capped at 4095 bytes and the main script ran into it (job 123867637679).
QA_READS_PATH = "/var/lib/deskos-qa/deskos-qa-reads.sh"
E2E_PROBE_PATH = "/var/lib/deskos-qa/e2e-apps.py"
QA_PROBE_PATH = "/var/lib/deskos-qa/deskos-qa-probe.sh"
QA_BROWSER_PATH = "/var/lib/deskos-qa/deskos-qa-browser.sh"
AUTOSTART_PATH = "/etc/xdg/autostart/deskos-qa-check.desktop"
# The plan's GSettings vendor defaults (the compiled `*.gschema.override` path)
# travel as schema and key per line, tab separated, for the same reason.
VENDOR_KEYS_PATH = "/var/lib/deskos-qa/vendor-defaults.keys"
# A profile that names only the image's own dconf database, so the value the
# image ships is read apart from the value the session ends up with. A default is
# not a lock: a user, or a local or site value, wins over it (AGENTS.md
# invariant 8; docs/research-notes.md, "GNOME administration").
DCONF_PROFILE_PATH = "/etc/dconf/profile/deskos-qa-defaults"
DEFAULT_CHECK_PREFIX = "plan default "
GREETER_CHECK_PREFIX = "greeter default "
BINARY_CHECK_PREFIX = "plan artifact file "
VENDOR_CHECK_PREFIX = "plan vendor default "
# Sentinels for what the guest read; neither can be a GVariant text value, so they
# are told apart from a value the key really has.
UNSET, ERROR = "<unset>", "<error>"
# The declared defaults whose dconf value is a *set*: GNOME treats these as an
# unordered collection, and the Shell itself rewrites them (enabling or disabling
# an extension rewrites the whole list). Every other array is compared in order,
# because the plan fixes that order: `favorite-apps` has its own order-sensitive
# check for exactly this reason, and comparing every array as a multiset would
# mask a reordered list. Adding a key here is a claim about GNOME's semantics, so
# it needs a reason, not convenience.
UNORDERED_DEFAULTS = {
    "/org/gnome/shell/enabled-extensions",
    "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings",
}
# A session value that differs from the declared default fails the check: the run
# is a fresh account on a fresh install, so nothing has had the chance to set a
# value a user would set. A key the desktop itself rewrites on first login is the
# exception, declared here with the reason the report prints, so the gap is written
# down instead of assumed. The entry names the delivery path as well as the key,
# because the same key can be declared on more than one (the wallpaper is a dconf
# default and a GSettings vendor default), and an exception on one path must not
# excuse the other: the key is "dconf:<dconf key>" or "gsettings:<schema> <key>".
EXPLAINED_SESSION_DIFFERENCES = {}


def difference_id(source, key):
    """The report's identity for one declared default: delivery path and key."""
    return f"{source}:{key}"


def extension_reads():
    """The script fragment that asks GNOME about every extension the plan enabled.

    Membership in `enabled-extensions` is only the declared default: whether the
    extension really loads is a session fact, and this is where it is read
    (docs/adr/0017). LC_ALL=C keeps the tool's labels English, so the parse does not
    depend on the guest's locale. What is read is the state GNOME Shell reports for a
    fresh user, not that the extension does its job: the state checks of the test
    design are what look at behavior.
    """
    return (
        "# Every extension the composition enabled, asked about in the session.\n"
        f"mapfile -t enabled_extensions < {ENABLED_EXTENSIONS_PATH}\n"
        'for i in "${!enabled_extensions[@]}"; do\n'
        '  u="${enabled_extensions[$i]}"\n'
        '  info=$(LC_ALL=C gnome-extensions info "$u" 2>/dev/null) || info=""\n'
        '  if [ -z "$info" ]; then\n'
        '    echo "extension_$i=missing"\n'
        '  else\n'
        # `gnome-extensions info` can print more than one State line. Keep the
        # last one, as GNOME's own output does, rather than emitting a value
        # with a newline that the host parser cannot read.
        '    e=$(printf %s "$info" | awk \'/^[[:space:]]*Enabled:/ { enabled=$NF } END { print enabled }\')\n'
        '    s=$(printf %s "$info" | awk \'/^[[:space:]]*State:/ { state=$NF } END { print state }\')\n'
        '    echo "extension_$i=$e/$s"\n'
        '  fi\n'
        "done\n"
    )


def tray_probe():
    """The appindicator extension's behaviour, read from the session bus.

    ``gnome-extensions info`` proves the shell loaded the extension; it does not
    prove the extension does anything. The AppIndicator extension owns
    ``org.kde.StatusNotifierWatcher`` on the session bus, so this asks the service
    to accept a registration. That is not the same as proving the item stays
    registered — the caller is a short-lived gdbus process — and it is not a
    promise that an icon is drawn for a real application; both are said plainly
    in the check names and in ADR 0021.
    """
    call = (
        "gdbus call --session --dest org.kde.StatusNotifierWatcher "
        "--object-path /StatusNotifierWatcher "
        "--method org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem "
        "/StatusNotifierItem"
    )
    return (
        "if gdbus introspect --session --dest org.kde.StatusNotifierWatcher "
        "--object-path /StatusNotifierWatcher > /dev/null 2>&1; then\n"
        '  echo "tray_watcher=present"\n'
        # The extension registers its watcher a moment after the shell starts and
        # can reject a registration while it settles. Retry instead of turning a
        # startup race into a failed session; the last error is kept in the report.
        '  ok=no\n'
        '  err=""\n'
        '  for i in 1 2 3 4 5; do\n'
        '    err="$(' + call + ' 2>&1)" && { ok=yes; break; }\n'
        '    sleep 2\n'
        '  done\n'
        '  if [ "$ok" = yes ]; then\n'
        '    echo "tray_register=accepted"\n'
        "  else\n"
        '    echo "tray_register=rejected"\n'
        '    echo "tray_register_error=$err"\n'
        "  fi\n"
        "else\n"
        '  echo "tray_watcher=absent"\n'
        '  echo "tray_register=skipped"\n'
        "fi\n"
    )


def network_reads():
    """The script fragment that lists the connections NetworkManager accepted.

    `nmcli connection show` is what the daemon parsed, not what the file says, so
    a keyfile the build wrote and NetworkManager refused is a failure rather than
    an assumption. It does not activate anything: a lab with a RADIUS server is
    what proves authentication.
    """
    return (
        "# Every connection NetworkManager accepted, so a declared profile can be found.\n"
        "printf 'nm_profiles='; "
        "nmcli -t -f NAME connection show 2>/dev/null | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | "
        "grep -v '^$' | sort -u | paste -sd'|' -; echo\n"
    )


def font_reads():
    """The script fragment that lists the font families the image carries.

    `fc-list` asks fontconfig, which is what an application asks, so the answer is
    the one a session gets. One read for every declared family: the list is the
    image's, and comparing it with the plan is this checker's job.
    """
    return (
        "# Every font family fontconfig can see, so a declared family can be found.\n"
        "printf 'font_families='; "
        "fc-list -f '%{family}\\n' 2>/dev/null | tr ',' '\\n' | "
        "sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -v '^$' | sort -u | paste -sd'|' -; echo\n"
    )


def binary_reads():
    """The script fragment that hashes every file the plan's artifacts place.

    The declared checksum is what the image downloaded and verified at build time; this
    reads the bytes that are really on the machine, so a file the build never wrote, or
    one replaced afterwards, is a difference rather than an assumption. The files are
    not all executables: a font is placed with no execute bit, and its mode is asserted
    against the plan like any other (ADR 0018, amended 2026-10-29).
    """
    return (
        "# Every file the plan's artifacts place, hashed where it landed.\n"
        f"mapfile -t plan_binaries < {BINARY_PATHS_PATH}\n"
        'for i in "${!plan_binaries[@]}"; do\n'
        '  b="${plan_binaries[$i]}"\n'
        '  if [ -f "$b" ]; then\n'
        # An executable file whose bytes cannot be read is not a file that changed: it
        # is a fact this machine would not give up, and the host separates it from a
        # mismatch, the same way a session read that fails is reported with its reason.
        '    h="$(sha256sum "$b" 2>/dev/null | cut -d" " -f1)"\n'
        '    if [ -n "$h" ]; then echo "binary_$i=$h"; else echo "binary_$i=unreadable"; fi\n'
        # The plan declares the mode too, so the guest reports it: a file with the right
        # bytes and the wrong mode is not the file the plan says it installs. The reads
        # follow a link, as the byte read does, so both describe the file that runs.
        '    m="$(stat -Lc %a "$b" 2>/dev/null)"\n'
        '    echo "binary_mode_$i=${m:-unreadable}"\n'
        '  else\n'
        '    echo "binary_$i=missing"\n'
        '    echo "binary_mode_$i=missing"\n'
        '  fi\n'
        "done\n"
    )


def default_reads(image_database, vendor):
    """The script fragment that reads every default the plan declares.

    Two reads per default, because they are two different claims: what the image
    carries, and what the session ends up with. For a dconf default the effective
    value comes first (what the session sees), then the value in the image's own
    database alone, through a profile that names only it, so a default something
    overrode is told apart from a default the image never carried. For a GSettings
    vendor default the session value comes first, then the compiled schema default
    (`GSETTINGS_BACKEND=memory` reads it), which is the value the schemas resolve to
    apart from any dconf value. That read says the compiled default is the declared
    one; it does not attribute the value to DeskOS's own override file, because a
    vendor's override carrying the same value reads the same. The drop-in's content
    is the backend's, covered by its own tests. Each list is read from a file the
    harness writes from the plan, so it is the plan's and not this script's.
    """
    image_read = ""
    if image_database:
        image_read = (
            f'  d=$(DCONF_PROFILE={DCONF_PROFILE_PATH} dconf read "$k" 2>/dev/null) || d="{ERROR}"\n'
            '  if [ -z "$d" ]; then d="' + UNSET + '"; fi\n'
            '  echo "dconfdb_$i=$d"\n'
        )
    vendor_read = ""
    if vendor:
        vendor_read = (
            "# Every GSettings vendor default the plan declares: the value the\n"
            "# session sees, and the compiled schema default that carries it.\n"
            f"mapfile -t vendor_keys < {VENDOR_KEYS_PATH}\n"
            'for i in "${!vendor_keys[@]}"; do\n'
            "  IFS=$'\\t' read -r s k <<< \"${vendor_keys[$i]}\"\n"
            f'  v=$(gsettings get "$s" "$k" 2>/dev/null) || v="{ERROR}"\n'
            '  if [ -z "$v" ]; then v="' + UNSET + '"; fi\n'
            '  echo "vendor_$i=$v"\n'
            f'  d=$(GSETTINGS_BACKEND=memory gsettings get "$s" "$k" 2>/dev/null) || d="{ERROR}"\n'
            '  if [ -z "$d" ]; then d="' + UNSET + '"; fi\n'
            '  echo "vendordb_$i=$d"\n'
            "done\n"
        )
    return (
        "# Every dconf default the plan declares, read back from the session.\n"
        f"mapfile -t dconf_keys < {DCONF_KEYS_PATH}\n"
        'for i in "${!dconf_keys[@]}"; do\n'
        '  k="${dconf_keys[$i]}"\n'
        '  case "$k" in\n'
        '    /org/gnome/login-screen/*)\n'
        '      # The greeter reads its own database, not the session\'s.\n'
        '      g=$(DCONF_PROFILE=gdm dconf read "$k" 2>/dev/null) || g="' + ERROR + '"\n'
        '      if [ -z "$g" ]; then g="' + UNSET + '"; fi\n'
        '      echo "greeter_$i=$g"\n'
        '      continue\n'
        '      ;;\n'
        '  esac\n'
        f'  v=$(dconf read "$k" 2>/dev/null) || v="{ERROR}"\n'
        '  if [ -z "$v" ]; then v="' + UNSET + '"; fi\n'
        '  echo "dconf_$i=$v"\n' + image_read + "done\n" + vendor_read
    )


JOIN_HEAD = """#!/bin/bash
set -u
join_log=/var/lib/deskos-qa/join.log
: > "$join_log"
ok=no

# One machine, one realm. Re-joining a disk that already belongs to a domain
# conflicts with the realm, sssd, keytab and authselect state it carries.
if realm list 2>/dev/null | grep -q .; then
  echo "already joined: $(realm list 2>/dev/null | head -1)" >&2
  exit 3
fi

# The link, bounded: NetworkManager-wait-online is what can hang, and the join
# must not inherit its lack of a deadline.
if command -v nm-online >/dev/null 2>&1; then
  timeout 90 nm-online -q -t 80 >>"$join_log" 2>&1 || true
fi

# Point the guest at the realm's DNS through NetworkManager, the way a joined
# workstation does. In this fixture the server is published on the host loopback
# and QEMU user networking exposes that as 10.0.2.2.
if [ -n "${JOIN_DNS:-}" ]; then
  con=$(nmcli -t -f NAME,DEVICE con show --active | awk -F: '$2 != "" && $2 != "lo" {print $1; exit}')
  if [ -n "$con" ]; then
    nmcli con mod "$con" ipv4.dns "$JOIN_DNS" ipv4.ignore-auto-dns yes >>"$join_log" 2>&1
    timeout 60 nmcli con up "$con" >>"$join_log" 2>&1 || true
  else
    echo "no active connection to carry the realm DNS" >>"$join_log"
  fi
fi

# The client itself is its own file, so neither branch competes with the driver
# for the 4095 B a tmpfiles argument can carry.
if timeout 480 /bin/bash /var/lib/deskos-qa/join-client.sh >>"$join_log" 2>&1; then
  ok=yes
fi
"""

JOIN_TAIL = """
# A failed join must explain itself even when no session ever starts: the unit's
# output goes to the serial console.
if [ "$ok" != yes ]; then
  echo "---- join failed; the last lines of $join_log"
  tail -40 "$join_log"
fi

# sssd can still be activating when the client installer returns.
for _ in $(seq 1 30); do
  [ "$(systemctl is-active sssd 2>/dev/null || true)" = active ] && break
  sleep 2
done
export JOIN_OK="$ok"

# The status is its own file: with the DNS answers and the sssd log it does not
# fit one tmpfiles argument together with the join itself.
/bin/bash /var/lib/deskos-qa/join-status.sh
"""

JOIN_STATUS = """#!/bin/bash
join_log=/var/lib/deskos-qa/join.log
{
  echo "join_ok=${JOIN_OK:-no}"
  echo "join_kind=$JOIN_KIND"
  echo "join_sssd=$(systemctl is-active sssd 2>/dev/null || true)"
  echo "join_user=$(getent passwd "$JOIN_USER" | cut -d: -f1,3,6,7 | paste -sd'|' -)"
  echo "join_sssctl=$(sssctl user-checks "$JOIN_USER" 2>&1 | tr '\\n' '|' | cut -c1-400)"
  echo "join_log=$(tr '\\n' '|' < "$join_log" 2>/dev/null | cut -c1-800)"
  echo "join_resolv=$(grep -E '^nameserver|^search' /etc/resolv.conf 2>/dev/null | paste -sd, -)"
  echo "join_dc_probe=$(for p in 88 389 445; do timeout 3 bash -c "echo > /dev/tcp/$REALM_SERVER/$p" 2>/dev/null && echo "$p=ok" || echo "$p=fail"; done | paste -sd, -)"
  echo "join_sssd_conf=$(sed -n '1,40p' /etc/sssd/sssd.conf 2>/dev/null | tr '\\n' '|' | cut -c1-500)"
  echo "join_domain_status=$(sssctl domain-status "$REALM_DOMAIN" 2>&1 | tr '\\n' '|' | cut -c1-400)"
  echo "join_sssd_log=$(journalctl -u sssd -b --no-pager 2>/dev/null | tail -40 | tr '\\n' '|' | cut -c1-900)"
  echo "join_keytab=$(klist -k 2>&1 | head -6 | tr '\\n' '|' | cut -c1-400)"
  echo "join_sssd_files=$(for f in /var/log/sssd/sssd.log /var/log/sssd/sssd_ad.lab.test.log; do echo "== $f"; tail -12 "$f" 2>/dev/null; done | tr '\\n' '|' | cut -c1-800)"
  join_home=$(getent passwd "$JOIN_USER" | cut -d: -f6)
  echo "join_home=$join_home"
  if [ -n "$join_home" ] && [ -d "$join_home" ]; then
    echo "join_home_before=present"
  else
    echo "join_home_before=missing"
  fi
} > /var/lib/deskos-qa/join.status
chmod 0644 /var/lib/deskos-qa/join.status
"""

JOIN_AD = """#!/bin/bash
set -u
# A real AD join: discovery through DNS, realmd/adcli, sssd-ad. The POSIX
# attributes live in the directory, so the Unix identity comes from the realm.
#
# realmd refuses a machine whose name is not a resolvable FQDN.
ad_host="${JOIN_HOSTNAME:-deskos-qa.$REALM_DOMAIN}"
hostnamectl set-hostname "$ad_host"
if ! grep -qE "[[:space:]]$ad_host([[:space:]]|$)" /etc/hosts; then
  printf '127.0.1.1 %s %s\n' "$ad_host" "${ad_host%%.*}" >> /etc/hosts
fi
realm discover "$REALM_DOMAIN"
# Record what DNS answered. The realm's DNS is the source of truth for the DC's
# address; overriding it with the published host breaks the join.
echo "dc address: $(getent hosts "$REALM_SERVER" | awk '{print $1}' | paste -sd, -)"
echo "srv: $(dig +short SRV "_ldap._tcp.$REALM_DOMAIN" | paste -sd, -)"
echo "a: $(dig +short A "$REALM_SERVER" | paste -sd, -)"
# The DC answers DNS and SRV while it can still refuse authentication, and a join
# that fails there asks for the password a second time with stdin already drained:
# the unit waits on a prompt instead of failing (issue #312). Wait for an
# authenticated exchange first, then join once, with a deadline on both.
dc_ready=no
for _ in $(seq 1 20); do
  if printf '%s\\n' "$JOIN_PASSWORD" | timeout 10 kinit "$JOIN_PRINCIPAL@$REALM_NAME" > /tmp/kinit.log 2>&1; then
    dc_ready=yes
    kdestroy 2>/dev/null || true
    break
  fi
  sleep 3
done
if [ "$dc_ready" != yes ]; then
  echo "the DC did not accept authentication for $JOIN_PRINCIPAL: $(tr '\\n' ' ' < /tmp/kinit.log | cut -c1-300)"
  exit 1
fi
if ! printf '%s\\n' "$JOIN_PASSWORD" | timeout 120 realm join --user="$JOIN_PRINCIPAL" "$REALM_DOMAIN"; then
  echo "realm join failed against $REALM_SERVER; the unit does not wait on a prompt"
  exit 1
fi
# realmd leaves sssd with access_provider = ad and no rule, which denies every
# account: the join succeeds, the user resolves, and pam_acct_mgmt says no.
# `realm permit --all` is the documented way, and here it exits zero without
# writing a rule, so the rule is set below instead. A real organization narrows
# this to groups; the fixture admits the whole realm.
realm permit --all || echo "realm permit exited $?"
# realmd configures sssd but not the home policy nor oddjob.
authselect select sssd with-mkhomedir --force || true
systemctl enable --now oddjobd || true

# sssd starts at boot, before this unit points the guest at the realm's DNS, so
# it can come up already marked offline. Restart it with debug on, pin the
# server (its own SRV discovery comes back empty here) and wait for the domain.
awk -v want="[domain/$REALM_DOMAIN]" -v l1="debug_level = 9" -v l2="ad_server = $REALM_SERVER" \
  '$0 == "access_provider = ad" { print "access_provider = permit"; next }
   { print }
   $0 == want { print l1; print l2 }' /etc/sssd/sssd.conf > /etc/sssd/sssd.conf.new &&
  { chown root:root /etc/sssd/sssd.conf.new
    chmod 0600 /etc/sssd/sssd.conf.new
    mv /etc/sssd/sssd.conf.new /etc/sssd/sssd.conf; } || true
systemctl restart sssd || true
for _ in $(seq 1 25); do
  sssctl domain-status "$REALM_DOMAIN" 2>/dev/null | grep -q 'Online status: Online' && break
  sleep 2
done
exit 0
"""

JOIN_IPA = """#!/bin/bash
set -u
# FreeIPA without integrated DNS: the guest maps the server name to the host and
# --force skips the checks that expect a resolvable name. --hostname gives the
# guest a name inside the realm instead of the image's.
if ! grep -qE "[[:space:]]${REALM_SERVER//./\\.}([[:space:]]|$)" /etc/hosts; then
  printf '10.0.2.2 %s\n' "$REALM_SERVER" >> /etc/hosts
fi
join_host="${JOIN_HOSTNAME:-deskos-qa.$REALM_DOMAIN}"
ipa-client-install --unattended --force --hostname="$join_host" \
    --domain="$REALM_DOMAIN" --realm="$REALM_NAME" \
    --server="$REALM_SERVER" --principal="$JOIN_PRINCIPAL" --password="$JOIN_PASSWORD" \
    --mkhomedir --no-ntp --force-join || exit 1
exit 0
"""


def join_script():
    """The join driver: guard, realm DNS, the client, and the report."""
    return JOIN_HEAD + JOIN_TAIL


def join_client_script(kind="ipa"):
    """The client for one realm.

    Only the branch the guest will run is delivered: together they exceed the
    4095 B a single tmpfiles argument can carry, and the harness refuses to
    truncate a credential in silence.
    """
    return JOIN_AD if kind == "ad" else JOIN_IPA


def join_environment(join):
    lines = [
        ("JOIN_KIND", join.get("kind", "ipa")),
        ("REALM_DOMAIN", join["domain"]),
        ("REALM_NAME", join["realm"]),
        ("REALM_SERVER", join["server"]),
        ("JOIN_PRINCIPAL", join["principal"]),
        ("JOIN_PASSWORD", join["password"]),
        ("JOIN_USER", join["user"]),
    ]
    if join.get("dns"):
        lines.append(("JOIN_DNS", join["dns"]))
    return "".join(f"{key}={value}\n" for key, value in lines)


def lock_reads():
    """The script fragment that tries to write every locked key.

    Writing the value the key already has must fail when dconf enforces the lock;
    an unlocked key accepts the write and reports so. The value is read again
    after the attempt, so a lock that lets the write through is caught even if the
    command exits zero for another reason.
    """
    return (
        "# Every dconf key the plan locks: writing its current value must fail.\n"
        f"mapfile -t lock_keys < {DCONF_LOCKS_PATH}\n"
        'for i in "${!lock_keys[@]}"; do\n'
        '  k="${lock_keys[$i]}"\n'
        '  before=$(dconf read "$k" 2>/dev/null) || before="' + ERROR + '"\n'
        '  if [ -z "$before" ] || [ "$before" = "' + ERROR + '" ]; then\n'
        '    echo "lock_$i=unknown"\n'
        '  elif dconf write "$k" "$before" 2>/dev/null; then\n'
        '    echo "lock_$i=writable"\n'
        '  else\n'
        '    echo "lock_$i=locked"\n'
        '  fi\n'
        '  after=$(dconf read "$k" 2>/dev/null) || after="' + ERROR + '"\n'
        '  echo "lock_before_$i=$before"\n'
        '  echo "lock_after_$i=$after"\n'
        "done\n"
    )


def browser_fixture():
    """Serve one page from the host for the guest's browser check.

    QEMU's user networking exposes the host as 10.0.2.2, so the guest fetches the
    page over the same network path a real desktop uses, without a host port
    mapping. Returns the server, already serving on a background thread, and the
    URL the guest should open. The caller stops it.
    """
    body = ("<!doctype html><html><head><title>DeskOS QA</title></head>"
            f"<body><h1>{BROWSER_MARKER}</h1></body></html>").encode()

    hits = []

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            hits.append(self.headers.get("User-Agent", ""))
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    server = http.server.ThreadingHTTPServer(("", 0), Handler)
    server.daemon_threads = True
    server.hits = hits
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server, f"http://{BROWSER_HOST}:{server.server_address[1]}/"


# tmpfiles.d caps one argument at 4095 B, so the session's probes are split
# across files that the autostart script calls in order.
def session_browser_probe(url=""):
    """The browser probe, delivered as its own file.

    It fetches the page the harness serves on the host side of QEMU's user
    network: about:blank only proves the binary starts (docs/checks.md).
    """
    if not url:
        return "#!/bin/bash\n# this run has no harness page to fetch\n"
    return (
        "#!/bin/bash\n"
        f'url={shlex.quote(url)}\n'
        "page=/var/tmp/deskos-qa/browser.html\n"
        # curl proves the network path and the bytes, without the browser's own
        # profile and sandbox in the way.
        'curl -fsS --max-time 60 "$url" -o "$page" 2>/dev/null '
        '&& echo "browser_fetch=ok" || echo "browser_fetch=failed"\n'
        f'grep -q "{BROWSER_MARKER}" "$page" 2>/dev/null && echo "browser_content=ok" '
        '|| echo "browser_content=missing"\n'
        # Firefox proves the renderer; the fixture records that the request came
        # from a browser, so a screenshot cannot pass on its own.
        'timeout 120 firefox --headless --screenshot /var/tmp/deskos-qa/browser.png '
        '"$url" >/dev/null 2>&1\n'
        'test -s /var/tmp/deskos-qa/browser.png && echo "browser_screenshot=yes" '
        '|| echo "browser_screenshot=no"\n'
    )


def session_probe():
    """The helper script for probes that would push the main script past 4095 B.

    It resolves every declared favorite and the command each launches, locks
    the screen, waits for the Flatpak preinstall and runs the shipped runtime
    tool, then calls the browser probe from its own file.
    """
    return r"""#!/bin/bash
# Every declared favorite must name an installed desktop entry, and the command
# that entry launches must exist: a correct id whose Exec names a binary this
# image does not ship still opens nothing.
i=0
for id in $(gsettings get org.gnome.shell favorite-apps | tr -d "[]' " | tr ',' '\n'); do
  case "$id" in *.desktop) ;; *) continue ;; esac
  found=missing; entry=""
  for d in /usr/share/applications /usr/local/share/applications \
           /var/lib/flatpak/exports/share/applications \
           "$HOME/.local/share/applications" \
           "$HOME/.local/share/flatpak/exports/share/applications"; do
    if [ -f "$d/$id" ]; then found=present; entry="$d/$id"; break; fi
  done
  echo "favorite_entry_$i=$id:$found"
  if [ -n "$entry" ]; then
    cmd=$(sed -n 's/^Exec=//p' "$entry" | head -n1 | sed 's/%[a-zA-Z]//g' | awk '{print $1}')
    # A Flatpak entry runs `flatpak run <app>`; whether that app is installed is
    # the Flatpak check's claim, not this one.
    state=missing
    if [ "$cmd" = flatpak ]; then state=flatpak
    elif [ -n "$cmd" ] && { [ -x "$cmd" ] || command -v "$cmd" >/dev/null 2>&1; }; then state=present
    fi
    echo "favorite_exec_$i=$id:$state"
  fi
  i=$((i+1))
done
# Lock before the slow probes; the VM is disposable and the report unit reads
# the results.
gdbus call --session --dest org.gnome.ScreenSaver \
  --object-path /org/gnome/ScreenSaver \
  --method org.gnome.ScreenSaver.Lock >/dev/null 2>&1
sleep 3
echo "screen_locked=$(gdbus call --session --dest org.gnome.ScreenSaver \
  --object-path /org/gnome/ScreenSaver \
  --method org.gnome.ScreenSaver.GetActive 2>/dev/null | tr -d '(), ' | tr 'A-Z' 'a-z')"
# Wait for the preinstall unit to settle: it retries every minute, and a unit
# still retrying is a read that did not happen, not a missing app.
for i in $(seq 1 90); do
  case "$(systemctl is-active deskos-flatpak-preinstall.service 2>/dev/null)" in
    active|activating) sleep 2 ;;
    *) break ;;
  esac
done
echo "flatpak_apps=$(flatpak list --system --app --columns=application 2>/dev/null | paste -sd, -)"
# The runtime tool the field checklist uses must read the plan this image
# carries and name this workstation; a shipped binary that cannot is a defect
# the hash check cannot see.
deskos report > /var/tmp/deskos-qa/deskos-report.txt 2>&1
echo "deskos_report_exit=$?"
echo "deskos_report_workstation=$(sed -n 's/^Workstation: //p' /var/tmp/deskos-qa/deskos-report.txt | head -n1 | cut -d' ' -f1)"
# `deskos check` exits 1 when it finds a difference, which is an answer, not a
# crash; exit 2 would mean the tool could not be used.
deskos check > /var/tmp/deskos-qa/deskos-check.txt 2>&1
echo "deskos_check_exit=$?"
# The login logo is a system dconf default: its file must exist, or GDM draws a
# bare screen (the compiler copies only the effective logo).
# A login-screen key belongs to the greeter's database, not the session's: read
# it through the profile GDM uses, which is what decides whether the logo is
# shown (issue #246). The session read is kept for the record.
echo "login_logo=$(dconf read /org/gnome/login-screen/logo 2>/dev/null | tr -d "'" | sed 's|^file://||')"
login_logo_greeter=$(DCONF_PROFILE=gdm dconf read /org/gnome/login-screen/logo 2>/dev/null | tr -d "'" | sed 's|^file://||')
echo "login_logo_greeter=$login_logo_greeter"
if [ -n "$login_logo_greeter" ]; then
  test -f "$login_logo_greeter" && echo "login_logo_file=present" || echo "login_logo_file=missing"
fi
# Report only, not an assertion: an /etc merge can rewrite fstab at boot after
# the generator ran, and the field bug is what the numbers explain.
echo "fstab_mtime=$(stat -c %Y /etc/fstab 2>/dev/null || echo missing)"
echo "fstab_verify=$(findmnt --verify 2>&1 | tr '\n' '|' | cut -c1-400)"
# The offline check needs to prove the boot really had no network.
echo "default_routes=$(ip route show default 2>/dev/null | wc -l)"
# Its own file: the browser block plus this script would exceed the cap.
""" + QA_BROWSER_PATH + """
"""


def session_reads(dconf_keys=(), dconf_db=None, vendor_keys=(), enabled_extensions=(), binaries=()):
    """The reads that live in their own file, sourced by the session script.

    The main script is one tmpfiles argument and systemd refuses anything over 4095
    bytes; these two fragments are the largest, so they travel separately and each
    file has its own budget.
    """
    text = ""
    if enabled_extensions:
        text += extension_reads()
    if binaries:
        text += binary_reads()
    if dconf_keys or vendor_keys:
        text += default_reads(dconf_db, bool(vendor_keys))
    return "\n".join(
        l for l in text.splitlines() if not l.lstrip().startswith("#")
    ) + "\n"


def session_script(seed_skel, app, skel_rels, dconf_keys=(), dconf_db=None, vendor_keys=(), enabled_extensions=(),
                   binaries=(), dconf_locks=(), join=None, font_families=(), network_profiles=(), reads=False):
    """The script the desktop autostart runs inside the GNOME session.

    It records the session state. With seed_skel it copies /etc/skel into the
    test user's HOME (the test user comes from sysusers, which does not copy
    skel, so without this the image's per-user seeds never apply). With app it
    opens a window before the report, so a captured frame shows the themed
    widgets. skel_rels are the seed paths (relative to /etc/skel) whose content
    the run must confirm reached the user. dconf_keys are the plan's dconf
    defaults to read back, dconf_db names the image's own dconf database, and
    vendor_keys are the plan's GSettings vendor defaults to read back.
    """
    seed = ""
    if seed_skel:
        seed = (
            "# The test user comes from sysusers, which does not copy /etc/skel.\n"
            "# Copy it as useradd -m would, so the session reflects a real account\n"
            "# and the image's per-user seeds apply.\n"
            'cp -a /etc/skel/. "$HOME"/ 2>/dev/null || true\n'
        )
    launch = ""
    if app:
        launch = (
            "# Open an application so a captured frame shows the themed widgets.\n"
            f"setsid {app} >/dev/null 2>&1 &\n"
            "sleep 5\n"
        )
    skel = ""
    if skel_rels:
        quoted = " ".join(shlex.quote(r) for r in skel_rels)
        skel = (
            f"skel_rels=({quoted})\n"
            'for i in "${!skel_rels[@]}"; do\n'
            '  p="$HOME/${skel_rels[$i]}"\n'
            '  if [ -f "$p" ]; then echo "skelseed_$i=present:$(sha256sum "$p" | cut -d" " -f1)"; '
            'else echo "skelseed_$i=missing"; fi\n'
            "done\n"
        )
    appcheck = ""
    if app:
        name = os.path.basename(app.split()[0])
        appcheck = f'echo "app_running=$(pgrep -x {shlex.quote(name)} | wc -l)"\n'
    reads = default_reads(dconf_db, bool(vendor_keys)) if (dconf_keys or vendor_keys) else ""
    locks = f". {LOCK_SCRIPT_PATH}\n" if dconf_locks else ""
    join_status = ""
    if join:
        join_status = (
            f'if [ -r {JOIN_STATUS_PATH} ]; then\n'
            f'  cat {JOIN_STATUS_PATH}\n'
            'fi\n'
        )
    reads_source = f". {QA_READS_PATH}\n" if reads else ""
    tray = f". {TRAY_SCRIPT_PATH}\n" if "appindicatorsupport@rgcjonas.gmail.com" in enabled_extensions else ""
    font_reads_fragment = font_reads() if font_families else ""
    network_reads_fragment = network_reads() if network_profiles else ""
    script = f"""#!/bin/bash
# Runs inside the autologin GNOME session; records what the session sees.
{seed}{launch}sleep 20
out={RESULTS}
{{
echo "session_type=$XDG_SESSION_TYPE"
echo "session_user=$(id -un)"
echo "session_uid=$(id -u)"
echo "session_home=$(getent passwd "$(id -un)" | cut -d: -f6)"
echo "home_owner=$(stat -c %u:%g "$HOME" 2>/dev/null || echo missing)"
echo "oddjobd=$(systemctl is-active oddjobd 2>/dev/null || true)"
{join_status}echo "gnome_shell=$(gnome-shell --version)"
echo "disable_user_extensions=$(gsettings get org.gnome.shell disable-user-extensions)"
echo "enabled_extensions=$(gsettings get org.gnome.shell enabled-extensions)"
echo "favorite_apps=$(gsettings get org.gnome.shell favorite-apps)"
echo "picture_uri=$(gsettings get org.gnome.desktop.background picture-uri)"
{reads_source}{tray}{font_reads_fragment}{network_reads_fragment}echo "user_failed_units=$(systemctl --user --failed --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
{reads}{locks}{skel}{appcheck}python3 {E2E_PROBE_PATH} 2>&1 || echo "e2e_error=exit:$?"
echo "firefox_present=$(command -v firefox >/dev/null 2>&1 && echo yes || echo no)"
timeout 120 firefox --headless --screenshot /var/tmp/deskos-qa/firefox.png about:blank >/dev/null 2>&1
echo "firefox_headless_exit=$?"
test -s /var/tmp/deskos-qa/firefox.png && echo "firefox_screenshot=yes" || echo "firefox_screenshot=no"
"{QA_PROBE_PATH}"
}} > "$out.tmp"
mv "$out.tmp" "$out"
"""
    # The script travels as one tmpfiles argument, and systemd refuses anything over
    # 4095 bytes; the network read pushed it over on a real run (job 123867637679).
    # The comments belong to this file, where a maintainer reads them, and the guest
    # runs the code, so they are dropped before it travels.
    return "\n".join(
        l for l in script.splitlines() if l.startswith("#!") or not l.lstrip().startswith("#")
    ) + "\n"

def e2e_probe():
    """The first E2E step: what the session exposes, and an application it opens.

    It is the same interface a screen reader uses, so it proves the accessibility
    stack is live before any interaction is scripted on top of it, and then it
    launches one declared favorite through its desktop entry -- the path a click
    takes -- and waits for a window to appear in the tree. It waits for states
    instead of assuming them; if the binding, the registry or the window is
    missing, it says which, and the summary reports a read that did not happen
    rather than a pass.

    Keyboard injection (Super, typing the name) is not here: Mutter on Wayland
    does not accept synthetic keys, and uinput needs a root helper. That is a
    separate step, and this one is worth having on its own.
    """
    return r'''#!/usr/bin/python3
# The desktop's own accessibility tree, through the binding the image carries
# (python3-pyatspi 2.46 in the published image; nothing older is assumed).
import os
import subprocess
import time

try:
    import pyatspi
except Exception as exc:  # an image without the desktop, or without the binding
    print(f"e2e_error=import:{exc}")
    raise SystemExit(0)

# A declared favorite that needs no network and starts in one window; PROCESS is
# the name its Exec launches, used only to clean up.
TARGET = "org.gnome.Calculator.desktop"
# pkill -x takes the 15-character comm, not the 21-character command name.
PROCESS = "gnome-calculator"
WINDOW_ROLES = (pyatspi.ROLE_FRAME, pyatspi.ROLE_WINDOW, pyatspi.ROLE_DIALOG)


def apps():
    """The applications the registry knows, keyed by name."""
    try:
        desktop = pyatspi.Registry.getDesktop(0)
    except Exception as exc:
        return None, f"registry:{exc}"
    found = {}
    for app in desktop:
        try:
            found[app.name] = app
        except Exception:
            continue
    return found, ""


def windows(app):
    """The toplevels an application exposes; a session daemon exposes none."""
    out = []
    try:
        for child in app:
            try:
                if child.getRole() in WINDOW_ROLES:
                    out.append(child.name or "?")
            except Exception:
                continue
    except Exception:
        pass
    return out


def entry_path(app_id):
    roots = ("/usr/share/applications", "/usr/local/share/applications",
             "/var/lib/flatpak/exports/share/applications",
             os.path.expanduser("~/.local/share/applications"))
    for root in roots:
        path = os.path.join(root, app_id)
        if os.path.exists(path):
            return path
    return ""


# 1. The stack is live before anything is scripted on it.
deadline = time.time() + 20
baseline, error = {}, ""
while time.time() < deadline:
    baseline, error = apps()
    if baseline:
        break
    time.sleep(1)
names = sorted(baseline or {})
if not names and error:
    print(f"e2e_error={error}")
print(f"e2e_apps={len(names)}")
print("e2e_app_names=" + ",".join(names)[:900])

# 2. One application, opened the way its icon opens it, and seen in the tree.
path = entry_path(TARGET)
print(f"e2e_launch_entry={path or 'missing'}")
if not path:
    print("e2e_launch_ok=no")
    print("e2e_launch_reason=no-desktop-entry")
    raise SystemExit(0)

proc = subprocess.Popen(["gio", "launch", path], start_new_session=True,
                        stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
deadline = time.time() + 30
opened, frame_names = "", []
while time.time() < deadline:
    now, _ = apps()
    for name, app in (now or {}).items():
        if name in baseline:
            continue
        frames = windows(app)
        if frames:
            opened, frame_names = name, frames
            break
    if opened:
        break
    time.sleep(1)

if opened:
    print(f"e2e_launch_ok=yes")
    print(f"e2e_launch_app={opened}")
    print("e2e_launch_frames=" + ",".join(frame_names)[:200])
else:
    print("e2e_launch_ok=no")
    print(f"e2e_launch_exit={proc.poll()}")

# 3. Leave the desktop as it was found: the close *through the tree* is the fourth
#    scenario, not this one, so the application is terminated, not clicked shut.
subprocess.run(["pkill", "-f", "/usr/bin/" + PROCESS], stdout=subprocess.DEVNULL,
               stderr=subprocess.DEVNULL)
deadline = time.time() + 10
gone = False
while time.time() < deadline:
    now, _ = apps()
    if now is not None and opened and opened not in now:
        gone = True
        break
    time.sleep(1)
print("e2e_launch_closed=" + ("yes" if gone else "no"))
'''


def gdm_conf(user):
    return f"[daemon]\nAutomaticLoginEnable=True\nAutomaticLogin={user}\n"

AUTOSTART = f"""[Desktop Entry]
Type=Application
Name=DeskOS QA session check
Exec=/bin/bash {QA_SCRIPT_PATH}
NoDisplay=true
X-GNOME-Autostart-enabled=true
"""

WATCHDOG_SCRIPT = f"""#!/bin/bash
# A boot that never reaches the session leaves the serial silent and the run
# stops on a timeout with nothing to read. If no results appeared by the time
# the timer fires, report what the machine has and power it off.
if [ -s {RESULTS} ]; then
  exit 0
fi
stty -F /dev/ttyS0 115200 2>/dev/null
{{
echo {MARK_BEGIN}
echo "session_results=missing"
echo "watchdog=yes"
cat /var/lib/deskos-qa/join.status 2>/dev/null || echo "join_status=missing"
echo "system_failed_units=$(systemctl --failed --no-legend --plain | awk '{{print $1}}' | paste -sd, -)"
echo "gdm=$(systemctl show gdm.service -p ActiveState -p SubState -p Result --value 2>/dev/null | paste -sd/ -)"
echo "join_unit=$(systemctl show deskos-qa-join.service -p ActiveState -p SubState -p Result -p ExecMainStatus --value 2>/dev/null | paste -sd/ -)"
echo "network_online=$(systemctl show network-online.target -p ActiveState -p SubState --value 2>/dev/null | paste -sd/ -)"
echo "target=$(systemctl is-active graphical.target 2>/dev/null || true)"
echo {MARK_END}
}} > /dev/ttyS0 2>&1
systemctl --no-block poweroff
"""


REPORT_SESSION = f"""#!/bin/bash
# Waits for the session results, then writes them and the journal to ttyS0.
for i in $(seq 1 240); do test -s {RESULTS} && break; sleep 2; done
stty -F /dev/ttyS0 115200 2>/dev/null
{{
echo {MARK_BEGIN}
test -s {RESULTS} && cat {RESULTS} || echo "session_results=missing"
if [ ! -s {RESULTS} ]; then
  # No session reported: GDM may be sitting at the greeter because the login
  # could not happen. Say what the join did, so the boot is not silent.
  echo "--- join status (no session reported)"
  cat {JOIN_STATUS_PATH} 2>/dev/null || echo "join_status=missing"
  tail -40 /var/lib/deskos-qa/join.log 2>/dev/null
fi
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


# systemd unescapes a tmpfiles.d argument and then expands specifiers into a
# PATH_MAX-1 buffer, so the cap is on the *resolved* content, counted in bytes,
# not on the bytes written in the file: escaping that only lengthens the text
# (%% or \n) does not count, but a multi-byte character does. Measured against
# systemd-tmpfiles: 4095 B is created and 4096 B is refused with "File name too
# long" and the file is not created (2047 "é" pass, 2048 do not). Anything larger
# is delivered as a credential of its own instead.
TMPFILES_ARG_MAX = 4095


def tmpfiles_arg(text):
    """Escape file content for a tmpfiles.d argument field.

    Refuses content the guest would drop in silence rather than waiting for a
    report that never comes.
    """
    n = len(text.encode("utf-8"))
    if n > TMPFILES_ARG_MAX:
        raise ValueError(
            f"tmpfiles argument is {n} B, over the {TMPFILES_ARG_MAX} B "
            f"systemd limit; deliver it as its own credential instead")
    return text.replace("\\", "\\\\").replace("%", "%%").replace("\n", "\\n")


def credentials(mode, seed_skel=False, app=None, skel_rels=(), dconf_keys=(), dconf_db=None, vendor_keys=(),
                enabled_extensions=(), binaries=(), browser_url="", locks=(), join=None, gdm_user=None,
                font_families=(), network_profiles=()):
    report = REPORT_SESSION if mode == "session" else REPORT_JOURNAL
    target = "graphical.target" if mode == "session" else "multi-user.target"
    tmpfiles = [
        "d /var/lib/deskos-qa 0755 root root -",
        f"f /var/lib/deskos-qa/report.sh 0755 root root - {tmpfiles_arg(report)}",
    ]
    creds = {}
    if mode == "session":
        reads = session_reads(dconf_keys, dconf_db, vendor_keys, enabled_extensions, binaries)
        if reads:
            tmpfiles.append(f"f {QA_READS_PATH} 0644 root root - {tmpfiles_arg(reads)}")
        script = session_script(seed_skel, app, skel_rels, dconf_keys, dconf_db, vendor_keys, enabled_extensions,
                                binaries, locks, join, font_families, network_profiles, reads=bool(reads))
        creds["sysusers.extra"] = f'u {USER} {UID} "DeskOS QA" {HOME} /bin/bash\n'
        tmpfiles += [
            "d /var/tmp/deskos-qa 1777 root root -",
            f"d {HOME} 0700 {USER} {USER} -",
            f"d {HOME}/.config 0755 {USER} {USER} -",
            f"f {HOME}/.config/gnome-initial-setup-done 0644 {USER} {USER} - yes",
            # The probes are outside the home and start for any account: with a
            # directory user nothing can be placed in its home before first login,
            # because the home does not exist until the login creates it.
            "d /etc/xdg/autostart 0755 root root -",
            f"f {AUTOSTART_PATH} 0644 root root - {tmpfiles_arg(AUTOSTART)}",
            f"f {QA_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(script)}",
            f"f {QA_PROBE_PATH} 0755 root root - {tmpfiles_arg(session_probe())}",
            f"f {E2E_PROBE_PATH} 0755 root root - {tmpfiles_arg(e2e_probe())}",
            f"f {QA_BROWSER_PATH} 0755 root root - {tmpfiles_arg(session_browser_probe(browser_url))}",
            # GDM's configuration path is the distribution's: /etc/gdm on the EL
            # family and Fedora, /etc/gdm3 on Debian and Ubuntu. The harness writes
            # the autologin to both and creates each directory, so the check does
            # not hardcode one distribution's layout; the guest's GDM reads its own.
            "d /etc/gdm 0755 root root -",
            f"f+ /etc/gdm/custom.conf 0644 root root - {tmpfiles_arg(gdm_conf(gdm_user or USER))}",
            "d /etc/gdm3 0755 root root -",
            f"f+ /etc/gdm3/custom.conf 0644 root root - {tmpfiles_arg(gdm_conf(gdm_user or USER))}",
        ]
        if gdm_user:
            # A directory user's home is created at its first login. The marker
            # that stops the initial-setup wizard has to come with the skeleton,
            # or the wizard covers the session this check is about.
            tmpfiles += [
                "d /etc/skel/.config 0755 root root -",
                "f /etc/skel/.config/gnome-initial-setup-done 0644 root root - yes",
            ]
        if dconf_keys:
            # The plan's defaults, one key per line, so the session script reads the
            # plan's list rather than carrying it. The file is world-readable in a
            # 0755 directory; the session user is not root and must read it.
            tmpfiles.append(
                f"f {DCONF_KEYS_PATH} 0644 root root - {tmpfiles_arg(chr(10).join(dconf_keys) + chr(10))}")
        if locks:
            # The keys the plan locks, one per line; the session tries to write
            # each one and must fail.
            tmpfiles.append(
                f"f {DCONF_LOCKS_PATH} 0644 root root - {tmpfiles_arg(chr(10).join(locks) + chr(10))}")
            tmpfiles.append(
                f"f {LOCK_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(lock_reads())}")
        if binaries:
            # The destinations of the binaries the plan installs, one per line.
            tmpfiles.append(
                f"f {BINARY_PATHS_PATH} 0644 root root - "
                f"{tmpfiles_arg(chr(10).join(binaries) + chr(10))}")
        if enabled_extensions:
            # The extensions the plan enabled, one uuid per line.
            tmpfiles.append(
                f"f {ENABLED_EXTENSIONS_PATH} 0644 root root - "
                f"{tmpfiles_arg(chr(10).join(enabled_extensions) + chr(10))}")
            if "appindicatorsupport@rgcjonas.gmail.com" in enabled_extensions:
                tmpfiles.append(
                    f"f {TRAY_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(tray_probe())}")
        if vendor_keys:
            # The plan's GSettings vendor defaults, schema and key per line.
            tmpfiles.append(
                f"f {VENDOR_KEYS_PATH} 0644 root root - "
                f"{tmpfiles_arg(chr(10).join(chr(9).join(v) for v in vendor_keys) + chr(10))}")
        if dconf_db:
            # A second profile naming the image's database alone. `dconf read` with
            # it reports the value the image ships, apart from the value the session
            # ends up with; this is a new file, never the guest's own profile.
            tmpfiles += [
                "d /etc/dconf/profile 0755 root root -",
                f"f {DCONF_PROFILE_PATH} 0644 root root - {tmpfiles_arg(f'system-db:{dconf_db}' + chr(10))}",
            ]
        if join:
            # The join must finish before GDM: the directory user and sssd have to
            # exist when the greeter starts. The admin password is root-only and
            # injected at runtime; it is never part of the image.
            # /bin/bash runs the script instead of exec'ing it: a file under /var
            # is var_lib_t, and the unit runs as init_t, which may read it but not
            # execute it (an exec is an AVC denial and 203/EXEC).
            tmpfiles += [
                f"f {JOIN_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(join_script())}",
                f"f {JOIN_CLIENT_SCRIPT_PATH} 0755 root root - "
                f"{tmpfiles_arg(join_client_script(join.get('kind', 'ipa')))}",
                f"f {JOIN_STATUS_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(JOIN_STATUS)}",
                f"f {JOIN_ENV_PATH} 0600 root root - {tmpfiles_arg(join_environment(join))}",
            ]
            creds["systemd.extra-unit.deskos-qa-join.service"] = (
                "[Unit]\nDescription=Join the disposable FreeIPA realm (DeskOS QA)\n"
                # Ordering against network-online.target is a trap: if that
                # target is never reached, the join never starts, gdm waits for
                # the join, and the machine sits at the splash with no timeout to
                # trigger. Wait for the link inside the script instead, bounded.
                "Wants=network.target\nAfter=network.target\nBefore=gdm.service\n"
                f"ConditionPathExists={JOIN_ENV_PATH}\n\n"
                # The join is Before=gdm.service: if it hangs, the greeter never
                # starts and nothing reports. Bound it, so a hang becomes a
                # failure with a log instead of a silent splash.
                "[Service]\nType=oneshot\nRemainAfterExit=yes\nTimeoutStartSec=420\n"
                f"EnvironmentFile={JOIN_ENV_PATH}\nExecStart=/bin/bash {JOIN_SCRIPT_PATH}\n"
                "StandardOutput=journal+console\nStandardError=journal+console\n"
            )
    creds["systemd.extra-unit.deskos-qa-report.service"] = (
        "[Unit]\nDescription=DeskOS QA report to the serial port\n"
        f"ConditionPathExists=!/etc/initrd-release\nAfter={target}\n\n"
        # The unit's own output goes to the console as well as the journal. The harness
        # reads the serial port, so a report script that does not run -- a syntax error, a
        # file that is not there -- says so on the channel being watched, instead of
        # leaving a marker that never arrives and a log that explains nothing. A bash
        # error and a truncated credential are otherwise indistinguishable from here.
        "[Service]\nType=oneshot\nStandardOutput=journal+console\nStandardError=journal+console\n"
        "ExecStart=/bin/bash /var/lib/deskos-qa/report.sh\n"
    )
    # The watchdog only fires when the report never ran: the report powers the
    # guest off, so a live guest at 11 minutes means a boot that stalled.
    creds["systemd.extra-unit.deskos-qa-watchdog.service"] = (
        "[Unit]\nDescription=DeskOS QA watchdog for a boot that never reported\n"
        "ConditionPathExists=!/etc/initrd-release\n\n"
        "[Service]\nType=oneshot\nStandardOutput=journal+console\nStandardError=journal+console\n"
        f"ExecStart=/bin/bash {WATCHDOG_SCRIPT_PATH}\n"
    )
    creds["systemd.extra-unit.deskos-qa-watchdog.timer"] = (
        "[Unit]\nDescription=DeskOS QA watchdog timer\n\n"
        "[Timer]\nOnBootSec=11min\nUnit=deskos-qa-watchdog.service\n\n"
        "[Install]\nWantedBy=timers.target\n"
    )
    creds["systemd.unit-dropin.timers.target"] = (
        "[Unit]\nWants=deskos-qa-watchdog.timer\n"
    )
    tmpfiles.append(
        f"f {WATCHDOG_SCRIPT_PATH} 0755 root root - {tmpfiles_arg(WATCHDOG_SCRIPT)}")

    wants = "deskos-qa-report.service"
    if join:
        wants += " deskos-qa-join.service"
    creds[f"systemd.unit-dropin.{target}"] = f"[Unit]\nWants={wants}\n"
    creds["tmpfiles.extra"] = "\n".join(tmpfiles) + "\n"
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


def plan_defaults(plan):
    """The dconf defaults the plan declares, in the order the plan lists them."""
    return list((plan.get("artifact", {}).get("dconf") or {}).get("defaults") or [])


def plan_locks(plan):
    """The dconf keys the plan locks, in the order the plan lists them."""
    return list((plan.get("artifact", {}).get("dconf") or {}).get("locks") or [])


def dconf_value(plan, key):
    for d in plan_defaults(plan):
        if d["key"] == key:
            return d["value"]
    return None


FONT_SIZE = re.compile(r"^(?P<family>.+?)\s+(?P<size>[0-9]+(?:\.[0-9]+)?)$")


def plan_font_families(plan):
    """The families the plan's font settings name, and the setting that names them.

    A font setting lowers to a Pango description, `"<family> <size>"`, so the
    family is what remains when the trailing size is removed. The claim this
    supports is narrow on purpose: the image carries a family by that name. It says
    nothing about substitution — a document that asks for Georgia gets its metric
    substitute, and that is fontconfig's decision, not a declaration here.
    """
    out = []
    for d in plan_defaults(plan):
        setting = d.get("setting") or ""
        if not setting.startswith("appearance.fonts.") or d.get("type") != "s":
            continue
        value = (d.get("value") or "").strip().strip("'")
        m = FONT_SIZE.match(value)
        if m:
            out.append({"setting": setting, "family": m.group("family").strip()})
    return out


NETWORK_KEYFILE_DIR = "/etc/NetworkManager/system-connections/"


def plan_network_profiles(plan):
    """The NetworkManager connections the plan generates, by name.

    A keyfile in the plan is a profile the image declares; this reads the names so
    the session can ask NetworkManager whether it accepted them. It says nothing
    about whether the network authenticates, which needs a RADIUS server.
    """
    out = []
    for f in plan.get("artifact", {}).get("generatedFiles") or []:
        p = f.get("path") or ""
        if p.startswith(NETWORK_KEYFILE_DIR) and p.endswith(".nmconnection"):
            out.append(p[len(NETWORK_KEYFILE_DIR):-len(".nmconnection")])
    return out


def plan_binaries(plan):
    """The files the plan's artifacts place, in the order the plan lists them."""
    return list(plan.get("artifact", {}).get("binaries") or [])


def binary_summary(checks):
    """The plan's binaries and how the run found them.

    A binary the plan installs is a declaration like any other, so it gets its
    assertion. What that assertion can be depends on what the plan pins: for a
    single-file artifact the plan's checksum *is* the installed file's, while for an
    archive it is the archive's, and the file inside it cannot be compared with it
    here (the build verified that download; the archive is gone by the time a session
    runs). Both are verified as far as they can be, and the report says which is
    which instead of pretending they are the same claim.
    """
    bins = [c for c in checks if c["check"].startswith(BINARY_CHECK_PREFIX)]
    strict = sum(1 for c in bins if c["coverage"]["image"] == "checked-bytes" and c["pass"])
    install = sum(1 for c in bins if c["coverage"]["image"] == "install-only" and c["pass"])
    # A read that did not happen counts whether or not the check could claim something
    # else: an archive whose member could not be hashed is installed and verified as
    # installed, and the read still has to appear here (the detail says which).
    unreadable = sum(1 for c in bins if c.get("unread"))
    return {"declared": len(bins), "checksum_verified": strict, "install_verified": install,
            "unreadable": unreadable,
            "failed": [{"check": c["check"], "detail": c["detail"]} for c in bins if not c["pass"]]}


def plan_vendor_defaults(plan):
    """The GSettings vendor defaults the plan declares, in the plan's order."""
    return list(plan.get("artifact", {}).get("gsettingsVendorDefaults") or [])


def canonical(text, typ=None):
    """GVariant text with the whitespace that carries no meaning removed.

    Only whitespace outside a quoted string is dropped, so `['a','b']`,
    `['a', 'b']` and `[ 'a' , 'b' ]` are the same value while `'a b'` keeps its
    space. `dconf read` prints a value the way the plan stores it, so this is a
    guard against formatting, not a different reading of the value. A leading type
    annotation is dropped only when it is the declared type's own: `@as []` is the
    `[]` an `as` key stores, while `@ai []` is a different value and must not match.
    """
    out, quote, i = [], False, 0
    if text.startswith("@"):
        annotation, _, rest = text.partition(" ")
        if annotation == "@" + (typ or ""):
            text = rest
    while i < len(text):
        c = text[i]
        if quote:
            out.append(c)
            if c == "\\" and i + 1 < len(text):
                out.append(text[i + 1])
                i += 2
                continue
            if c == "'":
                quote = False
        elif c == "'":
            quote = True
            out.append(c)
        elif c.isspace():
            nxt = text[i + 1] if i + 1 < len(text) else ""
            if out and out[-1] not in "[({," and nxt not in "])}, ":
                out.append(" ")
        else:
            out.append(c)
        i += 1
    return "".join(out)


def array_elements(text, typ=None):
    """The elements of a GVariant text array, or None when the text is not one."""
    t = canonical(text, typ).strip()
    if not (t.startswith("[") and t.endswith("]")):
        return None
    body, out, cur, depth, quote, i = t[1:-1], [], "", 0, False, 0
    while i < len(body):
        c = body[i]
        if quote:
            cur += c
            if c == "\\" and i + 1 < len(body):
                cur += body[i + 1]
                i += 2
                continue
            if c == "'":
                quote = False
        elif c == "'":
            quote = True
            cur += c
        elif c in "[({":
            depth += 1
            cur += c
        elif c in "])}":
            depth -= 1
            cur += c
        elif c == "," and depth == 0:
            out.append(cur.strip())
            cur = ""
        else:
            cur += c
        i += 1
    if cur.strip():
        out.append(cur.strip())
    return out


def compare_default(key, typ, want, got):
    """How a value read from the guest compares with the plan: "exact", "order" or None.

    An array is compared as a multiset only for the keys GNOME itself treats as a
    set (UNORDERED_DEFAULTS), where the Shell rewrites the whole list and the plan
    does not fix an order. Every other value, arrays included, is compared in
    order: `favorite-apps` and the app folders are written in a declared order, and
    accepting a reorder would report a reordered list as if it were the plan's.
    """
    if canonical(want, typ) == canonical(got, typ):
        return "exact"
    if key in UNORDERED_DEFAULTS:
        w, g = array_elements(want, typ), array_elements(got, typ)
        if w is not None and g is not None and sorted(w) == sorted(g):
            return "order"
    return None


def read_state(want, got, key, typ, verb):
    """How one read compared with the plan, as (state, phrase).

    `state` is "verified" (the read matches the plan), "different" (it read a value
    that is not the declared one), "unread" (there is no value to compare, with the
    reason) or "missing" (the guest reported nothing for the key at all). A read the
    guest could not make is "unread", never "different": an unreadable value is a
    coverage gap to report, not a contradiction to assert. A value that never
    arrived is "missing": the guest and the harness disagree about what to read.
    """
    if got is None:
        return "missing", f"{verb} nothing (the guest did not report it)"
    if got == ERROR:
        return "unread", f"{verb} nothing (the read failed)"
    if got == UNSET:
        return "unread", f"{verb} no value for this key"
    match = compare_default(key, typ, want, got)
    if match == "exact":
        return "verified", f"{verb} {got}, as the plan declares"
    if match == "order":
        return "verified", f"{verb} {got}, the plan's elements in a different order"
    return "different", f"{verb} {got}, want {want}"


def default_verdict(ident, image, session, where):
    """Pass, detail and coverage for one declared default, from its two reads.

    `ident` is the difference_id of the delivery path and key, and `where` names
    the image-side source of the default (the image's own dconf database, or the
    compiled schema default the override lands in).

    The image is the strict claim: a declared default the image does not carry is a
    failure, whether or not the session happens to see the same value, because the
    value could come from anywhere else. A session value that differs also fails:
    the run is a fresh account, so nothing legitimate has set it yet, and a declared
    default that does not reach the user is the defect this check exists for. A
    session read the guest could not make is reported with its reason and counted as
    unverified, not failed: the image carries the default and the session's own dconf
    profile is what decides whether it arrives, which is a platform question the
    report has to show rather than assert.
    """
    detail = f"{where} {image[1]}; the session {session[1]}"
    if image[0] != "verified":
        return False, f"{detail}; the image is not shown to carry the declared default", False
    if session[0] in ("different", "missing"):
        why = EXPLAINED_SESSION_DIFFERENCES.get(ident)
        if why:
            return True, f"{detail}; {why}", True
        return False, detail, False
    if session[0] == "unread":
        return True, f"{detail}; whether the default reaches the user is unverified", True
    return True, detail, False


def default_coverage(checks):
    """The plan's declared defaults and how far each was verified.

    Every declared default is one check carrying two independent reads, so the
    report says `declared`, how many were verified in the image, how many in the
    session, and lists every read that was not verified with its state and reason.
    A default is counted as verified on an axis only when the value read there
    matched the plan: "tried to assert" is not the same as "verified".
    """
    # Only the declared defaults: a binary carries a coverage of its own kind, and
    # counting it here would report it as a default with an axis it does not have.
    declared = [c for c in checks
                if c.get("coverage", {}).get("source") in ("dconf", "gsettings override")]
    unverified = [{"check": c["check"], "axis": axis, "state": c["coverage"][axis], "detail": c["detail"]}
                  for c in declared for axis in ("image", "session")
                  if c["coverage"][axis] != "verified"]
    return {"declared": len(declared),
            "in_image": sum(1 for c in declared if c["coverage"]["image"] == "verified"),
            "in_session": sum(1 for c in declared if c["coverage"]["session"] == "verified"),
            "unverified": unverified}


def greeter_coverage(checks):
    """The login-screen defaults and whether the greeter's database carries them.

    These keys live in the database GDM reads, so there is one read and one claim;
    a missing read is reported as unread, not counted as verified.
    """
    declared = [c for c in checks if c.get("coverage", {}).get("source") == "dconf greeter"]
    return {"declared": len(declared),
            "verified": sum(1 for c in declared if c["coverage"]["greeter"] == "verified"),
            "unverified": [{"check": c["check"], "state": c["coverage"]["greeter"], "detail": c["detail"]}
                           for c in declared if c["coverage"]["greeter"] != "verified"]}


def check_status(c):
    if c.get("skipped"):
        return "SKIP"
    return "PASS" if c["pass"] else "FAIL"


def evaluate(values, plan, journal="", skel_rels=(), offline=False, join_user=None, gdm_user=None):
    checks = []

    def check(name, ok, detail, skipped=False, coverage=None, unread=None):
        c = {"check": name, "pass": bool(ok), "skipped": bool(skipped), "detail": detail}
        if coverage is not None:
            c["coverage"] = coverage
        # A read that did not happen, and what it was: a check can be verified on one
        # axis and unmeasured on another, and the summary has to say so.
        if unread:
            c["unread"] = list(unread)
        checks.append(c)

    check("session results reported", values.get("session_results") != "missing" and "session_type" in values,
          f"session_type={values.get('session_type')}")
    # Accessibility: the session must expose its applications through AT-SPI, the
    # interface a screen reader uses. This is the ground the interactions are built
    # on; it proves the stack is live and that the driver can see the desktop. An
    # image without the binding reports which read did not happen, not a pass.
    apps = values.get("e2e_apps", "")
    check("the session exposes its applications through AT-SPI",
          values.get("e2e_error") in (None, "") and apps.isdigit() and int(apps) >= 1,
          f"e2e_apps={apps or 'unread'} e2e_error={values.get('e2e_error') or 'none'} "
          f"e2e_app_names={values.get('e2e_app_names') or 'unread'}")
    # Scenario 1: the entry launches, and the window is in the tree. A session that
    # starts an application the accessibility tree never sees is stuck for anyone
    # who uses the tree -- a screen reader, or the next scenario.
    check("an application opens from its desktop entry",
          values.get("e2e_launch_ok") == "yes",
          f"e2e_launch_ok={values.get('e2e_launch_ok') or 'unread'} "
          f"e2e_launch_app={values.get('e2e_launch_app') or 'unread'} "
          f"e2e_launch_frames={values.get('e2e_launch_frames') or 'unread'} "
          f"e2e_launch_reason={values.get('e2e_launch_reason') or 'none'} "
          f"e2e_launch_exit={values.get('e2e_launch_exit') or 'none'}")
    if join_user:
        check("domain join completed", values.get("join_ok") == "yes",
              f"join_ok={values.get('join_ok')}")
        check("SSSD is active after the join", values.get("join_sssd") == "active",
              f"join_sssd={values.get('join_sssd')}")
        check("directory user resolves", join_user in (values.get("join_user") or ""),
              f"join_user={values.get('join_user')}")
    if gdm_user:
        # The session itself must belong to the directory, not to a local
        # stand-in: this is what PAM, the greeter and the home policy meet.
        # Compare against the name the guest resolved: with AD, sssd may report
        # the short name while the autologin was configured with the qualified
        # one, and both are the same account.
        resolved = values.get("join_user") or ""
        expected = resolved.split(":")[0] or gdm_user
        check("the session is the directory user", values.get("session_user") == expected,
              f"session_user={values.get('session_user')} expected={expected}")
        # The uid the guest resolved for that user, not one the harness assumes.
        uid = resolved.split(":")[1] if ":" in resolved else None
        owner = (values.get("home_owner") or "").split(":")[0]
        check("the login created the home for that user",
              values.get("join_home_before") == "missing" and uid is not None and owner == uid,
              f"join_home_before={values.get('join_home_before')} "
              f"home_owner={values.get('home_owner')} uid={uid}")
        check("oddjobd is active", values.get("oddjobd") == "active",
              f"oddjobd={values.get('oddjobd')}")
    if offline:
        # The boot has no NIC, so a session here is a login that needed no
        # network. The route count proves the harness really removed it: a VM
        # that quietly kept user networking would make the claim untested.
        check("the offline boot had no default route", values.get("default_routes") == "0",
              f"default_routes={values.get('default_routes')}")
        check("GDM autologin reaches a session without network",
              values.get("session_type") in ("wayland", "x11"),
              f"session_type={values.get('session_type')}")
    check("user extensions allowed", values.get("disable_user_extensions") == "false",
          f"disable-user-extensions={values.get('disable_user_extensions')}")
    # Every file the plan's artifacts place must be there, with the mode the plan
    # declares, and -- when the plan pins the file itself -- the same bytes. For an
    # archive the plan pins the archive's checksum, so the file inside it is verified as
    # installed and the limit is stated rather than compared, which would report a
    # difference that is not one. Not every such file runs: a font is data.
    for i, b in enumerate(plan_binaries(plan)):
        name = BINARY_CHECK_PREFIX + b["destination"]
        got = values.get(f"binary_{i}", "missing")
        archived = b.get("archive", "none") != "none"
        # The plan declares the mode, so it is asserted: a file with the right bytes and
        # the wrong mode is not the file the plan installs. `stat` prints 755 where the
        # plan writes 0755, so the leading zero is not a difference. A mode that could
        # not be read is not a mode that differs: it is a read this machine would not
        # give up, and it is reported as one instead of as a difference nobody measured.
        declared_mode = b.get("mode") or ""
        read_mode = values.get(f"binary_mode_{i}", "missing")
        mode_read = read_mode not in ("missing", "unreadable")
        mode_differs = mode_read and bool(declared_mode) and read_mode.lstrip("0") != declared_mode.lstrip("0")
        mode_text = ""
        if declared_mode:
            if mode_differs:
                mode_text = f"mode {read_mode}, the plan declares {declared_mode}"
            elif mode_read:
                mode_text = f"mode {read_mode}, as the plan declares"
            else:
                mode_text = f"its mode could not be read here; the plan declares {declared_mode}"
        unread = []
        if got == "unreadable":
            unread.append("its bytes")
        if declared_mode and not mode_read:
            unread.append("its mode")
        # An archive is claimed as installed and a single file by its bytes; a file
        # whose bytes could not be read is claimed by neither, and is counted as a read
        # that did not happen rather than as one that disagreed.
        axis = "install-only" if archived else "checked-bytes"
        coverage = {"source": "binary", "key": b["destination"], "image": axis, "session": axis}
        if got == "missing":
            check(name, False, f"{b['destination']} is not a regular file",
                  coverage=coverage)
        elif mode_differs:
            check(name, False, f"{b['destination']} is installed with {mode_text}",
                  coverage=coverage)
        elif archived:
            read = f" (sha256 {got[:12]}…)" if got != "unreadable" else " (its bytes could not be read here)"
            check(name, True,
                  f"installed{read}, {mode_text}; the plan pins the {b['archive']} archive's checksum, which the build verified and this cannot compare",
                  skipped=bool(unread), unread=unread, coverage=coverage)
        elif got == "unreadable":
            check(name, True,
                  f"{b['destination']} is installed, {mode_text}; whether its bytes are the plan's is unreadable here",
                  skipped=True, unread=unread,
                  coverage=dict(coverage, image="unread", session="unread"))
        elif got != b["sha256"]:
            check(name, False, f"sha256 {got}, the plan pins {b['sha256']}",
                  coverage=coverage)
        else:
            check(name, True, f"sha256 {got}, {mode_text}",
                  skipped=bool(unread), unread=unread, coverage=coverage)
    # A font setting names a family, and the image has to carry one by that name:
    # composition cannot know what a package installs, so this is the read that
    # turns a typo or a missing family into a failure instead of a silent fallback.
    # It claims nothing about substitution.
    families = plan_font_families(plan)
    if families:
        raw = values.get("font_families")
        carried = {f.strip().lower() for f in (raw or "").split("|") if f.strip()}
        for f in families:
            name = f"font family {f['family']} is installed"
            if raw is None:
                check(name, True, "the families the image carries could not be read here",
                      skipped=True, unread=["the family list"])
            elif f["family"].lower() in carried:
                check(name, True,
                      f"{f['family']} is one of the {len(carried)} families fontconfig carries; "
                      f"declared by {f['setting']}")
            else:
                check(name, False,
                      f"{f['family']} is not among the {len(carried)} families fontconfig carries; "
                      f"declared by {f['setting']}")
    # A network profile the image declares has to be one NetworkManager accepted:
    # the file can be right and the daemon still refuse it. It says nothing about
    # authenticating, which needs a RADIUS server and is not part of CI.
    for name in plan_network_profiles(plan):
        raw = values.get("nm_profiles")
        listed = {p.strip() for p in (raw or "").split("|") if p.strip()}
        check_name = f"network profile {name} is loaded by NetworkManager"
        if raw is None:
            check(check_name, True, "the connections NetworkManager lists could not be read here",
                  skipped=True, unread=["the connection list"])
        elif name in listed:
            check(check_name, True,
                  f"{name} is one of the {len(listed)} connections NetworkManager lists")
        else:
            check(check_name, False,
                  f"{name} is not among the {len(listed)} connections NetworkManager lists")
    # Every extension the composition turned on must really load. Membership in
    # enabled-extensions is the declared default; this is the session fact.
    for i, uuid in enumerate(plan.get("artifact", {}).get("enabledExtensions") or []):
        got = values.get(f"extension_{i}", "missing")
        check(f"extension loads ({uuid})", got.split("/")[0] == "Yes" and got.split("/")[-1] in ("ACTIVE", "ENABLED"),
              f"Enabled/State={got}")
    if "appindicatorsupport@rgcjonas.gmail.com" in (plan.get("artifact", {}).get("enabledExtensions") or []):
        # Behaviour, not only load: the extension owns the StatusNotifierWatcher and
        # answers it (docs/adr/0021). An icon in the panel is still a pixel fact.
        check("tray watcher serves the session bus", values.get("tray_watcher") == "present",
              f"tray_watcher={values.get('tray_watcher')}")
        # It accepts a registration; the caller is short-lived, so the property list
        # is not evidence this harness can hold (ADR 0021).
        check("tray watcher accepts a registration", values.get("tray_register") == "accepted",
              f"tray_register={values.get('tray_register')}")
    fav = dconf_value(plan, "/org/gnome/shell/favorite-apps")
    if fav:
        check("favorites as planned", values.get("favorite_apps") == fav, f"got {values.get('favorite_apps')} want {fav}")
    # Only the ids the plan declares. The vendor's default favorite list can
    # name applications Core does not install (GNOME Software, for one); that is
    # not a DeskOS intent and this probe does not claim it.
    declared_favorites = set(re.findall(r"'([^']+\.desktop)'", fav or ""))
    for key, value in values.items():
        if not key.startswith("favorite_entry_"):
            continue
        app, _, found = value.rpartition(":")
        if app not in declared_favorites:
            continue
        check(f"favorite entry {app}", found == "present",
              f"{app} is {'installed' if found == 'present' else 'not installed'}")
    # The entry's Exec must resolve: a correct id that launches a binary this
    # image does not ship opens nothing. A Flatpak entry delegates that to the
    # Flatpak application, which the declared-Flatpak check owns.
    for key, value in values.items():
        if not key.startswith("favorite_exec_"):
            continue
        app, _, found = value.rpartition(":")
        if app not in declared_favorites:
            continue
        if found == "flatpak":
            check(f"favorite command {app}", True,
                  f"{app} starts through flatpak; whether that application is "
                  "installed is the Flatpak check's claim", skipped=True,
                  unread=["the Flatpak application"])
        else:
            check(f"favorite command {app}", found == "present",
                  f"{app} launches {'a command this image ships' if found == 'present' else 'a command this image does not ship'}")
    wp = dconf_value(plan, "/org/gnome/desktop/background/picture-uri")
    if wp:
        check("wallpaper as planned", values.get("picture_uri") == wp, f"got {values.get('picture_uri')} want {wp}")
    logo = dconf_value(plan, "/org/gnome/login-screen/logo")
    if logo:
        want = logo.strip("'").removeprefix("file://")
        got = values.get("login_logo_greeter") or ""
        state = "verified" if got == want else (f"unexpected: {got}" if got else "unread")
        check("login logo file exists", values.get("login_logo_file") == "present",
              f"greeter={got or 'unset'} file={values.get('login_logo_file') or 'not read'}")
        check("the greeter reads the login logo", got == want,
              f"greeter={got or 'unset'} plan={want}",
              coverage={"source": "dconf", "key": "/org/gnome/login-screen/logo",
                        "image": state, "session": state})
    check("screen locks on request", values.get("screen_locked") == "true",
          f"screen_locked={values.get('screen_locked')}")
    # The runtime companion ships in the image; the field checklist runs it. It
    # must load the plan the image carries and name the workstation it built.
    if plan.get("workstation", {}).get("name"):
        check("deskos report names the installed workstation",
              values.get("deskos_report_workstation") == plan["workstation"]["name"],
              f"got {values.get('deskos_report_workstation') or 'no report'} "
              f"(exit={values.get('deskos_report_exit')})")
        check("deskos check runs on the booted image",
              values.get("deskos_check_exit") in ("0", "1"),
              f"exit={values.get('deskos_check_exit')}")
    # A first boot with no network cannot install what the preinstall unit
    # downloads; the offline check claims the login, not the store.
    declared_flatpaks = set() if offline else {a["id"] for a in
                                              plan.get("provisioning", {}).get("flatpakApplications") or []}
    if declared_flatpaks:
        installed = {a for a in (values.get("flatpak_apps") or "").split(",") if a}
        missing = sorted(declared_flatpaks - installed)
        # The unit retries until it succeeds; still retrying is not a missing app,
        # it is a download this run did not wait out.
        still_running = any(s in (values.get("flatpak_preinstall") or "")
                            for s in ("active", "activating"))
        check("declared system Flatpaks are installed", not missing or still_running,
              f"missing={','.join(missing) or 'none'}; preinstall={values.get('flatpak_preinstall')}",
              skipped=bool(missing) and still_running,
              unread=["the Flatpak list"] if missing and still_running else None)
    # The plan has no rpmPackages key, so presence is asked of the guest: an
    # image that ships Firefox gets the checks, one that does not says so, and a
    # guest that answers neither fails instead of skipping in silence.
    firefox = values.get("firefox_present")
    check("the guest says whether Firefox is installed", firefox in ("yes", "no"),
          f"firefox_present={firefox}")
    if firefox == "yes":
        check("Firefox runs headless", values.get("firefox_headless_exit") == "0" and values.get("firefox_screenshot") == "yes",
              f"exit={values.get('firefox_headless_exit')} screenshot={values.get('firefox_screenshot')}")
        # The harness page needs the network this boot deliberately lacks. The
        # fixture records the browser's own request, so a leftover screenshot
        # cannot pass for a fetch.
        if not offline:
            check("Firefox fetches the harness page",
                  values.get("browser_content") == "ok"
                  and values.get("browser_firefox_requests", "0") not in ("", "0")
                  and values.get("browser_screenshot") == "yes",
                  f"fetch={values.get('browser_fetch')} content={values.get('browser_content')} "
                  f"requests={values.get('browser_firefox_requests')} "
                  f"screenshot={values.get('browser_screenshot')}")
    # Every default the plan declares is read back from the session, and each one
    # is two claims: what the image carries and what the session sees. The list is
    # the plan's, so a default declared later is asserted without editing this
    # harness, and the coverage says which defaults were verified on which axis and
    # which were not, with the reason.
    db = (plan.get("artifact", {}).get("dconf") or {}).get("name") or "image"
    where = f"the image's {db} database"
    for i, d in enumerate(plan_defaults(plan)):
        key, typ = d["key"], d.get("type")
        if key.startswith("/org/gnome/login-screen/"):
            # The greeter has its own database and its own check; asserting it
            # against the session's would fail on an image that is correct.
            continue
        image = read_state(d["value"], values.get(f"dconfdb_{i}"), key, typ, "carries")
        session = read_state(d["value"], values.get(f"dconf_{i}"), key, typ, "sees")
        ok, detail, skipped = default_verdict(difference_id("dconf", key), image, session, where)
        check(DEFAULT_CHECK_PREFIX + key, ok, detail, skipped=skipped,
              coverage={"source": "dconf", "key": key, "image": image[0], "session": session[0]})
    # A login-screen default has one read and one claim: the greeter's own
    # database carries the value. There is no second axis, because the session's
    # profile never sees the key and asking it would report a correct image as
    # unverified.
    for i, d in enumerate(plan_defaults(plan)):
        key, typ = d["key"], d.get("type")
        if not key.startswith("/org/gnome/login-screen/"):
            continue
        state, phrase = read_state(d["value"], values.get(f"greeter_{i}"), key, typ, "the greeter reads")
        check(GREETER_CHECK_PREFIX + key, state == "verified", phrase,
              coverage={"source": "dconf greeter", "key": key, "greeter": state})
    # A lock is enforcement, not a rendered file: the session tries to write the
    # key and must fail, and the value must be the one it had before the attempt.
    for i, lock in enumerate(plan_locks(plan)):
        state = values.get(f"lock_{i}", "missing")
        before = values.get(f"lock_before_{i}")
        after = values.get(f"lock_after_{i}")
        check("plan lock " + lock["key"] + " cannot be changed",
              state == "locked" and before == after and before not in (None, "", ERROR),
              f"write={state} before={before} after={after}")
    for i, d in enumerate(plan_vendor_defaults(plan)):
        key, typ = d["key"], d.get("type")
        ident = difference_id("gsettings", d["schema"] + " " + key)
        image = read_state(d["value"], values.get(f"vendordb_{i}"), key, typ, "carries")
        session = read_state(d["value"], values.get(f"vendor_{i}"), key, typ, "sees")
        ok, detail, skipped = default_verdict(ident, image, session, "the compiled schema default")
        check(VENDOR_CHECK_PREFIX + d["schema"] + " " + key, ok, detail, skipped=skipped,
              coverage={"source": "gsettings override", "key": d["schema"] + " " + key,
                        "image": image[0], "session": session[0]})
    failed = failed_units(values)
    known = known_failures(values, journal)
    unexpected = None if failed is None else [u for u in failed if u not in known]
    detail = f"failed={values.get('system_failed_units')}"
    if known:
        detail += f"; known non-blocking: {','.join(known)} (see Platform diagnostics)"
    check("no unexpected failed system units", unexpected == [], detail)
    check("no failed user units", values.get("user_failed_units", "?") == "", f"failed={values.get('user_failed_units')}")
    # With --seed-skel, the run proves the image's per-user seeds reached the
    # user: the file exists in $HOME and its content is the generated one.
    generated = plan.get("artifact", {}).get("generatedFiles") or []
    for i, rel in enumerate(skel_rels):
        want = next((f for f in generated if f.get("path") == "/etc/skel/" + rel), None)
        got = values.get(f"skelseed_{i}", "missing")
        expect = "present:" + hashlib.sha256(want["content"].encode()).hexdigest() if want else "?"
        check(f"theme seed reaches the user ({rel})", got == expect, f"got {got} want {expect}")
    if "app_running" in values:
        check("the sample app is running", values.get("app_running", "0") not in ("", "0"),
              f"app_running={values.get('app_running')}")
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
    p.add_argument("--offline", action="store_true",
                   help="boot with no network and assert the session still logs in")
    p.add_argument("--join-domain", help="FreeIPA domain for the join unit")
    p.add_argument("--join-realm", help="FreeIPA realm for the join unit")
    p.add_argument("--join-server", help="FreeIPA server host name for the join unit")
    p.add_argument("--join-principal", help="principal that joins (for example admin)")
    p.add_argument("--join-password", help="password of the join principal; runtime only")
    p.add_argument("--join-user", help="directory user the join must make resolvable")
    p.add_argument("--join-gdm", action="store_true",
                   help="log in at GDM as the directory user and check that session")
    p.add_argument("--join-kind", choices=("ipa", "ad"), default="ipa",
                   help="which client to run: ipa-client-install or realm/adcli")
    p.add_argument("--join-dns",
                   help="DNS server the guest must use before the join (the AD case)")
    p.add_argument("--seed-skel", action="store_true",
                   help="copy /etc/skel into the test user's HOME, as useradd -m would, and check the generated seeds")
    p.add_argument("--app", help="open this command in the session so a captured frame shows the themed widgets")
    p.add_argument("--qemu")
    p.add_argument("--ovmf-code")
    p.add_argument("--ovmf-vars")
    a = p.parse_args()
    if a.mode == "session" and not a.plan:
        p.error("--plan is required for --mode session")
    join_values = [a.join_domain, a.join_realm, a.join_server, a.join_principal, a.join_password, a.join_user]
    join = None
    if any(join_values):
        if not all(join_values):
            p.error("--join-domain, --join-realm, --join-server, --join-principal, --join-password and --join-user go together")
        for value in join_values:
            if "\n" in value or "\r" in value:
                p.error("a --join value contains a newline")
        join = {
            "kind": a.join_kind,
            "domain": a.join_domain,
            "realm": a.join_realm,
            "server": a.join_server,
            "principal": a.join_principal,
            "password": a.join_password,
            "user": a.join_user,
            "dns": a.join_dns,
        }
    if a.join_gdm and not join:
        p.error("--join-gdm needs the --join-* values")
    gdm_user = join["user"] if (join and a.join_gdm) else None
    plan = json.load(open(a.plan)) if a.plan else {}
    skel_rels = []
    if a.seed_skel:
        for f in plan.get("artifact", {}).get("generatedFiles") or []:
            path = f.get("path", "")
            if path.startswith("/etc/skel/"):
                skel_rels.append(path[len("/etc/skel/"):])
    # The plan's declared defaults drive the assertion list: their keys go to the
    # guest, and the same plan is compared with what the guest reads back.
    defaults = plan_defaults(plan)
    locks = plan_locks(plan)
    vendor = plan_vendor_defaults(plan)
    dconf_db = (plan.get("artifact", {}).get("dconf") or {}).get("name")

    # The browser check needs a page served from the host for as long as the
    # guest is up; journal mode adds no session, so it serves nothing.
    server, browser_url = (None, "")
    if a.mode == "session" and not a.offline:
        server, browser_url = browser_fixture()

    run_args = argparse.Namespace(
        disk=a.disk, out=a.out, qemu=a.qemu, ovmf_code=a.ovmf_code, ovmf_vars=a.ovmf_vars,
        memory=a.memory, cpus=a.cpus, boot_timeout=a.boot_timeout, shutdown_timeout=a.shutdown_timeout,
        interval=a.interval, stable_frames=5, expect_splash=False, hash_disk=True, keep_overlay=False,
        extra_qemu=smbios_args(credentials(a.mode, a.seed_skel, a.app, skel_rels,
                                           [d["key"] for d in defaults], dconf_db,
                                           [[d["schema"], d["key"]] for d in vendor],
                                           plan.get("artifact", {}).get("enabledExtensions") or [],
                                           [b["destination"] for b in plan_binaries(plan)],
                                           browser_url, locks=[l["key"] for l in locks], join=join,
                                           gdm_user=gdm_user,
                                           font_families=[f["family"] for f in plan_font_families(plan)],
                                           network_profiles=plan_network_profiles(plan))),
        until_serial=MARK_END, guest_powers_off=True, network=not a.offline,
    )
    try:
        summary = bootcheck.run(run_args)
    except bootcheck.HarnessError as e:
        print(f"sessioncheck: {e}", file=sys.stderr)
        return 2
    finally:
        if server is not None:
            server.shutdown()
            server.server_close()

    values, journal = parse_serial(os.path.join(a.out, "serial.log"))
    if server is not None:
        # What the fixture saw, not only what the guest said: the browser check
        # needs evidence that a browser made the request.
        values["browser_firefox_requests"] = str(
            sum(1 for ua in server.hits
                if "firefox" in ua.lower() or "gecko" in ua.lower()))
    with open(os.path.join(a.out, "journal.txt"), "w") as f:
        f.write(journal + "\n")
    result = {"mode": a.mode, "instrumented": True, "boot": summary["result"], "boot_failures": summary["failures"],
              "values": values, "plan_workstation": plan.get("workstation", {}).get("name")}
    if a.mode == "session":
        result["checks"] = evaluate(values, plan, journal, skel_rels, offline=a.offline,
                                    join_user=join["user"] if join else None, gdm_user=gdm_user)
        result["plan_defaults"] = default_coverage(result["checks"])
        result["plan_greeter"] = greeter_coverage(result["checks"])
        result["plan_binaries"] = binary_summary(result["checks"])
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
        lines.append(f"- {check_status(c)}: {c['check']} ({c['detail']})")
    if "plan_defaults" in result:
        s = result["plan_defaults"]
        lines.append(f"- Plan defaults: {s['declared']} declared; {s['in_image']} verified in the image, "
                     f"{s['in_session']} verified in the session, {len(s['unverified'])} unverified reads")
        for c in s["unverified"]:
            lines.append(f"  - {c['check']} [{c['axis']}: {c['state']}] ({c['detail']})")
    if result.get("plan_greeter", {}).get("declared"):
        g = result["plan_greeter"]
        lines.append(f"- Greeter defaults: {g['declared']} declared; {g['verified']} verified in the "
                     f"greeter database, {len(g['unverified'])} unverified reads")
        for c in g["unverified"]:
            lines.append(f"  - {c['check']} [{c['state']}] ({c['detail']})")
    if "plan_binaries" in result:
        b = result["plan_binaries"]
        lines.append(f"- Plan binaries: {b['declared']} declared; {b['checksum_verified']} verified against the checksum the plan pins, "
                     f"{b['install_verified']} verified as installed (the plan pins an archive)"
                     + (f", {b['unreadable']} unreadable here" if b.get("unreadable") else ""))
        for c in b["failed"]:
            lines.append(f"  - {c['check']} ({c['detail']})")
    if "platform_diagnostics" in result:
        lines += diagnostics_md(result["platform_diagnostics"])
    lines += ["", "Journal excerpt: journal.txt. Frames: frames/ and summary.md."]
    with open(os.path.join(a.out, "session.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"sessioncheck: {result['result']} ({a.out}/session.md)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
