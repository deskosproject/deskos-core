package main

import (
	"fmt"
	"runtime/debug"
)

// Set by release builds with -ldflags "-X main.version=vX.Y.Z -X main.commit=<sha>".
var (
	version = "dev"
	commit  = ""
)

func versionString() string {
	c := commit
	if c == "" {
		c = buildRevision()
	}
	if c == "" {
		c = "unknown"
	}
	return fmt.Sprintf("deskosctl %s (commit %s)\n", version, c)
}

// buildRevision is the VCS revision Go embeds in builds from a Git checkout.
func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev != "" && dirty {
		rev += "-dirty"
	}
	return rev
}
