// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/afero"

	"github.com/stretchr/testify/require"
)

func TestJobInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inJobName        string
		inDockerfilePath string
		inTimeout        string
		inRetries        int
		inSchedule       string

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid job name": {
			inJobName: "1234",
			wantedErr: fmt.Errorf("job name 1234 is invalid: %s", errValueBadFormat),
		},
		"invalid dockerfile directory path": {
			inDockerfilePath: "./hello/Dockerfile",
			wantedErr:        errors.New("open hello/Dockerfile: file does not exist"),
		},
		"invalid schedule; not cron": {
			inSchedule: "every 56 minutes",
			wantedErr:  errors.New("expected exactly 5 fields, found 3: [every 56 minutes]"),
		},
		"invalid schedule; cron interval in subseconds": {
			inSchedule: "@every 75.9s",
			wantedErr:  errDurationBadUnits,
		},
		"invalid schedule; cron interval in milliseconds": {
			inSchedule: "@every 3ms",
			wantedErr:  errDurationBadUnits,
		},
		"invalid schedule; cron interval too frequent": {
			inSchedule: "@every 30s",
			wantedErr:  errors.New("duration must be greater than 60s"),
		},
		"invalid schedule; cron interval is zero": {
			inSchedule: "@every 0s",
			wantedErr:  errors.New("duration must be greater than 60s"),
		},
		"invalid schedule; cron interval duration improperly formed": {
			inSchedule: "@every 5min",
			wantedErr:  errors.New("interval @every 5min must include a valid Go duration string (example: @every 1h30m)"),
		},
		"valid schedule; crontab": {
			inSchedule: "* * * * *",
			wantedErr:  nil,
		},
		"valid schedule; predefined schedule": {
			inSchedule: "@daily",
			wantedErr:  nil,
		},
		"valid schedule; interval": {
			inSchedule: "@every 5m",
			wantedErr:  nil,
		},
		"valid schedule; interval with 0 for some units": {
			inSchedule: "@every 1h0m0s",
			wantedErr:  nil,
		},
		"valid schedule; interval with carryover value for some units": {
			inSchedule: "@every 0h60m60s",
			wantedErr:  nil,
		},
		"invalid timeout duration; incorrect format": {
			inTimeout: "30 minutes",
			wantedErr: fmt.Errorf("timeout value 30 minutes is invalid: %s", errDurationInvalid),
		},
		"invalid timeout duration; subseconds": {
			inTimeout: "30m45.5s",
			wantedErr: fmt.Errorf("timeout value 30m45.5s is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout duration; milliseconds": {
			inTimeout: "3ms",
			wantedErr: fmt.Errorf("timeout value 3ms is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout; too short": {
			inTimeout: "0s",
			wantedErr: errors.New("timeout value 0s is invalid: duration must be greater than 1s"),
		},
		"invalid number of times to retry": {
			inRetries: -3,
			wantedErr: errors.New("number of retries must be non-negative"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := initJobOpts{
				initJobVars: initJobVars{
					Name:           tc.inJobName,
					DockerfilePath: tc.inDockerfilePath,
					Timeout:        tc.inTimeout,
					Retries:        tc.inRetries,
					Schedule:       tc.inSchedule,
					GlobalOpts:     &GlobalOpts{},
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
			}
			if tc.mockFileSystem != nil {
				tc.mockFileSystem(opts.fs)
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
