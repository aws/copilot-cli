// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadBalancedFargateManifest_Marshal(t *testing.T) {
	// GIVEN
	wantedContent := `# The manifest for the "frontend" application.
# Read the full specification for the "Load Balanced Web App" type at:
#  https://github.com/aws/amazon-ecs-cli-v2/wiki/Manifests#load-balanced-web-app

# Your application name will be used in naming your resources like log groups, services, etc.
name: frontend
# The "architecture" of the application you're running.
type: Load Balanced Web App

image:
  # Path to your application's Dockerfile.
  build: frontend/Dockerfile
  # Port exposed through your container to route traffic to it.
  port: 80

http:
  # Requests to this path will be forwarded to your service.
  path: '*'

healthcheck:
  path: '/'

# Number of CPU units for the task.
cpu: 512
# Amount of memory in MiB used by the task.
memory: 1024
# Number of tasks that should be running in your service.
count: 1

# Optional fields for more advanced use-cases.
#
#variables:                    # Pass environment variables as key value pairs.
#  LOG_LEVEL: info
#
#secrets:                      # Pass secrets from AWS Systems Manager (SSM) Parameter Store.
#  GITHUB_TOKEN: GITHUB_TOKEN  # The key is the name of the environment variable, the value is the name of the SSM parameter.
#
#scaling:                      # Optional configuration for scaling your service.
#  minCount: 1                   # Minimum number of tasks that should be running in your service.
#  maxCount: 3                   # Maximum number of tasks that should be running in your service.
#
#  # If the target value is crossed, ECS starts adding or removing tasks.
#  targetCPU: 75.0               # Target average CPU utilization percentage.

# You can override any of the values defined above by environment.
#environments:
#  test:
#    count: 2               # Number of tasks to run for the "test" environment.
`
	m := NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile", 80)

	// WHEN
	b, err := m.Marshal()

	// THEN
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}

func TestLBFargateManifest_EnvConf(t *testing.T) {
	testCases := map[string]struct {
		inDefaultConfig  LBFargateConfig
		inEnvNameToQuery string
		inEnvOverride    map[string]LBFargateConfig

		wantedConfig LBFargateConfig
	}{
		"with no existing environments": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
			},
			inEnvNameToQuery: "prod-iad",

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
			},
		},
		"with partial overrides": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
					Variables: map[string]string{
						"LOG_LEVEL":      "DEBUG",
						"DDB_TABLE_NAME": "awards",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "1111",
						"TWILIO_TOKEN": "1111",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:     1,
					MaxCount:     2,
					TargetCPU:    75.0,
					TargetMemory: 0,
				},
			},
			inEnvNameToQuery: "prod-iad",
			inEnvOverride: map[string]LBFargateConfig{
				"prod-iad": {
					ContainersConfig: ContainersConfig{
						CPU: 2046,
						Variables: map[string]string{
							"DDB_TABLE_NAME": "awards-prod",
						},
					},
					Scaling: &AutoScalingConfig{
						MaxCount: 5,
					},
				},
			},

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    2046,
					Memory: 1024,
					Count:  1,
					Variables: map[string]string{
						"LOG_LEVEL":      "DEBUG",
						"DDB_TABLE_NAME": "awards-prod",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "1111",
						"TWILIO_TOKEN": "1111",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:  1,
					MaxCount:  5,
					TargetCPU: 75.0,
				},
			},
		},
		"with complete override": {
			inDefaultConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/awards/*"},
				ContainersConfig: ContainersConfig{
					CPU:    1024,
					Memory: 1024,
					Count:  1,
				},
				Scaling: &AutoScalingConfig{
					MinCount:     1,
					MaxCount:     2,
					TargetCPU:    75.0,
					TargetMemory: 0,
				},
			},
			inEnvNameToQuery: "prod-iad",
			inEnvOverride: map[string]LBFargateConfig{
				"prod-iad": {
					RoutingRule: RoutingRule{Path: "/frontend*"},
					ContainersConfig: ContainersConfig{
						CPU:    2046,
						Memory: 2046,
						Count:  3,
						Variables: map[string]string{
							"LOG_LEVEL":      "WARN",
							"DDB_TABLE_NAME": "awards-prod",
						},
						Secrets: map[string]string{
							"GITHUB_TOKEN": "2222",
							"TWILIO_TOKEN": "2222",
						},
					},
					Scaling: &AutoScalingConfig{
						MinCount:     2,
						MaxCount:     5,
						TargetCPU:    75.0,
						TargetMemory: 75.0,
					},
				},
			},

			wantedConfig: LBFargateConfig{
				RoutingRule: RoutingRule{Path: "/frontend*"},
				ContainersConfig: ContainersConfig{
					CPU:    2046,
					Memory: 2046,
					Count:  3,
					Variables: map[string]string{
						"LOG_LEVEL":      "WARN",
						"DDB_TABLE_NAME": "awards-prod",
					},
					Secrets: map[string]string{
						"GITHUB_TOKEN": "2222",
						"TWILIO_TOKEN": "2222",
					},
				},
				Scaling: &AutoScalingConfig{
					MinCount:     2,
					MaxCount:     5,
					TargetCPU:    75.0,
					TargetMemory: 75.0,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			m := &LBFargateManifest{
				LBFargateConfig: tc.inDefaultConfig,
				Environments:    tc.inEnvOverride,
			}

			// WHEN
			conf := m.EnvConf(tc.inEnvNameToQuery)

			// THEN
			require.Equal(t, tc.wantedConfig, conf, "returned configuration should have overrides from the environment")
			require.Equal(t, m.LBFargateConfig, tc.inDefaultConfig, "values in the default configuration should not be overwritten")
		})
	}
}
