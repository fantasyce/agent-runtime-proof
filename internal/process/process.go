package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

type Observer interface {
	Snapshot(context.Context, int) (model.Candidate, error)
	List(context.Context, int) ([]model.Candidate, error)
	Revalidate(context.Context, model.Candidate) error
}

func SameIdentity(left, right model.Candidate) bool {
	if left.Process != right.Process {
		return false
	}
	if left.Executable.FileIDHash != "" || right.Executable.FileIDHash != "" {
		return left.Executable.FileIDHash == right.Executable.FileIDHash
	}
	return left.Executable.PathHash == right.Executable.PathHash
}

func hashIdentifier(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + "\x00" + value))
	return "sha256:" + hex.EncodeToString(digest[:])
}
