package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Firmware updates are a higher-risk operation than an image or package
// update: the machine writes a UEFI capsule, reboots into its own updater and
// comes back. DeskOS never stages or flashes one; `deskos update --firmware`
// only asks fwupd what is available. The 1.9-3 failure showed that a broken
// boot-time updater is worth naming before a person runs `fwupdmgr update`.
//
// These checks are warnings, not gates. Listing updates does not need a working
// loader, a writable ESP or free capsule space, and hiding the list would hide
// updates for devices that never use the affected path.
const firmwareESPMount = "/boot/efi"

type firmwareWarning struct {
	summary string
	detail  string
}

// firmwareWarnings reports conditions that would make a later manual firmware
// installation fail. It never stops or skips the listing.
//
// queryPrevious starts fwupd. That is not free of side effects: fwupd is
// D-Bus activatable, and its startup can process a previous result and clean up
// updater files or variables. A dry run therefore only reads the package and
// mount state.
func firmwareWarnings(run func(string, ...string) ([]byte, error), queryPrevious bool) []firmwareWarning {
	var warnings []firmwareWarning
	if w, ok := fwupdEFIWarning(run); ok {
		warnings = append(warnings, w)
	}
	warnings = append(warnings, espWarnings()...)
	if queryPrevious {
		if w, ok := previousFailureWarning(run); ok {
			warnings = append(warnings, w)
		}
	}
	return warnings
}

// fwupdEFIWarning finds the Fedora 1.9-3 build that carries a .pkgnote section
// at VirtualAddress 0. Shim and the firmware reject that executable, so a
// staged capsule is never applied (fwupd-efi#225, Fedora bug 2566576). Fedora
// fixed it in 1.9-4 by removing the build flag; EL10's 1.8 never had it.
//
// The release number alone does not identify a packaging stream, so the warning
// is limited to Fedora's .fc builds. An unreadable or malformed version is
// reported as "could not assess", never as safe.
func fwupdEFIWarning(run func(string, ...string) ([]byte, error)) (firmwareWarning, bool) {
	out, err := run("rpm", "-q", "--qf", "%{VERSION}-%{RELEASE}", "fwupd-efi")
	version := strings.TrimSpace(string(out))
	if err != nil {
		if version == "" || !strings.Contains(version, "not installed") {
			return firmwareWarning{
				summary: "could not read the fwupd-efi version",
				detail:  "whether the boot-time updater is affected is unknown: " + strings.TrimSpace(version+" "+err.Error()),
			}, true
		}
		return firmwareWarning{
			summary: "fwupd-efi is not installed",
			detail:  "the default fwupd UEFI updater is unavailable; a capsule installation may fail unless this machine uses another updater",
		}, true
	}
	parsed, ok := parseRelease(version)
	if !ok {
		return firmwareWarning{
			summary: "could not assess fwupd-efi " + version,
			detail:  "the version does not have the expected version-release form, so the .pkgnote regression cannot be ruled out",
		}, true
	}
	if parsed.version != "1.9" || !parsed.fedora || parsed.release != 3 {
		return firmwareWarning{}, false
	}
	return firmwareWarning{
		summary: "fwupd-efi " + version + " fails to run its boot-time updater",
		detail: "the Fedora build puts a .pkgnote section inside the PE headers; shim and the firmware " +
			"reject the executable, so a staged capsule is not applied (fwupd-efi#225, Fedora bug 2566576). " +
			"Update the image to a build with fwupd-efi 1.9-4 or later before installing firmware",
	}, true
}

// rpmRelease is the part of an RPM version-release string this check reads.
type rpmRelease struct {
	version string
	release int
	fedora  bool
}

// parseRelease accepts one version-release token with a numeric release and an
// optional .fcNN or .elNN distribution suffix. Anything else, including extra
// output on the same line, is unknown rather than silently treated as newer or
// unaffected.
func parseRelease(s string) (rpmRelease, bool) {
	i := strings.LastIndex(s, "-")
	if i <= 0 || i == len(s)-1 {
		return rpmRelease{}, false
	}
	version, release := s[:i], s[i+1:]
	if !digitsAndDots(version) {
		return rpmRelease{}, false
	}
	j := 0
	for j < len(release) && release[j] >= '0' && release[j] <= '9' {
		j++
	}
	if j == 0 {
		return rpmRelease{}, false
	}
	n, err := strconv.Atoi(release[:j])
	if err != nil {
		return rpmRelease{}, false
	}
	suffix := release[j:]
	if suffix == "" {
		return rpmRelease{version: version, release: n}, true
	}
	if !strings.HasPrefix(suffix, ".") {
		return rpmRelease{}, false
	}
	parts := strings.Split(suffix[1:], ".")
	first := parts[0]
	if len(first) < 3 {
		return rpmRelease{}, false
	}
	distro, distVersion := first[:2], first[2:]
	if !digits(distVersion) {
		return rpmRelease{}, false
	}
	fedora := false
	switch distro {
	case "fc":
		fedora = true
	case "el":
	default:
		return rpmRelease{}, false
	}
	for _, part := range parts[1:] {
		if !digits(part) {
			return rpmRelease{}, false
		}
	}
	return rpmRelease{version: version, release: n, fedora: fedora}, true
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func digitsAndDots(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if !digits(part) {
			return false
		}
	}
	return true
}

