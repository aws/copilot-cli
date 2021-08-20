// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package apprunner

import (
	"fmt"
)

// ErrWaitServiceOperationFailed occurs when the service operation failed.
type ErrWaitServiceOperationFailed struct {
	operationId string
}

func (e *ErrWaitServiceOperationFailed) Error() string {
	return fmt.Sprintf("operation failed %s", e.operationId)
}

// Timeout allows ErrWaitServiceOperationFailed to implement a timeout error interface.
func (e *ErrWaitServiceOperationFailed) Timeout() bool {
	return true
}
