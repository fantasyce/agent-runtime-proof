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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "0.1.0-dev"
	commit  = "0000000"
)

func main() {
	tool := model.ToolInfo{Name: "agent-runtime-proof", Version: version, Commit: commit, Toolchain: runtime.Version()}
	service := app.NewService(processobserver.NewObserver(), tool)
	if len(os.Args) == 2 && os.Args[1] == "mcp" {
		if err := mcpserver.New(service, version).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, "agent-runtime-proof: MCP server failed")
			os.Exit(cli.ExitInternal)
		}
		return
	}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, service))
}
