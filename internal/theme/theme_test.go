package theme

import "testing"

func TestNearestAccent(t *testing.T) {
	for color, want := range map[string]string{
		"#3584e4": "blue",
		"#3a944a": "green",
		"#e62d42": "red",
		"#ed5b00": "orange",
		"#7aa2f7": "blue",
		"#9ece6a": "green",
		"#f7768e": "red",
		"#b8bb26": "yellow",
		"#000000": "slate",
	} {
		if got, ok := NearestAccent(color); !ok || got != want {
			t.Errorf("NearestAccent(%q) = %q, %v; want %q", color, got, ok, want)
		}
	}
	for _, bad := range []string{"nope", "#12345", "#gggggg"} {
		if _, ok := NearestAccent(bad); ok {
			t.Errorf("NearestAccent(%q) accepted an invalid color", bad)
		}
	}
}
