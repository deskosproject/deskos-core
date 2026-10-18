package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGvariantStrings(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		ok   bool
	}{
		{"", nil, true},       // an unset key is a valid empty list
		{"[]", nil, true},     // an empty array
		{"@as []", nil, true}, // the explicit empty typed array
		{"['a']", []string{"a"}, true},
		{"['a', 'b']", []string{"a", "b"}, true},
		{"'abc-123'", nil, false}, // a scalar string is not a list
		{"garbage", nil, false},   // an unrecognized value must not read as empty
		{"['a'", nil, false},      // a truncated array
	}
	for _, c := range cases {
		got, ok := gvariantStrings(c.in)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("gvariantStrings(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestGvariantStringValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},   // an unset key
		{"''", "", true}, // an empty string
		{"'abc-123'", "abc-123", true},
		{"['a']", "", false}, // an array is not a scalar string
		{"garbage", "", false},
	}
	for _, c := range cases {
		got, ok := gvariantStringValue(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("gvariantStringValue(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestPtyxisPlan(t *testing.T) {
	// A user with two profiles and their own default keeps both and the
	// default; the DeskOS profile is appended once.
	list, def := ptyxisPlan([]string{"x", "y"}, "abc-123")
	want := []string{"x", "y", ptyxisProfileUUID}
	if !reflect.DeepEqual(list, want) || def != "abc-123" {
		t.Errorf("ptyxisPlan(x,y | abc-123) = %v, %q; want %v, abc-123", list, def, want)
	}

	// A fresh user gets the DeskOS profile as the list and the default.
	list, def = ptyxisPlan(nil, "")
	if !reflect.DeepEqual(list, []string{ptyxisProfileUUID}) || def != ptyxisProfileUUID {
		t.Errorf("ptyxisPlan(empty) = %v, %q; want [ours], ours", list, def)
	}

	// The DeskOS profile is not added twice, and the user's default is kept.
	list, def = ptyxisPlan([]string{ptyxisProfileUUID}, ptyxisProfileUUID)
	if !reflect.DeepEqual(list, []string{ptyxisProfileUUID}) || def != ptyxisProfileUUID {
		t.Errorf("ptyxisPlan(ours | ours) = %v, %q; want [ours], ours", list, def)
	}
}

func TestShellExtensionsComesFromThePlan(t *testing.T) {
	// The shell drop in sorts after the plan's own dconf keyfile, so it has to repeat
	// the list the image really enables: inventing one here would silently override the
	// plan (and a hardcoded pair is a lie on a platform that ships neither extension).
	root := t.TempDir()
	writePlanForTheme(t, root, `{"artifact":{"dconf":{"name":"distro","defaults":[
	  {"key":"/org/gnome/shell/enabled-extensions","type":"as","value":"['a@b', 'c@d']"}]}}}`)
	got, err := shellExtensions(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"a@b", "c@d"}) {
		t.Errorf("shellExtensions from the plan = %v", got)
	}

	// An explicit list wins, so the flag stays usable on an image that is not ours.
	got, err = shellExtensions(root, "x@y")
	if err != nil || !reflect.DeepEqual(got, []string{"x@y"}) {
		t.Errorf("explicit = %v, %v", got, err)
	}
}

func TestShellExtensionsRefusesRatherThanInventingOne(t *testing.T) {
	// No plan, and no explicit list: refuse, with the way out named.
	if _, err := shellExtensions(t.TempDir(), ""); err == nil || !strings.Contains(err.Error(), "pass --extensions explicitly") {
		t.Errorf("err = %v", err)
	}
	// A plan that declares no such default is the same answer, not an empty list.
	root := t.TempDir()
	writePlanForTheme(t, root, `{"artifact":{"dconf":{"name":"distro","defaults":[]}}}`)
	if _, err := shellExtensions(root, ""); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Errorf("err = %v", err)
	}
}

func writePlanForTheme(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "usr/share/deskos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
