package receipt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/canonical"
	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

type Input struct {
	CreatedAt       time.Time
	Tool            sdkmodel.ToolInfo
	Platform        sdkmodel.Platform
	Subject         *sdkmodel.Subject
	Process         sdkmodel.ProcessIdentity
	Command         sdkmodel.CommandObservation
	Expectation     *sdkmodel.ExpectationProjection
	Artifact        *sdkmodel.ArtifactObservation
	ObservationOnly bool
}

func Build(input Input) (sdkmodel.LaunchReceipt, error) {
	if input.Command.ArgumentFingerprints == nil {
		input.Command.ArgumentFingerprints = []sdkmodel.ArgumentFingerprint{}
	}
	value := sdkmodel.LaunchReceipt{
		SchemaVersion: "agent-runtime-launch-receipt/1.0",
		CreatedAt:     input.CreatedAt.UTC().Format(time.RFC3339Nano),
		Tool:          input.Tool, Platform: input.Platform, Subject: input.Subject,
		Process: input.Process, Command: input.Command, Expectation: input.Expectation,
		Artifact: input.Artifact, ObservationOnly: input.ObservationOnly,
		ReasonCodes: []string{},
		Privacy: sdkmodel.PrivacyProjection{
			RedactionMode: "safe-default", HomeRedacted: true,
			OmittedFields: []string{"command.argv", "process.environment", "process.command_line", "filesystem.paths"},
		},
	}
	if input.ObservationOnly {
		value.ReasonCodes = []string{"WITNESS_EXPECTATION_MISSING"}
	}
	if err := AssignID(&value); err != nil {
		return sdkmodel.LaunchReceipt{}, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return sdkmodel.LaunchReceipt{}, err
	}
	if err := contract.ValidateLaunchReceipt(encoded); err != nil {
		return sdkmodel.LaunchReceipt{}, err
	}
	return value, nil
}

func AssignID(value *sdkmodel.LaunchReceipt) error {
	if value == nil {
		return errors.New("launch receipt is nil")
	}
	clone := *value
	clone.ReceiptID = ""
	encoded, err := canonical.Marshal(clone)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	value.ReceiptID = "sha256:" + hex.EncodeToString(digest[:])
	return nil
}

func VerifyID(value sdkmodel.LaunchReceipt) error {
	want := value.ReceiptID
	if err := AssignID(&value); err != nil {
		return err
	}
	if value.ReceiptID != want {
		return errors.New("launch receipt ID does not match protected fields")
	}
	return nil
}

func Validate(document []byte) (sdkmodel.LaunchReceipt, error) {
	if err := contract.ValidateLaunchReceipt(document); err != nil {
		return sdkmodel.LaunchReceipt{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var value sdkmodel.LaunchReceipt
	if err := decoder.Decode(&value); err != nil {
		return sdkmodel.LaunchReceipt{}, fmt.Errorf("decode launch receipt: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return sdkmodel.LaunchReceipt{}, errors.New("launch receipt contains trailing JSON")
	}
	if err := VerifyID(value); err != nil {
		return sdkmodel.LaunchReceipt{}, err
	}
	return value, nil
}
