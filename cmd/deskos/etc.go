package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The paths that are runtime state rather than image intent. A file here can be
// different on the machine and in the image without meaning that anyone edited
// it: NetworkManager writes a connection when a person joins a network, sshd
// generates host keys, and machine-id is generated on first boot. The list is
// deliberately short; an organization extends it in /etc/deskos/deskos.json.
var etcDefaultIgnore = []string{
	"deskos/",
	"NetworkManager/system-connections/",
	"machine-id",
	"ssh/ssh_host_*",
	"bluetooth/",
	"cups/",
	"printcap",
	"resolv.conf",
	"mtab",
	"adjtime",
	"systemd/random-seed",
}

// etcPolicy is the optional organization policy. It only adds patterns; the
// defaults are not removable, so a mistake in the file cannot make the tool
// report generated runtime state as an edit.
type etcPolicy struct {
	Ignore []string `json:"ignore"`
}

type etcChange struct {
	Path string
	Kind string // "modified", "added", "removed"
}

func etcCommand(args []string, out, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "diff":
			return etcDiff(args[1:], out, errOut)
		case "reset":
			return etcReset(args[1:], out, errOut)
		}
	}
	return etcDiff(args, out, errOut)
}

func etcDiff(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("etc", flag.ContinueOnError)
	fs.SetOutput(errOut)
	root := fs.String("root", "/", "root of the machine to read, for tests and images")
	showIgnored := fs.Bool("ignored", false, "list the runtime paths the policy ignores")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	changes, ignored, policyDiff, err := etcCompare(*root, false)
	if err != nil {
		fmt.Fprintf(errOut, "deskos etc: %v\n", err)
		return 1
	}
	if len(changes) == 0 {
		fmt.Fprintln(out, "No differences between /etc and the image baseline.")
	} else {
		for _, kind := range []string{"modified", "added", "removed"} {
			var list []string
			for _, c := range changes {
				if c.Kind == kind {
					list = append(list, c.Path)
				}
			}
			if len(list) == 0 {
				continue
			}
			label := map[string]string{"modified": "Modified", "added": "Added", "removed": "Removed"}[kind]
			fmt.Fprintf(out, "%s (%d):\n", label, len(list))
			for _, p := range list {
				fmt.Fprintf(out, "  /etc/%s\n", p)
			}
		}
	}
	fmt.Fprintf(out, "Ignored: %d runtime path(s)\n", len(ignored))
	if *showIgnored {
		for _, p := range ignored {
			fmt.Fprintf(out, "  /etc/%s\n", p)
		}
	}
	if policyDiff {
		fmt.Fprintln(out, "Policy: /etc/deskos/deskos.json differs from the image; rebuild the image, or reset it with --include-policy.")
	}
	return 0
}

