// Package deskos carries DeskOS Core: the resources a deskosctl binary ships
// with, so a workstation compiles without a separate download.
package deskos

import "embed"

// Resources is the DeskOS Core resource tree — platforms, profiles, software,
// desktop, system, workstations and their assets. Its root is "resources",
// the directory a resource root points at.
//
// Core is embedded rather than fetched: it is small, it is versioned by the
// binary that carries it (so a Core and a compiler can never mismatch), and
// the release already builds both from the same commit.
//
//go:embed all:resources
var Resources embed.FS
