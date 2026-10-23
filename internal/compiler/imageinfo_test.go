package compiler_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
)

func TestImageInfoIsGeneratedFromThePlan(t *testing.T) {
	p := mustPlan(t, "deskos-core-centos-stream-10", resourcesRoot)
	var content string
	for _, f := range p.Artifact.GeneratedFiles {
		if f.Path == plan.ImageInfoPath {
			content = f.Content
			if f.Mode != "0644" {
				t.Errorf("image-info mode = %q, want 0644", f.Mode)
			}
		}
	}
	if content == "" {
		t.Fatalf("%s was not generated", plan.ImageInfoPath)
	}
	var info struct {
		Schema          string `json:"schema"`
		ImageName       string `json:"image-name"`
		ImageTitle      string `json:"image-title"`
		Platform        string `json:"platform"`
		PlatformFamily  string `json:"platform-family"`
		PlatformRelease string `json:"platform-release"`
		Architectures   []string
		BaseImage       string `json:"base-image"`
		PlanFormat      string `json:"plan-format"`
		Profiles        []struct {
			Name  string `json:"name"`
			Layer string `json:"layer"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(content), &info); err != nil {
		t.Fatalf("image-info is not JSON: %v\n%s", err, content)
	}
	if info.Schema != "deskos.image-info/v1" || info.ImageName != p.Workstation.Name ||
		info.Platform != p.Platform.Name || info.BaseImage != p.Artifact.BaseImage.Ref ||
		info.PlanFormat != p.Format {
		t.Errorf("image-info does not describe the plan: %+v", info)
	}
	if len(info.Profiles) == 0 || info.Profiles[0].Layer > info.Profiles[len(info.Profiles)-1].Layer {
		t.Errorf("profiles are not sorted by layer: %+v", info.Profiles)
	}
	if strings.Contains(content, p.Workstation.Source) || strings.Contains(content, p.Platform.Source) {
		t.Errorf("image-info leaks a build-host source path:\n%s", content)
	}

	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range files {
		if f.Path == "rootfs"+plan.ImageInfoPath {
			found = true
			if string(f.Data) != content {
				t.Error("rendered image-info differs from the plan")
			}
		}
	}
	if !found {
		t.Errorf("rendered context lacks rootfs%s", plan.ImageInfoPath)
	}
}
