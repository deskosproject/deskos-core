// Package track finds the checksum-pinned resources in a tree and checks them
// against upstream. It lives beside the command, not under internal/: the
// compiler and its packages must not be able to reach the network, and this is
// a maintenance tool. The compiler never calls it, and nothing it does may
// change a Plan by itself — a person reviews what it reports and moves the pins.
package track

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Pin is one checksum-pinned upstream artifact: a mapping that carries a
// version, a URL and a sha256. That is what a BinaryArtifact and a GNOME
// extension source have in common, and it is the shape this tool looks for.
type Pin struct {
	File    string
	Line    int
	Version string
	URL     string
	SHA256  string
}

// Result is what a check found for one pin.
type Result struct {
	Pin Pin
	// HashOK is true when the URL still serves a file with the pinned sha256.
	HashOK bool
	// HashErr is why the download or the comparison failed, if it did.
	HashErr error
	// LatestTag is the newest upstream release tag, when the URL is a GitHub
	// release and freshness was asked for.
	LatestTag string
	// FreshErr is why the upstream could not be asked, if it could not.
	FreshErr error
}

// Newer reports whether the upstream has a release the pin does not name.
func (r Result) Newer() bool { return r.LatestTag != "" }

// Options controls what a check does.
type Options struct {
	// Client is the HTTP client to use. Nil means http.DefaultClient.
	Client *http.Client
	// Token authenticates against the GitHub API, if set.
	Token string
	// Fresh asks the upstream for the newest release, not only the hash.
	Fresh bool
}

// Check walks the roots, finds every pin and verifies it.
func Check(roots []string, opts Options) ([]Result, error) {
	pins, err := Find(roots)
	if err != nil {
		return nil, err
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	var out []Result
	for _, p := range pins {
		r := Result{Pin: p}
		r.HashOK, r.HashErr = verify(client, p.URL, p.SHA256)
		if opts.Fresh {
			r.LatestTag, r.FreshErr = latestTag(client, opts.Token, p.URL)
		}
		out = append(out, r)
	}
	return out, nil
}

// Find returns every pin under the roots, in a deterministic order.
func Find(roots []string) ([]Pin, error) {
	if len(roots) == 0 {
		return nil, errors.New("no resource roots given")
	}
	var files []string
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("resource root %s: %w", root, err)
		}
		if !info.IsDir() {
			files = append(files, abs)
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != abs {
				return filepath.SkipDir
			}
			if d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(p)) {
			case ".yaml", ".yml":
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)

	var pins []Pin
	for _, f := range files {
		found, err := findInFile(f)
		if err != nil {
			return nil, err
		}
		pins = append(pins, found...)
	}
	return pins, nil
}

// findInFile parses every document in the file and collects the mappings that
// look like a pin. A YAML node keeps the line of each scalar, which is what a
// later writer needs to move the pin without touching a comment.
func findInFile(path string) ([]Pin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	var out []Pin
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		walk(&doc, nil, func(n, parent *yaml.Node) {
			if p, ok := pinOf(n, parent, path); ok {
				out = append(out, p)
			}
		})
	}
	return out, nil
}

func walk(n, parent *yaml.Node, fn func(n, parent *yaml.Node)) {
	if n == nil {
		return
	}
	fn(n, parent)
	for _, c := range n.Content {
		walk(c, n, fn)
	}
}

// pinOf reports a mapping that pins an artifact. A BinaryArtifact keeps the
// version beside the source (`spec.version`, `spec.source.url`), an extension
// source keeps all three together, so the version is looked for in the mapping
// and then in its parent.
func pinOf(n, parent *yaml.Node, path string) (Pin, bool) {
	if n.Kind != yaml.MappingNode {
		return Pin{}, false
	}
	var p Pin
	p.File = path
	version := ""
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if val.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "version":
			version = val.Value
			p.Line = val.Line
		case "url":
			p.URL = val.Value
		case "sha256":
			p.SHA256 = val.Value
		}
	}
	if version == "" && parent != nil && parent.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(parent.Content); i += 2 {
			if parent.Content[i].Value == "version" && parent.Content[i+1].Kind == yaml.ScalarNode {
				version = parent.Content[i+1].Value
				p.Line = parent.Content[i+1].Line
			}
		}
	}
	p.Version = version
	if p.Version == "" || p.URL == "" || p.SHA256 == "" {
		return Pin{}, false
	}
	return p, true
}

func verify(client *http.Client, raw, want string) (bool, error) {
	resp, err := client.Get(raw)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GET %s: %s", raw, resp.Status)
	}
	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return false, fmt.Errorf("sha256 is %s, the resource pins %s", got, want)
	}
	return true, nil
}

// latestTag asks GitHub for the newest release of the repository a release URL
// points at. A URL that is not a GitHub release has no answer, which is not an
// error: the hash check is all that applies to it.
func latestTag(client *http.Client, token, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil
	}
	if u.Host != "github.com" {
		return "", nil
	}
	// /<owner>/<repo>/releases/download/<tag>/<asset>
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 6 || parts[2] != "releases" || parts[3] != "download" {
		return "", nil
	}
	api := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", parts[0], parts[1])
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", api, resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.TagName == "" || tagOf(raw) == body.TagName {
		return "", nil
	}
	return body.TagName, nil
}

// tagOf returns the release tag in a GitHub release URL, or "".
func tagOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 6 || parts[3] != "download" {
		return ""
	}
	return parts[4]
}
