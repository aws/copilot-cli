// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrStackSetOutOfDate_Error(t *testing.T) {
	err := &ErrStackSetOutOfDate{
		name:      "demo-infrastructure",
		parentErr: errors.New("some error"),
	}

	require.EqualError(t, err, `stack set "demo-infrastructure" update was out of date (feel free to try again): some error`)
}

func TestErrStackSetNotFound_Error(t *testing.T) {
	err := &ErrStackSetNotFound{
		name: "demo-infrastructure",
	}

	require.EqualError(t, err, `stack set "demo-infrastructure" not found`)
}

func TestErrStackSetInstancesNotFound_Error(t *testing.T) {
	err := &ErrStackSetInstancesNotFound{
		name: "demo-infrastructure",
	}

	require.EqualError(t, err, `stack set "demo-infrastructure" has no instances`)
}
