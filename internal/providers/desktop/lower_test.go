package desktop

import "testing"

func TestGVStringEscapesControlCharacters(t *testing.T) {
	cases := map[string]string{
		"plain":      `'plain'`,
		"line\none":  `'line\none'`,
		"tab\tone":   `'tab\tone'`,
		"quote's":    `'quote\'s'`,
		`back\slash`: `'back\\slash'`,
	}
	for in, want := range cases {
		if got := gvString(in); got != want {
			t.Errorf("gvString(%q) = %q, want %q", in, got, want)
		}
	}
}
