// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"strings"
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

type ErrENIInfoNotFoundForTasks struct {
	taskARNs []string
	Errors   []*ecs.ErrTaskENIInfoNotFound
}

func (e *ErrENIInfoNotFoundForTasks) Error() string{
	errorMsgs := make([]string, len(e.Errors))
	for idx, err := range e.Errors {
		errorMsgs[idx]= err.Error()
	}
	return strings.Join(errorMsgs, "\n")
}