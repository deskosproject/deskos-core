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

  deskos etc [diff] [--root DIR] [--ignored]
        Compare /etc with the image's baseline (/usr/etc): what a person edited,
        what the image added, and the runtime paths the policy ignores.

  deskos etc reset PATH... [--root DIR] [--all] [--dry-run] [--include-policy]
        Put a path, a directory or every difference back to the image's version,
        with a timestamped backup. --all also removes local additions.

  deskos status
        Say what this machine is running: the image it follows, the booted
        deployment, anything staged, and the rollback.

  deskos update [--apply] [--dry-run]
        Update this machine's image with a guard: it refuses when a condition prevents
        a safe update and reports the rest. It is not a reconciler, and it does not
        touch what a user changed.

  deskos rollback [--dry-run]
        Ask bootc to stage the previous deployment for the next boot. Nothing
        changes until the machine reboots.

  deskos window --days mon,wed --at 03:00 [--grace 5m] [--timezone TZ]
  deskos window --days mon,wed --start 03:00 --duration 2h [--timezone TZ]
        Exit 0 only when the current local time is inside the requested scheduled
        apply window. Generated units use it as an ExecCondition so a timer that
        fires after suspend cannot become a surprise reboot.

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
	case "etc":
		return etcCommand(args[1:], out, errOut)
	case "status":
		return status(args[1:], out, errOut)
	case "update":
		return update(args[1:], out, errOut)
	case "rollback":
		return rollback(args[1:], out, errOut)
	case "window":
		return window(args[1:], out, errOut)
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
		fmt.Fprintln(errOut, "hint: install a DeskOS image, or pass --root DIR for another tree")
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
		fmt.Fprintln(errOut, "hint: install a DeskOS image, or pass --root DIR for another tree")
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
		fmt.Fprintln(errOut, "hint: update runs on the machine whose image carries the plan; there is no --root")
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

func status(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(errOut)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	h, err := (system{root: "/"}).Host()
	if err != nil {
		fmt.Fprintf(errOut, "deskos status: %v\n", err)
		fmt.Fprintln(errOut, "hint: run `sudo deskos status`; bootc may need root to read the deployment")
		return 1
	}
	fmt.Fprint(out, formatHost(h))
	return 0
}

func formatHost(h hostStatus) string {
	follows := h.Follows
	if follows == "" {
		follows = "unknown"
	}
	queued := "no"
	if h.RollbackQueued {
		queued = "yes"
	}
	return fmt.Sprintf("Follows:  %s\nBooted:   %s\nStaged:   %s\nRollback: %s\nRollback queued: %s\n",
		follows, deploymentLine(h.Booted), deploymentLine(h.Staged), deploymentLine(h.Rollback), queued)
}

func deploymentLine(d *bootcDeployment) string {
	if d == nil {
		return "none"
	}
	line := d.Image.Image.Image
	if d.Image.ImageDigest != "" {
		line += " @ " + d.Image.ImageDigest
	}
	if d.Image.Version != "" {
		line += " (" + d.Image.Version + ")"
	}
	if line == "" {
		return "unknown"
	}
	return line
}

func rollback(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dryRun := fs.Bool("dry-run", false, "say what would run, and change nothing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dryRun {
		fmt.Fprintln(out, "would run: bootc rollback")
		return 0
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(errOut, "deskos rollback: needs root to ask bootc to roll back; run `sudo deskos rollback`")
		return 1
	}
	if err := systemRunner("bootc", "rollback"); err != nil {
		fmt.Fprintf(errOut, "deskos rollback: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "rollback staged; reboot to boot the previous deployment")
	return 0
}
