// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type initJobMocks struct {
	mockPrompt       *mocks.Mockprompter
	mockSel          *mocks.MockinitJobSelector
	mockDockerEngine *mocks.MockdockerEngine
	mockMftReader    *mocks.MockmanifestReader
}

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
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		inJobType        string
		inJobName        string
		inImage          string
		inDockerfilePath string
		inJobSchedule    string

		setupMocks func(mocks initJobMocks)

		wantedErr      error
		wantedSchedule string
	}{
		"error if fail to get job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},

			wantedErr: fmt.Errorf("get job name: some error"),
		},
		"prompt for job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq(
					fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), color.HighlightUserInput(manifest.ScheduledJobType))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedJobName, nil)
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
			},

			wantedSchedule: wantedCronSchedule,
		},
		"error if fail to get local manifest": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, mockError)
			},

			wantedErr: fmt.Errorf("read manifest file for job cuteness-aggregator: mock error"),
		},
		"skip asking questions if local manifest file exists": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return([]byte(`name: cuteness-aggregator
type: Scheduled Job`), nil)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"skip selecting Dockerfile if image flag is set": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inImage:          "mockImage",
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
			},

			wantedSchedule: wantedCronSchedule,
		},
		"return error if fail to check if docker engine is running": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},

			wantedErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"skip selecting Dockerfile if docker command is not found": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(dockerengine.ErrDockerCommandNotFound)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"skip selecting Dockerfile if docker engine is not responsive": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(&dockerengine.ErrDockerDaemonNotResponsive{})
			},

			wantedSchedule: wantedCronSchedule,
		},
		"returns an error if fail to get image location": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("", mockError)
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedJobName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedJobName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

			wantedErr: fmt.Errorf("get image location: mock error"),
		},
		"using existing image": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inJobSchedule:    wantedCronSchedule,
			inDockerfilePath: "",

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedJobName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedJobName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"prompt for existing dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("cuteness-aggregator/Dockerfile", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"error if fail to get dockerfile": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: "",
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(wantedJobName))),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},

			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"asks for schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockSel.EXPECT().Schedule(
					gomock.Eq(jobInitSchedulePrompt),
					gomock.Eq(jobInitScheduleHelp),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedCronSchedule, nil)
			},

			wantedSchedule: wantedCronSchedule,
		},
		"error getting schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    "",

			setupMocks: func(m initJobMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockSel.EXPECT().Schedule(
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
			mockDockerEngine := mocks.NewMockdockerEngine(ctrl)
			mockManifestReader := mocks.NewMockmanifestReader(ctrl)
			mocks := initJobMocks{
				mockPrompt:       mockPrompt,
				mockSel:          mockSel,
				mockDockerEngine: mockDockerEngine,
				mockMftReader:    mockManifestReader,
			}
			tc.setupMocks(mocks)
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
				sel:          mockSel,
				dockerEngine: mockDockerEngine,
				mftReader:    mockManifestReader,
				prompt:       mockPrompt,
			}

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
		mockJobInit      func(m *mocks.MockjobInitializer)
		mockDockerfile   func(m *mocks.MockdockerfileParser)
		mockDockerEngine func(m *mocks.MockdockerEngine)

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
				m.EXPECT().GetHealthCheck().Return(&dockerfile.HealthCheck{
					Cmd:         []string{"mockCommand"},
					Interval:    second,
					Timeout:     second,
					StartPeriod: second,
					Retries:     zero,
				}, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(&initialize.JobProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "mailer",
						Type:           "Scheduled Job",
						DockerfilePath: "./Dockerfile",
						Platform:       manifest.PlatformArgsOrString{},
					},
					Schedule: "@hourly",
					HealthCheck: manifest.ContainerHealthCheck{
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
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("some error"),
		},
		"return error if platform detection fails": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("", "", errors.New("some error"))
			},
			wantedErr: errors.New("get docker engine platform: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockJobInitializer := mocks.NewMockjobInitializer(ctrl)
			mockDockerfile := mocks.NewMockdockerfileParser(ctrl)
			mockDockerEngine := mocks.NewMockdockerEngine(ctrl)

			if tc.mockJobInit != nil {
				tc.mockJobInit(mockJobInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}
			if tc.mockDockerEngine != nil {
				tc.mockDockerEngine(mockDockerEngine)
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
				dockerEngine: mockDockerEngine,
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
