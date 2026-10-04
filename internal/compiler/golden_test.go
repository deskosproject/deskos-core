package compiler_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

var update = flag.Bool("update", false, "rewrite golden files")

const (
	resourcesRoot = "../../resources"
	exampleRoot   = "../../examples/example-org"
)

func rawResource(apiVersion, kind, name, spec string) *model.Resource {
	return &model.Resource{
		APIVersion: apiVersion, Kind: kind, Metadata: model.Metadata{Name: name},
		Spec: json.RawMessage(spec), Source: model.Source{Root: "test", Path: "inline.yaml"},
	}
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/compiler -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from golden output; review the change and run go test ./internal/compiler -update", name)
	}
}

func TestGoldenCentOSReferencePlan(t *testing.T) {
	p := mustPlan(t, "deskos-core-centos10", resourcesRoot)
	b, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "deskos-core-centos10.plan.json", b)
}

func TestGoldenExampleRHELPlan(t *testing.T) {
	p := mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot)
	b, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "example-devops-rhel10.plan.json", b)
}

func TestGoldenContainerfiles(t *testing.T) {
	for _, tc := range []struct {
		ws    string
		roots []string
	}{
		{"deskos-core-centos10", []string{resourcesRoot}},
		{"example-devops-rhel10", []string{resourcesRoot, exampleRoot}},
	} {
		files, err := containerfile.Render(mustPlan(t, tc.ws, tc.roots...))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if f.Path == containerfile.ContainerfilePath {
				golden(t, tc.ws+".Containerfile", f.Data)
			}
		}
	}
}

// The preinstall unit only materializes image-declared Flatpaks: no restart
// loop, and no ordering that makes boot targets or GDM wait for it or for the
// network (see docs/architecture.md, System Flatpaks).
func TestFlatpakPreinstallUnitOrdering(t *testing.T) {
	files, err := containerfile.Render(mustPlan(t, "deskos-core-centos10", resourcesRoot))
	if err != nil {
		t.Fatal(err)
	}
	var unit []byte
	for _, f := range files {
		if strings.HasSuffix(f.Path, "/deskos-flatpak-preinstall.service") {
			unit = f.Data
		}
	}
	if unit == nil {
		t.Fatal("preinstall unit not rendered")
	}
	golden(t, "deskos-flatpak-preinstall.service", unit)
	u := string(unit)
	for _, want := range []string{"Type=exec", "WantedBy=multi-user.target", "After=network-online.target multi-user.target\n", "ExecStart=/usr/bin/flatpak preinstall "} {
		if !strings.Contains(u, want) {
			t.Errorf("unit lacks %q", want)
		}
	}
	for _, bad := range []string{"Type=oneshot", "Restart=", "Before=", "Requires=", "RequiredBy=", "graphical.target", "display-manager", "gdm", "Timer", "OnCalendar"} {
		if strings.Contains(u, bad) {
			t.Errorf("unit contains %q", bad)
		}
	}
}

func TestDeterministicPlanJSON(t *testing.T) {
	a, err := mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot).JSON()
	if err != nil {
		t.Fatal(err)
	}
	b, err := mustPlan(t, "example-devops-rhel10", exampleRoot, resourcesRoot).JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("plan JSON depends on root order")
	}
	var generic map[string]any
	if err := json.Unmarshal(a, &generic); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"artifact", "provisioning", "enrollment"} {
		if _, ok := generic[phase]; !ok {
			t.Errorf("plan has no %s phase", phase)
		}
	}
}

func TestDeterministicRender(t *testing.T) {
	render := func(dir string) map[string][]byte {
		files, err := containerfile.Render(mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot))
		if err != nil {
			t.Fatal(err)
		}
		if err := containerfile.WriteDir(dir, files); err != nil {
			t.Fatal(err)
		}
		out := map[string][]byte{}
		err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			data, err := os.ReadFile(p)
			out[rel+" "+info.Mode().String()] = data
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	d1, d2 := t.TempDir(), t.TempDir()
	a, b := render(d1), render(d2)
	if len(a) != len(b) {
		t.Fatalf("different file sets: %d vs %d", len(a), len(b))
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			t.Errorf("%s differs between renders", k)
		}
	}
	// Re-rendering into a previous render is allowed and identical.
	if c := render(d1); len(c) != len(a) {
		t.Fatal("re-render changed the file set")
	}
	// Repository assets are copied verbatim; only generated text is checked.
	copied := map[string]bool{}
	for _, f := range mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot).Artifact.Files {
		copied[filepath.Join("rootfs", f.Path)] = true
	}
	for k := range a {
		if path, _, _ := strings.Cut(k, " "); copied[path] {
			continue
		}
		if strings.Contains(string(a[k]), "\r") {
			t.Errorf("%s contains carriage returns", k)
		}
	}
}

func TestRenderRefusesForeignDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := containerfile.Render(mustPlan(t, "deskos-core-centos10", resourcesRoot))
	if err != nil {
		t.Fatal(err)
	}
	mustFail(t, containerfile.WriteDir(dir, files), "not a previous render")
}

