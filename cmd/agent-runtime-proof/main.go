package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/cli"
	"github.com/fantasyce/agent-runtime-proof/internal/mcpserver"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
	"github.com/fantasyce/agent-runtime-proof/internal/versioninfo"
	"github.com/fantasyce/agent-runtime-proof/internal/witness"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "1.0.0-dev"
	commit  = "0000000"
)

type localRuntime struct {
	*app.Service
	WitnessController *witness.Controller
}

func (value *localRuntime) RunWitness(ctx context.Context, request witness.RunRequest) (witness.Result, error) {
	return witness.Run(ctx, value.WitnessController, request)
}

func main() {
	if guarded, code := witness.RunGuardianIfRequested(); guarded {
		os.Exit(code)
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(versioninfo.Format("agent-runtime-proof", version, commit))
		return
	}
	tool := model.ToolInfo{Name: "agent-runtime-proof", Version: version, Commit: commit, Toolchain: runtime.Version()}
	service := app.NewService(processobserver.NewObserver(), tool)
	if len(os.Args) == 2 && os.Args[1] == "mcp" {
		if err := mcpserver.New(service, version).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, "agent-runtime-proof: MCP server failed")
			os.Exit(cli.ExitInternal)
		}
		return
	}
	local := &localRuntime{Service: service, WitnessController: witness.NewController(witness.Dependencies{Tool: tool})}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, local))
}
