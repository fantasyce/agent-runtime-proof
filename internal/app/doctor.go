package app

import (
	"context"
	"os"
	"runtime"
	"strings"

	"github.com/fantasyce/agent-runtime-proof/internal/contract"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
)

type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type DoctorResult struct {
	Status       string         `json:"status"`
	Platform     model.Platform `json:"platform"`
	Checks       []DoctorCheck  `json:"checks"`
	Capabilities []string       `json:"capabilities"`
	Limitations  []string       `json:"limitations"`
}

func (service *Service) Doctor(ctx context.Context) DoctorResult {
	result := DoctorResult{
		Status: "ok", Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Checks: []DoctorCheck{}, Capabilities: []string{"read-only-artifact-digest"},
		Limitations: []string{"host-profiles unavailable in Phase 1A", "MCP unavailable in Phase 1A", "Witness unavailable in Phase 1A"},
	}
	if err := contract.ValidateExpectation([]byte(doctorExpectation)); err != nil {
		result.Status = "warning"
		result.Checks = append(result.Checks, DoctorCheck{Name: "embedded-contracts", Status: "error", Detail: "embedded contract validation failed"})
	} else {
		result.Capabilities = append(result.Capabilities, "embedded-contracts")
		result.Checks = append(result.Checks, DoctorCheck{Name: "embedded-contracts", Status: "ok", Detail: "embedded schemas are available"})
	}
	if service.Observer == nil {
		result.Status = "warning"
		result.Checks = append(result.Checks, DoctorCheck{Name: "process-observation", Status: "error", Detail: "process observer is unavailable"})
		return result
	}
	if _, err := service.Observer.Snapshot(ctx, os.Getpid()); err != nil {
		result.Status = "warning"
		result.Checks = append(result.Checks, DoctorCheck{Name: "process-observation", Status: "warning", Detail: "current process could not be observed without additional access"})
	} else {
		result.Capabilities = append(result.Capabilities, "process-observation")
		result.Checks = append(result.Checks, DoctorCheck{Name: "process-observation", Status: "ok", Detail: "current process is observable"})
	}
	return result
}

var doctorExpectation = strings.ReplaceAll(`{
  "schema_version":"agent-runtime-expectation/1.0",
  "subject":{"id":"doctor","display_name":"Doctor","version":"1"},
  "launch":{"kind":"native","entrypoint":"runtime","argument_fingerprints":[]},
  "artifact":{"root":"runtime","include":["runtime"],"exclude":[],"sha256":"DIGEST","max_files":1,"max_bytes":1,"max_duration_ms":1},
  "policy":{"allowed_roots":["runtime"],"allow_symlinks":false},
  "source":{"kind":"user-file","locator_hash":"DIGEST","trust":"declared"}
}`, "DIGEST", strings.Repeat("0", 64))
