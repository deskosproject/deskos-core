package main

import (
	"reflect"
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
