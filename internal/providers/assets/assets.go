// Package assets resolves repository files referenced by resources. Paths
// are relative to the declaring resource file and must stay inside the
// resource root, so a resource can never pull arbitrary host files into a
// build context.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/deskosproject/deskos-core/internal/model"
)

// Asset is a resolved asset.
type Asset struct {
	// Path is root-relative for display and provenance ("root/dir/file").
	Path   string
	SHA256 string
	Data   []byte
	// File is the absolute host path.
	File string
}

// MaxSize bounds asset size; assets are reviewable repository files.
const MaxSize = 16 << 20

// Read resolves and reads ref relative to the declaring resource file.
func Read(res *model.Resource, ref string) (*Asset, error) {
	if ref == "" {
		return nil, fmt.Errorf("file is required")
	}
	if filepath.IsAbs(ref) || strings.Contains(ref, "\\") {
		return nil, fmt.Errorf("asset path %q must be relative to the resource file", ref)
	}
	root, err := filepath.EvalSymlinks(res.Source.RootDir)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(filepath.Dir(res.Source.File), filepath.FromSlash(ref))
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return nil, fmt.Errorf("asset %q: %w", ref, err)
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("asset %q resolves outside the resource root %s", ref, res.Source.Root)
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("asset %q is not a regular file", ref)
	}
	if info.Size() > MaxSize {
		return nil, fmt.Errorf("asset %q is larger than %d bytes", ref, MaxSize)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return &Asset{
		Path:   path.Join(res.Source.Root, filepath.ToSlash(rel)),
		SHA256: hex.EncodeToString(sum[:]),
		Data:   data,
		File:   real,
	}, nil
}
