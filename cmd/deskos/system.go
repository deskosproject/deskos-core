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

// bootcStatus is the subset of `bootc status --json` that says what is booted.
type bootcStatus struct {
	Status struct {
		Booted struct {
			Image struct {
				Image string `json:"image"`
			} `json:"image"`
			Digest string `json:"imageDigest"`
		} `json:"booted"`
	} `json:"status"`
}

func (s system) Booted() (evidence.Booted, error) {
	if _, err := os.Stat(path.Join(s.root, "/run/ostree-booted")); err != nil {
		return evidence.Booted{}, nil
	}
	out, err := exec.Command("bootc", "status", "--json").Output()
	if err != nil {
		return evidence.Booted{IsBootc: true}, fmt.Errorf("bootc status: %w", err)
	}
	var status bootcStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return evidence.Booted{IsBootc: true}, fmt.Errorf("bootc status: %w", err)
	}
	return evidence.Booted{
		IsBootc: true,
		Image:   status.Status.Booted.Image.Image,
		Digest:  status.Status.Booted.Digest,
	}, nil
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
