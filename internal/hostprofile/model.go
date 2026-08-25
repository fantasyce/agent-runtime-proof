package hostprofile

type Profile struct {
	SchemaVersion   string           `json:"schema_version"`
	HostID          string           `json:"host_id"`
	DisplayName     string           `json:"display_name"`
	FixtureVersion  string           `json:"fixture_version"`
	Platforms       []string         `json:"platforms"`
	ProcessMatchers []ProcessMatcher `json:"process_matchers"`
	ConfigSources   []ConfigSource   `json:"config_sources"`
}

type ProcessMatcher struct {
	Basenames      []string `json:"basenames"`
	BundleIDs      []string `json:"bundle_ids,omitempty"`
	PublisherNames []string `json:"publisher_names,omitempty"`
}

type ConfigSource struct {
	SourceID       string   `json:"source_id"`
	Platforms      []string `json:"platforms"`
	CandidatePaths []string `json:"candidate_paths"`
	Format         string   `json:"format"`
	Dialect        string   `json:"dialect"`
	MaximumBytes   int64    `json:"maximum_bytes"`
	SecretFields   []string `json:"secret_fields"`
}
