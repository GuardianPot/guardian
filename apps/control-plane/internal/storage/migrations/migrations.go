// Package migrations embeds the forward-only Control Plane schema.
package migrations

import "embed"

// Files contains every human-reviewable SQL migration shipped in the binary.
//
//go:embed *.sql
var Files embed.FS

// LatestVersion is the schema version required by this Control Plane build.
const LatestVersion int64 = 5
