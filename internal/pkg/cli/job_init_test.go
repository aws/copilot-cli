// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

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
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedJobName)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("cuteness-aggregator/Dockerfile", nil)
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
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(rate, nil)
				m.EXPECT().Get(
					gomock.Eq(jobInitRatePrompt),
					gomock.Eq(jobInitRateHelp),
					gomock.Any(),
					gomock.Any(),
				).Return("1h", nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedRate,
		},
		"error selecting schedule type": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(jobInitSchedulePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get schedule type: some error"),
		},
		"error getting rate": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(jobInitSchedulePrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(rate, nil)
				m.EXPECT().Get(
					gomock.Eq(jobInitRatePrompt),
					gomock.Eq(jobInitRateHelp),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get schedule rate: some error"),
		},
		"asks for Cron schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fixedSchedule, nil)
				m.EXPECT().SelectOne(
					gomock.Eq(jobInitCronSchedulePrompt),
					gomock.Eq(jobInitCronScheduleHelp),
					gomock.Eq(presetSchedules),
					gomock.Any(),
				).Return("Hourly", nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedPresetSchedule,
		},
		"error getting Cron preset": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fixedSchedule, nil)
				m.EXPECT().SelectOne(
					gomock.Eq(jobInitCronSchedulePrompt),
					gomock.Eq(jobInitCronScheduleHelp),
					gomock.Eq(presetSchedules),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get preset schedule: some error"),
		},
		"get custom schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fixedSchedule, nil)
				m.EXPECT().SelectOne(
					gomock.Eq(jobInitCronSchedulePrompt),
					gomock.Eq(jobInitCronScheduleHelp),
					gomock.Eq(presetSchedules),
					gomock.Any(),
				).Return("Custom", nil)
				m.EXPECT().Get(
					gomock.Eq(jobInitCronCustomSchedulePrompt),
					gomock.Eq(jobInitCronCustomScheduleHelp),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedCronSchedule, nil)
				m.EXPECT().Confirm(
					gomock.Any(),
					gomock.Any(),
				).Return(true, nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedCronSchedule,
		},
		"custom schedule skips confirm if easy to read": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fixedSchedule, nil)
				m.EXPECT().SelectOne(
					gomock.Eq(jobInitCronSchedulePrompt),
					gomock.Eq(jobInitCronScheduleHelp),
					gomock.Eq(presetSchedules),
					gomock.Any(),
				).Return("Custom", nil)
				m.EXPECT().Get(
					gomock.Eq(jobInitCronCustomSchedulePrompt),
					gomock.Eq(jobInitCronCustomScheduleHelp),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedPresetSchedule, nil)
			},
			wantedErr:      nil,
			wantedSchedule: wantedPresetSchedule,
		},
		"error getting custom schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fixedSchedule, nil)
				m.EXPECT().SelectOne(
					gomock.Eq(jobInitCronSchedulePrompt),
					gomock.Eq(jobInitCronScheduleHelp),
					gomock.Eq(presetSchedules),
					gomock.Any(),
				).Return("Custom", nil)
				m.EXPECT().Get(
					gomock.Eq(jobInitCronCustomSchedulePrompt),
					gomock.Eq(jobInitCronCustomScheduleHelp),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get custom schedule: some error"),
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
					jobType:        tc.inJobType,
					name:           tc.inJobName,
					dockerfilePath: tc.inDockerfilePath,
					schedule:       tc.inJobSchedule,
				},
				fs:     &afero.Afero{Fs: afero.NewMemMapFs()},
				prompt: mockPrompt,
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
			require.Equal(t, wantedJobType, opts.jobType)
			require.Equal(t, wantedJobName, opts.name)
			require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
			require.Equal(t, tc.wantedSchedule, opts.schedule)

		})
	}
}

func TestJobInitOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inDockerfilePath string
		inAppName        string
		mockWriter       func(m *mocks.MockjobDirManifestWriter)
		mockstore        func(m *mocks.Mockstore)
		mockappDeployer  func(m *mocks.MockappDeployer)
		mockProg         func(m *mocks.Mockprogress)
		mockDf           func(m *mocks.MockdockerfileParser)

		wantedErr error
	}{
		"writes Scheduled Job manifest, and creates repositories successfully": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Do(func(app *config.Workload) {
						require.Equal(t, &config.Workload{
							Name: "resizer",
							App:  "app",
							Type: manifest.ScheduledJobType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "resizer")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddJobToAppStart, "resizer"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddJobToAppComplete, "resizer"))
			},
		},
		"write manifest error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			wantedErr: errors.New("some error"),
		},
		"app error": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add job to app fails": {
			inJobType:        manifest.ScheduledJobType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddJobToAppStart, "resizer"))
				m.EXPECT().Stop(log.Serrorf(fmtAddJobToAppFailed, "resizer"))
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add job resizer to application app: some error"),
		},
		"error saving app": {
			inJobType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inJobName:        "resizer",
			inDockerfilePath: "resizer/Dockerfile",

			mockWriter: func(m *mocks.MockjobDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/resizer", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "resizer").Return("/resizer/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateJob(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddJobToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			wantedErr: fmt.Errorf("saving job resizer: oops"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMockjobDirManifestWriter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)
			mockappDeployer := mocks.NewMockappDeployer(ctrl)
			mockProg := mocks.NewMockprogress(ctrl)
			if tc.mockWriter != nil {
				tc.mockWriter(mockWriter)
			}
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			if tc.mockappDeployer != nil {
				tc.mockappDeployer(mockappDeployer)
			}
			if tc.mockProg != nil {
				tc.mockProg(mockProg)
			}
			opts := initJobOpts{
				initJobVars: initJobVars{
					appName:        tc.inAppName,
					name:           tc.inJobName,
					dockerfilePath: tc.inDockerfilePath,
					jobType:        tc.inJobType,
				},
				ws:          mockWriter,
				store:       mockstore,
				appDeployer: mockappDeployer,
				prog:        mockProg,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}
