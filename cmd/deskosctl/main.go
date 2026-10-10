// Command deskosctl validates, plans and renders DeskOS workstation
// definitions. It is the artifact-factory CLI; it never builds, pushes or
// touches running endpoints.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/compiler"
	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/corefs"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/sbom"
	"github.com/deskosproject/deskos-core/internal/textplan"
)

const usage = `deskosctl compiles DeskOS workstation definitions into build contexts.

Usage:
  deskosctl validate [--no-core] [ROOT...]
  deskosctl plan [--no-core] ROOT... --workstation NAME [--format text|json]
  deskosctl render [--no-core] ROOT... --workstation NAME --backend containerfile --output DIR
  deskosctl sbom [--no-core] ROOT... --workstation NAME [--output FILE]
  deskosctl core export DIR
  deskosctl version

DeskOS Core is embedded in this binary and loaded by default; ROOT... are an
organization's own resources, layered on top. A ROOT is a directory (searched
recursively for *.yaml and *.yml) or a single resource file. --no-core drops
the embedded Core, to compile against a Core tree you pass yourself.
`

// Exit codes.
const (
	exitOK      = 0
	exitInvalid = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		if len(args) == 0 {
			return exitUsage
		}
		return exitOK
	}
	var err error
	switch args[0] {
	case "validate":
		err = cmdValidate(args[1:], stdout, stderr)
	case "plan":
		err = cmdPlan(args[1:], stdout)
	case "render":
		err = cmdRender(args[1:], stdout)
	case "sbom":
		err = cmdSBOM(args[1:], stdout)
	case "core":
		err = cmdCore(args[1:], stdout)
	case "version":
		if len(args) > 1 {
			err = usageError{"version takes no arguments"}
			break
		}
		fmt.Fprint(stdout, versionString())
	default:
		err = usageError{fmt.Sprintf("unknown command %q", args[0])}
	}
	var ue usageError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &ue):
		fmt.Fprintf(stderr, "deskosctl: %s\n\n%s", ue.msg, usage)
		return exitUsage
	default:
		reportError(stderr, err)
		return exitInvalid
	}
}

func reportError(w io.Writer, err error) {
	var list model.ErrorList
	if errors.As(err, &list) {
		msgs := list.Messages()
		for _, m := range msgs {
			fmt.Fprintf(w, "error: %s\n\n", indentRest(m))
		}
		fmt.Fprintf(w, "%d error(s)\n", len(msgs))
		return
	}
	fmt.Fprintf(w, "error: %s\n", indentRest(err.Error()))
}

func indentRest(s string) string { return strings.ReplaceAll(s, "\n", "\n       ") }

// parse accepts flags before, between or after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err.Error()}
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// load composes the resource roots, prepending the embedded DeskOS Core
// unless noCore is set. The returned cleanup removes the materialized Core.
func load(roots []string, noCore bool) (*compiler.Compiler, *compose.Catalog, func(), error) {
	all := roots
	cleanup := func() {}
	if !noCore {
		tmp, err := os.MkdirTemp("", "deskos-core-")
		if err != nil {
			return nil, nil, cleanup, err
		}
		cleanup = func() { os.RemoveAll(tmp) }
		core, err := corefs.Materialize(tmp)
		if err != nil {
			cleanup()
			return nil, nil, func() {}, err
		}
		all = append([]string{core}, roots...)
	}
	if len(all) == 0 {
		return nil, nil, cleanup, usageError{"at least one resource root is required"}
	}
	c, err := compiler.New()
	if err != nil {
		cleanup()
		return nil, nil, func() {}, err
	}
	cat, err := c.Load(all)
	if err != nil {
		cleanup()
		return nil, nil, func() {}, err
	}
	return c, cat, cleanup, nil
}

