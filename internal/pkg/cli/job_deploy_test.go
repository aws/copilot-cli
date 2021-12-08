// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deployJobMocks struct {
	mockWs                 *mocks.MockwsJobDirReader
	mockimageBuilderPusher *mocks.MockimageBuilderPusher
	mockInterpolator       *mocks.Mockinterpolator
	mockS3Svc              *mocks.MockartifactUploader
	mockAddons             *mocks.Mocktemplater
}

func TestJobDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inJobName string

		mockWs    func(m *mocks.MockwsJobDirReader)
		mockStore func(m *mocks.Mockstore)

		wantedError error
	}{
		"no existing applications": {
			mockWs:    func(m *mocks.MockwsJobDirReader) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errNoAppInWorkspace,
		},
		"with workspace error": {
			inAppName: "phonetool",
			inJobName: "resizer",
			mockWs: func(m *mocks.MockwsJobDirReader) {
				m.EXPECT().ListJobs().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("list jobs in the workspace: some error"),
		},
		"with job not in workspace": {
			inAppName: "phonetool",
			inJobName: "resizer",
			mockWs: func(m *mocks.MockwsJobDirReader) {
				m.EXPECT().ListJobs().Return([]string{}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("job resizer not found in the workspace"),
		},
		"with unknown environment": {
			inAppName: "phonetool",
			inEnvName: "test",
			mockWs:    func(m *mocks.MockwsJobDirReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(nil, errors.New("unknown env"))
			},

			wantedError: errors.New("get environment test configuration: unknown env"),
		},
		"successful validation": {
			inAppName: "phonetool",
			inJobName: "resizer",
			inEnvName: "test",
			mockWs: func(m *mocks.MockwsJobDirReader) {
				m.EXPECT().ListJobs().Return([]string{"resizer"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWs := mocks.NewMockwsJobDirReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					appName: tc.inAppName,
					name:    tc.inJobName,
					envName: tc.inEnvName,
				},
				ws:    mockWs,
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJobDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inEnvName  string
		inJobName  string
		inImageTag string

		wantedCalls func(m *mocks.MockwsSelector)

		wantedJobName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"prompts for environment name and job names": {
			inAppName:  "phonetool",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job("Select a job from your workspace", "").Return("resizer", nil)
				m.EXPECT().Environment("Select an environment", "", "phonetool").Return("prod-iad", nil)
			},

			wantedJobName:  "resizer",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
		"don't call selector if flags are provided": {
			inAppName:  "phonetool",
			inEnvName:  "prod-iad",
			inJobName:  "resizer",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedJobName:  "resizer",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockSel := mocks.NewMockwsSelector(ctrl)

			tc.wantedCalls(mockSel)
			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					appName:  tc.inAppName,
					name:     tc.inJobName,
					envName:  tc.inEnvName,
					imageTag: tc.inImageTag,
				},
				sel: mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedJobName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
				require.Equal(t, tc.wantedImageTag, opts.imageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestJobDeployOpts_configureContainerImage(t *testing.T) {
	mockError := errors.New("mockError")
	mockManifest := []byte(`name: mailer
type: 'Scheduled Job'
image:
  build:
    dockerfile: path/to/Dockerfile
    context: path
on:
  schedule: "@daily"`)
	mockMftNoBuild := []byte(`name: mailer
type: 'Scheduled Job'
image:
  location: foo/bar
on:
  schedule: "@daily"`)
	mockMftBuildString := []byte(`name: mailer
type: 'Scheduled Job'
image:
  build: path/to/Dockerfile
on:
  schedule: "@daily"`)
	mockMftNoContext := []byte(`name: mailer
type: 'Scheduled Job'
image:
  build:
    dockerfile: path/to/Dockerfile
on:
  schedule: "@daily"`)

	tests := map[string]struct {
		inputJob   string
		setupMocks func(mocks deployJobMocks)

		wantErr      error
		wantedDigest string
	}{
		"should return error if ws ReadFile returns error": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(nil, mockError),
				)
			},
			wantErr: fmt.Errorf("read job %s manifest: %w", "mailer", mockError),
		},
		"should return error if interpolation fail": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest(gomock.Any()).Return(mockManifest, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockManifest)).Return("", mockError),
				)
			},
			wantErr: fmt.Errorf("interpolate environment variables for mailer manifest: %w", mockError),
		},
		"should return error if workspace methods fail": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest(gomock.Any()).Return(mockManifest, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockManifest)).Return(string(mockManifest), nil),
					m.mockWs.EXPECT().Path().Return("", mockError),
				)
			},
			wantErr: fmt.Errorf("get workspace path: %w", mockError),
		},
		"success without building and pushing": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(mockMftNoBuild, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockMftNoBuild)).Return(string(mockMftNoBuild), nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), gomock.Any()).Times(0),
				)
			},
		},
		"should return error if fail to build and push": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(mockManifest, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockManifest)).Return(string(mockManifest), nil),
					m.mockWs.EXPECT().Path().Return("/ws/root/copilot", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), gomock.Any()).Return("", mockError),
				)
			},
			wantErr: fmt.Errorf("build and push image: mockError"),
		},
		"success": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(mockManifest, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockManifest)).Return(string(mockManifest), nil),
					m.mockWs.EXPECT().Path().Return("/ws/root", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path"),
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
		"using simple buildstring (backwards compatible)": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(mockMftBuildString, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockMftBuildString)).Return(string(mockMftBuildString), nil),
					m.mockWs.EXPECT().Path().Return("/ws/root", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path", "to"),
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
		"without context field in overrides": {
			inputJob: "mailer",
			setupMocks: func(m deployJobMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadWorkloadManifest("mailer").Return(mockMftNoContext, nil),
					m.mockInterpolator.EXPECT().Interpolate(string(mockMftNoContext)).Return(string(mockMftNoContext), nil),
					m.mockWs.EXPECT().Path().Return("/ws/root", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path", "to"),
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockwsJobDirReader(ctrl)
			mockimageBuilderPusher := mocks.NewMockimageBuilderPusher(ctrl)
			mockInterpolator := mocks.NewMockinterpolator(ctrl)
			mocks := deployJobMocks{
				mockWs:                 mockWorkspace,
				mockimageBuilderPusher: mockimageBuilderPusher,
				mockInterpolator:       mockInterpolator,
			}
			test.setupMocks(mocks)
			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					name: test.inputJob,
				},
				unmarshal:          manifest.UnmarshalWorkload,
				imageBuilderPusher: mockimageBuilderPusher,
				ws:                 mockWorkspace,
				newInterpolator: func(app, env string) interpolator {
					return mockInterpolator
				},
			}

			gotErr := opts.configureContainerImage()

			if test.wantErr != nil {
				require.EqualError(t, gotErr, test.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, test.wantedDigest, opts.imageDigest)
			}
		})
	}
}

