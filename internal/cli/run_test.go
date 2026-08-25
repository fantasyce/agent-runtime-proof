package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/fantasyce/agent-runtime-proof/internal/witness"
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
		if code := Run(context.Background(), test.args, strings.NewReader(""), &stdout, &stderr, service); code != test.code {
			t.Fatalf("args %v code = %d, stderr=%q", test.args, code, stderr.String())
		}
	}
}

func TestRunWitnessRequiresDelimiterAndValidatedGracePeriod(t *testing.T) {
	service := &fakeService{}
	for _, args := range [][]string{
		{"witness"},
		{"witness", "helper"},
		{"witness", "--"},
		{"witness", "--grace-period", "0s", "--", "helper"},
		{"witness", "--grace-period", "2m", "--", "helper"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr, service); code != ExitInvalidInput {
			t.Fatalf("args=%#v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunWitnessPassesDirectArgvAndProtocolStreams(t *testing.T) {
	const secret = "token-super-secret"
	service := &fakeService{witnessResult: witness.Result{ExitCode: 7, ReceiptID: "sha256:" + strings.Repeat("a", 64), PID: 42}}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"witness", "--expectation", "expectation.json", "--grace-period", "250ms", "--", "helper", "$(not-a-shell)", secret,
	}, strings.NewReader("request-bytes"), &stdout, &stderr, service)
	if code != 7 || stdout.String() != "response-bytes" || !strings.Contains(stderr.String(), "child-stderr") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if service.lastWitness.ExpectationPath != "expectation.json" || service.lastWitness.GracePeriod != 250*time.Millisecond || service.lastWitness.Command[2] != secret {
		t.Fatalf("request = %#v", service.lastWitness)
	}
	if strings.Contains(stderr.String(), secret) || strings.Contains(stderr.String(), "not-a-shell") {
		t.Fatalf("CLI diagnostic leaked argv: %q", stderr.String())
	}
}

func TestRunWitnessMapsInvalidInputWithoutLeakingError(t *testing.T) {
	service := &fakeService{witnessErr: errors.Join(witness.ErrInvalidInput, errors.New("private /Users/example token-secret"))}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"witness", "--", "helper"}, strings.NewReader(""), &stdout, &stderr, service)
	if code != ExitInvalidInput || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid witness input") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "/Users/example") || strings.Contains(stderr.String(), "token-secret") {
		t.Fatalf("diagnostic leaked internal error: %q", stderr.String())
	}
}

func TestRunWitnessPreservesNativeChildExitCode(t *testing.T) {
	service := &fakeService{witnessResult: witness.Result{ExitCode: 301, ReceiptID: "sha256:" + strings.Repeat("a", 64), PID: 42}}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"witness", "--", "helper"}, strings.NewReader(""), &stdout, &stderr, service)
	if code != 301 {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
}

func TestRunInspectJSONIsOnePureValue(t *testing.T) {
	service := &fakeService{inspect: app.InspectResult{Proofs: []model.Proof{safeProof("UNKNOWN")}}}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "--pid", "42", "--format", "json"}, strings.NewReader(""), &stdout, &stderr, service)
	if code != ExitOK || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var value app.InspectResult
	if err := decoder.Decode(&value); err != nil || len(value.Proofs) != 1 {
		t.Fatalf("decode = %v, value=%#v", err, value)
	}
}