func etcReset(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(errOut)
	root := fs.String("root", "/", "root of the machine to write, for tests and images")
	dryRun := fs.Bool("dry-run", false, "say what would change, and change nothing")
	all := fs.Bool("all", false, "reset every difference, including local additions")
	includePolicy := fs.Bool("include-policy", false, "also reset /etc/deskos, ignored by default")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*all && len(fs.Args()) == 0 {
		fmt.Fprintln(errOut, "deskos etc reset: name a path under /etc, or pass --all")
		return 2
	}
	if *root == "/" && os.Geteuid() != 0 && !*dryRun {
		fmt.Fprintln(errOut, "deskos etc reset: needs root to write /etc; run `sudo deskos etc reset ...`")
		return 1
	}

	changes, _, _, err := etcCompare(*root, *includePolicy)
	if err != nil {
		fmt.Fprintf(errOut, "deskos etc reset: %v\n", err)
		return 1
	}
	byPath := map[string]etcChange{}
	if *all {
		for _, c := range changes {
			byPath[c.Path] = c
		}
	} else {
		for _, arg := range fs.Args() {
			rel, err := etcRel(arg)
			if err != nil {
				fmt.Fprintf(errOut, "deskos etc reset: %v\n", err)
				return 2
			}
			found := false
			for _, c := range changes {
				if c.Path == rel || strings.HasPrefix(c.Path, rel+"/") {
					byPath[c.Path] = c
					found = true
				}
			}
			if !found {
				fmt.Fprintf(errOut, "deskos etc reset: no difference under /etc/%s\n", rel)
				return 1
			}
		}
	}
	if len(byPath) == 0 {
		fmt.Fprintln(out, "Nothing to reset.")
		return 0
	}

	cur, base, err := etcRoots(*root)
	if err != nil {
		fmt.Fprintf(errOut, "deskos etc reset: %v\n", err)
		return 1
	}
	backupDir := ""
	if !*dryRun {
		backupDir, err = etcBackupDir(*root)
		if err != nil {
			fmt.Fprintf(errOut, "deskos etc reset: %v\n", err)
			return 1
		}
	}

	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		curPath := filepath.Join(cur, filepath.FromSlash(rel))
		basePath := filepath.Join(base, filepath.FromSlash(rel))
		if *dryRun {
			if _, err := os.Lstat(basePath); err == nil {
				fmt.Fprintf(out, "would reset /etc/%s from the image\n", rel)
			} else {
				fmt.Fprintf(out, "would delete /etc/%s (not in the image)\n", rel)
			}
			continue
		}
		if _, err := os.Lstat(curPath); err == nil {
			if err := etcCopy(curPath, filepath.Join(backupDir, filepath.FromSlash(rel))); err != nil {
				fmt.Fprintf(errOut, "deskos etc reset: backup /etc/%s: %v\n", rel, err)
				return 1
			}
		}
		if err := os.RemoveAll(curPath); err != nil {
			fmt.Fprintf(errOut, "deskos etc reset: remove /etc/%s: %v\n", rel, err)
			return 1
		}
		if _, err := os.Lstat(basePath); err == nil {
			if err := etcCopy(basePath, curPath); err != nil {
				fmt.Fprintf(errOut, "deskos etc reset: restore /etc/%s: %v\n", rel, err)
				return 1
			}
			fmt.Fprintf(out, "reset /etc/%s from the image\n", rel)
		} else {
			fmt.Fprintf(out, "deleted /etc/%s (not in the image)\n", rel)
		}
	}
	if backupDir != "" {
		fmt.Fprintf(out, "Backup: %s\n", backupDir)
	}
	return 0
}

