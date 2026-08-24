package expectation

import (
	"path"
	"strings"
)

func (resolved Resolved) Includes(relativeSlashPath string) bool {
	if validateRelativeSlashPath(relativeSlashPath) != nil {
		return false
	}
	included := false
	for _, pattern := range resolved.Value.Artifact.Include {
		if matchPattern(pattern, relativeSlashPath) {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, pattern := range resolved.Value.Artifact.Exclude {
		if matchPattern(pattern, relativeSlashPath) {
			return false
		}
	}
	return true
}

func validateRelativeSlashPath(value string) error {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return path.ErrBadPattern
	}
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return path.ErrBadPattern
		}
	}
	return nil
}

func validatePattern(pattern string) error {
	if err := validateRelativeSlashPath(pattern); err != nil {
		return err
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, "probe"); err != nil {
			return err
		}
	}
	return nil
}

func matchPattern(pattern, candidate string) bool {
	if validatePattern(pattern) != nil || validateRelativeSlashPath(candidate) != nil {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(candidate, "/"))
}

func matchSegments(pattern, candidate []string) bool {
	if len(pattern) == 0 {
		return len(candidate) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], candidate) {
			return true
		}
		return len(candidate) > 0 && matchSegments(pattern, candidate[1:])
	}
	if len(candidate) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], candidate[0])
	return err == nil && matched && matchSegments(pattern[1:], candidate[1:])
}
