// Package sampledata embeds the bundled sample dataset so the atlas binary is
// fully self-contained and can run without any external files on disk.
//
// The JSON under data/sample is the single source of truth for the seeded MVP
// graph. It is embedded here for the default (no --data) code path, and the
// exact same files can be loaded from disk via `--data data/sample`.
package sampledata

import "embed"

// FS holds the embedded sample dataset (entities, dependencies, scenarios).
//
//go:embed sample/*.json
var FS embed.FS

// Dir is the directory within FS that contains the dataset files.
const Dir = "sample"
