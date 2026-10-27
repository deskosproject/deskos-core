package track

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const asset = "deskos archive contents"

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindTakesAVersionURLAndChecksum(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "software.yaml", `apiVersion: software.deskos.org/v1alpha1
kind: BinaryArtifact
metadata:
  name: deskos
spec:
  version: v0.17.2
  source:
    url: https://example.test/deskos_0.17.2.tar.gz
    sha256: `+sha(asset)+`
  archive: tar.gz
---
apiVersion: core.deskos.org/v1alpha1
kind: PackageSet
metadata:
  name: not-a-pin
spec:
  packages:
    - firefox
`)
	pins, err := Find([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatalf("got %d pins, want 1", len(pins))
	}
	if pins[0].Version != "v0.17.2" || !strings.HasSuffix(pins[0].URL, "deskos_0.17.2.tar.gz") {
		t.Fatalf("unexpected pin: %+v", pins[0])
	}
	if pins[0].version.line == 0 {
		t.Fatal("the pin has no line, so it cannot be moved later")
	}
}

func TestCheckComparesTheServedFileWithThePin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(asset))
	}))
	defer srv.Close()

	dir := t.TempDir()
	write(t, dir, "artifact.yaml", `kind: BinaryArtifact
metadata:
  name: deskos
spec:
  version: v0.17.2
  source:
    url: `+srv.URL+`/deskos_0.17.2.tar.gz
    sha256: `+sha(asset)+`
`)
	results, err := Check([]string{dir}, Options{Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].HashOK {
		t.Fatalf("expected a verified pin, got %+v", results)
	}
}

func TestCheckReportsADifferentFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("something else"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	write(t, dir, "artifact.yaml", `kind: BinaryArtifact
metadata:
  name: deskos
spec:
  version: v0.17.2
  source:
    url: `+srv.URL+`/deskos_0.17.2.tar.gz
    sha256: `+sha(asset)+`
`)
	results, err := Check([]string{dir}, Options{Client: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].HashOK || results[0].HashErr == nil {
		t.Fatalf("expected a mismatch, got %+v", results[0])
	}
	if !strings.Contains(results[0].HashErr.Error(), "the resource pins") {
		t.Fatalf("the mismatch must name both hashes: %v", results[0].HashErr)
	}
}

func TestARuleReadsTheVersionOutOfTheTag(t *testing.T) {
	rule := Rule{Match: "github.com/micheleg/dash-to-dock", VersionFromTag: "extensions.gnome.org-v"}
	if got := rule.versionOf("extensions.gnome.org-v110"); got != "110" {
		t.Fatalf("version = %q, want 110", got)
	}
	got, ok := ruleFor([]Rule{rule}, "https://github.com/micheleg/dash-to-dock/releases/download/extensions.gnome.org-v110/x.zip")
	if !ok || got.VersionFromTag != "extensions.gnome.org-v" {
		t.Fatalf("rule not found: %+v %v", got, ok)
	}
	if _, ok := ruleFor([]Rule{rule}, "https://github.com/other/repo/releases/download/v1/x"); ok {
		t.Fatal("a rule must not match another repository")
	}
}

func TestRetargetChangesTheTagAndTheAssetName(t *testing.T) {
	got, err := retarget(
		"https://github.com/deskosproject/deskos-core/releases/download/v0.17.2/deskos-v0.17.2-linux-amd64",
		"v0.18.0", "0.17.2", "0.18.0")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/deskosproject/deskos-core/releases/download/v0.18.0/deskos-v0.18.0-linux-amd64"
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestApplyMovesThePinAndKeepsTheFileAroundIt(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "artifact.yaml", `kind: BinaryArtifact
metadata:
  name: deskos
spec:
  # the version is quoted because it looks like a number
  version: "0.17.2"
  source:
    url: https://github.com/o/r/releases/download/v0.17.2/tool_0.17.2.tar.gz
    sha256: `+strings.Repeat("a", 64)+`  # pinned
  archive: tar.gz
`)
	pins, err := Find([]string{dir})
	if err != nil || len(pins) != 1 {
		t.Fatalf("pins: %+v %v", pins, err)
	}
	r := Result{
		Pin:           pins[0],
		LatestVersion: "0.18.0",
		LatestURL:     "https://github.com/o/r/releases/download/v0.18.0/tool_0.18.0.tar.gz",
		LatestSHA256:  strings.Repeat("b", 64),
	}
	moved, err := Apply([]Result{r})
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d, want 1", moved)
	}
	got := readFile(t, path)
	for _, want := range []string{
		`version: "0.18.0"`,
		"url: https://github.com/o/r/releases/download/v0.18.0/tool_0.18.0.tar.gz",
		"sha256: " + strings.Repeat("b", 64) + "  # pinned",
		"# the version is quoted because it looks like a number",
		"  archive: tar.gz",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestAFreshReleaseBecomesACandidateOnlyWithARule(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	})
	mux.HandleFunc("/o/r/releases/download/v0.1.0/tool_0.1.0.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("old"))
	})
	mux.HandleFunc("/o/r/releases/download/v0.2.0/tool_0.2.0.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &http.Client{Transport: hostRewrite{to: srv.URL}}

	dir := t.TempDir()
	write(t, dir, "artifact.yaml", `kind: BinaryArtifact
metadata:
  name: tool
spec:
  version: 0.1.0
  source:
    url: https://github.com/o/r/releases/download/v0.1.0/tool_0.1.0.tar.gz
    sha256: `+sha("old")+`
`)
	// Without a rule the newer release is reported and nothing else.
	results, err := Check([]string{dir}, Options{Client: client, Fresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].LatestTag != "v0.2.0" || results[0].Movable() {
		t.Fatalf("without a rule: %+v", results[0])
	}
	if !strings.Contains(results[0].Note, "no curated rule") {
		t.Fatalf("note = %q", results[0].Note)
	}

	// With one, the candidate carries the new URL and its hash.
	results, err = Check([]string{dir}, Options{
		Client: client, Fresh: true,
		Rules: []Rule{{Match: "github.com/o/r", VersionFromTag: "v"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Movable() {
		t.Fatalf("with a rule: %+v", results[0])
	}
	if results[0].LatestVersion != "0.2.0" || results[0].LatestSHA256 != sha("new") {
		t.Fatalf("candidate: %+v", results[0])
	}
}

// hostRewrite sends every request to the test server, keeping the path.
type hostRewrite struct{ to string }

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(h.to, "http://")
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return http.DefaultTransport.RoundTrip(req2)
}

func TestAURLThatIsNotAGitHubReleaseHasNoUpstream(t *testing.T) {
	tag, err := latestTag(http.DefaultClient, "", "https://mirror.example.test/tool/tool_1.2.3.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "" {
		t.Fatalf("tag = %q, want empty", tag)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
