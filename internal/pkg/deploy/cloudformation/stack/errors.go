// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
)

// ErrTemplateNotFound occurs when we can't find a predefined template.
type ErrTemplateNotFound struct {
	templateLocation string
	parentErr        error
}

func (err *ErrTemplateNotFound) Error() string {
	return fmt.Sprintf("failed to find the cloudformation template at %s", err.templateLocation)
}

// Is returns true if the target's template location and parent error are equal to this error's template location and parent error.
func (err *ErrTemplateNotFound) Is(target error) bool {
	t, ok := target.(*ErrTemplateNotFound)
	if !ok {
		return false
	}
	return (err.templateLocation == t.templateLocation) &&
		(errors.Is(err.parentErr, t.parentErr))
}

// Unwrap returns the original error.
func (err *ErrTemplateNotFound) Unwrap() error {
	return err.parentErr
}
