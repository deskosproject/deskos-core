package main

import (
	"errors"
	"strings"
	"testing"
)

func fakeRunner(responses map[string]string, failures map[string]bool) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		if failures[key] {
			return []byte(responses[key]), errors.New("command failed")
		}
		if out, ok := responses[key]; ok {
			return []byte(out), nil
		}
		return nil, errors.New("unexpected command: " + key)
	}
}

func TestFWUPDEFIWarnsOnTheFedoraPkgnoteBuild(t *testing.T) {
	run := fakeRunner(map[string]string{
		"rpm -q --qf %{VERSION}-%{RELEASE} fwupd-efi": "1.9-3.fc45\n",
	}, nil)
	warning, ok := fwupdEFIWarning(run)
	if !ok {
		t.Fatal("1.9-3.fc45 did not warn")
	}
	if !strings.Contains(warning.detail, "1.9-4") {
		t.Errorf("the warning does not name the fixed build: %q", warning.detail)
	}
	if strings.Contains(warning.summary, "cannot stage") {
		t.Errorf("the warning overstates what failed: %q", warning.summary)
	}
}

func TestFWUPDEFIAcceptsFixedAndUnrelatedBuilds(t *testing.T) {
	for _, version := range []string{
		"1.9-4.fc45",
		"1.9-4.fc45.1",
		"1.9-10.fc45",
		"1.10-1.fc45",
		"1.8-1.el10",
		"1.9-3.el10",
		"1.9-4.el10",
	} {
		run := fakeRunner(map[string]string{
			"rpm -q --qf %{VERSION}-%{RELEASE} fwupd-efi": version + "\n",
		}, nil)
		if warning, ok := fwupdEFIWarning(run); ok {
			t.Errorf("%s warned: %q", version, warning.detail)
		}
	}
}

func TestFWUPDEFIWarnsWhenTheVersionCannotBeAssessed(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		failed  bool
		summary string
	}{
		{"missing package", "package fwupd-efi is not installed\n", true, "not installed"},
		{"failed rpm", "", true, "could not read"},
		{"malformed release", "1.9-garbage\n", false, "could not assess"},
		{"malformed distro", "1.9-3.fcgarbage\n", false, "could not assess"},
		{"empty version component", "1..9-3.fc45\n", false, "could not assess"},
		{"extra output", "1.9-4.fc45\nunexpected\n", false, "could not assess"},
		{"no release", "1.9\n", false, "could not assess"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "rpm -q --qf %{VERSION}-%{RELEASE} fwupd-efi"
			run := fakeRunner(map[string]string{key: tt.output}, map[string]bool{key: tt.failed})
			warning, ok := fwupdEFIWarning(run)
			if !ok {
				t.Fatal("an unassessable version was treated as safe")
			}
			if !strings.Contains(warning.summary, tt.summary) {
				t.Errorf("summary = %q, want it to contain %q", warning.summary, tt.summary)
			}
		})
	}
}

const espMountinfo = "36 1 259:1 / /boot/efi rw,relatime shared:7 - vfat /dev/nvme0n1p1 rw\n"

