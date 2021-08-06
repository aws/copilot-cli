// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseRules(t *testing.T) {
	testCases := map[string]struct {
		inRules []Rule

		wantedNodeUpserter []nodeUpserter
		wantedError        error
	}{
		"error when empty rule path": {
			inRules: []Rule{
				{
					Path: "",
				},
			},

			wantedError: fmt.Errorf("rule path is empty"),
		},
		"error when invalid rule path with nested sequence": {
			inRules: []Rule{
				{
					Path: "ContainerDefinition[0][0]",
				},
			},

			wantedError: fmt.Errorf("unrecognized path segment pattern ContainerDefinition[0][0]. Valid path segment examples are \"xyz[0]\", \"xyz[-]\" or \"xyz\""),
		},
		"error when invalid rule path with bad sequence index": {
			inRules: []Rule{
				{
					Path: "ContainerDefinition[0-]",
				},
			},

			wantedError: fmt.Errorf("unrecognized path segment pattern ContainerDefinition[0-]. Valid path segment examples are \"xyz[0]\", \"xyz[-]\" or \"xyz\""),
		},
		"success": {
			inRules: []Rule{
				{
					Path: "ContainerDefinitions[0].Ulimits[-].HardLimit",
				},
			},
			wantedNodeUpserter: []nodeUpserter{nil},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := parseRules(tc.inRules)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedNodeUpserter, got)
			}
		})
	}
}
