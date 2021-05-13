// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import "fmt"

type ErrMultipleContainersInTaskDef struct {
	taskDefIdentifier string
}

func (e *ErrMultipleContainersInTaskDef) Error() string {
	return fmt.Sprintf("found more than one container in task definition: %s", e.taskDefIdentifier)
}
