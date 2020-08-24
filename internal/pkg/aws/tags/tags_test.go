// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	// GIVEN
	projectTags := map[string]string{
		"owner": "boss",
	}
	envTags := map[string]string{
		"owner": "boss2",
		"stage": "test",
	}
	appTags := map[string]string{
		"owner":   "boss3",
		"stage":   "test2",
		"service": "gateway",
	}

	// WHEN
	out := Merge(projectTags, envTags, appTags)

	// THEN
	require.Equal(t, map[string]string{
		"owner":   "boss3",
		"stage":   "test2",
		"service": "gateway",
	}, out)
	// Ensure original tags are not modified.
	require.Equal(t, map[string]string{
		"owner": "boss",
	}, projectTags)
	require.Equal(t, map[string]string{
		"owner": "boss2",
		"stage": "test",
	}, envTags)
}