func TestJobDeployOpts_pushToS3Bucket(t *testing.T) {
	const (
		mockJobName         = "mockJob"
		mockEnvFile         = "foo.env"
		mockS3Bucket        = "mockBucket"
		mockAddonsS3URL     = "https://mockS3DomainName/mockPath"
		mockBadEnvFileS3URL = "badURL"
		mockEnvFileS3URL    = "https://stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf.s3.us-west-2.amazonaws.com/manual/1638391936/env"
		mockEnvFileS3ARN    = "arn:aws:s3:::stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/env"
	)
	mockError := errors.New("some error")
	tests := map[string]struct {
		inEnvFile     string
		inEnvironment *config.Environment
		inApp         *config.Application

		mock func(m *deployJobMocks)

		wantAddonsURL  string
		wantEnvFileARN string
		wantErr        error
	}{
		"error if fail to put env file to s3 bucket": {
			inEnvFile: mockEnvFile,
			mock: func(m *deployJobMocks) {
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, mockEnvFile, gomock.Any()).
					Return("", mockError)
			},
			wantErr: fmt.Errorf("put env file foo.env artifact to bucket mockBucket: some error"),
		},
		"error if fail to parse s3 url": {
			inEnvFile: mockEnvFile,
			mock: func(m *deployJobMocks) {
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, mockEnvFile, gomock.Any()).
					Return(mockBadEnvFileS3URL, nil)

			},
			wantErr: fmt.Errorf("parse s3 url: cannot parse S3 URL badURL into bucket name and key"),
		},
		"error if fail to find the partition": {
			inEnvFile: mockEnvFile,
			inEnvironment: &config.Environment{
				Region: "sun-south-0",
			},
			mock: func(m *deployJobMocks) {
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, mockEnvFile, gomock.Any()).
					Return(mockEnvFileS3URL, nil)
			},
			wantErr: fmt.Errorf("find the partition for region sun-south-0"),
		},
		"should push addons template to S3 bucket": {
			inEnvFile: mockEnvFile,
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: "mockApp",
			},
			mock: func(m *deployJobMocks) {
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, mockEnvFile, gomock.Any()).
					Return(mockEnvFileS3URL, nil)
				m.mockAddons.EXPECT().Template().Return("some data", nil)
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, "mockJob.addons.stack.yml", gomock.Any()).
					Return(mockAddonsS3URL, nil)
			},

			wantAddonsURL:  mockAddonsS3URL,
			wantEnvFileARN: mockEnvFileS3ARN,
		},
		"should return error if fail to upload to S3 bucket": {
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: "mockApp",
			},
			mock: func(m *deployJobMocks) {
				m.mockAddons.EXPECT().Template().Return("some data", nil)
				m.mockS3Svc.EXPECT().PutArtifact(mockS3Bucket, "mockJob.addons.stack.yml", gomock.Any()).
					Return("", mockError)
			},

			wantErr: fmt.Errorf("put addons artifact to bucket mockBucket: some error"),
		},
		"should return empty url if the service doesn't have any addons and env files": {
			mock: func(m *deployJobMocks) {
				m.mockAddons.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{
					WlName: "mockJob",
				})
			},
		},
		"should fail if addons cannot be retrieved from workspace": {
			mock: func(m *deployJobMocks) {
				m.mockAddons.EXPECT().Template().Return("", mockError)
			},
			wantErr: fmt.Errorf("retrieve addons template: %w", mockError),
		},
	}

	for name, tc := range tests {
		fs := afero.NewMemMapFs()
		afero.WriteFile(fs, mockEnvFile, []byte{}, 0644)
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployJobMocks{
				mockWs:     mocks.NewMockwsJobDirReader(ctrl),
				mockS3Svc:  mocks.NewMockartifactUploader(ctrl),
				mockAddons: mocks.NewMocktemplater(ctrl),
			}
			tc.mock(m)

			opts := deployJobOpts{
				deployWkldVars: deployWkldVars{
					name: mockJobName,
				},
				addons:            m.mockAddons,
				s3:                m.mockS3Svc,
				ws:                m.mockWs,
				fs:                &afero.Afero{Fs: fs},
				appliedManifest:   &mockWorkloadMft{tc.inEnvFile},
				workspacePath:     ".",
				targetEnvironment: tc.inEnvironment,
				targetApp:         tc.inApp,
				appEnvResources: &stack.AppRegionalResources{
					S3Bucket: mockS3Bucket,
				},
			}

			gotErr := opts.pushArtifactsToS3()

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantAddonsURL, opts.addonsURL)
				require.Equal(t, tc.wantEnvFileARN, opts.envFileARN)
			}
		})
	}
}
