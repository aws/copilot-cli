// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/require"
)

func Test_parseRules(t *testing.T) {
	testCases := map[string]struct {
		inRules []Rule

		wantedNodeUpserter func() []nodeUpserter
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

			wantedError: fmt.Errorf("invalid override path segment \"ContainerDefinition[0][0]\": segments must be of the form \"array[0]\", \"array[-]\" or \"key\""),
		},
		"error when invalid rule path with bad sequence index": {
			inRules: []Rule{
				{
					Path: "ContainerDefinition[0-]",
				},
			},

			wantedError: fmt.Errorf("invalid override path segment \"ContainerDefinition[0-]\": segments must be of the form \"array[0]\", \"array[-]\" or \"key\""),
		},
		"success": {
			inRules: []Rule{
				{
					Path: "ContainerDefinitions[0].Ulimits[-].HardLimit",
					Value: yaml.Node{
						Value: "testNode",
					},
				},
			},
			wantedNodeUpserter: func() []nodeUpserter {
				node3 := &mapUpsertNode{
					upsertNode: upsertNode{
						key: "HardLimit",
						valueToInsert: &yaml.Node{
							Value: "testNode",
						},
					},
				}
				node2 := &seqIdxUpsertNode{
					upsertNode: upsertNode{
						key:  "Ulimits",
						next: node3,
					},
					appendToLast: true,
				}
				node1 := &seqIdxUpsertNode{
					upsertNode: upsertNode{
						key:  "ContainerDefinitions",
						next: node2,
					},
					index: 0,
				}
				return []nodeUpserter{node1}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := parseRules(tc.inRules)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedNodeUpserter(), got)
			}
		})
	}
}
