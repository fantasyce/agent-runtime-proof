package schemas

import "embed"

// Files contains the authoritative public JSON Schemas used by the runtime.
//
//go:embed *.schema.json
var Files embed.FS
