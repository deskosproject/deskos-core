package core

import "testing"

// The typed pattern must reject the same traversals the schema pattern does,
// so a resource that bypasses the schema (or a future schema change) still
// cannot place a certificate outside /etc.
func TestTrustDirPatternRejectsTraversal(t *testing.T) {
	bad := []string{
		"/etc/../../usr/share/deskos",
		"/etc/pki/../..",
		"/etc/./anchors",
		"/etc/anchors/..",
		"/tmp/anchors",
		"/etc",
		"/etc/",
	}
	for _, d := range bad {
		if trustDirRE.MatchString(d) {
			t.Errorf("trustDirRE accepts %q", d)
		}
	}
	good := []string{
		"/etc/pki/ca-trust/source/anchors",
		"/etc/anchors",
		"/etc/pki.d/anchors",
		"/etc/.hidden",
	}
	for _, d := range good {
		if !trustDirRE.MatchString(d) {
			t.Errorf("trustDirRE rejects %q", d)
		}
	}
}