// espWarnings checks the layout DeskOS uses for its EFI System Partition. fwupd
// can also find an ESP at /boot or /efi, so this is a statement about the
// DeskOS layout, not a universal fwupd requirement. It never mounts or writes.
func espWarnings() []firmwareWarning {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return []firmwareWarning{{
			summary: "could not read /proc/self/mountinfo",
			detail:  "the EFI System Partition cannot be assessed: " + err.Error(),
		}}
	}
	return espWarningsFrom(string(data))
}

func espWarningsFrom(mountinfo string) []firmwareWarning {
	found := false
	readWrite := false
	fstype := ""
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[4] != firmwareESPMount {
			continue
		}
		found = true
		for _, opt := range strings.Split(fields[5], ",") {
			if opt == "rw" {
				readWrite = true
			}
		}
		sep := -1
		for i, field := range fields {
			if field == "-" {
				sep = i
				break
			}
		}
		if sep >= 0 && sep+1 < len(fields) {
			fstype = fields[sep+1]
		}
		break
	}
	if !found {
		return []firmwareWarning{{
			summary: firmwareESPMount + " is not a mountpoint",
			detail: "DeskOS mounts its EFI System Partition there. fwupd writes capsules to the ESP it selects, " +
				"which may be /boot or /efi; check where fwupd looks before installing firmware",
		}}
	}
	var warnings []firmwareWarning
	if fstype != "vfat" {
		warnings = append(warnings, firmwareWarning{
			summary: firmwareESPMount + " is not vfat",
			detail:  "the EFI System Partition is " + fstype + "; fwupd normally writes capsules to a vfat ESP",
		})
	}
	if !readWrite {
		warnings = append(warnings, firmwareWarning{
			summary: firmwareESPMount + " is mounted read-only",
			detail:  "fwupd may be unable to write the capsule; remount it read-write before installing firmware",
		})
	}
	return warnings
}

// previousFailureWarning reports a failure fwupd still remembers. It reads the
// structured device state: the human output is not a stable format, and the
// Boot Order Lock sentence in it is a conditional hint, not a BIOS measurement.
func previousFailureWarning(run func(string, ...string) ([]byte, error)) (firmwareWarning, bool) {
	out, err := run("fwupdmgr", "get-devices", "--json")
	if err != nil {
		return firmwareWarning{
			summary: "could not read fwupd's device state",
			detail:  "the previous-attempt check was skipped: " + strings.TrimSpace(string(out)+" "+err.Error()),
		}, true
	}
	var report struct {
		Devices *[]struct {
			Name        string
			UpdateError string
		}
	}
	if err := json.Unmarshal(out, &report); err != nil {
		return firmwareWarning{
			summary: "could not parse fwupd's device state",
			detail:  "the previous-attempt check was skipped: " + err.Error(),
		}, true
	}
	if report.Devices == nil {
		return firmwareWarning{
			summary: "could not parse fwupd's device state",
			detail:  "the previous-attempt check was skipped: the document has no Devices array",
		}, true
	}
	for _, device := range *report.Devices {
		if !strings.Contains(device.UpdateError, "boot entry missing") &&
			!strings.Contains(device.UpdateError, "Linux Firmware Updater") {
			continue
		}
		name := device.Name
		if name == "" {
			name = "A device"
		}
		return firmwareWarning{
			summary: "fwupd still reports a previous firmware failure",
			detail: name + " reports: " + device.UpdateError + ". fwupd adds the Boot Order Lock hint to " +
				"this error and does not read the BIOS setting. A remembered error can outlive its cause; " +
				"investigate the updater and the ESP before another attempt",
		}, true
	}
	return firmwareWarning{}, false
}

func printFirmwareWarnings(out io.Writer, warnings []firmwareWarning) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprint(out, "Firmware warnings (the check still runs)\n")
	for _, w := range warnings {
		fmt.Fprintf(out, "  [warning] %s: %s\n", w.summary, w.detail)
	}
}
