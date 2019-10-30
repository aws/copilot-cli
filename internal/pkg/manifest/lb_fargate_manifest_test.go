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
#   https://github.com/aws/amazon-ecs-cli-v2/docs/manifests/load-balanced-web-app.

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

# Determines whether the application will have a public IP or not.
public: false

# Number of CPU units for the task.
cpu: 256
# Amount of memory in MiB used by the task.
memory: 512
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
	m := NewLoadBalancedFargateManifest("frontend", "frontend/Dockerfile")

	// WHEN
	b, err := m.Marshal()

	// THEN
	require.NoError(t, err)
	require.Equal(t, wantedContent, strings.Replace(string(b), "\r\n", "\n", -1))
}
