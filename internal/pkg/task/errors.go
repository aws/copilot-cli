// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
)

var (
	errNoSubnetFound = errors.New("no subnets found")

	errVPCGetterNil     = errors.New("vpc getter is not set")
	errClusterGetterNil = errors.New("cluster getter is not set")
	errStarterNil       = errors.New("starter is not set")
)

type errRunTask struct {
	groupName string
	parentErr error
}

func (e *errRunTask) Error() string {
	return fmt.Sprintf("run task %s: %v", e.groupName, e.parentErr)
}

type errGetDefaultCluster struct {
	parentErr error
}

func (e *errGetDefaultCluster) Error() string {
	return fmt.Sprintf("get default cluster: %v", e.parentErr)
}

type errExitCode struct {
	containerName string
	taskId        string
	exitCode      int64
}

func (e *errExitCode) Error() string {
	return fmt.Sprintf("Container %s in task %s exited with status code %d", e.containerName, e.taskId, e.exitCode)
}
