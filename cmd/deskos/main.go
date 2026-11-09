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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

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

  deskos update [--apply] [--dry-run] [--flatpak] [--firmware] [--all] [--verbose]
        Update this machine's image with a guard: it refuses when a condition prevents
        a safe update and reports the rest. It prints what the machine booted, what is
        already staged, and one line saying whether anything changed and what to do
        next; --verbose adds every condition and the tool's own output. It is not a
        reconciler, and it does not touch what a user changed. --flatpak updates
        system Flatpak applications before the image; --firmware lists firmware
        updates without applying them; --all does the image, the Flatpak applications
        and the firmware check.

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
	flatpak := fs.Bool("flatpak", false, "update system Flatpak applications before the image")
	firmware := fs.Bool("firmware", false, "add the firmware update check; DeskOS does not apply firmware")
	all := fs.Bool("all", false, "the image, the Flatpak applications and the firmware check")
	verbose := fs.Bool("verbose", false, "print every condition and the tool's own output")
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
	// A refusal stops everything, including the extras: updating Flatpaks on a
	// machine whose image cannot be updated is work in the wrong order.
	if blocked := evidence.Blocking(conditions); len(blocked) > 0 {
		fmt.Fprint(out, "Not updating\n")
		for _, c := range blocked {
			fmt.Fprintf(out, "  [refused] %s: %s\n", c.Name, c.Detail)
		}
		return 1
	}
	if *verbose {
		fmt.Fprint(out, "Before updating\n")
		for _, c := range conditions {
			fmt.Fprintf(out, "  [ok] %s: %s\n", c.Name, c.Detail)
		}
	}

	// What the machine is now, in three lines: what it booted, whether something is
	// already waiting, and whether the image updates itself. The first two are read
	// from bootc; the third from the plan.
	before, err := sys.Host()
	if err != nil {
		fmt.Fprintf(errOut, "deskos update: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "  booted   %s\n", deploymentLine(before.Booted))
	fmt.Fprintf(out, "  staged   %s\n", stagedLine(before.Staged))
	if kinds := evidence.UpdatingKinds(p); len(kinds) > 0 {
		fmt.Fprintf(out, "  mode     the image also updates itself (%s); a manual update may find nothing to do\n",
			strings.Join(kinds, ", "))
	}

	// Firmware is not another package upgrade: DeskOS never stages or flashes
	// it. The warnings describe what would make a later manual installation
	// fail, but they do not skip the listing: get-updates does not need a
	// working loader, and the image update may be what carries the fix.
	doFlatpak := *flatpak || *all
	doFirmware := *firmware || *all
	if doFirmware {
		printFirmwareWarnings(out, firmwareWarnings(firmwareRunner, !*dryRun))
	}

	// Extras run before the image: with --apply the image command reboots, and a
	// Flatpak update queued after it would never run.
	firmwareFailed, err := runUpdateExtras(out, errOut, systemRunner, doFlatpak, doFirmware, *dryRun)
	if err != nil {
		fmt.Fprintf(errOut, "deskos update: %v\n", err)
		return 1
	}
	if *dryRun {
		name, args := evidence.UpdateCommand(*apply)
		fmt.Fprintf(out, "  would run: %s %s\n", name, strings.Join(args, " "))
		return 0
	}

	fmt.Fprint(out, "  checking bootc upgrade\n")
	prose, err := evidence.Update(conditions, systemRunnerOutput, *apply)
	if err != nil {
		if len(prose) > 0 {
			fmt.Fprintf(errOut, "%s", prose)
		}
		fmt.Fprintf(errOut, "deskos update: %v\n", err)
		return 1
	}
	if *verbose && len(prose) > 0 {
		fmt.Fprint(out, string(prose))
	}
	after, err := sys.Host()
	if err != nil {
		// The update already succeeded; the summary is what the machine said before.
		after = before
	}
	fmt.Fprint(out, resultLine(before, after, *apply))
	if firmwareFailed {
		return 1
	}
	return 0
}

// stagedLine says whether a deployment is already waiting for a reboot. A staged
// deployment is not a change this command made: it may have been there for days.
func stagedLine(staged *bootcDeployment) string {
	if staged == nil {
		return "nothing"
	}
	return shortDeployment(staged) + ", waiting for a reboot"
}

// shortDeployment is a digest and a version: the reference is already on the booted
// line, and a person comparing two deployments reads the digest.
func shortDeployment(d *bootcDeployment) string {
	if d == nil {
		return "none"
	}
	digest := d.Image.ImageDigest
	if strings.HasPrefix(digest, "sha256:") && len(digest) > 19 {
		digest = digest[:19]
	}
	line := d.Image.Version
	if digest != "" {
		if line != "" {
			line += " "
		}
		line += digest
	}
	if line == "" {
		line = d.Image.Image.Image
	}
	return line
}

// resultLine is the one thing a person asked for: did anything change, and what do I
// do now. It never repeats the tool's prose, which is what made the old output
// contradict itself.
func resultLine(before, after hostStatus, apply bool) string {
	if apply {
		return "Applied. The machine reboots to run the new deployment.\n"
	}
	changed := digestOf(after.Staged) != digestOf(before.Staged)
	if changed && after.Staged != nil {
		return fmt.Sprintf("Staged %s. Reboot to apply it, or run `deskos update --apply` now.\n",
			shortDeployment(after.Staged))
	}
	if after.Staged != nil && digestOf(after.Staged) != digestOf(after.Booted) {
		return fmt.Sprintf("Nothing to do: %s is already staged; reboot to apply it.\n",
			shortDeployment(after.Staged))
	}
	return "Nothing to do: this machine already runs the newest image.\n"
}

func digestOf(d *bootcDeployment) string {
	if d == nil {
		return ""
	}
	return d.Image.ImageDigest
}

// updateExtras is the on-demand side of the same work the image's timers do.
// It is not a new policy: an administrator asked for it on this machine.
func updateExtras(flatpak, firmware bool) [][]string {
	var cmds [][]string
	if flatpak {
		cmds = append(cmds, []string{"flatpak", "update", "--system", "--noninteractive", "--assumeyes"})
	}
	if firmware {
		// get-updates only: DeskOS never stages or flashes firmware. The command
		// runs without a TTY, so fwupd uses its metadata cache and skips the
		// interactive metadata and report checks; --no-unreported-check makes
		// the no-upload intent explicit.
		cmds = append(cmds, []string{"fwupdmgr", "get-updates", "--no-unreported-check"})
	}
	return cmds
}

func runExtras(out io.Writer, runner func(string, ...string) error, cmds [][]string, dryRun bool) error {
	for _, cmd := range cmds {
		line := strings.Join(cmd, " ")
		if dryRun {
			fmt.Fprintf(out, "would run: %s\n", line)
			continue
		}
		if err := runner(cmd[0], cmd[1:]...); err != nil {
			// fwupdmgr exits 2 when there are no updates; that is a successful check,
			// not a failure to update.
			var exit *exec.ExitError
			if cmd[0] == "fwupdmgr" && errors.As(err, &exit) && exit.ExitCode() == 2 {
				fmt.Fprintln(out, "No firmware updates available.")
				continue
			}
			return fmt.Errorf("%s: %w", line, err)
		}
	}
	return nil
}

// runUpdateExtras runs Flatpak first, then the firmware listing. A Flatpak
// failure stops the sequence, because the image should not be updated on a
// half-updated system. A firmware failure is reported and remembered instead:
// the image is where a fixed fwupd-efi arrives, so it still runs.
func runUpdateExtras(out, errOut io.Writer, runner func(string, ...string) error, flatpak, firmware, dryRun bool) (firmwareFailed bool, err error) {
	if flatpak {
		if err := runExtras(out, runner, updateExtras(true, false), dryRun); err != nil {
			return false, err
		}
	}
	if firmware {
		if err := runExtras(out, runner, updateExtras(false, true), dryRun); err != nil {
			fmt.Fprintf(errOut, "deskos update: %v; continuing with the image\n", err)
			return true, nil
		}
	}
	return false, nil
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
