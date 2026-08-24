package model

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Subject struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
}

type ArgumentFingerprint struct {
	Position int    `json:"position"`
	SHA256   string `json:"sha256"`
}

type LaunchExpectation struct {
	Kind                 string                `json:"kind"`
	Entrypoint           string                `json:"entrypoint"`
	ArgumentFingerprints []ArgumentFingerprint `json:"argument_fingerprints"`
}

type ArtifactExpectation struct {
	Root          string   `json:"root"`
	Include       []string `json:"include"`
	Exclude       []string `json:"exclude"`
	SHA256        string   `json:"sha256"`
	MaxFiles      int      `json:"max_files"`
	MaxBytes      int64    `json:"max_bytes"`
	MaxDurationMS int64    `json:"max_duration_ms"`
}

type ExpectationPolicy struct {
	AllowedRoots  []string `json:"allowed_roots"`
	AllowSymlinks bool     `json:"allow_symlinks"`
}

type ExpectationSource struct {
	Kind        string `json:"kind"`
	LocatorHash string `json:"locator_hash"`
	Trust       string `json:"trust"`
}

type Expectation struct {
	SchemaVersion string              `json:"schema_version"`
	Subject       Subject             `json:"subject"`
	Launch        LaunchExpectation   `json:"launch"`
	Artifact      ArtifactExpectation `json:"artifact"`
	Policy        ExpectationPolicy   `json:"policy"`
	Source        ExpectationSource   `json:"source"`
	Extensions    map[string]any      `json:"extensions,omitempty"`
}

type ProcessIdentity struct {
	PID               int    `json:"pid"`
	CreatedAtUnixNano string `json:"created_at_unix_nano"`
	BootIDHash        string `json:"boot_id_hash"`
}

type ExecutableObservation struct {
	Basename   string `json:"basename"`
	PathHash   string `json:"path_hash"`
	FileIDHash string `json:"file_id_hash,omitempty"`
}

type ArtifactObservation struct {
	SHA256                 string `json:"sha256"`
	FileCount              int    `json:"file_count"`
	ByteCount              int64  `json:"byte_count"`
	DurationMS             int64  `json:"duration_ms"`
	EntrypointFileIdentity string `json:"-"`
}

type Observation struct {
	Process            *ProcessIdentity       `json:"process"`
	Executable         *ExecutableObservation `json:"executable"`
	Artifact           *ArtifactObservation   `json:"artifact"`
	InaccessibleFields []string               `json:"inaccessible_fields"`
}

type Candidate struct {
	Platform               Platform              `json:"platform"`
	Process                ProcessIdentity       `json:"process"`
	Executable             ExecutableObservation `json:"executable"`
	ParentPID              int                   `json:"parent_pid,omitempty"`
	ExecutablePath         string                `json:"-"`
	DeclaredExecutablePath string                `json:"-"`
	ExecutableFileIdentity string                `json:"-"`
	ExecutableDeleted      bool                  `json:"-"`
	Inaccessible           []string              `json:"inaccessible_fields"`
}

type ExpectationProjection struct {
	SourceKind             string `json:"source_kind"`
	SourceLocatorHash      string `json:"source_locator_hash"`
	Trust                  string `json:"trust"`
	ExpectedVersion        string `json:"expected_version"`
	ExpectedArtifactSHA256 string `json:"expected_artifact_sha256"`
}

type HostAttribution struct {
	HostID           string `json:"host_id"`
	ConfigSourceHash string `json:"config_source_hash"`
	Confidence       string `json:"confidence"`
}

type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Toolchain string `json:"toolchain"`
}

type EvidenceItem struct {
	Type       string `json:"type"`
	Digest     string `json:"digest,omitempty"`
	ObservedAt string `json:"observed_at"`
}

type PrivacyProjection struct {
	RedactionMode string   `json:"redaction_mode"`
	HomeRedacted  bool     `json:"home_redacted"`
	OmittedFields []string `json:"omitted_fields"`
}

type Proof struct {
	SchemaVersion   string                 `json:"schema_version"`
	ProofID         string                 `json:"proof_id,omitempty"`
	ObservedAt      string                 `json:"observed_at"`
	Tool            ToolInfo               `json:"tool"`
	Platform        Platform               `json:"platform"`
	Subject         Subject                `json:"subject"`
	Expectation     *ExpectationProjection `json:"expectation"`
	Observation     Observation            `json:"observation"`
	HostAttribution *HostAttribution       `json:"host_attribution"`
	Verdict         string                 `json:"verdict"`
	ProofLevel      string                 `json:"proof_level"`
	ReasonCodes     []string               `json:"reason_codes"`
	Evidence        []EvidenceItem         `json:"evidence"`
	Privacy         PrivacyProjection      `json:"privacy"`
	Limitations     []string               `json:"limitations"`
	Extensions      map[string]any         `json:"extensions,omitempty"`
}
