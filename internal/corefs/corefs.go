// Package corefs materializes DeskOS Core, embedded in the deskosctl binary,
// into a directory the resource loader can read. The loader works with real
// files and paths relative to a root, so the tree is written once per run.
package corefs

import (
	"io/fs"
	"os"
	"path/filepath"

	deskos "github.com/deskosproject/deskos-core"
)

// Root is the directory in the embedded tree that a resource root points at.
const Root = "resources"

// Materialize writes the embedded Core under dir and returns the resource
// root path (dir/resources). The caller owns dir and removes it afterwards.
func Materialize(dir string) (string, error) {
	err := fs.WalkDir(deskos.Resources, Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := deskos.Resources.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, Root), nil
}
