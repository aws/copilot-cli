package dockerenginetest

import (
	"context"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
)

// Double is a test double for dockerengine.DockerCmdClient
type Double struct {
	StopFn               func(context.Context, string) error
	IsContainerRunningFn func(context.Context, string) (bool, error)
	RunFn                func(context.Context, *dockerengine.RunOptions) error
}

// Stop calls the stubbed function.
func (d *Double) Stop(ctx context.Context, name string) error {
	if d.StopFn == nil {
		return nil
	}
	return d.StopFn(ctx, name)
}

// IsContainerRunning calls the stubbed function.
func (d *Double) IsContainerRunning(ctx context.Context, name string) (bool, error) {
	if d.IsContainerRunningFn == nil {
		return false, nil
	}
	return d.IsContainerRunningFn(ctx, name)
}

// Run calls the stubbed function.
func (d *Double) Run(ctx context.Context, opts *dockerengine.RunOptions) error {
	if d.RunFn == nil {
		return nil
	}
	return d.RunFn(ctx, opts)
}
