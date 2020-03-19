// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
)

// ErrStackSetOutOfDate occurs when we try to read and then update a StackSet but
// between reading it and actually updating it, someone else either started or completed
// an update.
type ErrStackSetOutOfDate struct {
	projectName string
	parentErr   error
}

func (err *ErrStackSetOutOfDate) Error() string {
	return fmt.Sprintf("cannot update project resources for project %s because the stack set update was out of date (feel free to try again)", err.projectName)
}

// Is returns true if the target's template location and parent error are equal to this error's template location and parent error.
func (err *ErrStackSetOutOfDate) Is(target error) bool {
	t, ok := target.(*ErrStackSetOutOfDate)
	if !ok {
		return false
	}
	return err.projectName == t.projectName
}
