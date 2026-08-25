package mcpserver

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maximumInventoryLimit = 4096

var (
	errInvalidInput = errors.New("invalid tool input")
	errOperation    = errors.New("operation failed")
)

type Runtime interface {
	Inspect(context.Context, app.InspectRequest) (app.InspectResult, error)
	Verify(context.Context, app.VerifyRequest) (app.VerifyResult, error)
}

type ListCandidatesInput struct {
	HostID    string `json:"host_id,omitempty" jsonschema:"optional host profile identifier"`
	BindingID string `json:"binding_id,omitempty" jsonschema:"optional host binding identifier"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of current-user processes to inspect"`
}

type CandidateSummary struct {
	HostID             string                       `json:"host_id,omitempty"`
	BindingID          string                       `json:"binding_id,omitempty"`
	Platform           model.Platform               `json:"platform"`
	Process            *model.ProcessIdentity       `json:"process"`
	Executable         *model.ExecutableObservation `json:"executable"`
	InaccessibleFields []string                     `json:"inaccessible_fields"`
}

type ListCandidatesOutput struct {
	Candidates []CandidateSummary `json:"candidates"`
}

type InspectRuntimesInput struct {
	PID       int    `json:"pid,omitempty" jsonschema:"positive local process identifier"`
	BindingID string `json:"binding_id,omitempty" jsonschema:"optional host binding identifier"`
	All       bool   `json:"all,omitempty" jsonschema:"inspect a bounded current-user inventory"`
	Limit     int    `json:"limit,omitempty" jsonschema:"inventory limit when all is true"`
}

type InspectRuntimesOutput struct {
	Proofs []model.Proof `json:"proofs"`
}

type VerifyRuntimeInput struct {
	PID               int                `json:"pid,omitempty" jsonschema:"optional positive local process identifier"`
	BindingID         string             `json:"binding_id,omitempty" jsonschema:"optional host binding identifier"`
	ExpectationPath   string             `json:"expectation_path,omitempty" jsonschema:"local expectation JSON file"`
	Expectation       *model.Expectation `json:"expectation,omitempty" jsonschema:"inline expectation with absolute or home-relative roots"`
	KnownPriorDigests []string           `json:"known_prior_digests,omitempty" jsonschema:"directly known prior artifact SHA-256 values"`
}

type VerifyRuntimeOutput struct {
	Proof model.Proof `json:"proof"`
}

func New(runtime Runtime, version string) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "agent-runtime-proof", Version: version},
		&mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}}},
	)
	annotations := safeAnnotations()
	mcp.AddTool(server, &mcp.Tool{
		Name: "list_local_runtime_candidates", Description: "List safe local runtime candidate summaries without expensive artifact hashing.", Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListCandidatesInput) (*mcp.CallToolResult, ListCandidatesOutput, error) {
		if (input.HostID != "" && input.BindingID != "") || input.Limit < 0 || input.Limit > maximumInventoryLimit || (input.BindingID != "" && input.Limit != 0) {
			return nil, ListCandidatesOutput{}, errInvalidInput
		}
		request := app.InspectRequest{All: true, HostID: input.HostID, Limit: input.Limit}
		if input.BindingID != "" {
			request = app.InspectRequest{BindingID: input.BindingID}
		}
		result, err := runtime.Inspect(ctx, request)
		if err != nil {
			return nil, ListCandidatesOutput{}, safeError(err)
		}
		output := ListCandidatesOutput{Candidates: make([]CandidateSummary, 0, len(result.Proofs))}
		for _, value := range result.Proofs {
			hostID, bindingID := "", ""
			if value.HostAttribution != nil {
				hostID, bindingID = value.HostAttribution.HostID, value.HostAttribution.BindingID
			}
			output.Candidates = append(output.Candidates, CandidateSummary{
				HostID: hostID, BindingID: bindingID,
				Platform: value.Platform, Process: value.Observation.Process, Executable: value.Observation.Executable,
				InaccessibleFields: append([]string{}, value.Observation.InaccessibleFields...),
			})
		}
		return nil, output, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_local_runtimes", Description: "Inspect an explicit local PID or a bounded current-user inventory and return observation Proofs.", Annotations: safeAnnotations(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input InspectRuntimesInput) (*mcp.CallToolResult, InspectRuntimesOutput, error) {
		if boolInt(input.PID > 0)+boolInt(input.BindingID != "")+boolInt(input.All) != 1 || input.PID < 0 || input.Limit < 0 || input.Limit > maximumInventoryLimit || (!input.All && input.Limit != 0) {
			return nil, InspectRuntimesOutput{}, errInvalidInput
		}
		result, err := runtime.Inspect(ctx, app.InspectRequest{PID: input.PID, BindingID: input.BindingID, All: input.All, Limit: input.Limit})
		if err != nil {
			return nil, InspectRuntimesOutput{}, safeError(err)
		}
		return nil, InspectRuntimesOutput{Proofs: result.Proofs}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "verify_local_runtime", Description: "Verify an explicit local PID against a trusted local expectation file and return a complete Proof.", Annotations: safeAnnotations(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input VerifyRuntimeInput) (*mcp.CallToolResult, VerifyRuntimeOutput, error) {
		if boolInt(input.PID > 0)+boolInt(input.BindingID != "") != 1 || input.PID < 0 || (input.ExpectationPath == "") == (input.Expectation == nil) || !validDigests(input.KnownPriorDigests) {
			return nil, VerifyRuntimeOutput{}, errInvalidInput
		}
		known := make(map[string]bool, len(input.KnownPriorDigests))
		for _, digest := range input.KnownPriorDigests {
			known[digest] = true
		}
		result, err := runtime.Verify(ctx, app.VerifyRequest{PID: input.PID, BindingID: input.BindingID, ExpectationPath: input.ExpectationPath, Expectation: input.Expectation, KnownPriorDigests: known})
		if err != nil {
			return nil, VerifyRuntimeOutput{}, safeError(err)
		}
		return nil, VerifyRuntimeOutput{Proof: result.Proof}, nil
	})
	return server
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func safeAnnotations() *mcp.ToolAnnotations {
	no := false
	return &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &no, OpenWorldHint: &no}
}

func validDigests(values []string) bool {
	for _, value := range values {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 || value != strings.ToLower(value) {
			return false
		}
	}
	return true
}

func safeError(err error) error {
	if errors.Is(err, app.ErrInvalidInput) {
		return errInvalidInput
	}
	return errOperation
}
