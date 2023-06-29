// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type initJobMocks struct {
	mockPrompt        *mocks.Mockprompter
	mockDockerEngine  *mocks.MockdockerEngine
	mockMftReader     *mocks.MockmanifestReader
	mockStore         *mocks.Mockstore
	mockDockerfileSel *mocks.MockdockerfileSelector
	mockScheduleSel   *mocks.MockscheduleSelector
}

func TestJobInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName        string
		inJobName        string
		inDockerfilePath string
		inImage          string
		inTimeout        string
		inRetries        int

		setupMocks     func(mocks initJobMocks)
		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"fail if using different app name with the workspace": {
			inAppName: "demo",
			wantedErr: fmt.Errorf("cannot specify app demo because the workspace is already registered with app phonetool"),
		},
		"fail if cannot validate application": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get application phonetool configuration: some error"),
		},
		"invalid dockerfile directory path": {
			inAppName:        "phonetool",
			inDockerfilePath: "./hello/Dockerfile",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("open %s: file does not exist", filepath.FromSlash("hello/Dockerfile")),
		},
		"invalid timeout duration; incorrect format": {
			inAppName: "phonetool",
			inTimeout: "30 minutes",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("timeout value 30 minutes is invalid: %s", errDurationInvalid),
		},
		"invalid timeout duration; subseconds": {
			inAppName: "phonetool",
			inTimeout: "30m45.5s",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("timeout value 30m45.5s is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout duration; milliseconds": {
			inAppName: "phonetool",
			inTimeout: "3ms",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("timeout value 3ms is invalid: %s", errDurationBadUnits),
		},
		"invalid timeout; too short": {
			inAppName: "phonetool",
			inTimeout: "0s",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: errors.New("timeout value 0s is invalid: duration must be 1s or greater"),
		},
		"invalid number of times to retry": {
			inAppName: "phonetool",
			inRetries: -3,

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: errors.New("number of retries must be non-negative"),
		},
		"fail if both image and dockerfile are set": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("--dockerfile and --image cannot be specified together"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mocks := initJobMocks{
				mockStore: mockstore,
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}
			opts := initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						appName:        tc.inAppName,
						name:           tc.inJobName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
					},
					timeout: tc.inTimeout,
					retries: tc.inRetries,
				},
				store:     mockstore,
				fs:        &afero.Afero{Fs: afero.NewMemMapFs()},
				wsAppName: "phonetool",
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
		mockAppName          = "phonetool"
		wantedJobType        = manifestinfo.ScheduledJobType
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
		"invalid job type": {
			inJobType: "TestJobType",
			wantedErr: errors.New(`invalid job type TestJobType: must be one of "Scheduled Job"`),
		},
		"invalid job name": {
			inJobType: wantedJobType,
			inJobName: "1234",
			wantedErr: fmt.Errorf("job name 1234 is invalid: %s", errBasicNameRegexNotMatched),
		},
		"error if fail to get job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", mockError)
			},

			wantedErr: fmt.Errorf("get job name: mock error"),
		},
		"returns an error if job already exists": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this job?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedJobName, nil)
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(&config.Workload{}, nil)
			},
			wantedErr: fmt.Errorf("job cuteness-aggregator already exists"),
		},
		"returns an error if fail to validate service existence": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this job?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedJobName, nil)
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, mockError)
			},
			wantedErr: fmt.Errorf("validate if job exists: mock error"),
		},
		"prompt for job name": {
			inJobType:        wantedJobType,
			inJobName:        "",
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq(
					"What do you want to name this job?"),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(wantedJobName, nil)
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, mockError)
			},

			wantedErr: fmt.Errorf("read manifest file for job cuteness-aggregator: mock error"),
		},
		"error if manifest type doesn't match": {
			inJobType: "Scheduled Job",
			inJobName: wantedJobName,

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return([]byte(`
type: Backend Service`), nil)
			},
			wantedErr: fmt.Errorf("manifest file for job cuteness-aggregator exists with a different type Backend Service"),
		},
		"skip asking questions if local manifest file exists": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
			},

			wantedSchedule: wantedCronSchedule,
		},
		"return error if fail to check if docker engine is running": {
			inJobType:     wantedJobType,
			inJobName:     wantedJobName,
			inJobSchedule: wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
					Return("", mockError)
				m.mockDockerfileSel.EXPECT().Dockerfile(
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerfileSel.EXPECT().Dockerfile(
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockDockerfileSel.EXPECT().Dockerfile(
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockDockerfileSel.EXPECT().Dockerfile(
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{})
				m.mockScheduleSel.EXPECT().Schedule(
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
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
				m.mockScheduleSel.EXPECT().Schedule(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", fmt.Errorf("some error"))
			},

			wantedErr: fmt.Errorf("get schedule: some error"),
		},
		"valid schedule": {
			inJobType:        wantedJobType,
			inJobName:        wantedJobName,
			inDockerfilePath: wantedDockerfilePath,
			inJobSchedule:    wantedCronSchedule,

			setupMocks: func(m initJobMocks) {
				m.mockStore.EXPECT().GetJob(mockAppName, wantedJobName).Return(nil, &config.ErrNoSuchJob{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedJobName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedJobName})
			},

			wantedSchedule: wantedCronSchedule,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := initJobMocks{
				mockPrompt:        mocks.NewMockprompter(ctrl),
				mockDockerfileSel: mocks.NewMockdockerfileSelector(ctrl),
				mockScheduleSel:   mocks.NewMockscheduleSelector(ctrl),
				mockDockerEngine:  mocks.NewMockdockerEngine(ctrl),
				mockMftReader:     mocks.NewMockmanifestReader(ctrl),
				mockStore:         mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			opts := &initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						wkldType:       tc.inJobType,
						name:           tc.inJobName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
						appName:        mockAppName,
					},
					schedule: tc.inJobSchedule,
				},
				dockerfileSel:    m.mockDockerfileSel,
				scheduleSelector: m.mockScheduleSel,
				store:            m.mockStore,
				dockerEngine:     m.mockDockerEngine,
				mftReader:        m.mockMftReader,
				prompt:           m.mockPrompt,
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
	mockEnvironmentManifest := []byte(`name: test
type: Environment
network:
  vpc:
   id: 'vpc-mockid'
   subnets:
      private:
        - id: 'subnet-1'
        - id: 'subnet-2'
        - id: 'subnet-3'
        - id: 'subnet-4'`)
	second := time.Second
	zero := 0
	testCases := map[string]struct {
		mockJobInit      func(m *mocks.MockjobInitializer)
		mockDockerfile   func(m *mocks.MockdockerfileParser)
		mockDockerEngine func(m *mocks.MockdockerEngine)
		mockStore        func(m *mocks.Mockstore)
		mockEnvDescriber func(m *mocks.MockenvDescriber)

		inApp      string
		inName     string
		inType     string
		inDf       string
		inSchedule string

		inManifestExists bool

		wantedErr          error
		wantedManifestPath string
	}{
		"success on typical job props": {
			inApp:              "sample",
			inName:             "mailer",
			inType:             manifestinfo.ScheduledJobType,
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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
		},
		"fail to init job": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("").Return(nil, nil)
			},
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("some error"),
		},
		"doesn't attempt to detect and populate the platform if manifest already exists": {
			inApp:              "sample",
			inName:             "mailer",
			inType:             manifestinfo.ScheduledJobType,
			inDf:               "./Dockerfile",
			inSchedule:         "@hourly",
			inManifestExists:   true,
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
				m.EXPECT().CheckDockerEngineRunning().Times(0)
				m.EXPECT().GetPlatform().Times(0)
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
		},
		"doesn't complain if docker is unavailable": {
			inApp:              "sample",
			inName:             "mailer",
			inType:             manifestinfo.ScheduledJobType,
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
				m.EXPECT().CheckDockerEngineRunning().Return(dockerengine.ErrDockerCommandNotFound)
				m.EXPECT().GetPlatform().Times(0)
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
		},
		"return error if platform detection fails": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("", "", errors.New("some error"))
			},
			wantedErr: errors.New("get docker engine platform: some error"),
		},
		"initialize a job in environments with only private subnets": {
			inApp:              "sample",
			inName:             "mailer",
			inType:             manifestinfo.ScheduledJobType,
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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockJobInit: func(m *mocks.MockjobInitializer) {
				m.EXPECT().Job(&initialize.JobProps{
					WorkloadProps: initialize.WorkloadProps{
						App:                     "sample",
						Name:                    "mailer",
						Type:                    "Scheduled Job",
						DockerfilePath:          "./Dockerfile",
						Platform:                manifest.PlatformArgsOrString{},
						PrivateOnlyEnvironments: []string{"test"},
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return([]*config.Environment{
					{
						App:  "sample",
						Name: "test",
					},
				}, nil)
			},
			mockEnvDescriber: func(m *mocks.MockenvDescriber) {
				m.EXPECT().Manifest().Return(mockEnvironmentManifest, nil)
			},
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
			mockStore := mocks.NewMockstore(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)

			if tc.mockJobInit != nil {
				tc.mockJobInit(mockJobInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}
			if tc.mockDockerEngine != nil {
				tc.mockDockerEngine(mockDockerEngine)
			}
			if tc.mockStore != nil {
				tc.mockStore(mockStore)
			}
			if tc.mockEnvDescriber != nil {
				tc.mockEnvDescriber(mockEnvDescriber)
			}

			opts := initJobOpts{
				initJobVars: initJobVars{
					initWkldVars: initWkldVars{
						appName:           tc.inApp,
						name:              tc.inName,
						wkldType:          tc.inType,
						dockerfilePath:    tc.inDf,
						allowAppDowngrade: true,
					},
					schedule: tc.inSchedule,
				},
				init: mockJobInitializer,
				initParser: func(s string) dockerfileParser {
					return mockDockerfile
				},
				dockerEngine:   mockDockerEngine,
				manifestExists: tc.inManifestExists,
				store:          mockStore,
				initEnvDescriber: func(string, string) (envDescriber, error) {
					return mockEnvDescriber, nil
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

func Test_ValidateSchedule(t *testing.T) {
	testCases := map[string]struct {
		inSchedule string

		wantedErr error
	}{
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			err := validateSchedule(tc.inSchedule)
			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
