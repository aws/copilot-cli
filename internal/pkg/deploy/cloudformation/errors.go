// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import "fmt"

// ErrStackAlreadyExists occurs when a CloudFormation stack already exists with a given name.
type ErrStackAlreadyExists struct {
	stackName string
	parentErr error
}

func (err *ErrStackAlreadyExists) Error() string {
	return fmt.Sprintf("stack %s already exists", err.stackName)
}

// Unwrap returns the original CloudFormation error.
func (err *ErrStackAlreadyExists) Unwrap() error {
	return err.parentErr
}
