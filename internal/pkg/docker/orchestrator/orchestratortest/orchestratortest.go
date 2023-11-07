// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orchestratortest

import "github.com/aws/copilot-cli/internal/pkg/docker/orchestrator"

// Double is a test double for orchestrator.Orchestrator
type Double struct {
	StartFn   func() <-chan error
	RunTaskFn func(orchestrator.Task, ...orchestrator.RunTaskOption)
	StopFn    func()
}

// Start calls the stubbed function.
func (d *Double) Start() <-chan error {
	if d.StartFn == nil {
		return nil
	}
	return d.StartFn()
}

// RunTask calls the stubbed function.
func (d *Double) RunTask(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
	if d.RunTaskFn == nil {
		return
	}
	d.RunTaskFn(task, opts...)
}

// Stop calls the stubbed function.
func (d *Double) Stop() {
	if d.StopFn == nil {
		return
	}
	d.StopFn()
}
