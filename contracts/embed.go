package contracts

import "embed"

// Files contains the authoritative decision, privacy, and threat registries.
//
//go:embed *.json
var Files embed.FS
