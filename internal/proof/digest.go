package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/fantasyce/agent-runtime-proof/internal/canonical"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func AssignID(value *model.Proof) error {
	if value == nil {
		return errors.New("proof is nil")
	}
	clone := *value
	clone.ProofID = ""
	encoded, err := canonical.Marshal(clone)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	value.ProofID = "sha256:" + hex.EncodeToString(digest[:])
	return nil
}

func VerifyID(value model.Proof) error {
	want := value.ProofID
	if err := AssignID(&value); err != nil {
		return err
	}
	if value.ProofID != want {
		return errors.New("proof ID does not match protected fields")
	}
	return nil
}
