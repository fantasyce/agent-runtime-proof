package hostprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	profileassets "github.com/fantasyce/agent-runtime-proof/profiles"
)

type Catalog struct{ profiles map[string]Profile }

func LoadEmbeddedCatalog() (Catalog, error) {
	entries, err := fs.Glob(profileassets.Files, "hosts/*.json")
	if err != nil {
		return Catalog{}, err
	}
	result := Catalog{profiles: make(map[string]Profile, len(entries))}
	for _, name := range entries {
		document, err := profileassets.Files.ReadFile(name)
		if err != nil {
			return Catalog{}, fmt.Errorf("read host profile: %w", err)
		}
		profile, err := decodeProfile(document)
		if err != nil {
			return Catalog{}, fmt.Errorf("decode host profile %s: %w", name, err)
		}
		if _, exists := result.profiles[profile.HostID]; exists {
			return Catalog{}, fmt.Errorf("duplicate host profile %q", profile.HostID)
		}
		result.profiles[profile.HostID] = profile
	}
	if len(result.profiles) == 0 {
		return Catalog{}, errors.New("host profile catalog is empty")
	}
	return result, nil
}

func (catalog Catalog) HostIDs() []string {
	result := make([]string, 0, len(catalog.profiles))
	for hostID := range catalog.profiles {
		result = append(result, hostID)
	}
	sort.Strings(result)
	return result
}

func (catalog Catalog) Host(hostID string) (Profile, bool) {
	profile, ok := catalog.profiles[hostID]
	if !ok {
		return Profile{}, false
	}
	return cloneProfile(profile), true
}

func ValidateProfile(document []byte) error {
	_, err := decodeProfile(document)
	return err
}

func decodeProfile(document []byte) (Profile, error) {
	if err := contract.ValidateHostProfile(document); err != nil {
		return Profile{}, err
	}
	var result Profile
	if err := json.Unmarshal(document, &result); err != nil {
		return Profile{}, err
	}
	if err := validateProfileSemantics(result); err != nil {
		return Profile{}, err
	}
	return result, nil
}

func validateProfileSemantics(profile Profile) error {
	platforms := make(map[string]bool, len(profile.Platforms))
	for _, platform := range profile.Platforms {
		platforms[platform] = true
	}
	for _, matcher := range profile.ProcessMatchers {
		for _, basename := range matcher.Basenames {
			if strings.ContainsAny(basename, "/\\\x00") {
				return errors.New("process basename contains a path separator or NUL")
			}
		}
	}
	sources := map[string]bool{}
	for _, source := range profile.ConfigSources {
		if sources[source.SourceID] {
			return fmt.Errorf("duplicate config source %q", source.SourceID)
		}
		sources[source.SourceID] = true
		if source.MaximumBytes <= 0 || source.MaximumBytes > 1<<20 {
			return errors.New("config source has invalid byte limit")
		}
		for _, platform := range source.Platforms {
			if !platforms[platform] {
				return errors.New("config source platform is outside host platforms")
			}
		}
		for _, candidate := range source.CandidatePaths {
			if err := validateCandidatePath(candidate); err != nil {
				return err
			}
		}
		for _, field := range source.SecretFields {
			if strings.ContainsAny(field, "=:\x00${}") {
				return errors.New("secret field is not a safe identifier")
			}
		}
	}
	return nil
}

func validateCandidatePath(value string) error {
	if strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") {
		return errors.New("candidate path contains an unsupported character")
	}
	if !strings.HasPrefix(value, "${HOME}/") && !strings.HasPrefix(value, "${WORKSPACE}/") {
		return errors.New("candidate path must start with a bounded root placeholder")
	}
	remainder := strings.NewReplacer("${HOME}", "", "${WORKSPACE}", "", "${PROFILE}", "profile").Replace(value)
	if strings.Contains(remainder, "${") {
		return errors.New("candidate path contains an unknown placeholder")
	}
	for _, component := range strings.Split(remainder, "/") {
		if component == ".." || component == "." {
			return errors.New("candidate path contains traversal")
		}
	}
	return nil
}

func cloneProfile(profile Profile) Profile {
	encoded, _ := json.Marshal(profile)
	var result Profile
	_ = json.Unmarshal(encoded, &result)
	return result
}
