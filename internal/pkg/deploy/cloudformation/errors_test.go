// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrStackAlreadyExists_Error(t *testing.T) {
	err := &ErrStackAlreadyExists{
		stackName: "test-stack",
	}
	require.EqualError(t, err, "stack test-stack already exists")
}

func TestErrStackAlreadyExists_Unwrap(t *testing.T) {
	err := &ErrStackAlreadyExists{}
	require.Nil(t, errors.Unwrap(err))

	err = &ErrStackAlreadyExists{parentErr: errors.New("test-error")}
	require.EqualError(t, errors.Unwrap(err), "test-error")
}
