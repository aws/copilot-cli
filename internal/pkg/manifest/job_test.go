// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScheduledJob_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps ScheduledJobProps

		wantedTestData string
	}{
		"without timeout or retries": {
			inProps: ScheduledJobProps{
				ServiceProps: &ServiceProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Schedule: "@weekly",
			},
			wantedTestData: "scheduled-job-no-timeout-or-retries.yml",
		},
		"fully specified using cron schedule": {
			inProps: ScheduledJobProps{
				ServiceProps: &ServiceProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Schedule: "0 */2 * * *",
				Retries:  3,
				Timeout:  "1h30m",
			},
			wantedTestData: "scheduled-job-fully-specified.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestData)
			wantedBytes, err := ioutil.ReadFile(path)
			require.NoError(t, err)
			manifest := NewScheduledJob(tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}
