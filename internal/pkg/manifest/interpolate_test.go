// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterpolator_substitute(t *testing.T) {
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
		"success": {
			inputStr: "${0accountID}.dkr.${repo-provider}.${region}.amazonaws.com/vault/${COPILOT_ENVIRONMENT_NAME}:${tag}",
			inputEnvVar: map[string]string{
				"0accountID":               "1234567890",
				"repo-provider":            "ecr",
				"tag":                      "latest",
				"COPILOT_APPLICATION_NAME": "myApp",
				"region":                   "",
			},

			wanted: "${0accountID}.dkr.${repo-provider}..amazonaws.com/vault/test:latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			itpl := newInterpolator(
				"myApp",
				"test",
			)
			for k, v := range tc.inputEnvVar {
				require.NoError(t, os.Setenv(k, v))
				defer func(key string) {
					require.NoError(t, os.Unsetenv(key))
				}(k)
			}
			actual, actualErr := itpl.substitute(tc.inputStr)

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
