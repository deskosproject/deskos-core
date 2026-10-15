package model

// The package managers a Platform may declare. DeskOS keeps one package model;
// only the rendering differs (ADR 0015).
const (
	PackageManagerRpm = "rpm"
	PackageManagerDeb = "deb"
)
