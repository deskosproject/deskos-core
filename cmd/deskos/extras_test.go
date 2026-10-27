package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUpdateExtrasRunsFlatpakBeforeTheFirmwareCheck(t *testing.T) {
	var got []string
	runner := func(name string, args ...string) error {
		got = append(got, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := runExtras(io.Discard, runner, updateExtras(false, false, true), false); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"flatpak update --system --noninteractive --assumeyes",
		"fwupdmgr get-updates",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("commands = %q, want %q", got, want)
	}
}

func TestUpdateExtrasDryRunRunsNothing(t *testing.T) {
	var out bytes.Buffer
	called := false
	runner := func(string, ...string) error {
		called = true
		return nil
	}
	if err := runExtras(&out, runner, updateExtras(true, true, false), true); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("dry-run executed a command")
	}
	for _, want := range []string{
		"would run: flatpak update --system --noninteractive --assumeyes",
		"would run: fwupdmgr get-updates",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out lacks %q:\n%s", want, out.String())
		}
	}
}

func TestUpdateExtrasStopsAtAFailedCommand(t *testing.T) {
	var got []string
	runner := func(name string, _ ...string) error {
		got = append(got, name)
		return errors.New("boom")
	}
	err := runExtras(io.Discard, runner, updateExtras(false, false, true), false)
	if err == nil {
		t.Fatal("a failed command did not fail the run")
	}
	if len(got) != 1 || got[0] != "flatpak" {
		t.Errorf("commands after the failure ran: %q", got)
	}
}

func TestUpdateExtrasWithoutFlagsDoesNothing(t *testing.T) {
	called := false
	runner := func(string, ...string) error {
		called = true
		return nil
	}
	if err := runExtras(io.Discard, runner, updateExtras(false, false, false), false); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("no flag still ran a command")
	}
}
