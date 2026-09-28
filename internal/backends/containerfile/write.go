package containerfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const manifestFormat = "manifest.deskos.org/v1alpha1"

// WriteDir writes files into dir. dir must be missing, empty, or an
// unmodified previous render: every entry is a regular file or directory
// listed in its generated manifest with matching content and mode. All
// checks happen before anything is removed or written, and every operation
// goes through an os.Root, so nothing outside dir is ever touched.
func WriteDir(dir string, files []File) error {
	if err := checkNewFiles(files); err != nil {
		return err
	}
	// "link/" and "link/." would make Lstat follow a symlinked output dir.
	dir = filepath.Clean(dir)
	if err := ensureDir(dir); err != nil {
		return err
	}
	root, err := openConfined(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	old, err := previousRender(root, dir)
	if err != nil {
		return err
	}
	for _, p := range old.files {
		if err := root.Remove(p); err != nil {
			return err
		}
	}
	for _, d := range old.dirs {
		if err := root.Remove(d); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err := writeFile(root, f); err != nil {
			return err
		}
	}
	return nil
}

func checkNewFiles(files []File) error {
	seen := map[string]bool{}
	for _, f := range files {
		if !fs.ValidPath(f.Path) || f.Path == "." || strings.Contains(f.Path, `\`) {
			return fmt.Errorf("build context path %q is not a canonical relative path", f.Path)
		}
		if f.Mode&^fs.ModePerm != 0 {
			return fmt.Errorf("build context path %s has non-permission mode bits", f.Path)
		}
		if seen[f.Path] {
			return fmt.Errorf("build context path %s is generated twice", f.Path)
		}
		seen[f.Path] = true
	}
	for p := range seen {
		for _, d := range parents(p) {
			if seen[d] {
				return fmt.Errorf("build context path %s is both a file and a directory", d)
			}
		}
	}
	return nil
}

// ensureDir creates dir if needed and rejects a dir that is itself a
// symbolic link or not a directory. Symlinked ancestors are resolved once
// when the root is opened.
func ensureDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("output directory %s is a symbolic link; refusing to write through it", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %s is not a directory", dir)
	}
	return nil
}

// openConfined opens dir as a root and checks it is still the directory
// that was inspected, so a swap to a symlink in between is detected.
func openConfined(dir string) (*os.Root, error) {
	before, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || before.Mode()&fs.ModeSymlink != 0 || !os.SameFile(before, opened) {
		root.Close()
		return nil, fmt.Errorf("output directory %s changed while it was being opened", dir)
	}
	return root, nil
}

type removal struct {
	files []string
	dirs  []string // deepest first
}

var (
	sha256HexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	modeRE      = regexp.MustCompile(`^0[0-7]{3}$`)
)

// previousRender validates the existing directory content and returns what
// may be removed. An empty directory yields nothing to remove.
func previousRender(root *os.Root, dir string) (*removal, error) {
	type entry struct {
		dir  bool
		mode fs.FileMode
	}
	entries := map[string]entry{}
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		switch t := d.Type(); {
		case t&fs.ModeSymlink != 0:
			return fmt.Errorf("output directory %s contains a symbolic link (%s); refusing to modify it", dir, p)
		case t.IsDir():
			entries[p] = entry{dir: true}
		case t.IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			entries[p] = entry{mode: info.Mode().Perm()}
		default:
			return fmt.Errorf("output directory %s contains a special file (%s); refusing to modify it", dir, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return &removal{}, nil
	}
	if e, ok := entries[ManifestPath]; !ok || e.dir {
		return nil, fmt.Errorf("output directory %s is not empty and is not a previous render (no %s)", dir, ManifestPath)
	}

	listed, err := readManifest(root, dir)
	if err != nil {
		return nil, err
	}
	r := &removal{}
	ancestors := map[string]bool{}
	for p := range listed {
		for _, d := range parents(p) {
			ancestors[d] = true
		}
	}
	for p, e := range entries {
		switch {
		case p == ManifestPath:
		case e.dir:
			if !ancestors[p] {
				return nil, fmt.Errorf("refusing to modify %s: directory %s is not part of the previous render", dir, p)
			}
			r.dirs = append(r.dirs, p)
		default:
			want, ok := listed[p]
			if !ok {
				return nil, fmt.Errorf("refusing to modify %s: %s is not part of the previous render", dir, p)
			}
			if fmt.Sprintf("%04o", e.mode) != want.Mode {
				return nil, fmt.Errorf("refusing to modify %s: %s changed mode since the previous render", dir, p)
			}
			data, err := root.ReadFile(p)
			if err != nil {
				return nil, err
			}
			if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want.SHA256 {
				return nil, fmt.Errorf("refusing to modify %s: %s was edited since the previous render", dir, p)
			}
			r.files = append(r.files, p)
		}
	}
	r.files = append(r.files, ManifestPath)
	sort.Strings(r.files)
	sort.Slice(r.dirs, func(i, j int) bool {
		if a, b := strings.Count(r.dirs[i], "/"), strings.Count(r.dirs[j], "/"); a != b {
			return a > b
		}
		return r.dirs[i] > r.dirs[j]
	})
	return r, nil
}

type manifestEntry struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

// readManifest strictly parses a previous manifest. Paths are untrusted:
// they must be canonical relative paths and are only used to match files
// that were found by walking the root.
func readManifest(root *os.Root, dir string) (map[string]manifestEntry, error) {
	data, err := root.ReadFile(ManifestPath)
	if err != nil {
		return nil, err
	}
	var m struct {
		Format      string          `json:"format"`
		Backend     string          `json:"backend"`
		Workstation string          `json:"workstation"`
		Files       []manifestEntry `json:"files"`
	}
	bad := func(format string, a ...any) error {
		return fmt.Errorf("output directory %s has an invalid %s: %s", dir, ManifestPath, fmt.Sprintf(format, a...))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, bad("%v", err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, bad("trailing data")
	}
	if m.Format != manifestFormat || m.Backend != Name {
		return nil, bad("format %q, backend %q", m.Format, m.Backend)
	}
	listed := map[string]manifestEntry{}
	for _, e := range m.Files {
		if !fs.ValidPath(e.Path) || e.Path == "." || e.Path == ManifestPath || strings.Contains(e.Path, `\`) {
			return nil, bad("path %q is not a canonical relative path", e.Path)
		}
		if _, dup := listed[e.Path]; dup {
			return nil, bad("path %q is listed twice", e.Path)
		}
		if !sha256HexRE.MatchString(e.SHA256) || !modeRE.MatchString(e.Mode) {
			return nil, bad("entry %q has an invalid sha256 or mode", e.Path)
		}
		listed[e.Path] = e
	}
	return listed, nil
}

// parents lists the ancestor directories of a relative path.
func parents(p string) []string {
	var out []string
	for d := path.Dir(p); d != "." && d != "/" && d != ".."; d = path.Dir(d) {
		out = append(out, d)
	}
	return out
}

func writeFile(root *os.Root, f File) error {
	if d := path.Dir(f.Path); d != "." {
		if err := root.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	perm := f.Mode.Perm()
	out, err := root.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := out.Write(f.Data); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return root.Chmod(f.Path, perm)
}