// etcCompare returns the differences, the ignored paths, whether the policy
// file differs, and an error. The policy is always ignored as a subject; its
// own difference is reported separately.
func etcCompare(root string, includePolicy bool) ([]etcChange, []string, bool, error) {
	cur, base, err := etcRoots(root)
	if err != nil {
		return nil, nil, false, err
	}
	patterns, err := etcPatterns(root, includePolicy)
	if err != nil {
		return nil, nil, false, err
	}
	curFiles, curIgnored, err := etcWalk(cur, patterns)
	if err != nil {
		return nil, nil, false, err
	}
	baseFiles, baseIgnored, err := etcWalk(base, patterns)
	if err != nil {
		return nil, nil, false, err
	}
	ignored := map[string]bool{}
	for p := range curIgnored {
		ignored[p] = true
	}
	for p := range baseIgnored {
		ignored[p] = true
	}

	all := map[string]bool{}
	for p := range curFiles {
		all[p] = true
	}
	for p := range baseFiles {
		all[p] = true
	}
	var changes []etcChange
	for p := range all {
		_, inCur := curFiles[p]
		_, inBase := baseFiles[p]
		switch {
		case inCur && !inBase:
			changes = append(changes, etcChange{Path: p, Kind: "added"})
		case !inCur && inBase:
			changes = append(changes, etcChange{Path: p, Kind: "removed"})
		default:
			same, err := etcSame(filepath.Join(cur, filepath.FromSlash(p)), filepath.Join(base, filepath.FromSlash(p)))
			if err != nil {
				return nil, nil, false, err
			}
			if !same {
				changes = append(changes, etcChange{Path: p, Kind: "modified"})
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	list := make([]string, 0, len(ignored))
	for p := range ignored {
		list = append(list, p)
	}
	sort.Strings(list)

	policyDiff, err := etcPolicyDiff(cur, base)
	if err != nil {
		return nil, nil, false, err
	}
	return changes, list, policyDiff, nil
}

func etcRoots(root string) (string, string, error) {
	cur := filepath.Join(root, "etc")
	base := filepath.Join(root, "usr/etc")
	for _, p := range []string{cur, base} {
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			return "", "", fmt.Errorf("%s is not a bootc deployment: %s is missing; only a deployment that keeps a /etc baseline can be compared", root, p)
		}
	}
	return cur, base, nil
}

func etcPatterns(root string, includePolicy bool) ([]string, error) {
	patterns := append([]string(nil), etcDefaultIgnore...)
	data, err := os.ReadFile(filepath.Join(root, "etc/deskos/deskos.json"))
	switch {
	case err == nil:
		var policy etcPolicy
		if err := json.Unmarshal(data, &policy); err != nil {
			return nil, fmt.Errorf("/etc/deskos/deskos.json: %w", err)
		}
		for _, p := range policy.Ignore {
			if strings.TrimSpace(p) == "" {
				return nil, fmt.Errorf("/etc/deskos/deskos.json: an ignore pattern is empty")
			}
			patterns = append(patterns, p)
		}
	case errors.Is(err, fs.ErrNotExist):
	default:
		return nil, fmt.Errorf("/etc/deskos/deskos.json: %w", err)
	}
	if includePolicy {
		filtered := patterns[:0]
		for _, p := range patterns {
			if p != "deskos/" {
				filtered = append(filtered, p)
			}
		}
		patterns = filtered
	}
	return patterns, nil
}

func etcIgnored(rel string, patterns []string) bool {
	for _, raw := range patterns {
		p := strings.TrimPrefix(raw, "/")
		if strings.HasSuffix(p, "/") {
			p = strings.TrimSuffix(p, "/")
			if rel == p || strings.HasPrefix(rel, p+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(p, rel); ok {
			return true
		}
	}
	return false
}

func etcWalk(root string, patterns []string) (map[string]bool, map[string]bool, error) {
	files := map[string]bool{}
	ignored := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if etcIgnored(rel, patterns) {
			ignored[rel] = true
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			files[rel] = true
		}
		return nil
	})
	return files, ignored, err
}

func etcSame(cur, base string) (bool, error) {
	a, err := os.Lstat(cur)
	if err != nil {
		return false, err
	}
	b, err := os.Lstat(base)
	if err != nil {
		return false, err
	}
	if a.Mode().Type() != b.Mode().Type() {
		return false, nil
	}
	switch {
	case a.Mode()&os.ModeSymlink != 0:
		ta, err := os.Readlink(cur)
		if err != nil {
			return false, err
		}
		tb, err := os.Readlink(base)
		if err != nil {
			return false, err
		}
		return ta == tb, nil
	case a.Mode().IsRegular():
		if a.Mode().Perm() != b.Mode().Perm() {
			return false, nil
		}
		ha, err := etcHash(cur)
		if err != nil {
			return false, err
		}
		hb, err := etcHash(base)
		if err != nil {
			return false, err
		}
		return ha == hb, nil
	default:
		return true, nil
	}
}

func etcHash(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func etcPolicyDiff(cur, base string) (bool, error) {
	c := filepath.Join(cur, "deskos/deskos.json")
	b := filepath.Join(base, "deskos/deskos.json")
	if _, err := os.Lstat(c); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, err := os.Lstat(b); errors.Is(err, fs.ErrNotExist) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	same, err := etcSame(c, b)
	return !same, err
}

func etcRel(arg string) (string, error) {
	p := strings.TrimPrefix(filepath.ToSlash(arg), "/")
	p = strings.TrimPrefix(p, "etc/")
	p = path.Clean(p)
	if p == "" || p == "." || p == ".." || p == "etc" || strings.HasPrefix(p, "../") {
		return "", fmt.Errorf("%q is not a path under /etc", arg)
	}
	return p, nil
}

func etcCopy(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := etcCopy(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(dst, info.Mode().Perm())
	default:
		return fmt.Errorf("%s: unsupported file type", src)
	}
}

func etcBackupDir(root string) (string, error) {
	base := filepath.Join(root, "var/lib/deskos/etc-backup")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(base, stamp)
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return dir, nil
	}
	for i := 1; ; i++ {
		dir = filepath.Join(base, fmt.Sprintf("%s-%d", stamp, i))
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			return dir, nil
		}
	}
}
