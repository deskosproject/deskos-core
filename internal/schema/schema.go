// Package schema validates resource documents against the embedded public
// JSON Schemas. Schemas are resolved only from the embedded set; nothing is
// fetched from the network.
package schema

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/schemas"
)

// Validator holds compiled schemas keyed by their path in the schemas tree.
type Validator struct {
	byPath map[string]*jsonschema.Schema
}

// New compiles every embedded schema.
func New() (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(noNetwork{})
	var paths []string
	err := fs.WalkDir(schemas.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		data, err := fs.ReadFile(schemas.FS, p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("schema %s: %w", p, err)
		}
		if err := c.AddResource(schemas.BaseURI+p, doc); err != nil {
			return fmt.Errorf("schema %s: %w", p, err)
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	v := &Validator{byPath: map[string]*jsonschema.Schema{}}
	for _, p := range paths {
		s, err := c.Compile(schemas.BaseURI + p)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", p, err)
		}
		v.byPath[p] = s
	}
	return v, nil
}

// Paths lists compiled schema paths.
func (v *Validator) Paths() []string {
	var out []string
	for p := range v.byPath {
		out = append(out, p)
	}
	return out
}

// Validate checks a resource document against the schema at path.
func (v *Validator) Validate(res *model.Resource, path string) error {
	s, ok := v.byPath[path]
	if !ok {
		return fmt.Errorf("no schema %s for %s", path, res.GVK())
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(res.Document))
	if err != nil {
		return model.Errorf(res, "%v", err)
	}
	if err := s.Validate(inst); err != nil {
		return model.Errorf(res, "schema validation failed:\n%s", describe(err))
	}
	return nil
}

func describe(err error) string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return "  " + err.Error()
	}
	var lines []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			loc := "/" + strings.Join(e.InstanceLocation, "/")
			lines = append(lines, fmt.Sprintf("  at %s: %s", loc, e.ErrorKind.LocalizedString(printer)))
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return strings.Join(lines, "\n")
}

var printer = message.NewPrinter(language.English)

type noNetwork struct{}

func (noNetwork) Load(url string) (any, error) {
	return nil, fmt.Errorf("schema %s is not embedded; remote schemas are never fetched", url)
}
