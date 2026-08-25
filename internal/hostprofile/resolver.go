package hostprofile

import (
	"context"
	"os"
	"runtime"
	"strings"
)

type Resolver struct {
	Platform    string
	Home        string
	Workspace   string
	ProfileName string
}

func NewLocalResolver() *Resolver {
	home, _ := os.UserHomeDir()
	workspace, _ := os.Getwd()
	return &Resolver{Platform: runtime.GOOS, Home: home, Workspace: workspace, ProfileName: "default"}
}

func (resolver *Resolver) Bindings(ctx context.Context, hostID string) ([]Binding, error) {
	if resolver == nil {
		return nil, newError("HOST_PROFILE_NOT_FOUND")
	}
	hostIDs := []string{hostID}
	if hostID == "" {
		catalog, err := LoadEmbeddedCatalog()
		if err != nil {
			return nil, newError("HOST_PROFILE_INVALID")
		}
		hostIDs = catalog.HostIDs()
	}
	result := []Binding{}
	for _, current := range hostIDs {
		discovered, err := Discover(ctx, Request{HostID: current, Platform: resolver.Platform, Home: resolver.Home, Workspace: resolver.Workspace, ProfileName: resolver.ProfileName})
		if err != nil {
			return nil, err
		}
		result = append(result, discovered.Bindings...)
	}
	return result, nil
}

func (resolver *Resolver) Binding(ctx context.Context, bindingID string) (Binding, error) {
	separator := strings.IndexByte(bindingID, '.')
	if separator <= 0 || separator == len(bindingID)-1 {
		return Binding{}, newError("HOST_BINDING_NOT_FOUND")
	}
	bindings, err := resolver.Bindings(ctx, bindingID[:separator])
	if err != nil {
		return Binding{}, err
	}
	for _, binding := range bindings {
		if binding.ID == bindingID {
			return binding, nil
		}
	}
	return Binding{}, newError("HOST_BINDING_NOT_FOUND")
}
