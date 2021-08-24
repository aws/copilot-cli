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

// ErrWaitServiceStableTimeout occurs when the max retries number waiting for the service to be stable exceeded the limit.
type ErrWaitServiceStableTimeout struct {
	maxRetries int
}

func (e *ErrWaitServiceStableTimeout) Error() string {
	return fmt.Sprintf("max retries %v exceeded", e.maxRetries)
}

// Timeout allows ErrWaitServiceStableTimeout to implement a timeout error interface.
func (e *ErrWaitServiceStableTimeout) Timeout() bool {
	return true
}

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

// ErrExecuteCommand occurs when ecs:ExecuteCommand fails.
type ErrExecuteCommand struct {
	err error
}

func (e *ErrExecuteCommand) Error() string {
	return fmt.Sprintf("execute command: %s", e.err.Error())
}

const (
	missingFieldAttachment         = "attachment"
	missingFieldDetailENIID        = "detailENIID"
	missingFieldPrivateIPv4Address = "privateIPv4"
)

// ErrTaskENIInfoNotFound when some ENI information is not found in a ECS task.
type ErrTaskENIInfoNotFound struct {
	MissingField string
	TaskARN      string
}

func (e *ErrTaskENIInfoNotFound) Error() string {
	switch e.MissingField {
	case missingFieldAttachment:
		return fmt.Sprintf("cannot find network interface attachment for task %s", e.TaskARN)
	case missingFieldDetailENIID:
		return fmt.Sprintf("cannot find network interface ID for task %s", e.TaskARN)
	case missingFieldPrivateIPv4Address:
		return fmt.Sprintf("cannot find private IPv4 address for task %s", e.TaskARN)
	}
	return ""
}
