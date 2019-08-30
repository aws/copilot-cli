// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNoSuchProject(t *testing.T) {
	err := &ErrNoSuchProject{ProjectName: "chicken", AccountID: "12345", Region: "us-west-2"}
	require.EqualError(t, err, "couldn't find a project named chicken in account 12345 and region us-west-2")
}

func TestErrProjectAlreadyExists(t *testing.T) {
	err := &ErrProjectAlreadyExists{ProjectName: "chicken"}
	require.EqualError(t, err, "a project named chicken already exists")
}

func TestErrEnvironmentAlreadyExists(t *testing.T) {
	err := &ErrEnvironmentAlreadyExists{EnvironmentName: "test", ProjectName: "chicken"}
	require.EqualError(t, err, "environment test already exists in project chicken")
}

func TestErrNoSuchEnvironment(t *testing.T) {
	err := &ErrNoSuchEnvironment{EnvironmentName: "test", ProjectName: "chicken"}
	require.EqualError(t, err, "couldn't find environment test in the project chicken")
}
