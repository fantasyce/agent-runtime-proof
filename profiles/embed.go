package profiles

import "embed"

// Files contains reviewed, versioned host Profiles. Profiles are data only.
//
//go:embed hosts/*.json
var Files embed.FS