func TestRunBindingSelectorsAndHostDoctor(t *testing.T) {
	service := &fakeService{inspect: app.InspectResult{Proofs: []model.Proof{safeProof("UNKNOWN")}}, verify: app.VerifyResult{Proof: safeProof("MATCHED")}}
	for _, test := range []struct {
		args        []string
		wantInspect string
		wantVerify  string
		wantHost    string
	}{
		{args: []string{"inspect", "--binding", "cursor.arp", "--format", "json"}, wantInspect: "cursor.arp"},
		{args: []string{"verify", "--binding", "cursor.arp", "--expectation", "expectation.json", "--format", "json"}, wantVerify: "cursor.arp"},
		{args: []string{"doctor", "--host", "cursor", "--format", "json"}, wantHost: "cursor"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), test.args, strings.NewReader(""), &stdout, &stderr, service); code != ExitOK || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
			t.Fatalf("args=%v code/output=%q/%q", test.args, stdout.String(), stderr.String())
		}
		if test.wantInspect != "" && service.lastInspect.BindingID != test.wantInspect {
			t.Fatalf("inspect request = %#v", service.lastInspect)
		}
		if test.wantVerify != "" && service.lastVerify.BindingID != test.wantVerify {
			t.Fatalf("verify request = %#v", service.lastVerify)
		}
		if test.wantHost != "" && service.lastDoctor.HostID != test.wantHost {
			t.Fatalf("doctor request = %#v", service.lastDoctor)
		}
	}
	for _, args := range [][]string{{"inspect", "--pid", "42", "--binding", "cursor.arp"}, {"verify", "--pid", "42", "--binding", "cursor.arp", "--expectation", "x"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr, service); code != ExitInvalidInput {
			t.Fatalf("accepted conflicting selectors: %v", args)
		}
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
		code := Run(context.Background(), []string{"verify", "--pid", "42", "--expectation", "expectation.json", "--format", "json"}, strings.NewReader(""), &stdout, &stderr, service)
		if code != test.code || !json.Valid(stdout.Bytes()) || stderr.Len() != 0 {
			t.Fatalf("verdict=%s code=%d stdout=%q stderr=%q", test.verdict, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunVerifyPassesValidatedKnownPriorDigests(t *testing.T) {
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	service := &fakeService{verify: app.VerifyResult{Proof: safeProof("STALE")}}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"verify", "--pid", "42", "--expectation", "expectation.json", "--format", "json",
		"--known-prior-digest", digestA, "--known-prior-digest", digestB,
	}, strings.NewReader(""), &stdout, &stderr, service)
	if code != ExitNegative || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !service.lastVerify.KnownPriorDigests[digestA] || !service.lastVerify.KnownPriorDigests[digestB] || len(service.lastVerify.KnownPriorDigests) != 2 {
		t.Fatalf("known prior digests = %#v", service.lastVerify.KnownPriorDigests)
	}
}

func TestRunVerifyRejectsMalformedKnownPriorDigest(t *testing.T) {
	service := &fakeService{}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"verify", "--pid", "42", "--expectation", "expectation.json",
		"--known-prior-digest", "ABC123",
	}, strings.NewReader(""), &stdout, &stderr, service)
	if code != ExitInvalidInput || stdout.Len() != 0 || service.verifyCalls != 0 {
		t.Fatalf("code=%d stdout=%q calls=%d stderr=%q", code, stdout.String(), service.verifyCalls, stderr.String())
	}
}

func TestRunSeparatesDiagnosticsFromStdout(t *testing.T) {
	service := &fakeService{err: errors.New("private /Users/example token-secret")}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "--pid", "42", "--format", "json"}, strings.NewReader(""), &stdout, &stderr, service)
	if code != ExitInternal || stdout.Len() != 0 || !strings.Contains(stderr.String(), "operation failed") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "/Users/example") || strings.Contains(stderr.String(), "token-secret") {
		t.Fatalf("diagnostic leaked internal error: %q", stderr.String())
	}
}

type fakeService struct {
	inspect       app.InspectResult
	verify        app.VerifyResult
	doctor        app.DoctorResult
	err           error
	lastVerify    app.VerifyRequest
	lastInspect   app.InspectRequest
	lastDoctor    app.DoctorRequest
	verifyCalls   int
	witnessResult witness.Result
	witnessErr    error
	lastWitness   witness.RunRequest
}

func (fake *fakeService) Inspect(_ context.Context, request app.InspectRequest) (app.InspectResult, error) {
	fake.lastInspect = request
	return fake.inspect, fake.err
}

func (fake *fakeService) Verify(_ context.Context, request app.VerifyRequest) (app.VerifyResult, error) {
	fake.lastVerify = request
	fake.verifyCalls++
	return fake.verify, fake.err
}

func (fake *fakeService) Doctor(_ context.Context, request app.DoctorRequest) app.DoctorResult {
	fake.lastDoctor = request
	return fake.doctor
}

func (fake *fakeService) RunWitness(_ context.Context, request witness.RunRequest) (witness.Result, error) {
	fake.lastWitness = request
	if fake.witnessErr != nil {
		return fake.witnessResult, fake.witnessErr
	}
	if request.Stdin != nil {
		_, _ = io.ReadAll(request.Stdin)
	}
	if request.Stdout != nil {
		_, _ = io.WriteString(request.Stdout, "response-bytes")
	}
	if request.Stderr != nil {
		_, _ = io.WriteString(request.Stderr, "child-stderr")
	}
	return fake.witnessResult, nil
}

func safeProof(verdict string) model.Proof {
	return model.Proof{ProofID: "sha256:" + strings.Repeat("a", 64), Verdict: verdict, ProofLevel: "PROCESS_OBSERVED", Subject: model.Subject{ID: "runtime", DisplayName: "Runtime", Version: "unknown"}}
}
