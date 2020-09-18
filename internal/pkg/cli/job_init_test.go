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
					name:           tc.inJobName,
					dockerfilePath: tc.inDockerfilePath,
					timeout:        tc.inTimeout,
					retries:        tc.inRetries,
					schedule:       tc.inSchedule,
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
	)
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inDockerfilePath string
		inJobSchedule    string

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *mocks.Mockprompter)
		mockSel        func(m *mocks.MockinitJobSelector)

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
			mockSel:        func(m *mocks.MockinitJobSelector) {},
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
			mockSel:   func(m *mocks.MockinitJobSelector) {},
			wantedErr: fmt.Errorf("get job name: some error"),
		},
		"prompt for existing dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("cuteness-aggregator/Dockerfile", nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"error if fail to get dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"asks for schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Schedule(
					gomock.Eq(jobInitSchedulePrompt),
					gomock.Eq(jobInitScheduleHelp),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedCronSchedule, nil)
			},
			mockPrompt:     func(m *mocks.Mockprompter) {},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"error getting schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Schedule(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", fmt.Errorf("some error"))
			},
			wantedErr: fmt.Errorf("get schedule: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockinitJobSelector(ctrl)
			opts := &initJobOpts{
				initJobVars: initJobVars{
					jobType:        tc.inJobType,
					name:           tc.inJobName,
					dockerfilePath: tc.inDockerfilePath,
					schedule:       tc.inJobSchedule,
				},
				fs:     &afero.Afero{Fs: afero.NewMemMapFs()},
				sel:    mockSel,
				prompt: mockPrompt,
			}

			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)
			tc.mockSel(mockSel)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, wantedJobType, opts.jobType)
			require.Equal(t, wantedJobName, opts.name)
			require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
			require.Equal(t, tc.wantedSchedule, opts.schedule)

		})
	}
}
