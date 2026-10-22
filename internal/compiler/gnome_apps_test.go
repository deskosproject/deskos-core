package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/plan"
)

func generatedFile(p *plan.Plan, path string) (plan.GeneratedFile, bool) {
	for _, f := range p.Artifact.GeneratedFiles {
		if f.Path == path {
			return f, true
		}
	}
	return plan.GeneratedFile{}, false
}

func TestGnomeApplicationsValidation(t *testing.T) {
	load := func(defaults string) error {
		_, err := newCompiler(t).Load([]string{fixture{"g.yaml": gnome("g", defaults)}.dir(t)})
		return err
	}
	valid := "    applications:\n" +
		"      browser: google-chrome.desktop\n" +
		"      fileManager: org.gnome.Nautilus.desktop\n" +
		"      textEditor: org.gnome.TextEditor.desktop\n" +
		"      imageViewer: org.gnome.Loupe.desktop\n" +
		"      videoPlayer: org.gnome.Totem.desktop\n" +
		"      pdfViewer: org.gnome.Evince.desktop\n"
	if err := load(valid); err != nil {
		t.Fatalf("valid applications rejected: %v", err)
	}
	for name, defaults := range map[string]string{
		"browser is not an id": "    applications:\n      browser: chrome\n",
		"unknown role":         "    applications:\n      mailClient: thunderbird.desktop\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(defaults), "schema validation failed")
		})
	}
}

// The typed decoder enforces the same rules as the schema on its own.
func TestGnomeApplicationsTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct{ spec, want string }{
		"browser": {
			`{"defaults":{"applications":{"browser":"chrome"}}}`,
			`applications.browser: "chrome" is not a desktop file id`,
		},
		"unknown role": {
			`{"defaults":{"applications":{"mailClient":"thunderbird.desktop"}}}`,
			"unknown field",
		},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource("desktop.deskos.org/v1alpha1", "GnomeProfile", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestGnomeDefaultApplicationsLowering(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml": gnome("g", "    applications:\n"+
			"      browser: google-chrome.desktop\n"+
			"      fileManager: org.gnome.Nautilus.desktop\n"+
			"      textEditor: org.gnome.TextEditor.desktop\n"+
			"      imageViewer: org.gnome.Loupe.desktop\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	f, ok := generatedFile(p, "/etc/xdg/mimeapps.list")
	if !ok {
		t.Fatal("mimeapps.list was not generated")
	}
	if f.Mode != "0644" {
		t.Errorf("mimeapps.list mode = %q, want 0644", f.Mode)
	}
	for _, want := range []string{
		"[Default Applications]",
		"image/gif=org.gnome.Loupe.desktop;",
		"image/jpeg=org.gnome.Loupe.desktop;",
		"image/png=org.gnome.Loupe.desktop;",
		"image/webp=org.gnome.Loupe.desktop;",
		"inode/directory=org.gnome.Nautilus.desktop;",
		"text/html=google-chrome.desktop;",
		"text/plain=org.gnome.TextEditor.desktop;",
		"x-scheme-handler/http=google-chrome.desktop;",
		"x-scheme-handler/https=google-chrome.desktop;",
	} {
		if !strings.Contains(f.Content, want+"\n") {
			t.Errorf("mimeapps.list lacks %q:\n%s", want, f.Content)
		}
	}
	if strings.Contains(f.Content, "video/") || strings.Contains(f.Content, "application/pdf") {
		t.Errorf("mimeapps.list contains a role that was not declared:\n%s", f.Content)
	}
}

func TestGnomeDefaultApplicationsComposition(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core", "org"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"org.yaml":  profile("org", "organization", "GnomeProfile/org"),
		"g1.yaml":   gnome("core", "    applications:\n      browser: firefox.desktop\n"),
		"g2.yaml":   gnome("org", "    applications:\n      browser: google-chrome.desktop\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	f, ok := generatedFile(p, "/etc/xdg/mimeapps.list")
	if !ok {
		t.Fatal("mimeapps.list was not generated")
	}
	if !strings.Contains(f.Content, "text/html=google-chrome.desktop;") {
		t.Errorf("higher layer did not win:\n%s", f.Content)
	}
	if strings.Contains(f.Content, "firefox.desktop") {
		t.Errorf("lower layer survived the override:\n%s", f.Content)
	}

	conflict := base.with(fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "GnomeProfile/one"),
		"b.yaml":  profile("b", "organization", "GnomeProfile/two"),
		"g1.yaml": gnome("one", "    applications:\n      browser: firefox.desktop\n"),
		"g2.yaml": gnome("two", "    applications:\n      browser: google-chrome.desktop\n"),
	}).dir(t)
	_, err := planOf(t, "ws", conflict)
	mustFail(t, err, `composition conflict for GNOME setting "applications.browser"`,
		"organization GnomeProfile/one", "organization GnomeProfile/two")
}
