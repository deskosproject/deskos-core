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
	if pins[0].Line == 0 {
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
	if results[0].HashOK || results[0].HashErr == nil {
		t.Fatalf("expected a mismatch, got %+v", results[0])
	}
	if !strings.Contains(results[0].HashErr.Error(), "the resource pins") {
		t.Fatalf("the mismatch must name both hashes: %v", results[0].HashErr)
	}
}

func TestFreshnessAsksGitHubForTheNewestRelease(t *testing.T) {
	var api string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/deskosproject/deskos/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("authorization header = %q", got)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.18.0"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	api = srv.URL

	raw := api + "/repos/deskosproject/deskos/releases/latest"
	_ = raw

	dir := t.TempDir()
	// A GitHub release URL that resolves to the fake server: the tool builds the
	// API URL from github.com, so exercise latestTag directly with a client whose
	// transport rewrites the host.
	client := &http.Client{Transport: rewrite{to: api, base: http.DefaultTransport}}
	tag, err := latestTag(client, "tok", "https://github.com/deskosproject/deskos/releases/download/v0.17.2/deskos_0.17.2.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.18.0" {
		t.Fatalf("tag = %q, want v0.18.0", tag)
	}

	// The same release the pin names is not newer.
	tag, err = latestTag(client, "tok", "https://github.com/deskosproject/deskos/releases/download/v0.18.0/deskos_0.18.0.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "" {
		t.Fatalf("tag = %q, want empty for the current release", tag)
	}
	_ = dir
}

type rewrite struct {
	to   string
	base http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(r.to, "http://")
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return r.base.RoundTrip(req2)
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
