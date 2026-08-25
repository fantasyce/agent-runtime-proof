package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/witness"
)

const (
	ExitOK           = 0
	ExitNegative     = 2
	ExitUnknown      = 3
	ExitInvalidInput = 64
	ExitInternal     = 70
)

type Runtime interface {
	Inspect(context.Context, app.InspectRequest) (app.InspectResult, error)
	Verify(context.Context, app.VerifyRequest) (app.VerifyResult, error)
	Doctor(context.Context) app.DoctorResult
}

type WitnessRuntime interface {
	RunWitness(context.Context, witness.RunRequest) (witness.Result, error)
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, runtime Runtime) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		writeUsage(stdout)
		return ExitOK
	}
	if len(args) == 0 {
		writeDiagnostic(stderr, "a command is required")
		return ExitInvalidInput
	}
	switch args[0] {
	case "inspect":
		return runInspect(ctx, args[1:], stdout, stderr, runtime)
	case "verify":
		return runVerify(ctx, args[1:], stdout, stderr, runtime)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout, stderr, runtime)
	case "witness":
		return runWitness(ctx, args[1:], stdin, stdout, stderr, runtime)
	default:
		writeDiagnostic(stderr, "unknown command")
		return ExitInvalidInput
	}
}

func runWitness(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, runtime Runtime) int {
	if helpRequested(args) {
		fmt.Fprintln(stdout, "Usage: agent-runtime-proof witness [--expectation FILE] [--grace-period DURATION] -- COMMAND [ARG...]")
		return ExitOK
	}
	delimiter := -1
	for index, value := range args {
		if value == "--" {
			delimiter = index
			break
		}
	}
	if delimiter < 0 || delimiter+1 >= len(args) {
		writeDiagnostic(stderr, "invalid witness arguments")
		return ExitInvalidInput
	}
	flags := newFlagSet("witness")
	expectationPath := flags.String("expectation", "", "expectation file")
	gracePeriod := flags.Duration("grace-period", 5*time.Second, "graceful shutdown period")
	if err := flags.Parse(args[:delimiter]); err != nil || flags.NArg() != 0 || *gracePeriod <= 0 || *gracePeriod > time.Minute {
		writeDiagnostic(stderr, "invalid witness arguments")
		return ExitInvalidInput
	}
	witnessRuntime, ok := runtime.(WitnessRuntime)
	if !ok {
		writeDiagnostic(stderr, "witness runtime is unavailable")
		return ExitInternal
	}
	result, err := witnessRuntime.RunWitness(ctx, witness.RunRequest{
		Command: append([]string{}, args[delimiter+1:]...), ExpectationPath: *expectationPath,
		Stdin: stdin, Stdout: stdout, Stderr: stderr, GracePeriod: *gracePeriod,
	})
	if err != nil {
		if errors.Is(err, witness.ErrInvalidInput) {
			writeDiagnostic(stderr, "invalid witness input")
			return ExitInvalidInput
		}
		writeDiagnostic(stderr, "witness operation failed")
		return ExitInternal
	}
	if result.ReceiptID != "" {
		fmt.Fprintln(stderr, "\nagent-runtime-proof: launch receipt", result.ReceiptID)
	}
	if result.ExitCode < 0 {
		writeDiagnostic(stderr, "witness child exit status is invalid")
		return ExitInternal
	}
	return result.ExitCode
}

func runInspect(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime) int {
	if helpRequested(args) {
		fmt.Fprintln(stdout, "Usage: agent-runtime-proof inspect (--pid PID | --all) [--limit N] [--format table|json]")
		return ExitOK
	}
	flags := newFlagSet("inspect")
	pid := flags.Int("pid", 0, "process ID")
	all := flags.Bool("all", false, "inspect a bounded current-user inventory")
	limit := flags.Int("limit", 0, "inventory limit")
	format := flags.String("format", "table", "table or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validFormat(*format) || (*pid > 0) == *all || *pid < 0 || *limit < 0 || *limit > 4096 {
		writeDiagnostic(stderr, "invalid inspect arguments")
		return ExitInvalidInput
	}
	result, err := runtime.Inspect(ctx, app.InspectRequest{PID: *pid, All: *all, Limit: *limit})
	if err != nil {
		return handleError(stderr, err)
	}
	if *format == "json" {
		return writeJSON(stdout, stderr, result)
	}
	writer := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(writer, "SUBJECT\tVERDICT\tPROOF LEVEL\tPROOF ID")
	for _, value := range result.Proofs {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", value.Subject.DisplayName, value.Verdict, value.ProofLevel, shortID(value.ProofID))
	}
	if err := writer.Flush(); err != nil {
		writeDiagnostic(stderr, "could not write output")
		return ExitInternal
	}
	return ExitOK
}

