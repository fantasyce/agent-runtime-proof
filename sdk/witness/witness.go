// Package witness exposes the launch preparation and receipt API for local
// Agent hosts that embed ARP instead of using the CLI proxy.
package witness

import (
	"context"
	"errors"
	"regexp"

	internalmodel "github.com/fantasyce/agent-runtime-proof/internal/model"
	internalwitness "github.com/fantasyce/agent-runtime-proof/internal/witness"
	sdkmodel "github.com/fantasyce/agent-runtime-proof/sdk/model"
)

var safeBuildValue = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type Config struct {
	Home string
	Tool sdkmodel.ToolInfo
}

type Request struct {
	Command         []string
	ExpectationPath string
}

type Controller struct{ inner *internalwitness.Controller }

type PreparedLaunch struct {
	inner *internalwitness.PreparedLaunch
}

func New(config Config) (*Controller, error) {
	if config.Tool.Name != "agent-runtime-proof" || config.Tool.Version == "" || !safeBuildValue.MatchString(config.Tool.Commit) || config.Tool.Toolchain == "" {
		return nil, errors.New("invalid ARP tool identity")
	}
	return &Controller{inner: internalwitness.NewController(internalwitness.Dependencies{
		Home: config.Home,
		Tool: internalmodel.ToolInfo{
			Name: config.Tool.Name, Version: config.Tool.Version, Commit: config.Tool.Commit, Toolchain: config.Tool.Toolchain,
		},
	})}, nil
}

func (controller *Controller) PrepareLaunch(ctx context.Context, request Request) (*PreparedLaunch, error) {
	if controller == nil || controller.inner == nil {
		return nil, errors.New("witness controller is unavailable")
	}
	prepared, err := controller.inner.PrepareLaunch(ctx, internalwitness.Request{
		Command: append([]string{}, request.Command...), ExpectationPath: request.ExpectationPath,
	})
	if err != nil {
		return nil, err
	}
	return &PreparedLaunch{inner: prepared}, nil
}

func (prepared *PreparedLaunch) Command() (string, []string) {
	if prepared == nil || prepared.inner == nil {
		return "", []string{}
	}
	return prepared.inner.Command()
}

func (prepared *PreparedLaunch) Spawned(ctx context.Context, pid int) (sdkmodel.LaunchReceipt, error) {
	if prepared == nil || prepared.inner == nil {
		return sdkmodel.LaunchReceipt{}, errors.New("prepared witness launch is unavailable")
	}
	return prepared.inner.Spawned(ctx, pid)
}
