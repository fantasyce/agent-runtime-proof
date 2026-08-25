package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestHumanOutputIsStableAndPathFree(t *testing.T) {
	service := &fakeService{
		inspect: app.InspectResult{Proofs: []model.Proof{safeProof("UNKNOWN")}},
		doctor:  app.DoctorResult{Status: "ok", Platform: model.Platform{OS: "darwin", Arch: "arm64"}, Capabilities: []string{"process-observation"}, Limitations: []string{"host-profiles unavailable in Phase 1A"}},
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"inspect", "--pid", "42"}, "SUBJECT  VERDICT  PROOF LEVEL       PROOF ID\nRuntime  UNKNOWN  PROCESS_OBSERVED  sha256:aaaaaaaaaaaa…\n"},
		{[]string{"doctor"}, "STATUS  PLATFORM      CAPABILITIES\nok      darwin/arm64  process-observation\n\nLimitations:\n- host-profiles unavailable in Phase 1A\n"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), test.args, strings.NewReader(""), &stdout, &stderr, service); code != ExitOK {
			t.Fatalf("code = %d, stderr=%q", code, stderr.String())
		}
		if stdout.String() != test.want {
			t.Fatalf("output mismatch\nwant: %q\n got: %q", test.want, stdout.String())
		}
		if strings.Contains(stdout.String(), "/Users/") {
			t.Fatalf("local path leaked: %q", stdout.String())
		}
	}
}
