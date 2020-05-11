// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
)

func TestAppDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string

		mockWs    func(m *mocks.MockwsAppReader)
		mockStore func(m *mocks.Mockstore)

		wantedError error
	}{
		"no existing projects": {
			mockWs:    func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errNoAppInWorkspace,
		},
		"with workspace error": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("list applications in the workspace: some error"),
		},
		"with application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return([]string{}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("application frontend not found in the workspace"),
		},
		"with unknown environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			mockWs:        func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(nil, errors.New("unknown env"))
			},

			wantedError: errors.New("get environment test from metadata store: unknown env"),
		},
		"successful validation": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inEnvName:     "test",
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
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
			mockWs := mocks.NewMockwsAppReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inProjectName,
					},
					Name:    tc.inAppName,
					EnvName: tc.inEnvName,
				},
				workspaceService: mockWs,
				store:            mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string
		inImageTag    string

		mockWs     func(m *mocks.MockwsAppReader)
		mockStore  func(m *mocks.Mockstore)
		mockPrompt func(m *mocks.Mockprompter)

		wantedAppName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"no applications in the workspace": {
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return([]string{}, nil)
			},
			mockStore:  func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: errors.New("no applications found in the workspace"),
		},
		"default to single application": {
			inEnvName:  "test",
			inImageTag: "latest",
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			mockStore:  func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"prompts for application name if there are more than one option": {
			inEnvName:  "test",
			inImageTag: "latest",
			mockWs: func(m *mocks.MockwsAppReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend", "webhook"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne("Select an application", "", []string{"frontend", "webhook"}).
					Return("frontend", nil)
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"fails to list environments": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("phonetool").Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {
			},

			wantedError: errors.New("get environments for project phonetool from metadata store: some error"),
		},
		"no existing environments": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
			},

			wantedError: errors.New("no environments found in project phonetool"),
		},
		"defaults to single environment": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					{
						Name: "test",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"prompts for environment name if there are more than one option": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *mocks.MockwsAppReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*config.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod-iad",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne("Select an environment", "", []string{"test", "prod-iad"}).
					Return("prod-iad", nil)
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockWs := mocks.NewMockwsAppReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			tc.mockPrompt(mockPrompt)

			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inProjectName,
						prompt:  mockPrompt,
					},
					Name:     tc.inAppName,
					EnvName:  tc.inEnvName,
					ImageTag: tc.inImageTag,
				},
				workspaceService: mockWs,
				store:            mockStore,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.Nil(t, err)
				require.Equal(t, tc.wantedAppName, opts.Name)
				require.Equal(t, tc.wantedEnvName, opts.EnvName)
				require.Equal(t, tc.wantedImageTag, opts.ImageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestAppDeployOpts_getAppDockerfilePath(t *testing.T) {
	var mockWorkspace *mocks.MockwsAppReader

	mockError := errors.New("mockError")
	mockManifest := []byte(`name: appA
type: 'Load Balanced Web Service'
image:
  build: appA/Dockerfile
`)

	tests := map[string]struct {
		inputApp   string
		setupMocks func(controller *gomock.Controller)

		wantPath string
		wantErr  error
	}{
		"should return error if workspaceService ReadFile returns error": {
			inputApp: "appA",
			setupMocks: func(controller *gomock.Controller) {
				mockWorkspace = mocks.NewMockwsAppReader(controller)

				gomock.InOrder(
					mockWorkspace.EXPECT().ReadServiceManifest("appA").Times(1).Return(nil, mockError),
				)
			},
			wantPath: "",
			wantErr:  fmt.Errorf("read manifest file %s: %w", "appA", mockError),
		},
		"should trim the manifest DockerfilePath if it contains /Dockerfile": {
			inputApp: "appA",
			setupMocks: func(controller *gomock.Controller) {
				mockWorkspace = mocks.NewMockwsAppReader(controller)

				gomock.InOrder(
					mockWorkspace.EXPECT().ReadServiceManifest("appA").Times(1).Return(mockManifest, nil),
				)
			},
			wantPath: "appA",
			wantErr:  nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					Name: test.inputApp,
				},
				workspaceService: mockWorkspace,
			}

			gotPath, gotErr := opts.getAppDockerfilePath()

			require.Equal(t, test.wantPath, gotPath)
			require.Equal(t, test.wantErr, gotErr)
		})
	}
}