func TestESPAcceptsAMountedWritableVfatPartition(t *testing.T) {
	if warnings := espWarningsFrom(espMountinfo); len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestESPRequiresTheMountpoint(t *testing.T) {
	warnings := espWarningsFrom("36 1 259:1 / / rw - btrfs /dev/nvme0n1p2 rw\n")
	if len(warnings) != 1 || !strings.Contains(warnings[0].summary, "not a mountpoint") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestESPRequiresVfatAndWriteAccess(t *testing.T) {
	warnings := espWarningsFrom("36 1 259:1 / /boot/efi ro,relatime shared:7 - ext4 /dev/nvme0n1p1 rw\n")
	if len(warnings) != 2 {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestESPWarnsAboutReadOnly(t *testing.T) {
	warnings := espWarningsFrom("36 1 259:1 / /boot/efi ro,relatime shared:7 - vfat /dev/nvme0n1p1 rw\n")
	if len(warnings) != 1 || !strings.Contains(warnings[0].summary, "read-only") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

const previousFailureJSON = `{"Devices":[{"Name":"System Firmware","UpdateError":"boot entry missing; perhaps 'Boot Order Lock' enabled in the BIOS: no 'Linux Firmware Updater' entry found"}]}`

func TestPreviousFailureIsReadFromStructuredState(t *testing.T) {
	run := fakeRunner(map[string]string{"fwupdmgr get-devices --json": previousFailureJSON}, nil)
	warning, ok := previousFailureWarning(run)
	if !ok {
		t.Fatal("the known boot-entry failure was not detected")
	}
	if !strings.Contains(warning.summary, "previous firmware failure") {
		t.Errorf("summary = %q", warning.summary)
	}
}

func TestCleanAndUnrelatedStatesDoNotWarn(t *testing.T) {
	for _, out := range []string{
		`{"Devices":[]}`,
		`{"Devices":[{"Name":"System Firmware"}]}`,
		`{"Devices":[{"Name":"System Firmware","UpdateError":"failed to write-firmware: device was not in supported mode"}]}`,
	} {
		run := fakeRunner(map[string]string{"fwupdmgr get-devices --json": out}, nil)
		if warning, ok := previousFailureWarning(run); ok {
			t.Errorf("state warned: %q", warning.detail)
		}
	}
}

func TestMissingDeviceArrayIsUnknown(t *testing.T) {
	for _, out := range []string{`null`, `{}`, `{"Devices":null}`} {
		run := fakeRunner(map[string]string{"fwupdmgr get-devices --json": out}, nil)
		warning, ok := previousFailureWarning(run)
		if !ok || !strings.Contains(warning.summary, "could not parse") {
			t.Errorf("%s: warning = %#v", out, warning)
		}
	}
}

func TestPreviousFailureIsUnknownWhenStateCannotBeRead(t *testing.T) {
	key := "fwupdmgr get-devices --json"
	for _, tt := range []struct {
		output  string
		failed  bool
		summary string
	}{
		{output: "daemon is not running", failed: true, summary: "could not read"},
		{output: "not json", failed: false, summary: "could not parse"},
	} {
		run := fakeRunner(map[string]string{key: tt.output}, map[string]bool{key: tt.failed})
		warning, ok := previousFailureWarning(run)
		if !ok || !strings.Contains(warning.summary, tt.summary) {
			t.Errorf("warning = %#v", warning)
		}
	}
}

func TestFirmwareWarningsSkipsTheDaemonWhenNotAsked(t *testing.T) {
	var called []string
	run := func(name string, _ ...string) ([]byte, error) {
		called = append(called, name)
		if name == "rpm" {
			return []byte("1.9-4.fc45\n"), nil
		}
		return nil, errors.New("unexpected command: " + name)
	}
	_ = firmwareWarnings(run, false)
	for _, name := range called {
		if name == "fwupdmgr" {
			t.Fatal("a dry run queried the fwupd daemon")
		}
	}
}

func TestFirmwareListingDoesNotUploadReports(t *testing.T) {
	cmds := updateExtras(false, true)
	if len(cmds) != 1 || cmds[0][0] != "fwupdmgr" {
		t.Fatalf("firmware commands = %q", cmds)
	}
	line := strings.Join(cmds[0], " ")
	if !strings.Contains(line, "--no-unreported-check") {
		t.Errorf("%q does not disable the report-upload path", line)
	}
}

func TestUpdateExtrasNeverFlashesFirmware(t *testing.T) {
	for _, flatpak := range []bool{false, true} {
		for _, firmware := range []bool{false, true} {
			for _, cmd := range updateExtras(flatpak, firmware) {
				line := strings.Join(cmd, " ")
				if cmd[0] == "fwupdmgr" && line != "fwupdmgr get-updates --no-unreported-check" {
					t.Errorf("a firmware command other than the check was built: %q", line)
				}
			}
		}
	}
}
