// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"testing"
)

// checkYAMLRoundtrip validates that the given reference value can be marshalled & unmarshalled without data loss.
func checkYAMLRoundtrip[T any](t *testing.T, ref T) {
	t.Run("roundtrip", func(t *testing.T) {
		roundtrip, err := yaml.Marshal(ref)
		require.NoError(t, err)
		fmt.Printf("marshalled form:\n%s\n", string(roundtrip))
		var rt T
		require.NoError(t, yaml.Unmarshal(roundtrip, &rt))
		require.Equal(t, ref, rt)
	})
}
