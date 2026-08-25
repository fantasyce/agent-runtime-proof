package witness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	internalmodel "github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
	"github.com/fantasyce/agent-runtime-proof/internal/receipt"
	"github.com/fantasyce/agent-runtime-proof/internal/store"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

const maximumCommandArguments = 256

type Clock interface {
	Now() time.Time
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

type Dependencies struct {
	Tool            internalmodel.ToolInfo
	Home            string
	Observer        processobserver.Observer
	Clock           Clock
	LoadExpectation func(string) (expectation.Resolved, error)
	DigestArtifact  func(context.Context, expectation.Resolved, artifact.Clock) (internalmodel.ArtifactObservation, error)
	LookPath        func(string) (string, error)
	EvalSymlinks    func(string) (string, error)
	WriteReceipt    func(string, sdkmodel.LaunchReceipt) (string, error)
}

type Controller struct{ dependencies Dependencies }

type Request struct {
	Command         []string
	ExpectationPath string
}

type PreparedLaunch struct {
	controller      *Controller
	command         string
	arguments       []string
	safeCommand     sdkmodel.CommandObservation
	subject         *sdkmodel.Subject
	expectation     *sdkmodel.ExpectationProjection
	artifact        *sdkmodel.ArtifactObservation
	observationOnly bool
	mu              sync.Mutex
	spawned         bool
}

func NewController(dependencies Dependencies) *Controller {
	if dependencies.Observer == nil {
		dependencies.Observer = processobserver.NewObserver()
	}
	if dependencies.Clock == nil {
		dependencies.Clock = wallClock{}
	}
	if dependencies.LoadExpectation == nil {
		dependencies.LoadExpectation = expectation.Load
	}
	if dependencies.DigestArtifact == nil {
		dependencies.DigestArtifact = artifact.Digest
	}
	if dependencies.LookPath == nil {
		dependencies.LookPath = exec.LookPath
	}
	if dependencies.EvalSymlinks == nil {
		dependencies.EvalSymlinks = filepath.EvalSymlinks
	}
	if dependencies.WriteReceipt == nil {
		dependencies.WriteReceipt = store.WriteReceipt
	}
	return &Controller{dependencies: dependencies}
}

func (controller *Controller) PrepareLaunch(ctx context.Context, request Request) (*PreparedLaunch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(request.Command) == 0 || len(request.Command) > maximumCommandArguments {
		return nil, errors.New("invalid witness command")
	}
	for _, value := range request.Command {
		if value == "" || strings.ContainsRune(value, 0) {
			return nil, errors.New("invalid witness command")
		}
	}
	resolvedExecutable, err := controller.dependencies.LookPath(request.Command[0])
	if err != nil {
		return nil, errors.New("witness executable is unavailable")
	}
	resolvedExecutable, err = filepath.Abs(resolvedExecutable)
	if err != nil {
		return nil, errors.New("witness executable cannot be resolved")
	}
	resolvedExecutable, err = controller.dependencies.EvalSymlinks(resolvedExecutable)
	if err != nil {
		return nil, errors.New("witness executable cannot be resolved")
	}
	arguments := append([]string{}, request.Command[1:]...)
	prepared := &PreparedLaunch{
		controller: controller, command: resolvedExecutable, arguments: arguments,
		safeCommand: commandProjection(resolvedExecutable, arguments), observationOnly: request.ExpectationPath == "",
	}
	if request.ExpectationPath == "" {
		return prepared, nil
	}
	resolvedExpectation, err := controller.dependencies.LoadExpectation(request.ExpectationPath)
	if err != nil {
		return nil, errors.New("witness expectation is invalid")
	}
	if err := bindCommand(resolvedExpectation, resolvedExecutable, arguments, controller.dependencies.EvalSymlinks); err != nil {
		return nil, err
	}
	observedArtifact, err := controller.dependencies.DigestArtifact(ctx, resolvedExpectation, controller.dependencies.Clock)
	if err != nil {
		return nil, errors.New("witness artifact cannot be observed")
	}
	if observedArtifact.SHA256 != resolvedExpectation.Value.Artifact.SHA256 {
		return nil, errors.New("witness artifact does not match expectation")
	}
	prepared.subject = &sdkmodel.Subject{
		ID: resolvedExpectation.Value.Subject.ID, DisplayName: resolvedExpectation.Value.Subject.DisplayName, Version: resolvedExpectation.Value.Subject.Version,
	}
	prepared.expectation = &sdkmodel.ExpectationProjection{
		SourceKind: resolvedExpectation.Value.Source.Kind, SourceLocatorHash: resolvedExpectation.Value.Source.LocatorHash,
		Trust: resolvedExpectation.Value.Source.Trust, ExpectedVersion: resolvedExpectation.Value.Subject.Version,
		ExpectedArtifactSHA256: resolvedExpectation.Value.Artifact.SHA256,
	}
	prepared.artifact = &sdkmodel.ArtifactObservation{
		SHA256: observedArtifact.SHA256, FileCount: observedArtifact.FileCount,
		ByteCount: observedArtifact.ByteCount, DurationMS: observedArtifact.DurationMS,
	}
	return prepared, nil
}

func (prepared *PreparedLaunch) Command() (string, []string) {
	return prepared.command, append([]string{}, prepared.arguments...)
}

func (prepared *PreparedLaunch) Spawned(ctx context.Context, pid int) (sdkmodel.LaunchReceipt, error) {
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	if prepared.spawned {
		return sdkmodel.LaunchReceipt{}, errors.New("witness launch was already recorded")
	}
	if pid <= 0 {
		return sdkmodel.LaunchReceipt{}, errors.New("witness process identity is invalid")
	}
	candidate, err := prepared.controller.dependencies.Observer.Snapshot(ctx, pid)
	if err != nil {
		return sdkmodel.LaunchReceipt{}, errors.New("witness process identity is unavailable")
	}
	if candidate.Process.PID != pid || candidate.Process.CreatedAtUnixNano == "" || candidate.Process.BootIDHash == "" {
		return sdkmodel.LaunchReceipt{}, errors.New("witness process identity is incomplete")
	}
	if err := prepared.controller.dependencies.Observer.Revalidate(ctx, candidate); err != nil {
		return sdkmodel.LaunchReceipt{}, errors.New("witness process identity changed")
	}
	declaredExecutable, err := prepared.controller.dependencies.EvalSymlinks(candidate.DeclaredExecutablePath)
	if err != nil || !samePath(declaredExecutable, prepared.command) {
		return sdkmodel.LaunchReceipt{}, errors.New("witness process executable does not match launch")
	}
	value, err := receipt.Build(receipt.Input{
		CreatedAt: prepared.controller.dependencies.Clock.Now(),
		Tool: sdkmodel.ToolInfo{
			Name: prepared.controller.dependencies.Tool.Name, Version: prepared.controller.dependencies.Tool.Version,
			Commit: prepared.controller.dependencies.Tool.Commit, Toolchain: prepared.controller.dependencies.Tool.Toolchain,
		},
		Platform: sdkmodel.Platform{OS: candidate.Platform.OS, Arch: candidate.Platform.Arch},
		Subject:  prepared.subject,
		Process: sdkmodel.ProcessIdentity{
			PID: candidate.Process.PID, CreatedAtUnixNano: candidate.Process.CreatedAtUnixNano, BootIDHash: candidate.Process.BootIDHash,
		},
		Command: prepared.safeCommand, Expectation: prepared.expectation, Artifact: prepared.artifact,
		ObservationOnly: prepared.observationOnly,
	})
	if err != nil {
		return sdkmodel.LaunchReceipt{}, errors.New("witness launch receipt is invalid")
	}
	home, err := resolveHome(prepared.controller.dependencies.Home)
	if err != nil {
		return sdkmodel.LaunchReceipt{}, errors.New("witness state root is unavailable")
	}
	if _, err := prepared.controller.dependencies.WriteReceipt(home, value); err != nil {
		return sdkmodel.LaunchReceipt{}, errors.New("witness launch receipt could not be stored")
	}
	prepared.spawned = true
	return value, nil
}

func bindCommand(resolved expectation.Resolved, executable string, arguments []string, evalSymlinks func(string) (string, error)) error {
	entrypoint := filepath.Join(resolved.ArtifactRoot, filepath.FromSlash(resolved.Value.Launch.Entrypoint))
	entrypoint, err := evalSymlinks(entrypoint)
	if err != nil {
		return errors.New("witness entrypoint cannot be resolved")
	}
	switch resolved.Value.Launch.Kind {
	case "native", "declared-tree":
		if !samePath(entrypoint, executable) {
			return errors.New("witness command does not match expectation entrypoint")
		}
	case "interpreter-script":
		if len(arguments) == 0 {
			return errors.New("witness command does not include expectation entrypoint")
		}
		script, err := filepath.Abs(arguments[0])
		if err != nil {
			return errors.New("witness entrypoint argument cannot be resolved")
		}
		script, err = evalSymlinks(script)
		if err != nil || !samePath(entrypoint, script) {
			return errors.New("witness command does not match expectation entrypoint")
		}
	default:
		return errors.New("witness expectation launch kind is invalid")
	}
	fullCommand := append([]string{executable}, arguments...)
	for _, fingerprint := range resolved.Value.Launch.ArgumentFingerprints {
		if fingerprint.Position >= len(fullCommand) || rawDigest(fullCommand[fingerprint.Position]) != fingerprint.SHA256 {
			return errors.New("witness command arguments do not match expectation")
		}
	}
	return nil
}

func commandProjection(executable string, arguments []string) sdkmodel.CommandObservation {
	fingerprints := make([]sdkmodel.ArgumentFingerprint, len(arguments))
	for index, argument := range arguments {
		fingerprints[index] = sdkmodel.ArgumentFingerprint{Position: index + 1, SHA256: "sha256:" + rawDigest(argument)}
	}
	return sdkmodel.CommandObservation{
		ExecutableBasename: filepath.Base(executable), ExecutablePathHash: hashIdentifier("arp:command-path:v1", normalizePath(executable)),
		ArgumentFingerprints: fingerprints,
	}
}

func rawDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func hashIdentifier(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + "\x00" + value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func normalizePath(value string) string {
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
}

func samePath(left, right string) bool { return normalizePath(left) == normalizePath(right) }

func resolveHome(configured string) (string, error) {
	if configured == "" {
		configured = os.Getenv("AGENT_RUNTIME_PROOF_HOME")
	}
	if configured == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configured = filepath.Join(home, ".across", "data", "agent-runtime-proof")
	}
	resolved, err := filepath.Abs(configured)
	if err != nil {
		return "", fmt.Errorf("resolve ARP home: %w", err)
	}
	return resolved, nil
}
