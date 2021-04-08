// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskName(t *testing.T) {
	taskStackName := TaskStackName("foo-bar")
	name := taskStackName.TaskName()

	require.Equal(t, name, "bar")
}

func TestNameForEnv(t *testing.T) {
	name := NameForEnv("foo", "bar")

	require.Equal(t, name, "foo-bar")
}

func TestNameForTask(t *testing.T) {
	name := NameForTask("foo")

	require.Equal(t, name, TaskStackName("task-foo"))
}

func TestNameForAppStack(t *testing.T) {
	name := NameForAppStack("foo")

	require.Equal(t, name, "foo-infrastructure-roles")
}

func TestNameForAppStackSet(t *testing.T) {
	name := NameForAppStackSet("foo")

	require.Equal(t, name, "foo-infrastructure")
}
