// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestJobInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName        string
		inJobName        string
		inDockerfilePath string
		inImage          string
		inTimeout        string
		inRetries        int
		inSchedule       string

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid app name": {
			inAppName: "",
			wantedErr: errNoAppInWorkspace,
		},
		"invalid job name": {
			inAppName: "phonetool",
			inJobName: "1234",
			wantedErr: fmt.Errorf("job name 1234 is invalid: %s", errValueBadFormat),
		},
		"invalid dockerfile directory path": {
			inAppName:        "phonetool",
			inDockerfilePath: "./hello/Dockerfile",
			wantedErr:        errors.New("open hello/Dockerfile: file does not exist"),
		},
		"invalid schedule; not cron": {
			inAppName:  "phonetool",
			inSchedule: "every 56 minutes",
			wantedErr:  fmt.Errorf("schedule every 56 minutes is invalid: %s", errScheduleInvalid),
		},
		"invalid schedule; cron interval in subseconds": {
			inAppName:  "phonetool",
			inSchedule: "@every 75.9s",
			wantedErr:  fmt.Errorf("interval @every 75.9s is invalid: %s", errDurationBadUnits),
		},
		"invalid schedule; cron interval in milliseconds": {
			inAppName:  "phonetool",
			inSchedule: "@every 3ms",
			wantedErr:  fmt.Errorf("interval @every 3ms is invalid: %s", errDurationBadUnits),
		},
		"invalid schedule; cron interval too frequent": {
			inAppName:  "phonetool",
			inSchedule: "@every 30s",
			wantedErr:  errors.New("interval @every 30s is invalid: duration must be 1m0s or greater"),
		},
		"invalid schedule; cron interval is zero": {
			inAppName:  "phonetool",
			inSchedule: "@every 0s",
			wantedErr:  errors.New("interval @every 0s is invalid: duration must be 1m0s or greater"),
		},
		"invalid schedule; cron interval duration improperly formed": {
			inAppName:  "phonetool",
			inSchedule: "@every 5min",
			wantedErr:  errors.New("interval @every 5min must include a valid Go duration string (example: @every 1h30m)"),
		},
		"valid schedule; crontab": {
			inAppName:  "phonetool",
			inSchedule: "* * * * *",
			wantedErr:  nil,
		},
		"valid schedule; predefined schedule": {
			inAppName:  "phonetool",
			inSchedule: "@daily",
			wantedErr:  nil,
		},
		"valid schedule; interval": {
			inAppName:  "phonetool",
			inSchedule: "@every 5m",
			wantedErr:  nil,
		},
		"valid schedule; interval with 0 for some units": {
			inAppName:  "phonetool",
			inSchedule: "@every 1h0m0s",
			wantedErr:  nil,
		},
		"valid schedule; interval with carryover value for some units": {
			inAppName:  "phonetool",
			inSchedule: "@every 0h60m60s",
			wantedErr:  nil,
		},
		"invalid timeout duration; incorrect format": {
			inAppName: "phonetool",
			inTimeout: "30 minutes",
			wantedErr: fmt.Errorf("timeout value 30 minutes is invalid: %s", errDurationInvalid),
		},
		"invalid timeout duration; subseconds": {
			inAppName: "phonetool",
			inTimeout: "30m45.5s",
			wantedErr: fmt.Errorf("timeout value 30m45.5s is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout duration; milliseconds": {
			inAppName: "phonetool",
			inTimeout: "3ms",
			wantedErr: fmt.Errorf("timeout value 3ms is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout; too short": {
			inAppName: "phonetool",
			inTimeout: "0s",
			wantedErr: errors.New("timeout value 0s is invalid: duration must be 1s or greater"),
		},
		"invalid number of times to retry": {
			inAppName: "phonetool",
			inRetries: -3,
			wantedErr: errors.New("number of retries must be non-negative"),
		},
		"fail if both image and dockerfile are set": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",
			wantedErr:        fmt.Errorf("--dockerfile and --image cannot be specified together"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						appName:        tc.inAppName,
						name:           tc.inJobName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
					},
					timeout:  tc.inTimeout,
					retries:  tc.inRetries,
					schedule: tc.inSchedule,
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
		wantedImage          = "mockImage"
		wantedCronSchedule   = "0 9-17 * * MON-FRI"
	)
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inImage          string
		inDockerfilePath string
		inJobSchedule    string

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *mocks.Mockprompter)
		mockSel        func(m *mocks.MockinitJobSelector)
		mockValidator  func(m *mocks.MockdockerEngineValidator)

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
			mockSel:       func(m *mocks.MockinitJobSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},

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
			mockSel:       func(m *mocks.MockinitJobSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},

			wantedErr: fmt.Errorf("get job name: some error"),
		},
		"skip selecting Dockerfile if image flag is set": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inImage:          "mockImage",
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel:        func(m *mocks.MockinitJobSelector) {},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},

			wantedSchedule: wantedCronSchedule,
		},
		"return error if fail to check if docker engine is running": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel:        func(m *mocks.MockinitJobSelector) {},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},

			wantedErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"skip selecting Dockerfile if docker command is not found": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
			},
			mockSel:        func(m *mocks.MockinitJobSelector) {},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(exec.ErrDockerCommandNotFound)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"skip selecting Dockerfile if docker engine is not responsive": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
			},
			mockSel:        func(m *mocks.MockinitJobSelector) {},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(&exec.ErrDockerDaemonNotResponsive{})
			},

			wantedSchedule: wantedCronSchedule,
		},
		"returns an error if fail to get image location": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("", mockError)
			},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedJobName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedJobName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
			},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {},

			wantedErr: fmt.Errorf("get image location: mock error"),
		},
		"using existing image": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inJobSchedule:    wantedCronSchedule,
			inDockerfilePath: "",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
			},
			mockSel: func(m *mocks.MockinitJobSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedJobName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedJobName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

			wantedSchedule: wantedCronSchedule,
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
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

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
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
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
			mockPrompt:    func(m *mocks.Mockprompter) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},

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
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},

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
			mockValidator := mocks.NewMockdockerEngineValidator(ctrl)
			opts := &initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						wkldType:       tc.inJobType,
						name:           tc.inJobName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
					},
					schedule: tc.inJobSchedule,
				},
				fs:                    &afero.Afero{Fs: afero.NewMemMapFs()},
				sel:                   mockSel,
				dockerEngineValidator: mockValidator,
				prompt:                mockPrompt,
			}

			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)
			tc.mockSel(mockSel)
			tc.mockValidator(mockValidator)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, wantedJobType, opts.wkldType)
			require.Equal(t, wantedJobName, opts.name)
			if opts.dockerfilePath != "" {
				require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
			}
			if opts.image != "" {
				require.Equal(t, wantedImage, opts.image)
			}
			require.Equal(t, tc.wantedSchedule, opts.schedule)
		})
	}
}