func runVerify(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime) int {
	if helpRequested(args) {
		fmt.Fprintln(stdout, "Usage: agent-runtime-proof verify --expectation FILE --pid PID [--known-prior-digest SHA256] [--format table|json]")
		return ExitOK
	}
	flags := newFlagSet("verify")
	pid := flags.Int("pid", 0, "process ID")
	expectationPath := flags.String("expectation", "", "expectation file")
	format := flags.String("format", "table", "table or json")
	var knownPriorDigests digestList
	flags.Var(&knownPriorDigests, "known-prior-digest", "directly known prior artifact SHA-256 (repeatable)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validFormat(*format) || *pid <= 0 || *expectationPath == "" || !knownPriorDigests.valid() {
		writeDiagnostic(stderr, "invalid verify arguments")
		return ExitInvalidInput
	}
	result, err := runtime.Verify(ctx, app.VerifyRequest{PID: *pid, ExpectationPath: *expectationPath, KnownPriorDigests: knownPriorDigests.set()})
	if err != nil {
		return handleError(stderr, err)
	}
	if *format == "json" {
		if code := writeJSON(stdout, stderr, result.Proof); code != ExitOK {
			return code
		}
	} else {
		writer := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(writer, "SUBJECT\tVERDICT\tPROOF LEVEL\tPROOF ID")
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", result.Proof.Subject.DisplayName, result.Proof.Verdict, result.Proof.ProofLevel, shortID(result.Proof.ProofID))
		if err := writer.Flush(); err != nil {
			writeDiagnostic(stderr, "could not write output")
			return ExitInternal
		}
	}
	return verdictExit(result.Proof.Verdict)
}

func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime) int {
	if helpRequested(args) {
		fmt.Fprintln(stdout, "Usage: agent-runtime-proof doctor [--format table|json]")
		return ExitOK
	}
	flags := newFlagSet("doctor")
	format := flags.String("format", "table", "table or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validFormat(*format) {
		writeDiagnostic(stderr, "invalid doctor arguments")
		return ExitInvalidInput
	}
	result := runtime.Doctor(ctx)
	if *format == "json" {
		return writeJSON(stdout, stderr, result)
	}
	writer := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(writer, "STATUS\tPLATFORM\tCAPABILITIES")
	fmt.Fprintf(writer, "%s\t%s/%s\t%s\n", result.Status, result.Platform.OS, result.Platform.Arch, strings.Join(result.Capabilities, ", "))
	if len(result.Limitations) > 0 {
		fmt.Fprintln(writer, "\nLimitations:")
		for _, limitation := range result.Limitations {
			fmt.Fprintf(writer, "- %s\n", limitation)
		}
	}
	if err := writer.Flush(); err != nil {
		writeDiagnostic(stderr, "could not write output")
		return ExitInternal
	}
	return ExitOK
}

func newFlagSet(name string) *flag.FlagSet {
	result := flag.NewFlagSet(name, flag.ContinueOnError)
	result.SetOutput(io.Discard)
	return result
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		writeDiagnostic(stderr, "could not write JSON output")
		return ExitInternal
	}
	return ExitOK
}

func handleError(stderr io.Writer, err error) int {
	if errors.Is(err, app.ErrInvalidInput) {
		writeDiagnostic(stderr, "invalid command input")
		return ExitInvalidInput
	}
	writeDiagnostic(stderr, "operation failed")
	return ExitInternal
}

func verdictExit(verdict string) int {
	switch verdict {
	case "MATCHED":
		return ExitOK
	case "UNKNOWN":
		return ExitUnknown
	default:
		return ExitNegative
	}
}

func validFormat(value string) bool { return value == "table" || value == "json" }

func helpRequested(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h")
}

func shortID(value string) string {
	if len(value) <= 19 {
		return value
	}
	return value[:19] + "…"
}

func writeDiagnostic(stderr io.Writer, message string) {
	fmt.Fprintln(stderr, "agent-runtime-proof:", message)
}

func writeUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: agent-runtime-proof <inspect|verify|witness|doctor|mcp> [options]")
}

type digestList []string

func (values *digestList) String() string { return strings.Join(*values, ",") }

func (values *digestList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func (values digestList) valid() bool {
	for _, value := range values {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 || value != strings.ToLower(value) {
			return false
		}
	}
	return true
}

func (values digestList) set() map[string]bool {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
