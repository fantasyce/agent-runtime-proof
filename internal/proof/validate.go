package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func Validate(document []byte) (model.Proof, error) {
	if err := contract.ValidateProof(document); err != nil {
		return model.Proof{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var value model.Proof
	if err := decoder.Decode(&value); err != nil {
		return model.Proof{}, fmt.Errorf("decode proof: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return model.Proof{}, errors.New("proof contains trailing JSON")
	}
	if err := VerifyID(value); err != nil {
		return model.Proof{}, err
	}
	return value, nil
}
