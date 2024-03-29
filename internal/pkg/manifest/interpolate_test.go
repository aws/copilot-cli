// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterpolator_Interpolate(t *testing.T) {
	testCases := map[string]struct {
		inputEnvVar map[string]string
		inputStr    string

		wanted    string
		wantedErr error
	}{
		"should return error if env var is not defined": {
			inputStr: "/copilot/my-app/${env}/secrets/db_password",

			wantedErr: fmt.Errorf(`environment variable "env" is not defined`),
		},
		"should return error if trying to override predefined env var": {
			inputStr: "/copilot/my-app/${COPILOT_ENVIRONMENT_NAME}/secrets/db_password",
			inputEnvVar: map[string]string{
				"COPILOT_ENVIRONMENT_NAME": "prod",
			},

			wantedErr: fmt.Errorf(`predefined environment variable "COPILOT_ENVIRONMENT_NAME" cannot be overridden by OS environment variable with the same name`),
		},
		"success with no matches": {
			inputStr: "1234567890.dkr.ecr.us-west-2.amazonaws.com/vault/test:latest",

			wanted: "1234567890.dkr.ecr.us-west-2.amazonaws.com/vault/test:latest\n",
		},
		"success": {
			inputStr: `# The manifest for the ${name} service.

# Your service name will be used in naming your resources like log groups, ECS services, etc.
name: loadtester
type: Backend Service

# Your service is reachable at "http://loadtester.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:80" but is not public.

http:
  allowed_source_ips:
    - ${ip}
# Configuration for your containers and service.
image:
  # Docker build arguments. For additional overrides: https://aws.github.io/copilot-cli/docs/manifest/backend-service/#image-build
  location: ${0accountID}.dkr.${repo-provider}.${region}.amazonaws.com/vault/${COPILOT_ENVIRONMENT_NAME}:${tag}
  port: 80
  labels: |
    ["label1","label2"]
  "com.datadoghq.ad.instances": |
    [
      {
        "prometheus_url": "http://metrics",
      }
    ]

cpu: 256#${CPU}
memory: 512    # ${Memory}
variables:
  ${foo}: ${bar}
network:
  vpc:
    security_groups: ${SECURITY_GROUPS}
`,
			inputEnvVar: map[string]string{
				"0accountID":               "1234567890",
				"repo-provider":            "ecr",
				"tag":                      "latest",
				"COPILOT_APPLICATION_NAME": "myApp",
				"region":                   "",
				"CPU":                      "512",
				"bar":                      "bar",
				"ip":                       "10.24.34.0/23",
				"SECURITY_GROUPS":          `["sg-1","sg-2","sg-3"]`,
			},

			wanted: `# The manifest for the ${name} service.

# Your service name will be used in naming your resources like log groups, ECS services, etc.
name: loadtester
type: Backend Service
# Your service is reachable at "http://loadtester.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:80" but is not public.
http:
  allowed_source_ips:
    - 10.24.34.0/23
# Configuration for your containers and service.
image:
  # Docker build arguments. For additional overrides: https://aws.github.io/copilot-cli/docs/manifest/backend-service/#image-build
  location: ${0accountID}.dkr.${repo-provider}..amazonaws.com/vault/test:latest
  port: 80
  labels: |
    ["label1","label2"]
  "com.datadoghq.ad.instances": |
    [
      {
        "prometheus_url": "http://metrics",
      }
    ]
cpu: 256#512
memory: 512 # ${Memory}
variables:
  ${foo}: bar
network:
  vpc:
    security_groups:
      - sg-1
      - sg-2
      - sg-3
`,
		},
		"should not substitute escaped dollar signs": {
			inputStr: "echo \\${name}",
			inputEnvVar: map[string]string{
				"name": "this variable should not be read",
			},
			wanted: "echo ${name}\n",
		},
		"should substitute variables right after one another": {
			inputStr: "${a}${b}\\${c}${d}",
			inputEnvVar: map[string]string{
				"a": "A",
				"b": "B",
				"c": "C",
				"d": "D",
			},
			wanted: "AB${c}D\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			itpl := NewInterpolator(
				"myApp",
				"test",
			)
			for k, v := range tc.inputEnvVar {
				require.NoError(t, os.Setenv(k, v))
				defer func(key string) {
					require.NoError(t, os.Unsetenv(key))
				}(k)
			}
			actual, actualErr := itpl.Interpolate(tc.inputStr)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error())
			} else {
				require.NoError(t, actualErr)
				require.Equal(t, tc.wanted, actual)
			}
		})
	}
}