func TestAppDeployOpts_pushAddonsTemplateToS3Bucket(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		inputApp      string
		inEnvironment *config.Environment
		inProject     *config.Application

		mockProjectResourcesGetter func(m *mocks.MockprojectResourcesGetter)
		mockS3Svc                  func(m *mocks.MockartifactUploader)
		mockAddons                 func(m *mocks.Mocktemplater)

		wantPath string
		wantErr  error
	}{
		"should push addons template to S3 bucket": {
			inputApp: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inProject: &config.Application{
				Name: "mockApp",
			},
			mockProjectResourcesGetter: func(m *mocks.MockprojectResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name: "mockApp",
				}, "us-west-2").Return(&stack.AppRegionalResources{
					S3Bucket: "mockBucket",
				}, nil)
			},
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("some data", nil)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {
				m.EXPECT().PutArtifact("mockBucket", "mockSvc.addons.stack.yml", gomock.Any()).Return("https://mockS3DomainName/mockPath", nil)
			},

			wantErr:  nil,
			wantPath: "https://mockS3DomainName/mockPath",
		},
		"should return error if fail to get project resources": {
			inputApp: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inProject: &config.Application{
				Name: "mockApp",
			},
			mockProjectResourcesGetter: func(m *mocks.MockprojectResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name: "mockApp",
				}, "us-west-2").Return(nil, mockError)
			},
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("some data", nil)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {},

			wantErr: fmt.Errorf("get project resources: some error"),
		},
		"should return error if fail to upload to S3 bucket": {
			inputApp: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inProject: &config.Application{
				Name: "mockApp",
			},

			mockProjectResourcesGetter: func(m *mocks.MockprojectResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name: "mockApp",
				}, "us-west-2").Return(&stack.AppRegionalResources{
					S3Bucket: "mockBucket",
				}, nil)
			},
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("some data", nil)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {
				m.EXPECT().PutArtifact("mockBucket", "mockSvc.addons.stack.yml", gomock.Any()).Return("", mockError)
			},

			wantErr: fmt.Errorf("put addons artifact to bucket mockBucket: some error"),
		},
		"should return empty url if the application doesn't have any addons": {
			inputApp: "mockSvc",
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("", &addons.ErrDirNotExist{
					SvcName: "mockSvc",
				})
			},
			mockProjectResourcesGetter: func(m *mocks.MockprojectResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Times(0)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {
				m.EXPECT().PutArtifact(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantPath: "",
		},
		"should fail if addons cannot be retrieved from workspace": {
			inputApp: "mockSvc",
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("", mockError)
			},
			mockProjectResourcesGetter: func(m *mocks.MockprojectResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Times(0)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {
				m.EXPECT().PutArtifact(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: fmt.Errorf("retrieve addons template: %w", mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectSvc := mocks.NewMockstore(ctrl)
			mockProjectResourcesGetter := mocks.NewMockprojectResourcesGetter(ctrl)
			mockS3Svc := mocks.NewMockartifactUploader(ctrl)
			mockAddons := mocks.NewMocktemplater(ctrl)
			tc.mockProjectResourcesGetter(mockProjectResourcesGetter)
			tc.mockS3Svc(mockS3Svc)
			tc.mockAddons(mockAddons)

			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					Name: tc.inputApp,
				},
				store:             mockProjectSvc,
				projectCFSvc:      mockProjectResourcesGetter,
				addonsSvc:         mockAddons,
				s3Service:         mockS3Svc,
				targetEnvironment: tc.inEnvironment,
				targetProject:     tc.inProject,
			}

			gotPath, gotErr := opts.pushAddonsTemplateToS3Bucket()

			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantPath, gotPath)
			}
		})
	}
}
