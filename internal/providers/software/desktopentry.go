package software

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
)

// DesktopEntrySpec is the application launcher of a BinaryArtifact.
type DesktopEntrySpec struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Comment        string   `json:"comment,omitempty"`
	Icon           string   `json:"icon"`
	Categories     []string `json:"categories"`
	StartupWMClass string   `json:"startupWMClass,omitempty"`
}

// DesktopEntry is the composed launcher of one executable, keyed by ID.
type DesktopEntry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Comment        string   `json:"comment,omitempty"`
	Exec           string   `json:"exec"`
	Categories     []string `json:"categories"`
	StartupWMClass string   `json:"startupWMClass,omitempty"`
	IconPath       string   `json:"iconPath"`
	IconAsset      string   `json:"iconAsset"`
	IconSHA256     string   `json:"iconSHA256"`
	// IconFile is the absolute host path of IconAsset.
	IconFile string `json:"-"`
}

// BinaryArtifactObject is the decoded BinaryArtifact with its desktop entry resolved.
type BinaryArtifactObject struct {
	BinaryArtifactSpec
	Entry *DesktopEntry
}

// Desktop entry install locations.
const (
	ApplicationsDir = "/usr/share/applications"
	HicolorDir      = "/usr/share/icons/hicolor"
)

var (
	// A D-Bus well-known name, as the Desktop Entry Specification requires for application file names.
	desktopIDRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)+$`)
	wmClassRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// hicolorSizes are the fixed-size application icon directories of hicolor's index.theme.
var hicolorSizes = []int{16, 22, 24, 32, 36, 48, 64, 72, 96, 128, 192, 256, 512}

// mainCategories are the freedesktop Main Categories known to the platforms' desktop-file-validate.
var mainCategories = setOf("AudioVideo", "Audio", "Video", "Development", "Education", "Game",
	"Graphics", "Network", "Office", "Science", "Settings", "System", "Utility")

// additionalCategories are the freedesktop Additional Categories known to the platforms'
// desktop-file-validate, without ConsoleOnly, which contradicts Terminal=false.
var additionalCategories = setOf(
	"Building", "Debugger", "IDE", "GUIDesigner", "Profiling", "RevisionControl", "Translation",
	"Calendar", "ContactManagement", "Database", "Dictionary", "Chart", "Email", "Finance", "FlowChart",
	"PDA", "ProjectManagement", "Presentation", "Spreadsheet", "WordProcessor",
	"2DGraphics", "VectorGraphics", "RasterGraphics", "3DGraphics", "Scanning", "OCR", "Photography",
	"Publishing", "Viewer", "TextTools", "DesktopSettings", "HardwareSettings", "Printing", "PackageManager",
	"Dialup", "InstantMessaging", "Chat", "IRCClient", "Feed", "FileTransfer", "HamRadio", "News", "P2P",
	"RemoteAccess", "Telephony", "TelephonyTools", "VideoConference", "WebBrowser", "WebDevelopment",
	"Midi", "Mixer", "Sequencer", "Tuner", "TV", "AudioVideoEditing", "Player", "Recorder", "DiscBurning",
	"ActionGame", "AdventureGame", "ArcadeGame", "BoardGame", "BlocksGame", "CardGame", "KidsGame",
	"LogicGame", "RolePlaying", "Shooter", "Simulation", "SportsGame", "StrategyGame",
	"Art", "Construction", "Music", "Languages", "ArtificialIntelligence", "Astronomy", "Biology",
	"Chemistry", "ComputerScience", "DataVisualization", "Economy", "Electricity", "Geography", "Geology",
	"Geoscience", "History", "Humanities", "ImageProcessing", "Literature", "Maps", "Math",
	"NumericalAnalysis", "MedicalSoftware", "Physics", "Robotics", "Spirituality", "Sports",
	"ParallelComputing", "Amusement", "Archiving", "Compression", "Electronics", "Emulator", "Engineering",
	"FileTools", "FileManager", "TerminalEmulator", "Filesystem", "Monitor", "Security", "Accessibility",
	"Calculator", "Clock", "TextEditor", "Documentation", "Adult", "Core",
	"KDE", "COSMIC", "GNOME", "LXQt", "XFCE", "DDE", "GTK", "Qt", "Motif", "Java")

func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

// displayText validates a single-line, user-visible value.
func displayText(v string) error {
	switch {
	case !utf8.ValidString(v) || !textRE.MatchString(v):
		return fmt.Errorf("must be a single line of UTF-8 text without control characters")
	case strings.TrimFunc(v, unicode.IsSpace) != v:
		return fmt.Errorf("must not start or end with whitespace")
	}
	return nil
}

// decodeDesktopEntry validates spec.DesktopEntry and resolves its icon.
func decodeDesktopEntry(res *model.Resource, spec *BinaryArtifactSpec, errs *model.ErrorList) *DesktopEntry {
	d := spec.DesktopEntry
	n := len(*errs)
	bad := func(format string, a ...any) { errs.Add(model.Errorf(res, "desktopEntry."+format, a...)) }

	// One executable per artifact keeps Exec unambiguous without a selector.
	if len(spec.Files) != 1 {
		errs.Add(model.Errorf(res, "desktopEntry requires an artifact with exactly one file; it has %d", len(spec.Files)))
	}
	if !desktopIDRE.MatchString(d.ID) || len(d.ID) > 255 {
		bad("id %q must be a reverse-DNS name such as com.example.App (dot-separated elements of [A-Za-z0-9_-], none starting with a digit)", d.ID)
	}
	if d.Name == "" {
		bad("name is required")
	} else if err := displayText(d.Name); err != nil {
		bad("name %v", err)
	}
	if d.Comment != "" {
		if err := displayText(d.Comment); err != nil {
			bad("comment %v", err)
		}
	}
	if d.StartupWMClass != "" && !wmClassRE.MatchString(d.StartupWMClass) {
		bad("startupWMClass %q must match %s", d.StartupWMClass, wmClassRE)
	}
	if len(d.Categories) == 0 {
		bad("categories requires at least one category")
	}
	hasMain := false
	for i, c := range d.Categories {
		switch {
		case mainCategories[c]:
			hasMain = true
		case !additionalCategories[c]:
			bad("categories[%d] %q is not a registered freedesktop category", i, c)
		}
		if slices.Index(d.Categories, c) != i {
			bad("categories[%d] %q is listed twice", i, c)
		}
	}
	if len(d.Categories) > 0 && !hasMain {
		bad("categories needs one main category (AudioVideo, Audio, Video, Development, Education, Game, Graphics, Network, Office, Science, Settings, System or Utility)")
	}

	e := &DesktopEntry{ID: d.ID, Name: d.Name, Comment: d.Comment, Categories: d.Categories, StartupWMClass: d.StartupWMClass}
	if len(spec.Files) == 1 {
		e.Exec = spec.Files[0].Destination
	}
	icon, err := assets.Read(res, d.Icon)
	if err != nil {
		bad("icon: %v", err)
	} else if iconPath, err := hicolorPath(d.ID, icon); err != nil {
		bad("icon %s %v", icon.Path, err)
	} else {
		e.IconPath, e.IconAsset, e.IconSHA256, e.IconFile = iconPath, icon.Path, icon.SHA256, icon.File
	}
	if len(*errs) > n {
		return nil
	}
	return e
}

// hicolorPath checks a PNG or SVG icon and returns its hicolor install path.
func hicolorPath(id string, icon *assets.Asset) (string, error) {
	switch strings.ToLower(path.Ext(icon.Path)) {
	case ".png":
		cfg, err := png.DecodeConfig(bytes.NewReader(icon.Data))
		if err != nil {
			return "", fmt.Errorf("is not a PNG image: %v", err)
		}
		if cfg.Width != cfg.Height || !slices.Contains(hicolorSizes, cfg.Width) {
			return "", fmt.Errorf("is %dx%d; a PNG icon must be square with a hicolor size %v", cfg.Width, cfg.Height, hicolorSizes)
		}
		return fmt.Sprintf("%s/%dx%d/apps/%s.png", HicolorDir, cfg.Width, cfg.Height, id), nil
	case ".svg":
		if !isSVG(icon.Data) {
			return "", fmt.Errorf("is not an SVG document (root element must be svg in the SVG namespace)")
		}
		return fmt.Sprintf("%s/scalable/apps/%s.svg", HicolorDir, id), nil
	}
	return "", fmt.Errorf("must be a .png or .svg file")
}

func isSVG(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if el, ok := tok.(xml.StartElement); ok {
			return el.Name.Local == "svg" && el.Name.Space == "http://www.w3.org/2000/svg"
		}
	}
}
