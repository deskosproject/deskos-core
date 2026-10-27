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

// scalar is where a pinned value lives and how it was written, so a moved pin
// can be put back without touching a comment, a blank line or a quote.
type scalar struct {
	line  int
	style yaml.Style
}

// Pin is one checksum-pinned upstream artifact: a mapping that carries a
// version, a URL and a sha256. That is what a BinaryArtifact and a GNOME
// extension source have in common, and it is the shape this tool looks for.
type Pin struct {
	File    string
	Version string
	URL     string
	SHA256  string

	version, source, digest scalar
}

// Result is what a check found for one pin.
type Result struct {
	Pin Pin
	// HashOK is true when the pinned URL still serves the pinned sha256.
	HashOK  bool
	HashErr error
	// Latest* describe the upstream release the pin could move to. They are
	// empty when there is nothing newer, no rule to read it, or freshness was
	// not asked for.
	LatestTag     string
	LatestVersion string
	LatestURL     string
	LatestSHA256  string
	// Note says why a newer release did not produce a candidate.
	Note     string
	FreshErr error
}

// VersionLine is where the version is written, for a report a person can open.
func (p Pin) VersionLine() int { return p.version.line }

// Newer reports whether the upstream has a release the pin does not name.
func (r Result) Newer() bool { return r.LatestTag != "" }

// Movable reports whether a candidate URL and hash were resolved.
func (r Result) Movable() bool { return r.LatestURL != "" && r.LatestSHA256 != "" }

// Options controls what a check does.
type Options struct {
	// Client is the HTTP client to use. Nil means http.DefaultClient.
	Client *http.Client
	// Token authenticates against the GitHub API, if set.
	Token string
	// Fresh asks the upstream for the newest release, not only the hash.
	Fresh bool
	// Rules read a version out of a release tag. Without one, a newer release is
	// reported but never turned into a candidate.
	Rules []Rule
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
			checkFresh(client, opts, &r)
		}
		out = append(out, r)
	}
	return out, nil
}

func checkFresh(client *http.Client, opts Options, r *Result) {
	tag, err := latestTag(client, opts.Token, r.Pin.URL)
	if err != nil {
		r.FreshErr = err
		return
	}
	if tag == "" {
		return
	}
	r.LatestTag = tag
	rule, ok := ruleFor(opts.Rules, r.Pin.URL)
	if !ok {
		r.Note = "no curated rule for this repository; the pin is not moved"
		return
	}
	version := rule.versionOf(tag)
	if version == r.Pin.Version {
		r.Note = "the newest release names the same version"
		return
	}
	target, err := retarget(r.Pin.URL, tag, r.Pin.Version, version)
	if err != nil {
		r.FreshErr = err
		return
	}
	okHash, err := verify(client, target, "")
	if err != nil {
		r.FreshErr = err
		return
	}
	if !okHash {
		r.FreshErr = errors.New("the candidate released without a checksum to compare")
		return
	}
	r.LatestVersion = version
	r.LatestURL = target
	r.LatestSHA256 = hashOf(client, target)
}

// hashOf downloads a URL and returns its sha256, or "" when it cannot.
func hashOf(client *http.Client, raw string) string {
	resp, err := client.Get(raw)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Apply writes the resolved candidates back, one scalar at a time, so the rest
// of the file — comments, blank lines, order — is untouched.
func Apply(results []Result) (int, error) {
	byFile := map[string][]Result{}
	for _, r := range results {
		if r.Movable() {
			byFile[r.Pin.File] = append(byFile[r.Pin.File], r)
		}
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	moved := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return moved, err
		}
		text := string(data)
		lines := strings.Split(text, "\n")
		for _, r := range byFile[f] {
			var err error
			lines, err = setScalar(lines, r.Pin.version, "version", r.LatestVersion)
			if err != nil {
				return moved, fmt.Errorf("%s: %w", f, err)
			}
			lines, err = setScalar(lines, r.Pin.source, "url", r.LatestURL)
			if err != nil {
				return moved, fmt.Errorf("%s: %w", f, err)
			}
			lines, err = setScalar(lines, r.Pin.digest, "sha256", r.LatestSHA256)
			if err != nil {
				return moved, fmt.Errorf("%s: %w", f, err)
			}
			moved++
		}
		if err := os.WriteFile(f, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return moved, err
		}
	}
	return moved, nil
}

