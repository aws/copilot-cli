// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerenginetest

import (
	"context"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
)

// Double is a test double for dockerengine.DockerCmdClient
type Double struct {
	StopFn               func(context.Context, string) error
	IsContainerRunningFn func(context.Context, string) (bool, error)
	RunFn                func(context.Context, *dockerengine.RunOptions) error
	BuildFn              func(context.Context, *dockerengine.BuildArguments, io.Writer) error
	ExecFn               func(context.Context, string, io.Writer, string, ...string) error
	IsContainerHealthyFn func(ctx context.Context, containerName string) (bool, error)
	ContainerExitCodeFn  func(ctx context.Context, containerName string) (int, error)
	RmFn                 func(context.Context, string) error
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

// Build calls the stubbed function.
func (d *Double) Build(ctx context.Context, in *dockerengine.BuildArguments, w io.Writer) error {
	if d.BuildFn == nil {
		return nil
	}
	return d.BuildFn(ctx, in, w)
}

// Exec calls the stubbed function.
func (d *Double) Exec(ctx context.Context, container string, out io.Writer, cmd string, args ...string) error {
	if d.ExecFn == nil {
		return nil
	}
	return d.ExecFn(ctx, container, out, cmd, args...)
}

// Rm calls the stubbed function.
func (d *Double) Rm(ctx context.Context, name string) error {
	if d.RmFn == nil {
		return nil
	}
	return d.RmFn(ctx, name)
}

// ContainerExitCode implements orchestrator.DockerEngine.
func (d *Double) ContainerExitCode(ctx context.Context, containerName string) (int, error) {
	if d.ContainerExitCodeFn == nil {
		return 0, nil
	}
	return d.ContainerExitCodeFn(ctx, containerName)
}

// IsContainerHealthy implements orchestrator.DockerEngine.
func (d *Double) IsContainerHealthy(ctx context.Context, containerName string) (bool, error) {
	if d.IsContainerHealthyFn == nil {
		return false, nil
	}
	return d.IsContainerHealthyFn(ctx, containerName)
}