func TestJobInitOpts_Execute(t *testing.T) {
	second := time.Second
	zero := 0
	testCases := map[string]struct {
		mockJobInit    func(m *mocks.MockjobInitializer)
		mockDockerfile func(m *mocks.MockdockerfileParser)

		inApp  string
		inName string
		inType string
		inDf   string

		inSchedule string

		wantedErr          error
		wantedManifestPath string
	}{
		"success on typical job props": {
			inApp:              "sample",
			inName:             "mailer",
			inType:             manifest.ScheduledJobType,
			inDf:               "./Dockerfile",
			inSchedule:         "@hourly",
			wantedManifestPath: "manifest/path",

			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(&exec.HealthCheck{
					Cmd:         []string{"mockCommand"},
					Interval:    second,
					Timeout:     second,
					StartPeriod: second,
					Retries:     zero,
				}, nil)
			},
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(&initialize.JobProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "mailer",
						Type:           "Scheduled Job",
						DockerfilePath: "./Dockerfile",
					},
					Schedule: "@hourly",
					HealthCheck: &manifest.ContainerHealthCheck{
						Command:     []string{"mockCommand"},
						Interval:    &second,
						Retries:     &zero,
						Timeout:     &second,
						StartPeriod: &second,
					},
				}).Return("manifest/path", nil)
			},
		},
		"fail to init job": {
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockJobInitializer := mocks.NewMockjobInitializer(ctrl)
			mockDockerfile := mocks.NewMockdockerfileParser(ctrl)

			if tc.mockJobInit != nil {
				tc.mockJobInit(mockJobInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}

			opts := initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						appName:        tc.inApp,
						name:           tc.inName,
						wkldType:       tc.inType,
						dockerfilePath: tc.inDf,
					},
					schedule: tc.inSchedule,
				},
				init: mockJobInitializer,
				initParser: func(s string) dockerfileParser {
					return mockDockerfile
				},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedManifestPath, opts.manifestPath)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}
