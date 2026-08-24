package main

import (
	"context"
	"os"
	"runtime"

	"github.com/fantasyce/agent-runtime-proof/internal/app"
	"github.com/fantasyce/agent-runtime-proof/internal/cli"
	"github.com/fantasyce/agent-runtime-proof/internal/model"
	processobserver "github.com/fantasyce/agent-runtime-proof/internal/process"
)

var (
	version = "0.1.0-dev"
	commit  = "0000000"
)

func main() {
	tool := model.ToolInfo{Name: "agent-runtime-proof", Version: version, Commit: commit, Toolchain: runtime.Version()}
	service := app.NewService(processobserver.NewObserver(), tool)
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, service))
}
