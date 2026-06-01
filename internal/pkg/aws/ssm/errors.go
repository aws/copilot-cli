// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssm

import "fmt"

// ErrParameterAlreadyExists occurs when the parameter with name already existed.
type ErrParameterAlreadyExists struct {
	name string
}

func (e *ErrParameterAlreadyExists) Error() string {
	return fmt.Sprintf("parameter %s already exists", e.name)
}

// ErrParameterNotFound occurs when the parameter with name does not exist.
type ErrParameterNotFound struct {
	name      string
	parentErr error
}

func (e *ErrParameterNotFound) Error() string {
	return fmt.Sprintf("parameter %s does not exist", e.name)
}

func (e *ErrParameterNotFound) Unwrap() error {
	return e.parentErr
}
