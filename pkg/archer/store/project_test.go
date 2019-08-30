// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProject_Marshal(t *testing.T) {
	// GIVEN
	project := &Project{
		Name:    "chicken",
		Version: "1.0",
	}

	want := `{"name":"chicken","version":"1.0"}`

	// WHEN
	got, _ := project.Marshal()
	require.Equal(t, want, got)
}
