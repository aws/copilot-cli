// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrEnvAlreadyExists_Error(t *testing.T) {
	err := &ErrEnvAlreadyExists{Name: "test", Project: "chicken"}
	require.EqualError(t, err, "environment test already exists under project chicken, please specify a different name")
}