// Every dnf transaction keeps subscription-manager state out of its layer:
// RHSM directories are tmpfs, and the generated redhat.repo is removed in the
// same RUN. Declared vendor repository files stay in the build context.
func TestRpmTransactionsKeepRHSMStateOutOfLayers(t *testing.T) {
	for _, tc := range []struct {
		ws    string
		roots []string
	}{
		{"deskos-core-centos10", []string{resourcesRoot}},
		{"example-devops-centos10", []string{resourcesRoot, exampleRoot}},
		{"example-devops-rhel10", []string{resourcesRoot, exampleRoot}},
	} {
		p := mustPlan(t, tc.ws, tc.roots...)
		files, err := containerfile.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		byPath := map[string]string{}
		for _, f := range files {
			byPath[f.Path] = string(f.Data)
		}
		checkRHSMRuns(t, tc.ws, p, byPath[containerfile.ContainerfilePath])
		for _, r := range p.Artifact.RpmRepositories {
			if _, ok := byPath["repos/etc/yum.repos.d/"+r.ID+".repo"]; !ok {
				t.Errorf("%s: repository file for %s missing", tc.ws, r.ID)
			}
		}
	}
}

// checkRHSMRuns asserts that every dnf RUN of a Containerfile keeps RHSM state out of its layer.
func checkRHSMRuns(t *testing.T, name string, p *plan.Plan, cf string) {
	t.Helper()
	var runs []string
	for _, block := range strings.Split(cf, "\n\n") {
		if i := strings.Index(block, "RUN "); i >= 0 && strings.Contains(block, "dnf -y") {
			runs = append(runs, block[i:])
		}
	}
	want := 0
	for _, n := range []int{len(p.Artifact.RpmGroups), len(p.Artifact.RpmPackages), len(p.Artifact.RpmFiles)} {
		if n > 0 {
			want++
		}
	}
	if len(runs) != want {
		t.Fatalf("%s: %d dnf RUNs, want %d", name, len(runs), want)
	}
	for _, run := range runs {
		for _, need := range []string{
			"RUN --mount=type=tmpfs,target=/var/lib/rhsm \\\n    --mount=type=tmpfs,target=/var/log/rhsm \\\n",
			"if [ -e /etc/yum.repos.d/redhat.repo ]; then",
			"&& dnf clean all \\\n",
		} {
			if !strings.Contains(run, need) {
				t.Errorf("%s: dnf RUN lacks %q:\n%s", name, need, run)
			}
		}
		if !strings.HasSuffix(strings.TrimSpace(run), "&& rm -f /etc/yum.repos.d/redhat.repo") {
			t.Errorf("%s: redhat.repo removal is not the last command of the RUN", name)
		}
	}
}

// The effective wallpaper also becomes a GLib schema default, so sessions
// whose dconf profile skips the DeskOS database (GNOME Initial Setup) show
// it too. An organization's wallpaper replaces Core's there as well.
func TestWallpaperReachesInitialSetupAsSchemaDefault(t *testing.T) {
	const header = "# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n"
	for _, tc := range []struct {
		ws          string
		roots       []string
		light, dark string
	}{
		{"deskos-core-centos10", []string{resourcesRoot}, "deskos-light.svg", "deskos-dark.svg"},
		{"example-devops-rhel10", []string{resourcesRoot, exampleRoot}, "example-org.svg", "example-org.svg"},
	} {
		p := mustPlan(t, tc.ws, tc.roots...)
		want := header + "[org.gnome.desktop.background]\n" +
			"picture-uri='file:///usr/share/deskos/backgrounds/" + tc.light + "'\n" +
			"picture-uri-dark='file:///usr/share/deskos/backgrounds/" + tc.dark + "'\n"
		if len(p.Artifact.GSettings) != 2 {
			t.Fatalf("%s: %d GSettings vendor defaults, want 2", tc.ws, len(p.Artifact.GSettings))
		}
		for _, d := range p.Artifact.GSettings {
			if d.Setting != "appearance.wallpaper" || len(d.Provenance) == 0 {
				t.Errorf("%s: %+v lacks setting or provenance", tc.ws, d)
			}
		}
		files, err := containerfile.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		var override, cf string
		for _, f := range files {
			switch f.Path {
			case "rootfs/usr/share/glib-2.0/schemas/50_deskos.gschema.override":
				override = string(f.Data)
			case containerfile.ContainerfilePath:
				cf = string(f.Data)
			}
		}
		if override != want {
			t.Errorf("%s: override\n%s\nwant\n%s", tc.ws, override, want)
		}
		strict := strings.Index(cf, `cp /usr/share/glib-2.0/schemas/*.xml /usr/share/glib-2.0/schemas/50_deskos.gschema.override "$tmp"/`+" \\\n    && glib-compile-schemas --strict --dry-run \"$tmp\"")
		compile := strings.Index(cf, "&& glib-compile-schemas /usr/share/glib-2.0/schemas \\\n")
		if strict < 0 || strict < strings.Index(cf, "COPY rootfs/ /") || compile < strict || compile > strings.Index(cf, "dconf update") {
			t.Errorf("%s: the DeskOS override must be checked strictly, then schemas compiled, after COPY rootfs and before dconf update", tc.ws)
		}
		if strings.Contains(cf, "glib-compile-schemas --strict /usr/share/glib-2.0/schemas") {
			t.Errorf("%s: distribution overrides must not be compiled strictly", tc.ws)
		}
	}
}

