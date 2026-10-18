package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/deskosproject/deskos-core/internal/evidence"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// The machine's access lives here, in the command, not in the logic it runs: nothing
// under internal/ executes a command (there is a structural test for it), so the
// package that decides what to compare stays testable without a machine.

// system reads the machine this binary runs on.
type system struct{ root string }

func (s system) ReadFile(p string) ([]byte, error) { return os.ReadFile(path.Join(s.root, p)) }

// bootcStatus is the subset of `bootc status --json` this command reads, in the shape
// the tool really emits (verified against a `bootc status --json` sample: the image
// reference is nested, `status.booted.image.image.image`, and the digest is
// `status.booted.image.imageDigest`):
//
//	{"spec":{"image":{"image":"<ref>","transport":"registry"}},
//	 "status":{"booted":{"image":{"image":{"image":"<ref>"},"version":"…","imageDigest":"sha256:…"}},
//	           "staged":null|{"image":{…}}}}
type bootcStatus struct {
	Kind   string `json:"kind"`
	Spec   bootcSpec
	Status struct {
		Booted *bootcDeployment `json:"booted"`
		Staged *bootcDeployment `json:"staged"`
	} `json:"status"`
}

type bootcSpec struct {
	Image struct {
		Image string `json:"image"`
	} `json:"image"`
}

type bootcDeployment struct {
	Image struct {
		Image struct {
			Image string `json:"image"`
		} `json:"image"`
		Version     string `json:"version"`
		ImageDigest string `json:"imageDigest"`
	} `json:"image"`
}

func (s system) Booted() (evidence.Booted, error) {
	// The marker ostree-based systems create at boot: without it this machine did not
	// boot from a deployment at all, and there is nothing for bootc to read.
	if _, err := os.Stat(path.Join(s.root, "/run/ostree-booted")); err != nil {
		return evidence.Booted{}, nil
	}
	out, err := exec.Command("bootc", "status", "--json").Output()
	if err != nil {
		return evidence.Booted{IsBootc: true}, fmt.Errorf("bootc status: %w", err)
	}
	var status bootcStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return evidence.Booted{IsBootc: true}, fmt.Errorf("bootc status: unexpected shape: %w", err)
	}
	b := evidence.Booted{
		IsBootc: status.Kind == "BootcHost" || true,
		Follows: status.Spec.Image.Image,
	}
	if d := status.Status.Booted; d != nil {
		b.Image = d.Image.Image.Image
		b.Digest = d.Image.ImageDigest
		b.Version = d.Image.Version
	}
	if d := status.Status.Staged; d != nil {
		b.StagedImage = d.Image.Image.Image
		b.StagedDigest = d.Image.ImageDigest
	}
	return b, nil
}

func (s system) DconfRead(key string) (string, error) {
	out, err := exec.Command("dconf", "read", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// systemRunner runs the update command for real.
func systemRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// loadPlan reads the plan the image carries, under root.
func loadPlan(root string) (*plan.Plan, error) {
	data, err := os.ReadFile(path.Join(root, evidence.PlanPath))
	if err != nil {
		return nil, fmt.Errorf("this machine does not carry a plan (%s): %w", evidence.PlanPath, err)
	}
	var p plan.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s is not a readable plan: %w", evidence.PlanPath, err)
	}
	return &p, nil
}
