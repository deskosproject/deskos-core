package main

import (
	"encoding/json"
	"testing"
)

// The shape of `bootc status --json` is not obvious: the image reference is nested
// (`status.booted.image.image.image`) and the digest and version sit beside it under
// `status.booted.image`. Getting it wrong makes the guard refuse every update and the
// report read no deployment, and it does so silently, so the shape is pinned here with
// a sample of the real output (a `bootc status --json` from a working machine).
const bootcStatusSample = `{
  "apiVersion": "org.containers.bootc/v1",
  "kind": "BootcHost",
  "metadata": {"name": "host"},
  "spec": {"image": {"image": "quay.io/deskos/deskos-core-centos10:latest", "transport": "registry"}, "bootOrder": "default"},
  "status": {
    "staged": null,
    "booted": {
      "image": {
        "image": {"image": "quay.io/deskos/deskos-core-centos10:latest", "transport": "registry"},
        "version": "10.20261017",
        "timestamp": "2026-10-17T20:46:00Z",
        "imageDigest": "sha256:39dff9371234"
      },
      "cachedUpdate": null,
      "incompatible": false,
      "pinned": false,
      "store": "ostreeContainer",
      "ostree": {"checksum": "abc", "deploySerial": 1}
    },
    "rollback": null,
    "rollbackQueued": false,
    "type": "bootcHost"
  }
}`

func TestBootcStatusShapeIsTheRealOne(t *testing.T) {
	var status bootcStatus
	if err := json.Unmarshal([]byte(bootcStatusSample), &status); err != nil {
		t.Fatalf("the shape this command reads does not match a real bootc status: %v", err)
	}
	if status.Status.Booted == nil {
		t.Fatal("booted deployment not read")
	}
	if got := status.Status.Booted.Image.Image.Image; got != "quay.io/deskos/deskos-core-centos10:latest" {
		t.Errorf("booted image = %q", got)
	}
	if got := status.Status.Booted.Image.ImageDigest; got != "sha256:39dff9371234" {
		t.Errorf("booted digest = %q", got)
	}
	if got := status.Status.Booted.Image.Version; got != "10.20261017" {
		t.Errorf("booted version = %q", got)
	}
	if got := status.Spec.Image.Image; got != "quay.io/deskos/deskos-core-centos10:latest" {
		t.Errorf("the image the machine follows = %q", got)
	}
	if status.Status.Staged != nil {
		t.Errorf("nothing is staged in this sample, got %+v", status.Status.Staged)
	}
}

func TestBootcStatusReadsAStagedDeployment(t *testing.T) {
	staged := `{"kind":"BootcHost","status":{"booted":{"image":{"image":{"image":"img:1"},"imageDigest":"sha256:aa"}},
	  "staged":{"image":{"image":{"image":"img:2"},"imageDigest":"sha256:bb"}}}}`
	var status bootcStatus
	if err := json.Unmarshal([]byte(staged), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status.Staged == nil || status.Status.Staged.Image.Image.Image != "img:2" ||
		status.Status.Staged.Image.ImageDigest != "sha256:bb" {
		t.Errorf("staged = %+v", status.Status.Staged)
	}
}
