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
	if err := runExtras(io.Discard, runner, updateExtras(true, true), false); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"flatpak update --system --noninteractive --assumeyes",
		"fwupdmgr get-updates --no-unreported-check",
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
	if err := runExtras(&out, runner, updateExtras(true, true), true); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("dry-run executed a command")
	}
	for _, want := range []string{
		"would run: flatpak update --system --noninteractive --assumeyes",
		"would run: fwupdmgr get-updates --no-unreported-check",
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
	err := runExtras(io.Discard, runner, updateExtras(true, true), false)
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
	if err := runExtras(io.Discard, runner, updateExtras(false, false), false); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("no flag still ran a command")
	}
}

func TestUpdateExtrasContinuesAfterTheFirmwareListingFails(t *testing.T) {
	var out, errOut bytes.Buffer
	runner := func(name string, _ ...string) error {
		if name == "fwupdmgr" {
			return errors.New("boom")
		}
		return nil
	}
	firmwareFailed, err := runUpdateExtras(&out, &errOut, runner, false, true, false)
	if err != nil {
		t.Fatalf("a failed listing stopped the run: %v", err)
	}
	if !firmwareFailed {
		t.Fatal("the firmware failure was not remembered")
	}
	if !strings.Contains(errOut.String(), "continuing with the image") {
		t.Errorf("errOut lacks the continuation message:\n%s", errOut.String())
	}
}

func TestUpdateExtrasStopsBeforeFirmwareWhenFlatpakFails(t *testing.T) {
	var ran []string
	runner := func(name string, _ ...string) error {
		ran = append(ran, name)
		return errors.New("boom")
	}
	_, err := runUpdateExtras(io.Discard, io.Discard, runner, true, true, false)
	if err == nil {
		t.Fatal("a failed Flatpak update did not fail the run")
	}
	if len(ran) != 1 || ran[0] != "flatpak" {
		t.Errorf("commands after the Flatpak failure ran: %q", ran)
	}
}