func TestConflictingGSettingsVendorDefaultsFail(t *testing.T) {
	p := &plan.Plan{}
	p.Artifact.GSettings = []plan.GSettingsVendorDefault{
		{Schema: "org.gnome.desktop.background", Key: "picture-uri", Type: "s", Value: "'file:///a.svg'", Setting: "appearance.wallpaper"},
		{Schema: "org.gnome.desktop.background", Key: "picture-uri", Type: "s", Value: "'file:///b.svg'", Setting: "appearance.wallpaper"},
	}
	mustFail(t, p.Normalize(), "conflicting GSettings vendor defaults")

	same := &plan.Plan{}
	same.Artifact.GSettings = []plan.GSettingsVendorDefault{
		{Schema: "s", Key: "k", Type: "s", Value: "'v'"},
		{Schema: "s", Key: "k", Type: "s", Value: "'v'"},
	}
	if err := same.Normalize(); err != nil || len(same.Artifact.GSettings) != 1 {
		t.Fatalf("identical defaults should merge: %v, %d", err, len(same.Artifact.GSettings))
	}
}

// Core boots with the stock graphical splash: kargs.d carries rhgb and
// quiet, and the initramfs is rebuilt with Plymouth after the files it needs.
func TestCoreBootRendering(t *testing.T) {
	files, err := containerfile.Render(mustPlan(t, "deskos-core-centos10", resourcesRoot))
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Data)
	}
	if got := byPath["rootfs/usr/lib/bootc/kargs.d/50-deskos.toml"]; !strings.HasSuffix(got, "kargs = [\"quiet\", \"rhgb\"]\n") {
		t.Errorf("kargs.d = %q", got)
	}
	if got := byPath["rootfs/usr/lib/dracut/dracut.conf.d/50-deskos.conf"]; !strings.HasSuffix(got, "add_dracutmodules+=\" plymouth \"\n") {
		t.Errorf("dracut drop-in = %q", got)
	}
	cf := byPath[containerfile.ContainerfilePath]
	dracut := strings.Index(cf, "env DRACUT_NO_XATTR=1 dracut --force")
	if dracut < 0 || dracut < strings.Index(cf, "COPY rootfs/ /") || dracut > strings.Index(cf, "bootc container lint") {
		t.Error("initramfs must be rebuilt after COPY rootfs and before lint")
	}
	if strings.Contains(cf, "uname -r") {
		t.Error("the build host kernel must not select the image kernel")
	}

	const themeDir = "rootfs/usr/share/plymouth/themes/deskos/"
	asset, err := os.ReadFile(filepath.Join(resourcesRoot, "assets/deskos/deskos-splash-watermark.png"))
	if err != nil {
		t.Fatal(err)
	}
	if byPath[themeDir+"watermark.png"] != string(asset) {
		t.Error("the theme watermark is not the Core splash watermark asset")
	}
	theme := byPath[themeDir+"deskos.plymouth"]
	if !strings.Contains(theme, "\nModuleName=two-step\n") || !strings.Contains(theme, "\nImageDir=/usr/share/plymouth/themes/deskos\n") || strings.Contains(theme, "UseFirmwareBackground=true") {
		t.Errorf("theme file = %q", theme)
	}
	for p := range byPath {
		if strings.HasPrefix(p, "rootfs/") && strings.Contains(p, "plymouth") && !strings.HasPrefix(p, themeDir) && !strings.HasSuffix(p, "50-deskos.conf") {
			t.Errorf("%s: the build context writes Plymouth files outside the DeskOS theme", p)
		}
	}
	selectTheme := strings.Index(cf, "plymouth-set-default-theme 'deskos';")
	if selectTheme < 0 || selectTheme < strings.Index(cf, "COPY rootfs/ /") || selectTheme > dracut {
		t.Error("the DeskOS theme must be selected after COPY rootfs and before the initramfs is rebuilt")
	}
	for _, want := range []string{
		"for f in '/usr/share/plymouth/themes/spinner'/*.png; do",
		`[ "$name" != 'watermark.png' ] || continue;`,
		`install -m 0644 "$f" '/usr/share/plymouth/themes/deskos'/"$name";`,
		"grep -qF 'usr/share/plymouth/themes/deskos/watermark.png'",
		"lsinitrd -f etc/plymouth/plymouthd.conf \"${kdir}initramfs.img\" | grep -qx 'Theme=deskos'",
	} {
		if !strings.Contains(cf, want) {
			t.Errorf("Containerfile lacks %q", want)
		}
	}
}
