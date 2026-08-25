package versioninfo

import (
	"fmt"
	"regexp"
)

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
	commitPattern  = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

func Format(name, version, commit string) string {
	if !namePattern.MatchString(name) || !versionPattern.MatchString(version) || !commitPattern.MatchString(commit) {
		return "agent-runtime-proof unknown (unknown)"
	}
	return fmt.Sprintf("%s %s (%s)", name, version, commit)
}
