// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package generator generates a command given an ECS service or a workload.
package generator

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"

	"github.com/stretchr/testify/require"
)

func TestGenerateCommandOpts_String(t *testing.T) {
	testCases := map[string]struct {
		inGenerateCommandOpts GenerateCommandOpts
		wantedCommand         string
	}{
		"return the correct command string": {
			inGenerateCommandOpts: GenerateCommandOpts{
				networkConfiguration: ecs.NetworkConfiguration{
					AssignPublicIp: "1.2.3.4",
					Subnets:        []string{"sbn-1", "sbn-2"},
					SecurityGroups: []string{"sg-1", "sg-2"},
				},
				executionRole: "good-doggo",
				taskRole:      "good-kitty",

				containerInfo: containerInfo{
					image:      "beautiful-image",
					entryPoint: []string{"enter", "from", "here"},
					command:    []string{"do", "not", "enter"},
					envVars: map[string]string{
						"weather":         "snowy",
						"hasHotChocolate": "yes",
					},
					secrets: map[string]string{
						"truth": "ask-the-wise",
						"lie":   "ask-the-villagers",
					},
				},

				cluster: "kamura-village",
			},
			wantedCommand: `copilot task run \
--execution-role good-doggo \
--task-role good-kitty \
--image beautiful-image \
--entrypoint "enter from here" \
--command "do not enter" \
--env-vars hasHotChocolate=yes,weather=snowy \
--secrets lie=ask-the-villagers,truth=ask-the-wise \
--subnets sbn-1,sbn-2 \
--security-groups sg-1,sg-2 \
--cluster kamura-village`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := tc.inGenerateCommandOpts
			got := opts.String()
			require.Equal(t, tc.wantedCommand, got)
		})
	}
}
