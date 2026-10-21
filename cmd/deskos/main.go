// Command deskos is the runtime companion to deskosctl. deskosctl builds an
// artifact: no network, no privileges, deterministic. deskos operates a machine that
// was built from one: it reads the plan the image carries and says what the machine
// was built from, measures the drift it can measure and publishes what it leaves out,
// and updates the image with a guard (docs/adr/0018).
//
// It carries no recipe catalog, writes nothing to a user's home, crosses no layers and
// reconciles nothing: a person asked, so it acts, and it says what it did.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/deskosproject/deskos-core/internal/evidence"
)

const usageText = `deskos: run and inspect a machine built by deskosctl.

  deskos report [--root DIR]
        Say what this machine was built from, reading the plan the image carries. The
        plan is installed; the SBOM is CI evidence and is not, and this report says so.

  deskos check [--root DIR]
        Compare the plan's declared scope (the files it installs and generates, the
        dconf defaults it declares) with this machine, and publish which paths and
        states it checks and which it leaves out.

  deskos update [--apply] [--dry-run]
        Update this machine's image with a guard: it refuses when a condition prevents
        a safe update and reports the rest. It is not a reconciler, and it does not
        touch what a user changed.

  deskos version
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, usageText)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(out, usageText)
		return 0
	case "version":
		fmt.Fprintf(out, "deskos %s (commit %s)\n", version, commit)
		return 0
	case "report":
		return report(args[1:], out, errOut)
	case "check":
		return check(args[1:], out, errOut)
	case "update":
		return update(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "deskos: unknown command %q\n\n%s", args[0], usageText)
		return 2
	}
}

// The release build sets these, the same way it sets deskosctl's, so a machine can
// say which build it carries.
var (
	version = "dev"
	commit  = "unknown"
)

func report(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(errOut)
	root := fs.String("root", "/", "root of the machine to read, for tests and images")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sys := system{root: *root}
	p, err := loadPlan(*root)
	if err != nil {
		fmt.Fprintf(errOut, "deskos report: %v\n", err)
		return 1
	}
	var present *evidence.Booted
	if b, err := sys.Booted(); err == nil {
		present = &b
	} else {
		fmt.Fprintf(errOut, "deskos report: %v\n", err)
	}
	evidence.Report(out, p, present)
	return 0
}

func check(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(errOut)
	root := fs.String("root", "/", "root of the machine to read, for tests and images")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	p, err := loadPlan(*root)
	if err != nil {
		fmt.Fprintf(errOut, "deskos check: %v\n", err)
		return 1
	}
	findings, scope := evidence.Check(system{root: *root}, p)
	evidence.Text(out, findings, scope)
	if _, differ, _ := evidence.Summary(findings); differ > 0 {
		return 1
	}
	return 0
}

func update(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(errOut)
	// No --root here: an update acts on the machine it runs on, and reading another
	// tree's plan while asking the host's bootc to upgrade would mix two machines.
	apply := fs.Bool("apply", false, "apply now, instead of staging until the next reboot")
	dryRun := fs.Bool("dry-run", false, "say what would run, and change nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sys := system{root: "/"}
	p, err := loadPlan("/")
	if err != nil {
		// Without a plan the machine cannot be told what it is: refuse instead of
		// updating something into something else.
		fmt.Fprintf(errOut, "deskos update: %v\n", err)
		return 1
	}
	if !*dryRun && os.Geteuid() != 0 {
		fmt.Fprintln(errOut, "deskos update: needs root to ask bootc to upgrade; run `sudo deskos update`")
		return 1
	}
	conditions := evidence.Guard(sys)
	conditions = append(conditions, evidence.ReportedConditions(p)...)
	fmt.Fprint(out, "Before updating\n")
	for _, c := range conditions {
		mark := "ok"
		if c.Blocks {
			mark = "REFUSED"
		}
		fmt.Fprintf(out, "  [%s] %s: %s\n", mark, c.Name, c.Detail)
	}
	if err := evidence.Update(out, conditions, systemRunner, *apply, *dryRun); err != nil {
		fmt.Fprintf(errOut, "deskos update: %v\n", err)
		return 1
	}
	return 0
}
