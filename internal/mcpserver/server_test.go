package mcpserver

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerPublishesExactlyThreeClosedWorldReadOnlyTools(t *testing.T) {
	session := connect(t, New(&fakeRuntime{}, "test"))
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"inspect_local_runtimes", "list_local_runtime_candidates", "verify_local_runtime"}
	if len(result.Tools) != len(want) {
		t.Fatalf("tool count = %d, want %d", len(result.Tools), len(want))
	}
	seen := map[string]bool{}
	for _, tool := range result.Tools {
		seen[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("unsafe annotations for %s: %#v", tool.Name, tool.Annotations)
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("missing typed schema for %s", tool.Name)
		}
	}
	for _, name := range want {
		if !seen[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestToolsDelegateValidatedRequestsAndKeepDomainVerdictsSuccessful(t *testing.T) {
	digest := strings.Repeat("a", 64)
	runtime := &fakeRuntime{
		inspect: app.InspectResult{Proofs: []model.Proof{{Verdict: "UNKNOWN", Subject: model.Subject{DisplayName: "fixture"}}}},
		verify:  app.VerifyResult{Proof: model.Proof{Verdict: "STALE", Subject: model.Subject{DisplayName: "fixture"}}},
	}
	session := connect(t, New(runtime, "test"))

	list, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_local_runtime_candidates", Arguments: map[string]any{"limit": 7}})
	if err != nil || list.IsError || runtime.lastInspect != (app.InspectRequest{All: true, Limit: 7}) {
		t.Fatalf("list err=%v isError=%v request=%#v", err, list.IsError, runtime.lastInspect)
	}
	listStructured := list.StructuredContent.(map[string]any)
	candidates := listStructured["candidates"].([]any)
	if fields := candidates[0].(map[string]any)["inaccessible_fields"]; fields == nil {
		t.Fatal("candidate inaccessible_fields encoded as null, want an array")
	}

	inspect, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": 42}})
	if err != nil || inspect.IsError || runtime.lastInspect != (app.InspectRequest{PID: 42}) {
		t.Fatalf("inspect err=%v isError=%v request=%#v", err, inspect.IsError, runtime.lastInspect)
	}

	verify, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "verify_local_runtime", Arguments: map[string]any{
		"pid": 42, "expectation_path": "expectation.json", "known_prior_digests": []any{digest},
	}})
	if err != nil || verify.IsError || runtime.lastVerify.PID != 42 || runtime.lastVerify.ExpectationPath != "expectation.json" || !runtime.lastVerify.KnownPriorDigests[digest] {
		t.Fatalf("verify err=%v isError=%v request=%#v", err, verify.IsError, runtime.lastVerify)
	}
	structured, ok := verify.StructuredContent.(map[string]any)
	if !ok || structured["proof"] == nil {
		t.Fatalf("structured result = %#v", verify.StructuredContent)
	}
}

func TestInvalidAndInternalFailuresAreSafeToolErrors(t *testing.T) {
	runtime := &fakeRuntime{err: errors.New("private /Users/example token-secret")}
	session := connect(t, New(runtime, "test"))

	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": 0}})
	if err != nil || !invalid.IsError {
		t.Fatalf("invalid err=%v result=%#v", err, invalid)
	}
	failed, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": 42}})
	if err != nil || !failed.IsError {
		t.Fatalf("failed err=%v result=%#v", err, failed)
	}
	text := failed.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "/Users/example") || strings.Contains(text, "token-secret") || text != "operation failed" {
		t.Fatalf("unsafe tool error %q", text)
	}
}

func TestToolsAcceptExplicitHostAndBindingSelectors(t *testing.T) {
	runtime := &fakeRuntime{inspect: app.InspectResult{Proofs: []model.Proof{{HostAttribution: &model.HostAttribution{HostID: "cursor", BindingID: "cursor.arp"}}}}, verify: app.VerifyResult{Proof: model.Proof{Verdict: "MATCHED"}}}
	session := connect(t, New(runtime, "test"))
	listed, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_local_runtime_candidates", Arguments: map[string]any{"host_id": "cursor", "limit": 9}})
	if err != nil || listed.IsError || runtime.lastInspect != (app.InspectRequest{All: true, HostID: "cursor", Limit: 9}) {
		t.Fatalf("host list=%#v err=%v request=%#v", listed, err, runtime.lastInspect)
	}
	row := listed.StructuredContent.(map[string]any)["candidates"].([]any)[0].(map[string]any)
	if row["host_id"] != "cursor" || row["binding_id"] != "cursor.arp" {
		t.Fatalf("candidate row = %#v", row)
	}
	inspected, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"binding_id": "cursor.arp"}})
	if err != nil || inspected.IsError || runtime.lastInspect.BindingID != "cursor.arp" {
		t.Fatalf("binding inspect=%#v err=%v request=%#v", inspected, err, runtime.lastInspect)
	}
	verified, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "verify_local_runtime", Arguments: map[string]any{"binding_id": "cursor.arp", "expectation_path": "expectation.json"}})
	if err != nil || verified.IsError || runtime.lastVerify.BindingID != "cursor.arp" {
		t.Fatalf("binding verify=%#v err=%v request=%#v", verified, err, runtime.lastVerify)
	}
}

func TestConcurrentCallsAreIndependentAndCancellationReachesRuntime(t *testing.T) {
	runtime := &concurrentRuntime{started: make(chan struct{}, 1), cancelled: make(chan struct{})}
	session := connect(t, New(runtime, "test"))
	ctx, cancel := context.WithCancel(context.Background())
	type callOutcome struct {
		result *mcp.CallToolResult
		err    error
	}
	done := make(chan callOutcome, 1)
	go func() {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": 41}})
		done <- callOutcome{result: result, err: err}
	}()
	<-runtime.started

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "inspect_local_runtimes", Arguments: map[string]any{"pid": 42}})
	if err != nil || result.IsError {
		t.Fatalf("independent call err=%v result=%#v", err, result)
	}
	cancel()
	cancelled := <-done
	if !errors.Is(cancelled.err, context.Canceled) {
		t.Fatalf("cancellation result=%#v err=%v", cancelled.result, cancelled.err)
	}
	<-runtime.cancelled
}

func connect(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "arp-test", Version: "1"}, &mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

type fakeRuntime struct {
	inspect     app.InspectResult
	verify      app.VerifyResult
	err         error
	lastInspect app.InspectRequest
	lastVerify  app.VerifyRequest
}

type concurrentRuntime struct {
	started   chan struct{}
	cancelled chan struct{}
	once      sync.Once
}

func (runtime *concurrentRuntime) Inspect(ctx context.Context, request app.InspectRequest) (app.InspectResult, error) {
	if request.PID == 41 {
		runtime.once.Do(func() { runtime.started <- struct{}{} })
		<-ctx.Done()
		close(runtime.cancelled)
		return app.InspectResult{}, ctx.Err()
	}
	return app.InspectResult{Proofs: []model.Proof{{Verdict: "UNKNOWN"}}}, nil
}

func (*concurrentRuntime) Verify(context.Context, app.VerifyRequest) (app.VerifyResult, error) {
	return app.VerifyResult{}, nil
}

func (runtime *fakeRuntime) Inspect(_ context.Context, request app.InspectRequest) (app.InspectResult, error) {
	runtime.lastInspect = request
	return runtime.inspect, runtime.err
}

func (runtime *fakeRuntime) Verify(_ context.Context, request app.VerifyRequest) (app.VerifyResult, error) {
	runtime.lastVerify = request
	return runtime.verify, runtime.err
}
