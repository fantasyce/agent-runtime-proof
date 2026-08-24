//go:build windows

package artifact

import "os"

func fileChangeToken(info os.FileInfo) string { return info.ModTime().UTC().String() }
