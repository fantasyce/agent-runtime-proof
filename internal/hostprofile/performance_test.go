package hostprofile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/fantasyce/agent-runtime-proof/internal/artifact"
	"github.com/fantasyce/agent-runtime-proof/internal/expectation"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

func TestPhase4ReferencePerformance(t *testing.T) {
	if os.Getenv("ARP_RUN_PERFORMANCE") != "1" {
		t.Skip("set ARP_RUN_PERFORMANCE=1 for the reference-runner gate")
	}
	binding := performanceBinding(t)
	candidates := make([]model.Candidate, 1000)
	for index := range candidates {
		candidates[index] = candidateAtPerformance(index+1, filepath.Join(string(filepath.Separator), "other", fmt.Sprintf("runtime-%d", index)))
	}
	candidates[777] = candidateAtPerformance(778, binding.resolvedCommand)
	durations := make([]time.Duration, 100)
	for index := range durations {
		started := time.Now()
		matched, err := binding.Match(candidates)
		if err != nil || matched.Process.PID != 778 {
			t.Fatalf("match = %#v, %v", matched, err)
		}
		durations[index] = time.Since(started)
	}
	if p95 := percentile95(durations); p95 > 250*time.Millisecond {
		t.Fatalf("1,000-candidate p95 = %s", p95)
	}

	temporaryRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(temporaryRoot, "tree")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 20_000; index++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f-%05d", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved := expectation.Resolved{ArtifactRoot: root, AllowedRoots: []string{root}}
	resolved.Value.Artifact = model.ArtifactExpectation{Include: []string{"**"}, Exclude: []string{}, MaxFiles: 20_000, MaxBytes: 256 << 20, MaxDurationMS: 30_000}
	observed, err := artifact.Digest(context.Background(), resolved, performanceClock{})
	if err != nil || observed.FileCount != 20_000 || observed.ByteCount != 0 {
		t.Fatalf("bounded tree = %#v, %v", observed, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err = artifact.Digest(cancelled, resolved, performanceClock{})
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("cancellation = %v after %s", err, time.Since(started))
	}
	t.Logf("candidate_p95=%s tree_duration_ms=%d", percentile95(durations), observed.DurationMS)
}

func performanceBinding(t *testing.T) Binding {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
	command := filepath.Join(string(filepath.Separator), "opt", "arp", "agent-runtime-proof")
	document := []byte(fmt.Sprintf(`{"mcpServers":{"arp":{"command":%q,"args":["mcp"]}}}`, command))
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Discover(context.Background(), Request{HostID: "cursor", Platform: runtimePlatform(), ExplicitConfigPath: path})
	if err != nil || len(result.Bindings) != 1 {
		t.Fatalf("binding = %#v, %v", result, err)
	}
	return result.Bindings[0]
}

func candidateAtPerformance(pid int, path string) model.Candidate {
	return model.Candidate{Process: model.ProcessIdentity{PID: pid}, DeclaredExecutablePath: path, ArgumentFingerprints: []model.ArgumentFingerprint{{Position: 1, SHA256: hashValue("arp:host-argument:v1", "mcp")}}}
}
func percentile95(values []time.Duration) time.Duration {
	copied := append([]time.Duration{}, values...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	return copied[(len(copied)*95+99)/100-1]
}
func runtimePlatform() string { return runtime.GOOS }

type performanceClock struct{}

func (performanceClock) Now() time.Time { return time.Now() }
