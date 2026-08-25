// Package model contains the public, serialization-safe SDK contract types.
package model

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type ToolInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Toolchain string `json:"toolchain"`
}

type Subject struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
}

type ProcessIdentity struct {
	PID               int    `json:"pid"`
	CreatedAtUnixNano string `json:"created_at_unix_nano"`
	BootIDHash        string `json:"boot_id_hash"`
}

type ArgumentFingerprint struct {
	Position int    `json:"position"`
	SHA256   string `json:"sha256"`
}

type CommandObservation struct {
	ExecutableBasename   string                `json:"executable_basename"`
	ExecutablePathHash   string                `json:"executable_path_hash"`
	ArgumentFingerprints []ArgumentFingerprint `json:"argument_fingerprints"`
}

type ExpectationProjection struct {
	SourceKind             string `json:"source_kind"`
	SourceLocatorHash      string `json:"source_locator_hash"`
	Trust                  string `json:"trust"`
	ExpectedVersion        string `json:"expected_version"`
	ExpectedArtifactSHA256 string `json:"expected_artifact_sha256"`
}

type ArtifactObservation struct {
	SHA256     string `json:"sha256"`
	FileCount  int    `json:"file_count"`
	ByteCount  int64  `json:"byte_count"`
	DurationMS int64  `json:"duration_ms"`
}

type PrivacyProjection struct {
	RedactionMode string   `json:"redaction_mode"`
	HomeRedacted  bool     `json:"home_redacted"`
	OmittedFields []string `json:"omitted_fields"`
}

type LaunchReceipt struct {
	SchemaVersion   string                 `json:"schema_version"`
	ReceiptID       string                 `json:"receipt_id,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	Tool            ToolInfo               `json:"tool"`
	Platform        Platform               `json:"platform"`
	Subject         *Subject               `json:"subject"`
	Process         ProcessIdentity        `json:"process"`
	Command         CommandObservation     `json:"command"`
	Expectation     *ExpectationProjection `json:"expectation"`
	Artifact        *ArtifactObservation   `json:"artifact"`
	ObservationOnly bool                   `json:"observation_only"`
	ReasonCodes     []string               `json:"reason_codes"`
	Privacy         PrivacyProjection      `json:"privacy"`
}
