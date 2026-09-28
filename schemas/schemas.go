// Package schemas embeds the public JSON Schemas of the DeskOS resource
// model. The schemas are part of the API contract.
package schemas

import "embed"

// BaseURI prefixes every schema $id.
const BaseURI = "https://deskos.org/schemas/"

// FS holds every schema file, addressed by its path relative to this
// directory.
//
//go:embed common/*.json */v1alpha1/*.json
var FS embed.FS
