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
		"invalid schedule; cron and rate": {
			inSchedule: "every 56 minutes",
			wantedErr:  errors.New("schedule value every 56 minutes is invalid"),
		},
		"invalid schedule; rate too frequent": {
			inSchedule: "7s",
			wantedErr:  errors.New("schedule rate 7s must be greater than a minute"),
		},
		"invalid schedule; rate in subseconds": {
			inSchedule: "75.9s",
			wantedErr:  errors.New("schedule rate 75.9s cannot be in units smaller than a second"),
		},
		"valid schedule; cron": {
			inSchedule: "* * * * *",
			wantedErr:  nil,
		},
		"valid schedule; rate": {
			inSchedule: "1h23m45s",
			wantedErr:  nil,
		},
		"invalid timeout duration; subseconds": {
			inTimeout: "30m45.5s",
			wantedErr: errors.New("timeout duration 30m45.5s cannot be in units smaller than a second"),
		},
		"invalid timeout duration; incorrect format": {
			inTimeout: "30 minutes",
			wantedErr: errors.New("time: unknown unit  minutes in duration 30 minutes"),
		},
		"invalid number of times to retry": {
			inRetries: -3,
			wantedErr: errors.New("number of times to retry -3 must be positive"),
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
