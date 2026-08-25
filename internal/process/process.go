package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/fantasyce/agent-runtime-proof/internal/model"
	gopsprocess "github.com/shirou/gopsutil/v4/process"
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

func observeArguments(ctx context.Context, pid int, candidate *model.Candidate) {
	value, err := gopsprocess.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		candidate.Inaccessible = append(candidate.Inaccessible, "process.arguments")
		return
	}
	arguments, err := value.CmdlineSliceWithContext(ctx)
	if err != nil || len(arguments) == 0 {
		candidate.Inaccessible = append(candidate.Inaccessible, "process.arguments")
		return
	}
	candidate.ArgumentFingerprints = make([]model.ArgumentFingerprint, len(arguments)-1)
	for index, argument := range arguments[1:] {
		candidate.ArgumentFingerprints[index] = model.ArgumentFingerprint{Position: index + 1, SHA256: hashIdentifier("arp:host-argument:v1", argument)}
	}
}

func hashIdentifier(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + "\x00" + value))
	return "sha256:" + hex.EncodeToString(digest[:])
}
