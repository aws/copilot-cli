// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	// DesiredStatusStopped represents the desired status "STOPPED" for a task.
	DesiredStatusStopped = ecs.DesiredStatusStopped

	fmtErrTaskStopped = "task %s: %s"
)

// ErrNoDefaultCluster occurs when the default cluster is not found.
var ErrNoDefaultCluster = errors.New("default cluster does not exist")

// ErrWaiterResourceNotReadyForTasks contains the STOPPED reason for the container of the first task that failed to start.
type ErrWaiterResourceNotReadyForTasks struct {
	tasks                  []*Task
	awsErrResourceNotReady error
}

func (e *ErrWaiterResourceNotReadyForTasks) Error() string {
	for _, task := range e.tasks {
		if aws.StringValue(task.LastStatus) != DesiredStatusStopped {
			continue
		}
		taskID, err := TaskID(aws.StringValue(task.TaskArn))
		if err != nil {
			return err.Error()
		}
		// Combine both task stop reason and container stop reason.
		var errMsg string
		if task.StoppedReason != nil {
			errMsg = aws.StringValue(task.StoppedReason)
		}
		// TODO: generalize this to be any essential container.
		container := task.Containers[0] // NOTE: right now we only support one container per task
		if aws.StringValue(container.LastStatus) == DesiredStatusStopped {
			if container.Reason != nil {
				errMsg = fmt.Sprintf("%s: %s", errMsg, aws.StringValue(container.Reason))
			}
		}
		if errMsg != "" {
			return fmt.Sprintf(fmtErrTaskStopped, taskID[:shortTaskIDLength], errMsg)
		}
	}
	return e.awsErrResourceNotReady.Error()
}
