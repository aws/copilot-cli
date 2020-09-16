// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/golang/mock/gomock"
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
			wantedErr:  fmt.Errorf("schedule every 56 minutes is invalid: %s", errScheduleInvalid),
		},
		"invalid schedule; cron interval in subseconds": {
			inSchedule: "@every 75.9s",
			wantedErr:  fmt.Errorf("interval @every 75.9s is invalid: %s", errDurationBadUnits),
		},
		"invalid schedule; cron interval in milliseconds": {
			inSchedule: "@every 3ms",
			wantedErr:  fmt.Errorf("interval @every 3ms is invalid: %s", errDurationBadUnits),
		},
		"invalid schedule; cron interval too frequent": {
			inSchedule: "@every 30s",
			wantedErr:  errors.New("interval @every 30s is invalid: duration must be 1m0s or greater"),
		},
		"invalid schedule; cron interval is zero": {
			inSchedule: "@every 0s",
			wantedErr:  errors.New("interval @every 0s is invalid: duration must be 1m0s or greater"),
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
			wantedErr: errors.New("timeout value 0s is invalid: duration must be 1s or greater"),
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

func TestJobInitOpts_Ask(t *testing.T) {
	const (
		wantedJobType        = manifest.ScheduledJobType
		wantedJobName        = "cuteness-aggregator"
		wantedDockerfilePath = "cuteness-aggregator/Dockerfile"
		wantedCronSchedule   = "0 9-17 * * MON-FRI"
		wantedRate           = "@every 1h"
		wantedPresetSchedule = "@hourly"
	)
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inDockerfilePath string
		inJobSchedule    string

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *mocks.Mockprompter)

		wantedErr      error
		wantedSchedule string
	}{
		"prompt for job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(
					fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), color.HighlightUserInput(manifest.ScheduledJobType))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedJobName, nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"error if fail to get job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get job name: some error"),
		},
		"prompt for existing dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("cuteness-aggregator", 0755)
				mockFS.MkdirAll("frontend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "cuteness-aggregator/Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(
					fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.Emphasize("Dockerfile"), color.HighlightUserInput(wantedJobName))),
					wkldInitDockerfileHelpPrompt,
					gomock.Eq([]string{
						"./Dockerfile",
						"cuteness-aggregator/Dockerfile",
						"frontend/Dockerfile",
					}),
					gomock.Any(),
				).Return("cuteness-aggregator/Dockerfile", nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"prompt for custom path if no dockerfiles found": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedJobname)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("cuteness-aggragator/Dockerfile", nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"error if fail to get custom dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedJobName)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr:      fmt.Errorf("get custom Dockerfile path: some error"),
			wantedSchedule: wantedCronSchedule,
		},
		"asks for rate": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedJobname)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr:      fmt.Errorf("get custom Dockerfile path: some error"),
			wantedSchedule: wantedCronSchedule,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			opts := &initJobOpts{
				initJobVars: initJobVars{
					JobType:        tc.inJobType,
					Name:           tc.inJobName,
					DockerfilePath: tc.inDockerfilePath,
					Schedule:       tc.inJobSchedule,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
			}
			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, wantedJobType, opts.JobType)
			require.Equal(t, wantedJobName, opts.Name)
			require.Equal(t, wantedDockerfilePath, opts.DockerfilePath)
			require.Equal(t, tc.wantedSchedule, opts.Schedule)

		})
	}
}