// setScalar replaces one value on its line, keeping indentation, quoting and a
// trailing comment.
func setScalar(lines []string, s scalar, key, value string) ([]string, error) {
	if s.line <= 0 || s.line > len(lines) {
		return nil, fmt.Errorf("line %d for %s is out of range", s.line, key)
	}
	line := lines[s.line-1]
	idx := strings.Index(line, key+":")
	if idx < 0 {
		return nil, fmt.Errorf("line %d does not declare %s", s.line, key)
	}
	prefix := line[:idx+len(key)+1]
	rest := line[idx+len(key)+1:]
	comment := ""
	if i := strings.Index(rest, "#"); i >= 0 {
		// Keep the whitespace before the comment exactly as it was.
		before := strings.TrimRight(rest[:i], " \t")
		comment = rest[len(before):]
	}
	lines[s.line-1] = prefix + " " + quote(value, s.style) + comment
	return lines, nil
}

func quote(v string, style yaml.Style) string {
	switch style {
	case yaml.DoubleQuotedStyle:
		return `"` + v + `"`
	case yaml.SingleQuotedStyle:
		return "'" + v + "'"
	default:
		return v
	}
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
// look like a pin. A YAML node keeps the line of each scalar, which is what the
// writer needs to move the pin without touching a comment.
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
	var version string
	var versionNode *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if val.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "version":
			version, versionNode = val.Value, val
		case "url":
			p.URL = val.Value
			p.source = scalar{line: val.Line, style: val.Style}
		case "sha256":
			p.SHA256 = val.Value
			p.digest = scalar{line: val.Line, style: val.Style}
		}
	}
	if versionNode == nil && parent != nil && parent.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(parent.Content); i += 2 {
			if parent.Content[i].Value == "version" && parent.Content[i+1].Kind == yaml.ScalarNode {
				version, versionNode = parent.Content[i+1].Value, parent.Content[i+1]
			}
		}
	}
	if version == "" || p.URL == "" || p.SHA256 == "" {
		return Pin{}, false
	}
	p.Version = version
	p.version = scalar{line: versionNode.Line, style: versionNode.Style}
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
	if want == "" {
		return true, nil
	}
	if !strings.EqualFold(got, want) {
		return false, fmt.Errorf("sha256 is %s, the resource pins %s", got, want)
	}
	return true, nil
}

// release is the shape of a GitHub release URL.
type release struct {
	owner, repo, tag, asset string
}

// releaseParts reads a GitHub release URL, or reports that it is not one.
func releaseParts(raw string) (release, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "github.com" {
		return release{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 6 || parts[2] != "releases" || parts[3] != "download" {
		return release{}, false
	}
	return release{owner: parts[0], repo: parts[1], tag: parts[4], asset: parts[5]}, true
}

// retarget builds the same asset at another release: the tag segment changes and
// the version inside the asset name follows it.
func retarget(raw, newTag, oldVersion, newVersion string) (string, error) {
	rel, ok := releaseParts(raw)
	if !ok {
		return "", fmt.Errorf("%s is not a GitHub release URL", raw)
	}
	moved := strings.Replace(raw, "/download/"+rel.tag+"/", "/download/"+newTag+"/", 1)
	idx := strings.LastIndex(moved, "/")
	asset := moved[idx+1:]
	// The tag may contain the version (`v0.17.2` and `0.17.2`): only the asset
	// name is rewritten.
	if oldVersion != "" && newVersion != "" && oldVersion != newVersion {
		asset = strings.ReplaceAll(asset, oldVersion, newVersion)
	}
	return moved[:idx+1] + asset, nil
}

// latestTag asks GitHub for the newest release of the repository a release URL
// points at. A URL that is not a GitHub release has no answer, which is not an
// error: the hash check is all that applies to it.
func latestTag(client *http.Client, token, raw string) (string, error) {
	rel, ok := releaseParts(raw)
	if !ok {
		return "", nil
	}
	api := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", rel.owner, rel.repo)
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
	if body.TagName == "" || body.TagName == rel.tag {
		return "", nil
	}
	return body.TagName, nil
}
