package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestRunHelpAndInvalidArguments(t *testing.T) {
	service := &fakeService{}
	for _, test := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, ExitOK},
		{[]string{"inspect", "--help"}, ExitOK},
		{[]string{}, ExitInvalidInput},
		{[]string{"unknown"}, ExitInvalidInput},
		{[]string{"inspect", "--pid", "42", "--all"}, ExitInvalidInput},
		{[]string{"verify", "--pid", "42"}, ExitInvalidInput},
		{[]string{"doctor", "--format", "xml"}, ExitInvalidInput},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), test.args, &stdout, &stderr, service); code != test.code {
			t.Fatalf("args %v code = %d, stderr=%q", test.args, code, stderr.String())
		}
	}
}

func TestRunInspectJSONIsOnePureValue(t *testing.T) {
	service := &fakeService{inspect: app.InspectResult{Proofs: []model.Proof{safeProof("UNKNOWN")}}}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "--pid", "42", "--format", "json"}, &stdout, &stderr, service)
	if code != ExitOK || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var value app.InspectResult
	if err := decoder.Decode(&value); err != nil || len(value.Proofs) != 1 {
		t.Fatalf("decode = %v, value=%#v", err, value)
	}
}

func TestRunVerifyExitSeverity(t *testing.T) {
	for _, test := range []struct {
		verdict string
		code    int
	}{
		{"MATCHED", ExitOK}, {"STALE", ExitNegative}, {"LEAKED", ExitNegative}, {"CONFLICT", ExitNegative}, {"NOT_RUNNING", ExitNegative}, {"UNKNOWN", ExitUnknown},
	} {
		service := &fakeService{verify: app.VerifyResult{Proof: safeProof(test.verdict)}}
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"verify", "--pid", "42", "--expectation", "expectation.json", "--format", "json"}, &stdout, &stderr, service)
		if code != test.code || !json.Valid(stdout.Bytes()) || stderr.Len() != 0 {
			t.Fatalf("verdict=%s code=%d stdout=%q stderr=%q", test.verdict, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunSeparatesDiagnosticsFromStdout(t *testing.T) {
	service := &fakeService{err: errors.New("private /Users/example token-secret")}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "--pid", "42", "--format", "json"}, &stdout, &stderr, service)
	if code != ExitInternal || stdout.Len() != 0 || !strings.Contains(stderr.String(), "operation failed") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "/Users/example") || strings.Contains(stderr.String(), "token-secret") {
		t.Fatalf("diagnostic leaked internal error: %q", stderr.String())
	}
}

type fakeService struct {
	inspect app.InspectResult
	verify  app.VerifyResult
	doctor  app.DoctorResult
	err     error
}

func (fake *fakeService) Inspect(context.Context, app.InspectRequest) (app.InspectResult, error) {
	return fake.inspect, fake.err
}

func (fake *fakeService) Verify(context.Context, app.VerifyRequest) (app.VerifyResult, error) {
	return fake.verify, fake.err
}

func (fake *fakeService) Doctor(context.Context) app.DoctorResult { return fake.doctor }

func safeProof(verdict string) model.Proof {
	return model.Proof{ProofID: "sha256:" + strings.Repeat("a", 64), Verdict: verdict, ProofLevel: "PROCESS_OBSERVED", Subject: model.Subject{ID: "runtime", DisplayName: "Runtime", Version: "unknown"}}
}
