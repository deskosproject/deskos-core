// Package loader reads DeskOS resource documents from one or more resource
// roots. It performs YAML parsing and envelope checks only; it does not know
// any resource kind.
package loader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/deskosproject/deskos-core/internal/model"
)

// Load reads every *.yaml and *.yml file under each root. A root may be a
// directory (walked recursively, hidden entries skipped) or a single file.
// Resources are returned in a deterministic order independent of root order.
func Load(roots []string) ([]*model.Resource, error) {
	if len(roots) == 0 {
		return nil, errors.New("no resource roots given")
	}
	var errs model.ErrorList
	var out []*model.Resource
	for _, root := range roots {
		res, err := loadRoot(root)
		errs.Add(err)
		out = append(out, res...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Source, out[j].Source
		if a.Root != b.Root {
			return a.Root < b.Root
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	return out, errs.Err()
}

func loadRoot(root string) ([]*model.Resource, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("resource root %s: %w", root, err)
	}
	rootDir := abs
	var files []string
	if info.IsDir() {
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p != abs && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && isYAML(p) {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("resource root %s: %w", root, err)
		}
	} else {
		rootDir = filepath.Dir(abs)
		files = []string{abs}
	}
	sort.Strings(files)
	var errs model.ErrorList
	var out []*model.Resource
	for _, f := range files {
		rel, err := filepath.Rel(rootDir, f)
		if err != nil {
			return nil, err
		}
		src := model.Source{
			Root:    filepath.Base(rootDir),
			Path:    filepath.ToSlash(rel),
			RootDir: rootDir,
			File:    f,
		}
		if info.IsDir() {
			if err := insideRoot(rootDir, f); err != nil {
				errs.Add(fmt.Errorf("%s: %w", src, err))
				continue
			}
		}
		data, err := os.ReadFile(f)
		if err != nil {
			errs.Add(fmt.Errorf("%s: %w", src, err))
			continue
		}
		res, err := Parse(data, src)
		errs.Add(err)
		out = append(out, res...)
	}
	return out, errs.Err()
}

// insideRoot rejects a discovered file whose symlinks resolve outside the
// root, matching the confinement applied to assets.
func insideRoot(rootDir, file string) error {
	root, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("resource file resolves outside the resource root")
	}
	return nil
}

func isYAML(p string) bool {
	ext := filepath.Ext(p)
	return ext == ".yaml" || ext == ".yml"
}

// Parse decodes a possibly multi-document YAML stream. Empty documents are
// skipped. src.Line is filled per document.
func Parse(data []byte, src model.Source) ([]*model.Resource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var errs model.ErrorList
	var out []*model.Resource
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			errs.Add(fmt.Errorf("%s: malformed YAML: %v", src, err))
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		node := doc.Content[0]
		if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
			continue
		}
		s := src
		s.Line = node.Line
		res, err := parseDocument(node, s)
		if err != nil {
			errs.Add(err)
			continue
		}
		out = append(out, res)
	}
	return out, errs.Err()
}

func parseDocument(node *yaml.Node, src model.Source) (*model.Resource, error) {
	value, err := toJSONValue(node, src)
	if err != nil {
		return nil, err
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: resource document must be a mapping", src)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", src, err)
	}
	res := &model.Resource{Document: raw, Source: src}

	apiVersion, _ := doc["apiVersion"].(string)
	kind, _ := doc["kind"].(string)
	if apiVersion == "" {
		return nil, fmt.Errorf("%s: missing or non-string apiVersion", src)
	}
	if _, _, err := model.ParseAPIVersion(apiVersion); err != nil {
		return nil, fmt.Errorf("%s: %v", src, err)
	}
	if kind == "" {
		return nil, fmt.Errorf("%s: missing or non-string kind", src)
	}
	res.APIVersion, res.Kind = apiVersion, kind

	meta, ok := doc["metadata"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: %s: missing metadata", src, kind)
	}
	name, _ := meta["name"].(string)
	if !model.ValidName(name) {
		return nil, fmt.Errorf("%s: %s: metadata.name %q must be a lowercase DNS label (a-z, 0-9, '-')", src, kind, name)
	}
	res.Metadata.Name = name

	if spec, ok := doc["spec"]; ok {
		b, err := json.Marshal(spec)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", src, err)
		}
		res.Spec = b
		res.SpecLine = specLine(node)
	}
	return res, nil
}

func specLine(n *yaml.Node) int {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "spec" {
			return n.Content[i+1].Line
		}
	}
	return 0
}

// toJSONValue converts a YAML node into plain JSON-compatible values. Anchors,
// aliases, merge keys and custom tags are rejected: resources are plain data.
func toJSONValue(n *yaml.Node, src model.Source) (any, error) {
	at := func(format string, a ...any) error {
		return fmt.Errorf("%s:%d: %s", src.Root+"/"+src.Path, n.Line, fmt.Sprintf(format, a...))
	}
	if n.Anchor != "" {
		return nil, at("YAML anchors are not supported; resources are plain data")
	}
	switch n.Kind {
	case yaml.AliasNode:
		return nil, at("YAML aliases are not supported; resources are plain data")
	case yaml.MappingNode:
		if n.Tag != "!!map" {
			return nil, at("unsupported YAML tag %s", n.Tag)
		}
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				if k.Tag == "!!merge" {
					return nil, at("YAML merge keys are not supported; resources are plain data")
				}
				return nil, at("mapping keys must be strings")
			}
			if _, dup := m[k.Value]; dup {
				return nil, at("duplicate key %q", k.Value)
			}
			v, err := toJSONValue(n.Content[i+1], src)
			if err != nil {
				return nil, err
			}
			m[k.Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		if n.Tag != "!!seq" {
			return nil, at("unsupported YAML tag %s", n.Tag)
		}
		s := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := toJSONValue(c, src)
			if err != nil {
				return nil, err
			}
			s = append(s, v)
		}
		return s, nil
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!str", "!!int", "!!float", "!!bool", "!!null":
		default:
			return nil, at("unsupported YAML tag %s", n.Tag)
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return nil, at("%v", err)
		}
		return v, nil
	}
	return nil, at("unsupported YAML node")
}