// cmdCore writes the embedded Core so it can be inspected or pinned.
func cmdCore(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "export" {
		return usageError{"usage: deskosctl core export DIR"}
	}
	if len(args) != 2 {
		return usageError{"core export takes exactly one output directory"}
	}
	root, err := corefs.Materialize(args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote the embedded DeskOS Core to %s\n", root)
	return nil
}

func cmdValidate(args []string, stdout, stderr io.Writer) error {
	fs := newFlags("validate")
	noCore := fs.Bool("no-core", false, "do not load the embedded DeskOS Core")
	roots, err := parse(fs, args)
	if err != nil {
		return err
	}
	c, cat, cleanup, err := load(roots, *noCore)
	defer cleanup()
	var errs model.ErrorList
	errs.Add(err)
	if cat == nil {
		return errs.Err()
	}
	names := compiler.Workstations(cat)
	var warnings []string
	for _, ws := range names {
		p, err := c.Plan(cat, ws)
		if err != nil {
			errs.Add(fmt.Errorf("Workstation/%s: %w", ws, err))
			continue
		}
		for _, w := range p.Warnings {
			warnings = append(warnings, fmt.Sprintf("warning: Workstation/%s: %s", ws, w))
		}
	}
	if err := errs.Err(); err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(stderr, w)
	}
	fmt.Fprintf(stdout, "ok: %d workstation(s) compose cleanly", len(names))
	if len(names) > 0 {
		fmt.Fprintf(stdout, ": %s", strings.Join(names, ", "))
	}
	fmt.Fprintln(stdout)
	return nil
}

func cmdPlan(args []string, stdout io.Writer) error {
	fs := newFlags("plan")
	ws := fs.String("workstation", "", "workstation to plan")
	format := fs.String("format", "text", "output format: text or json")
	noCore := fs.Bool("no-core", false, "do not load the embedded DeskOS Core")
	roots, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *ws == "" {
		return usageError{"--workstation is required"}
	}
	if *format != "text" && *format != "json" {
		return usageError{fmt.Sprintf("unknown format %q", *format)}
	}
	c, cat, cleanup, err := load(roots, *noCore)
	defer cleanup()
	if err != nil {
		return err
	}
	p, err := c.Plan(cat, *ws)
	if err != nil {
		return err
	}
	if *format == "json" {
		b, err := p.JSON()
		if err != nil {
			return err
		}
		_, err = stdout.Write(b)
		return err
	}
	return textplan.Write(stdout, p)
}

func cmdRender(args []string, stdout io.Writer) error {
	fs := newFlags("render")
	ws := fs.String("workstation", "", "workstation to render")
	backend := fs.String("backend", containerfile.Name, "artifact backend")
	output := fs.String("output", "", "output directory for the build context")
	noCore := fs.Bool("no-core", false, "do not load the embedded DeskOS Core")
	roots, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *ws == "" || *output == "" {
		return usageError{"--workstation and --output are required"}
	}
	if *backend != containerfile.Name {
		return usageError{fmt.Sprintf("unknown backend %q (available: %s)", *backend, containerfile.Name)}
	}
	c, cat, cleanup, err := load(roots, *noCore)
	defer cleanup()
	if err != nil {
		return err
	}
	p, err := c.Plan(cat, *ws)
	if err != nil {
		return err
	}
	files, err := containerfile.Render(p)
	if err != nil {
		return err
	}
	if err := containerfile.WriteDir(*output, files); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rendered %s (%d files) to %s\n", p.Workstation.Name, len(files), *output)
	return nil
}

// cmdSBOM writes the declared CycloneDX SBOM of one workstation: the
// inputs the Plan selects, not the contents of a built image.
func cmdSBOM(args []string, stdout io.Writer) error {
	fs := newFlags("sbom")
	ws := fs.String("workstation", "", "workstation to build the SBOM for")
	output := fs.String("output", "", "write the SBOM to FILE instead of stdout")
	noCore := fs.Bool("no-core", false, "do not load the embedded DeskOS Core")
	roots, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *ws == "" {
		return usageError{"--workstation is required"}
	}
	c, cat, cleanup, err := load(roots, *noCore)
	defer cleanup()
	if err != nil {
		return err
	}
	p, err := c.Plan(cat, *ws)
	if err != nil {
		return err
	}
	b, err := sbom.Build(p)
	if err != nil {
		return err
	}
	if *output == "" {
		_, err = stdout.Write(b)
		return err
	}
	if err := os.WriteFile(*output, b, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote the declared SBOM of %s to %s\n", p.Workstation.Name, *output)
	return nil
}
