package compiler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

const (
	appURL = "https://github.com/pkgforge-dev/ghostty-appimage/releases/download/v1.3.1/Ghostty-1.3.1-x86_64.AppImage"
	appSHA = "fde48d2b716afd1978766879bbf1aae30dd305e8ad86a1037a2614a14d82dc28"
	appSVG = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>`

	ghosttyEntry = "    id: com.mitchellh.ghostty\n    name: Ghostty\n    comment: Fast, native terminal emulator\n    icon: icons/ghostty.png\n    categories: [System, TerminalEmulator]\n    startupWMClass: com.mitchellh.ghostty\n"
)

// appArtifact builds a single-file BinaryArtifact; entry is the indented
// desktopEntry body, or empty for none.
func appArtifact(name, dest, entry string) string {
	doc := fmt.Sprintf("apiVersion: software.deskos.org/v1alpha1\nkind: BinaryArtifact\nmetadata:\n  name: %s\nspec:\n  version: 1.3.1\n  source:\n    url: %s\n    sha256: %s\n  archive: none\n  files:\n    - destination: %s\n", name, appURL, appSHA, dest)
	if entry != "" {
		doc += "  desktopEntry:\n" + entry
	}
	return doc
}

func TestDesktopEntryValidation(t *testing.T) {
	icons := fixture{
		"icons/ghostty.png": pngOf(t, 128, 128),
		"icons/wide.png":    pngOf(t, 64, 32),
		"icons/huge.png":    pngOf(t, 1024, 1024),
		"icons/odd.png":     pngOf(t, 100, 100),
		"icons/fake.png":    "not a png",
		"icons/app.svg":     appSVG,
		"icons/html.svg":    "<html><body/></html>",
		"icons/app.txt":     "x",
	}
	load := func(entry string) error {
		_, err := newCompiler(t).Load([]string{icons.with(fixture{"a.yaml": appArtifact("ghostty", "/usr/local/bin/ghostty", entry)}).dir(t)})
		return err
	}
	with := func(old, new string) string { return strings.Replace(ghosttyEntry, old, new, 1) }

	if err := load(ghosttyEntry); err != nil {
		t.Fatalf("valid desktopEntry rejected: %v", err)
	}
	if err := load(with("icons/ghostty.png", "icons/app.svg")); err != nil {
		t.Fatalf("valid SVG icon rejected: %v", err)
	}
	for name, tc := range map[string]struct {
		entry string
		want  []string
	}{
		"id with one element":        {with("id: com.mitchellh.ghostty", "id: ghostty"), []string{"schema validation failed"}},
		"id element with digit":      {with("id: com.mitchellh.ghostty", "id: com.7zip.App"), []string{"schema validation failed"}},
		"id with slash":              {with("id: com.mitchellh.ghostty", "id: com/evil.App"), []string{"schema validation failed"}},
		"name with newline":          {with("name: Ghostty", `name: "Ghost\nty"`), []string{"schema validation failed"}},
		"comment with tab":           {with("comment: Fast, native terminal emulator", `comment: "Fast\tterminal"`), []string{"schema validation failed"}},
		"name with outer whitespace": {with("name: Ghostty", `name: " Ghostty"`), []string{"desktopEntry.name must not start or end with whitespace"}},
		"missing name":               {with("    name: Ghostty\n", ""), []string{"schema validation failed"}},
		"unregistered category":      {with("[System, TerminalEmulator]", "[System, Terminal]"), []string{`categories[1] "Terminal" is not a registered freedesktop category`}},
		"ConsoleOnly category":       {with("[System, TerminalEmulator]", "[System, ConsoleOnly]"), []string{`"ConsoleOnly" is not a registered freedesktop category`}},
		"no main category":           {with("[System, TerminalEmulator]", "[TerminalEmulator]"), []string{"categories needs one main category"}},
		"no categories":              {with("[System, TerminalEmulator]", "[]"), []string{"schema validation failed"}},
		"invalid startupWMClass":     {with("startupWMClass: com.mitchellh.ghostty", `startupWMClass: "a b"`), []string{"schema validation failed"}},
		"authored Exec":              {ghosttyEntry + "    exec: /bin/sh -c id\n", []string{"schema validation failed"}},
		"missing icon":               {with("icons/ghostty.png", "icons/missing.png"), []string{"desktopEntry.icon: asset", "missing.png"}},
		"icon not square":            {with("icons/ghostty.png", "icons/wide.png"), []string{"is 64x32; a PNG icon must be square"}},
		"icon too large":             {with("icons/ghostty.png", "icons/huge.png"), []string{"is 1024x1024", "hicolor size"}},
		"icon size without hicolor":  {with("icons/ghostty.png", "icons/odd.png"), []string{"is 100x100", "hicolor size"}},
		"icon not a PNG":             {with("icons/ghostty.png", "icons/fake.png"), []string{"is not a PNG image"}},
		"icon not an SVG":            {with("icons/ghostty.png", "icons/html.svg"), []string{"is not an SVG document"}},
		"icon of another type":       {with("icons/ghostty.png", "icons/app.txt"), []string{"must be a .png or .svg file"}},
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(tc.entry), tc.want...)
		})
	}
	t.Run("icon outside the resource root", func(t *testing.T) {
		dir := icons.with(fixture{"a.yaml": appArtifact("ghostty", "/usr/local/bin/ghostty", with("icons/ghostty.png", "../outside.png"))}).dir(t)
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.png"), []byte(pngOf(t, 48, 48)), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := newCompiler(t).Load([]string{dir})
		mustFail(t, err, "desktopEntry.icon", "outside the resource root")
	})
	t.Run("artifact with several files", func(t *testing.T) {
		doc := strings.Replace(appArtifact("tools", "/usr/local/bin/a", ghosttyEntry), "  archive: none\n  files:\n    - destination: /usr/local/bin/a\n",
			"  archive: tar.gz\n  files:\n    - {path: a, destination: /usr/local/bin/a}\n    - {path: b, destination: /usr/local/bin/b}\n", 1)
		_, err := newCompiler(t).Load([]string{icons.with(fixture{"a.yaml": doc}).dir(t)})
		mustFail(t, err, "desktopEntry requires an artifact with exactly one file; it has 2")
	})
}

// The typed decoder enforces the same rules as the schema on its own.
func TestDesktopEntryTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	spec := func(files, entry string) string {
		return `{"version":"1.3.1","source":{"url":"` + appURL + `","sha256":"` + appSHA + `"},"archive":"tar.gz","files":[` + files + `],"desktopEntry":{` + entry + `}}`
	}
	one := `{"path":"g","destination":"/usr/local/bin/g"}`
	const valid = `"id":"com.mitchellh.ghostty","name":"Ghostty","icon":"g.png","categories":["System"]`
	for name, tc := range map[string]struct {
		spec string
		want string
	}{
		"invalid id":           {spec(one, strings.Replace(valid, "com.mitchellh.ghostty", "ghostty", 1)), `desktopEntry.id "ghostty" must be a reverse-DNS name`},
		"name with newline":    {spec(one, strings.Replace(valid, `"Ghostty"`, `"Ghost\nty"`, 1)), "desktopEntry.name must be a single line"},
		"empty name":           {spec(one, strings.Replace(valid, `"Ghostty"`, `""`, 1)), "desktopEntry.name is required"},
		"invalid category":     {spec(one, strings.Replace(valid, `"System"`, `"System","Shell"`, 1)), `categories[1] "Shell" is not a registered freedesktop category`},
		"duplicate category":   {spec(one, strings.Replace(valid, `"System"`, `"System","System"`, 1)), `categories[1] "System" is listed twice`},
		"no categories":        {spec(one, strings.Replace(valid, `["System"]`, `[]`, 1)), "categories requires at least one category"},
		"invalid wm class":     {spec(one, valid+`,"startupWMClass":"a;b"`), `startupWMClass "a;b" must match`},
		"missing icon":         {spec(one, strings.Replace(valid, `"g.png"`, `""`, 1)), "desktopEntry.icon: file is required"},
		"several files":        {spec(one+`,{"path":"h","destination":"/usr/local/bin/h"}`, valid), "desktopEntry requires an artifact with exactly one file"},
		"unknown field (Exec)": {spec(one, valid+`,"exec":"/bin/sh"`), "unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource("software.deskos.org/v1alpha1", "BinaryArtifact", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestDesktopEntriesKeyed(t *testing.T) {
	ws := base.with(fixture{
		"ws.yaml":           workstation("ws", "org", "role"),
		"icons/ghostty.png": pngOf(t, 128, 128),
	})
	t.Run("same artifact through two profiles deduplicates", func(t *testing.T) {
		dir := ws.with(fixture{
			"org.yaml":  profile("org", "organization", "BinaryArtifact/ghostty"),
			"role.yaml": profile("role", "role", "BinaryArtifact/ghostty"),
			"a.yaml":    appArtifact("ghostty", "/usr/local/bin/ghostty", ghosttyEntry),
		}).dir(t)
		p := mustPlan(t, "ws", dir)
		if n := len(p.Artifact.DesktopEntries); n != 1 {
			t.Fatalf("got %d desktop entries, want 1", n)
		}
		if n := len(p.Artifact.DesktopEntries[0].Provenance); n != 2 {
			t.Errorf("deduplicated desktop entry should keep both profiles, got %d", n)
		}
	})
	t.Run("same id in two artifacts conflicts", func(t *testing.T) {
		dir := ws.with(fixture{
			"org.yaml":  profile("org", "organization", "BinaryArtifact/ghostty"),
			"role.yaml": profile("role", "role", "BinaryArtifact/ghostty-nightly"),
			"a.yaml":    appArtifact("ghostty", "/usr/local/bin/ghostty", ghosttyEntry),
			"b.yaml":    appArtifact("ghostty-nightly", "/usr/local/bin/ghostty-nightly", ghosttyEntry),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `composition conflict for desktop entry "com.mitchellh.ghostty"`,
			"BinaryArtifact/ghostty", "BinaryArtifact/ghostty-nightly", "a.yaml", "b.yaml",
			"/usr/local/bin/ghostty-nightly")
	})
}

func TestDesktopEntriesRender(t *testing.T) {
	icon := pngOf(t, 128, 128)
	files := base.with(fixture{
		"a.yaml":            profile("a", "organization", "BinaryArtifact/ghostty", "PackageSet/tools"),
		"b.yaml":            profile("b", "role", "BinaryArtifact/editor"),
		"tools.yaml":        packageSet("tools", "git"),
		"ghostty.yaml":      appArtifact("ghostty", "/usr/local/bin/ghostty", ghosttyEntry),
		"editor.yaml":       appArtifact("editor", "/usr/local/bin/editor", "    id: org.example.Editor\n    name: 'C:\\Edit'\n    icon: icons/editor.svg\n    categories: [Utility, TextEditor]\n"),
		"icons/ghostty.png": icon,
		"icons/editor.svg":  appSVG,
	})
	render := func(dir string) map[string][]byte {
		out, err := containerfile.Render(mustPlan(t, "ws", dir))
		if err != nil {
			t.Fatal(err)
		}
		m := map[string][]byte{}
		for _, f := range out {
			m[f.Path] = f.Data
		}
		return m
	}
	dir := files.with(fixture{"ws.yaml": workstation("ws", "a", "b")}).dir(t)
	got := render(dir)

	const desktop = "rootfs/usr/share/applications/com.mitchellh.ghostty.desktop"
	const want = `# Generated by deskosctl. Do not edit; change the DeskOS resources instead.
[Desktop Entry]
Type=Application
Name=Ghostty
Comment=Fast, native terminal emulator
Exec=/usr/local/bin/ghostty
TryExec=/usr/local/bin/ghostty
Icon=com.mitchellh.ghostty
Terminal=false
Categories=System;TerminalEmulator;
StartupWMClass=com.mitchellh.ghostty
`
	if string(got[desktop]) != want {
		t.Errorf("%s =\n%s\nwant\n%s", desktop, got[desktop], want)
	}
	const editor = "rootfs/usr/share/applications/org.example.Editor.desktop"
	if !strings.Contains(string(got[editor]), "\nName=C:\\\\Edit\n") || strings.Contains(string(got[editor]), "Comment=") || strings.Contains(string(got[editor]), "StartupWMClass=") {
		t.Errorf("%s does not escape the backslash or writes unset keys:\n%s", editor, got[editor])
	}
	if string(got["rootfs/usr/share/icons/hicolor/128x128/apps/com.mitchellh.ghostty.png"]) != icon {
		t.Error("PNG icon is not installed in hicolor/128x128/apps")
	}
	if string(got["rootfs/usr/share/icons/hicolor/scalable/apps/org.example.Editor.svg"]) != appSVG {
		t.Error("SVG icon is not installed in hicolor/scalable/apps")
	}

	var manifest struct {
		Files []struct{ Path, Mode, SHA256 string } `json:"files"`
	}
	if err := json.Unmarshal(got[containerfile.ManifestPath], &manifest); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{desktop, editor, "rootfs/usr/share/icons/hicolor/128x128/apps/com.mitchellh.ghostty.png"} {
		i := slices.IndexFunc(manifest.Files, func(f struct{ Path, Mode, SHA256 string }) bool { return f.Path == p })
		if i < 0 || manifest.Files[i].Mode != "0644" || manifest.Files[i].SHA256 != sha(string(got[p])) {
			t.Errorf("manifest entry for %s missing or wrong", p)
		}
	}

	cf := string(got[containerfile.ContainerfilePath])
	copyAt := strings.Index(cf, "COPY rootfs/ /\n")
	cacheAt := strings.Index(cf, "gtk-update-icon-cache --force --quiet '/usr/share/icons/hicolor'")
	if copyAt < 0 || cacheAt < copyAt || strings.Count(cf, "gtk-update-icon-cache --force") != 1 {
		t.Errorf("icon cache is not regenerated exactly once after the image files are copied:\n%s", cf)
	}
	if !strings.Contains(cf, "\n        'gtk-update-icon-cache'") {
		t.Error("gtk-update-icon-cache is not installed")
	}

	again := render(dir)
	swapped := render(files.with(fixture{"ws.yaml": workstation("ws", "b", "a")}).dir(t))
	for _, other := range []map[string][]byte{again, swapped} {
		if len(other) != len(got) {
			t.Fatalf("renders have %d and %d files", len(got), len(other))
		}
		for k, v := range got {
			if !bytes.Equal(v, other[k]) {
				t.Errorf("%s differs between renders or profile orders", k)
			}
		}
	}
}
