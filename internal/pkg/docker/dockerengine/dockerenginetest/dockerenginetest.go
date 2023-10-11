package dockerenginetest

import (
	"context"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
)

type Double struct {
	StopFn               func(context.Context, string) error
	IsContainerRunningFn func(context.Context, string) (bool, error)
	RunFn                func(context.Context, *dockerengine.RunOptions) error
}

func (d *Double) Stop(ctx context.Context, name string) error {
	if d.StopFn == nil {
		return nil
	}
	return d.StopFn(ctx, name)
}

func (d *Double) IsContainerRunning(ctx context.Context, name string) (bool, error) {
	if d.IsContainerRunningFn == nil {
		return false, nil
	}
	return d.IsContainerRunningFn(ctx, name)
}

func (d *Double) Run(ctx context.Context, opts *dockerengine.RunOptions) error {
	if d.RunFn == nil {
		return nil
	}
	return d.RunFn(ctx, opts)
}
