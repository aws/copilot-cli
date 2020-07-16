// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import "fmt"

type errRunTask struct {
	groupName string
	parentErr error
}

func (e *errRunTask) Error() string {
	return fmt.Sprintf("run task %s: %v", e.groupName, e.parentErr)
}
