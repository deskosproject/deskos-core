package system

import (
	"strings"
	"testing"
)

func TestKeyfileEscape(t *testing.T) {
	cases := map[string]string{
		`CORP\user`: `CORP\\user`,
		"line\none": `line\none`,
		" leading":  `\sleading`,
		"tab\tone":  `tab\tone`,
	}
	for in, want := range cases {
		if got := keyfileEscape(in); got != want {
			t.Errorf("keyfileEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNetworkKeyfileEscapesIdentity(t *testing.T) {
	got := networkKeyfile(NetworkProfile{
		Name: "corp", Type: "wifi", SSID: "corp", EAP: "peap",
		Phase2: "mschapv2", Identity: `CORP\user`, Secret: "agent",
	})
	if !strings.Contains(got, `identity=CORP\\user`) {
		t.Errorf("keyfile does not escape the identity:\n%s", got)
	}
}
