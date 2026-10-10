// Command deskos-theme applies a theme's palette to the GNOME surfaces
// DeskOS supports. It is the runtime counterpart of the compiler's Theme
// kind: the compiler bakes the palette into an image (system mode), and this
// command applies it to a session (user mode), reading either a DeskOS Theme
// or an palette theme directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/theme"
)

// ptyxisProfileUUID is the fixed UUID of the profile the tool provisions so a
// terminal uses the palette by default.
const ptyxisProfileUUID = "9a1f0f9a-6f2b-4a0e-8e0b-0d9f4a1c2b30"

func main() {
	if len(os.Args) < 2 || os.Args[1] != "apply" {
		fmt.Fprintln(os.Stderr, "usage: deskos-theme apply --palette <dir> [--user | --root <dir>]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	palette := fs.String("palette", "", "an palette theme directory (colors.toml + backgrounds)")
	user := fs.Bool("user", false, "write to $HOME instead of a system root")
	root := fs.String("root", "/", "system root to write under")
	shellCSS := fs.String("shell-css", "", "the distro's compiled gnome-shell.css to theme (best effort)")
	extensions := fs.String("extensions", "background-logo@fedorahosted.org,dash-to-dock@micxgx.gmail.com",
		"the enabled-extensions the image already writes, to repeat in the shell drop in")
	_ = fs.Parse(os.Args[2:])

	if *palette == "" {
		fmt.Fprintln(os.Stderr, "deskos-theme: --palette is required")
		os.Exit(2)
	}
	p, err := paletteFrompalette(*palette)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskos-theme: %v\n", err)
		os.Exit(1)
	}

	var base string
	if *user {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "deskos-theme: %v\n", err)
			os.Exit(1)
		}
		base = home
	} else {
		base = *root
	}
	files := artifacts(base, *user, p)
	if *shellCSS != "" {
		css, err := os.ReadFile(*shellCSS)
		if err != nil {
			fmt.Fprintf(os.Stderr, "deskos-theme: %v\n", err)
			os.Exit(1)
		}
		patched := theme.PatchShellCSS(string(css), p)
		shellDir := filepath.Join(base, "usr/share/themes")
		dconfPath := ""
		if *user {
			shellDir = filepath.Join(base, ".local/share/themes")
		} else {
			dconfPath = filepath.Join(base, "etc/dconf/db/distro.d/61-deskos-shell")
		}
		for _, pack := range []string{"a", "b"} {
			files[filepath.Join(shellDir, theme.ShellTheme(pack), "gnome-shell/gnome-shell.css")] = patched
		}
		if dconfPath != "" {
			files[dconfPath] = theme.ShellDconf(strings.Split(*extensions, ","), theme.ShellTheme("a"))
		} else {
			fmt.Println("user mode: enable the shell theme with:")
			fmt.Println("  gnome-extensions enable " + theme.UserThemeExtension)
			fmt.Println("  gsettings set org.gnome.shell.extensions.user-theme name " + theme.ShellTheme("a"))
		}
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := writeFile(n, files[n]); err != nil {
			fmt.Fprintf(os.Stderr, "deskos-theme: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(n)
	}
	fmt.Printf("applied theme %q (%d files)\n", p.Name, len(files))
	fmt.Println("note: the GNOME Shell theme needs the User Themes extension and a logout; see docs.")
}

// paletteFrompalette reads an palette theme's colors.toml into a palette.
func paletteFrompalette(dir string) (theme.Palette, error) {
	data, err := os.ReadFile(filepath.Join(dir, "colors.toml"))
	if err != nil {
		return theme.Palette{}, err
	}
	colors := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		colors[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if colors["background"] == "" || colors["foreground"] == "" {
		return theme.Palette{}, fmt.Errorf("%s does not define background and foreground", dir)
	}
	return theme.Palette{
		Name:       filepath.Base(filepath.Clean(dir)),
		Accent:     colors["accent"],
		Background: colors["background"],
		Foreground: colors["foreground"],
		Red:        colors["red"],
		Green:      colors["green"],
		Yellow:     colors["yellow"],
		Blue:       colors["blue"],
		Magenta:    colors["magenta"],
		Cyan:       colors["cyan"],
	}, nil
}

// artifacts returns the absolute paths and contents to write. System mode
// seeds the user skeleton and the dconf database a build's dconf update
// compiles; user mode writes the same files into a running session.
func artifacts(base string, user bool, p theme.Palette) map[string]string {
	var skel, dconf string
	if user {
		skel = base
		dconf = ""
	} else {
		skel = filepath.Join(base, "etc/skel")
		dconf = filepath.Join(base, "etc/dconf/db/distro.d")
	}
	files := map[string]string{
		filepath.Join(skel, ".config/gtk-4.0/gtk.css"): p.GTK4CSS(),
		filepath.Join(skel, ".config/gtk-3.0/gtk.css"): p.GTK3CSS(),
	}
	if p.TerminalReady() {
		files[filepath.Join(skel, ".local/share/org.gnome.Ptyxis/palettes", p.Name+".palette")] = p.PtyxisPalette()
		files[filepath.Join(skel, ".config/ghostty/themes", p.Name)] = p.GhosttyConfig()
		if dconf != "" {
			files[filepath.Join(dconf, "60-deskos-ptyxis")] = p.PtyxisProfile(ptyxisProfileUUID)
		}
	}
	return files
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
