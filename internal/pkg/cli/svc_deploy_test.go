// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/aws-sdk-go/aws"
	addon "github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
)

type deploySvcMocks struct {
	mockWs                 *mocks.MockwsSvcDirReader
	mockimageBuilderPusher *mocks.MockimageBuilderPusher
	mockAppResourcesGetter *mocks.MockappResourcesGetter
	mockAppVersionGetter   *mocks.MockversionGetter
	mockEndpointGetter     *mocks.MockendpointGetter
	mockServiceDeployer    *mocks.MockserviceDeployer
	mockSpinner            *mocks.Mockprogress
	mockServiceUpdater     *mocks.MockserviceUpdater
}

func TestSvcDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inSvcName string

		mockWs    func(m *mocks.MockwsSvcDirReader)
		mockStore func(m *mocks.Mockstore)

		wantedError error
	}{
		"no existing applications": {
			mockWs:    func(m *mocks.MockwsSvcDirReader) {},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errNoAppInWorkspace,
		},
		"with workspace error": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			mockWs: func(m *mocks.MockwsSvcDirReader) {
				m.EXPECT().ServiceNames().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("list services in the workspace: some error"),
		},
		"with service not in workspace": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			mockWs: func(m *mocks.MockwsSvcDirReader) {
				m.EXPECT().ServiceNames().Return([]string{}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: errors.New("service frontend not found in the workspace"),
		},
		"with unknown environment": {
			inAppName: "phonetool",
			inEnvName: "test",
			mockWs:    func(m *mocks.MockwsSvcDirReader) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(nil, errors.New("unknown env"))
			},

			wantedError: errors.New("get environment test configuration: unknown env"),
		},
		"successful validation": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			inEnvName: "test",
			mockWs: func(m *mocks.MockwsSvcDirReader) {
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

			mockWs := mocks.NewMockwsSvcDirReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName: tc.inAppName,
					name:    tc.inSvcName,
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

func TestSvcDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inEnvName  string
		inSvcName  string
		inImageTag string

		wantedCalls func(m *mocks.MockwsSelector)

		wantedSvcName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"prompts for environment name and service names": {
			inAppName:  "phonetool",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service("Select a service in your workspace", "").Return("frontend", nil)
				m.EXPECT().Environment("Select an environment", "", "phonetool").Return("prod-iad", nil)
			},

			wantedSvcName:  "frontend",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
		"don't call selector if flags are provided": {
			inAppName:  "phonetool",
			inEnvName:  "prod-iad",
			inSvcName:  "frontend",
			inImageTag: "latest",
			wantedCalls: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedSvcName:  "frontend",
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
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					appName:  tc.inAppName,
					name:     tc.inSvcName,
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
				require.Equal(t, tc.wantedSvcName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
				require.Equal(t, tc.wantedImageTag, opts.imageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestSvcDeployOpts_configureContainerImage(t *testing.T) {
	mockError := errors.New("mockError")
	mockManifest := []byte(`name: serviceA
type: 'Load Balanced Web Service'
image:
  build:
    dockerfile: path/to/Dockerfile
    context: path
  port: 80
`)
	mockManifestWithBadPlatform := []byte(`name: serviceA
type: 'Load Balanced Web Service'
platform: linus/abc123
image:
  build:
    dockerfile: path/to/Dockerfile
    context: path
  port: 80
`)
	mockManifestWithGoodPlatform := []byte(`name: serviceA
type: 'Load Balanced Web Service'
platform: linux/amd64
image:
  build:
    dockerfile: path/to/Dockerfile
    context: path
  port: 80
`)
	mockMftNoBuild := []byte(`name: serviceA
type: 'Load Balanced Web Service'
image:
  location: foo/bar
  port: 80
`)
	mockMftBuildString := []byte(`name: serviceA
type: 'Load Balanced Web Service'
image:
  build: path/to/Dockerfile
  port: 80
`)
	mockMftNoContext := []byte(`name: serviceA
type: 'Load Balanced Web Service'
image:
  build:
    dockerfile: path/to/Dockerfile
  port: 80`)

	tests := map[string]struct {
		inputSvc   string
		setupMocks func(mocks deploySvcMocks)

		wantErr      error
		wantedDigest string
	}{
		"should return error if ws ReadFile returns error": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(nil, mockError),
				)
			},
			wantErr: fmt.Errorf("read service %s manifest file: %w", "serviceA", mockError),
		},
		"should return error if workspace methods fail": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest(gomock.Any()).Return(mockManifest, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("", mockError),
				)
			},
			wantErr: fmt.Errorf("get copilot directory: %w", mockError),
		},
		"should return error if platform is invalid": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockManifestWithBadPlatform, nil),
				)
			},
			wantErr: errors.New("validate manifest against environment : validate \"platform\": platform linus/abc123 is invalid; valid platforms are: linux/amd64, linux/x86_64, windows/amd64 and windows/x86_64"),
		},
		"success with valid platform": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockManifestWithGoodPlatform, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("/ws/root/copilot", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path"),
						Platform:   "linux/amd64",
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
		"success without building and pushing": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockMftNoBuild, nil),
					m.mockWs.EXPECT().CopilotDirPath().Times(0),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), gomock.Any()).Times(0),
				)
			},
		},
		"should return error if fail to build and push": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockManifest, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("/ws/root/copilot", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), gomock.Any()).Return("", mockError),
				)
			},
			wantErr: fmt.Errorf("build and push image: mockError"),
		},
		"success": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockManifest, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("/ws/root/copilot", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path"),
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
		"using simple buildstring (backwards compatible)": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockMftBuildString, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("/ws/root/copilot", nil),
					m.mockimageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
						Dockerfile: filepath.Join("/ws", "root", "path", "to", "Dockerfile"),
						Context:    filepath.Join("/ws", "root", "path", "to"),
					}).Return("sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49", nil),
				)
			},
			wantedDigest: "sha256:741d3e95eefa2c3b594f970a938ed6e497b50b3541a5fdc28af3ad8959e76b49",
		},
		"without context field in overrides": {
			inputSvc: "serviceA",
			setupMocks: func(m deploySvcMocks) {
				gomock.InOrder(
					m.mockWs.EXPECT().ReadServiceManifest("serviceA").Return(mockMftNoContext, nil),
					m.mockWs.EXPECT().CopilotDirPath().Return("/ws/root/copilot", nil),
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

			mockWorkspace := mocks.NewMockwsSvcDirReader(ctrl)
			mockimageBuilderPusher := mocks.NewMockimageBuilderPusher(ctrl)
			mocks := deploySvcMocks{
				mockWs:                 mockWorkspace,
				mockimageBuilderPusher: mockimageBuilderPusher,
			}
			test.setupMocks(mocks)
			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					name: test.inputSvc,
				},
				unmarshal:          manifest.UnmarshalWorkload,
				imageBuilderPusher: mockimageBuilderPusher,
				ws:                 mockWorkspace,
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

func TestSvcDeployOpts_pushAddonsTemplateToS3Bucket(t *testing.T) {
	mockError := errors.New("some error")
	tests := map[string]struct {
		inputSvc      string
		inEnvironment *config.Environment
		inApp         *config.Application

		mockAppResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockS3Svc              func(m *mocks.MockartifactUploader)
		mockAddons             func(m *mocks.Mocktemplater)

		wantPath string
		wantErr  error
	}{
		"should push addons template to S3 bucket": {
			inputSvc: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: "mockApp",
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {
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
		"should return error if fail to get app resources": {
			inputSvc: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: "mockApp",
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name: "mockApp",
				}, "us-west-2").Return(nil, mockError)
			},
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("some data", nil)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {},

			wantErr: fmt.Errorf("get application mockApp resources from region us-west-2: some error"),
		},
		"should return error if fail to upload to S3 bucket": {
			inputSvc: "mockSvc",
			inEnvironment: &config.Environment{
				Name:   "mockEnv",
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: "mockApp",
			},

			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {
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
		"should return empty url if the service doesn't have any addons": {
			inputSvc: "mockSvc",
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{
					WlName: "mockSvc",
				})
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetAppResourcesByRegion(gomock.Any(), gomock.Any()).Times(0)
			},
			mockS3Svc: func(m *mocks.MockartifactUploader) {
				m.EXPECT().PutArtifact(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantPath: "",
		},
		"should fail if addons cannot be retrieved from workspace": {
			inputSvc: "mockSvc",
			mockAddons: func(m *mocks.Mocktemplater) {
				m.EXPECT().Template().Return("", mockError)
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {
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

			mockAppSvc := mocks.NewMockstore(ctrl)
			mockAppResourcesGetter := mocks.NewMockappResourcesGetter(ctrl)
			mockS3Svc := mocks.NewMockartifactUploader(ctrl)
			mockAddons := mocks.NewMocktemplater(ctrl)
			tc.mockAppResourcesGetter(mockAppResourcesGetter)
			tc.mockS3Svc(mockS3Svc)
			tc.mockAddons(mockAddons)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					name: tc.inputSvc,
				},
				store:             mockAppSvc,
				appCFN:            mockAppResourcesGetter,
				addons:            mockAddons,
				s3:                mockS3Svc,
				targetEnvironment: tc.inEnvironment,
				targetApp:         tc.inApp,
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

func TestSvcDeployOpts_deploySvc(t *testing.T) {
	mockError := errors.New("some error")
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockSvcName   = "mockSvc"
		mockAddonsURL = "mockAddonsURL"
	)
	tests := map[string]struct {
		inAliases      manifest.Alias
		inApp          *config.Application
		inEnvironment  *config.Environment
		inBuildRequire bool
		inForceDeploy  bool

		mock func(m *deploySvcMocks)

		wantErr error
	}{
		"fail to read service manifest": {
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("read service %s manifest file: %w", mockSvcName, mockError),
		},
		"fail to get app resources": {
			inBuildRequire: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppResourcesGetter.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name: mockAppName,
				}, "us-west-2").Return(nil, mockError)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf("get application %s resources from region us-west-2: %w", mockAppName, mockError),
		},
		"alias used while app is not associated with a domain": {
			inAliases: manifest.Alias{String: aws.String("mockAlias")},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: errors.New("alias specified when application is not associated with a domain"),
		},
		"cannot to find ECR repo": {
			inBuildRequire: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:      mockAppName,
				AccountID: "1234567890",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppResourcesGetter.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:      mockAppName,
					AccountID: "1234567890",
				}, "us-west-2").Return(&stack.AppRegionalResources{
					RepositoryURLs: map[string]string{},
				}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf("ECR repository not found for service mockSvc in region us-west-2 and account 1234567890"),
		},
		"fail to get app version": {
			inAliases: manifest.Alias{String: aws.String("mockAlias")},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("", mockError)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf("get version for app %s: %w", mockAppName, mockError),
		},
		"fail to enable https alias because of incompatible app version": {
			inAliases: manifest.Alias{String: aws.String("mockAlias")},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v0.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf("alias is not compatible with application versions below %s", deploy.AliasLeastAppTemplateVersion),
		},
		"fail to enable https alias because of invalid alias": {
			inAliases: manifest.Alias{String: aws.String("v1.v2.mockDomain")},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)

			},
			wantErr: fmt.Errorf(`alias "v1.v2.mockDomain" is not supported in hosted zones managed by Copilot`),
		},
		"error if fail to deploy service": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantErr: fmt.Errorf("deploy service: some error"),
		},
		"error if change set is empty but force flag is not set": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).Return(cloudformation.NewMockErrChangeSetEmpty())
			},
			wantErr: fmt.Errorf("deploy service: change set with name mockChangeSet for stack mockStack has no changes"),
		},
		"error if fail to force an update": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockSvcName, mockEnvName))
				m.mockServiceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockSvcName).Return(mockError)
				m.mockSpinner.EXPECT().Stop(log.Serrorf(fmtForceUpdateSvcFailed, mockSvcName, mockEnvName, mockError))
			},
			wantErr: fmt.Errorf("force an update for service mockSvc: some error"),
		},
		"error if fail to force an update because of timeout": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockSvcName, mockEnvName))
				m.mockServiceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockSvcName).
					Return(&ecs.ErrWaitServiceStableTimeout{})
				m.mockSpinner.EXPECT().Stop(
					log.Serror(fmt.Sprintf("%s  Run %s to check for the fail reason.\n",
						fmt.Sprintf(fmtForceUpdateSvcFailed, mockSvcName, mockEnvName, &ecs.ErrWaitServiceStableTimeout{}),
						color.HighlightCode(fmt.Sprintf("copilot svc status --name %s --env %s", mockSvcName, mockEnvName)))))
			},
			wantErr: fmt.Errorf("force an update for service mockSvc: max retries 0 exceeded"),
		},
		"success": {
			inAliases: manifest.Alias{
				StringSlice: []string{
					"v1.mockDomain",
					"mockDomain",
				},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		"success with force update": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deploySvcMocks) {
				m.mockWs.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), gomock.Any()).Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockSvcName, mockEnvName))
				m.mockServiceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockSvcName).Return(nil)
				m.mockSpinner.EXPECT().Stop(log.Ssuccessf(fmtForceUpdateSvcComplete, mockSvcName, mockEnvName))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deploySvcMocks{
				mockWs:                 mocks.NewMockwsSvcDirReader(ctrl),
				mockAppResourcesGetter: mocks.NewMockappResourcesGetter(ctrl),
				mockAppVersionGetter:   mocks.NewMockversionGetter(ctrl),
				mockEndpointGetter:     mocks.NewMockendpointGetter(ctrl),
				mockServiceDeployer:    mocks.NewMockserviceDeployer(ctrl),
				mockServiceUpdater:     mocks.NewMockserviceUpdater(ctrl),
				mockSpinner:            mocks.NewMockprogress(ctrl),
			}
			tc.mock(m)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					name:           mockSvcName,
					appName:        mockAppName,
					envName:        mockEnvName,
					forceNewUpdate: tc.inForceDeploy,
				},
				ws:            m.mockWs,
				buildRequired: tc.inBuildRequire,
				appCFN:        m.mockAppResourcesGetter,
				newAppVersionGetter: func(s string) (versionGetter, error) {
					return m.mockAppVersionGetter, nil
				},
				endpointGetter:    m.mockEndpointGetter,
				targetApp:         tc.inApp,
				targetEnvironment: tc.inEnvironment,
				unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.LoadBalancedWebService{
						Workload: manifest.Workload{
							Name: aws.String(mockSvcName),
						},
						LoadBalancedWebServiceConfig: manifest.LoadBalancedWebServiceConfig{
							ImageConfig: manifest.ImageWithPortAndHealthcheck{
								ImageWithPort: manifest.ImageWithPort{
									Image: manifest.Image{
										Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
									},
									Port: aws.Uint16(80),
								},
							},
							RoutingRule: manifest.RoutingRule{
								Alias: tc.inAliases,
							},
						},
					}, nil
				},
				svcCFN:        m.mockServiceDeployer,
				svcUpdater:    m.mockServiceUpdater,
				newSvcUpdater: func(f func(*session.Session) serviceUpdater) {},
				spinner:       m.mockSpinner,
			}

			gotErr := opts.deploySvc(mockAddonsURL)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

type deployRDSvcMocks struct {
	mockWorkspace          *mocks.MockwsSvcDirReader
	mockAppResourcesGetter *mocks.MockappResourcesGetter
	mockAppVersionGetter   *mocks.MockversionGetter
	mockEndpointGetter     *mocks.MockendpointGetter
	mockIdentity           *mocks.MockidentityService
	mockUploader           *mocks.MockcustomResourcesUploader
}

func TestSvcDeployOpts_rdWebServiceStackConfiguration(t *testing.T) {
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockSvcName   = "mockSvc"
		mockAddonsURL = "mockAddonsURL"
	)
	tests := map[string]struct {
		inAlias        string
		inApp          *config.Application
		inEnvironment  *config.Environment
		inBuildRequire bool

		mock func(m *deployRDSvcMocks)

		wantURLs map[string]string
		wantErr  error
	}{
		"alias used while app is not associated with a domain": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},

			wantErr: errors.New("alias specified when application is not associated with a domain"),
		},
		"fail to get identity for rd web service": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))
			},

			wantErr: fmt.Errorf("get identity: some error"),
		},
		"errors getting app resources by region": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
				m.mockAppResourcesGetter.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:   mockAppName,
					Domain: "mockDomain",
				}, "us-west-2").Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("get application mockApp resources from region us-west-2: some error"),
		},
		"invalid alias with unknown domain": {
			inAlias: "v1.someRandomDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
			},

			wantErr: fmt.Errorf("alias is not supported in hosted zones that are not managed by Copilot"),
		},
		"invalid environment level alias": {
			inAlias: "mockEnv.mockApp.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
			},

			wantErr: fmt.Errorf("mockEnv.mockApp.mockDomain is an environment-level alias, which is not supported yet"),
		},
		"invalid application level alias": {
			inAlias: "someSub.mockApp.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
			},

			wantErr: fmt.Errorf("someSub.mockApp.mockDomain is an application-level alias, which is not supported yet"),
		},
		"invalid root level alias": {
			inAlias: "mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
			},

			wantErr: fmt.Errorf("mockDomain is a root domain alias, which is not supported yet"),
		},
		"fail to upload custom resource scripts": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
				m.mockAppResourcesGetter.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:   mockAppName,
					Domain: "mockDomain",
				}, "us-west-2").Return(&stack.AppRegionalResources{
					S3Bucket: "mockBucket",
				}, nil)
				m.mockUploader.EXPECT().UploadRequestDrivenWebServiceCustomResources(gomock.Any()).Return(nil, errors.New("some error"))
			},

			wantErr: fmt.Errorf("upload custom resources to bucket mockBucket: some error"),
		},
		"success": {
			inAlias: "v1.mockDomain",
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployRDSvcMocks) {
				m.mockWorkspace.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockIdentity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "1234",
				}, nil)
				m.mockAppResourcesGetter.EXPECT().GetAppResourcesByRegion(&config.Application{
					Name:   mockAppName,
					Domain: "mockDomain",
				}, "us-west-2").Return(&stack.AppRegionalResources{
					S3Bucket: "mockBucket",
				}, nil)
				m.mockUploader.EXPECT().UploadRequestDrivenWebServiceCustomResources(gomock.Any()).Return(map[string]string{
					"mockResource2": "mockURL2",
				}, nil)
			},
			wantURLs: map[string]string{
				"mockResource1": "mockURL1",
				"mockResource2": "mockURL2",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployRDSvcMocks{
				mockWorkspace:          mocks.NewMockwsSvcDirReader(ctrl),
				mockAppResourcesGetter: mocks.NewMockappResourcesGetter(ctrl),
				mockAppVersionGetter:   mocks.NewMockversionGetter(ctrl),
				mockEndpointGetter:     mocks.NewMockendpointGetter(ctrl),
				mockIdentity:           mocks.NewMockidentityService(ctrl),
				mockUploader:           mocks.NewMockcustomResourcesUploader(ctrl),
			}
			tc.mock(m)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					name:    mockSvcName,
					appName: mockAppName,
					envName: mockEnvName,
				},
				ws:     m.mockWorkspace,
				appCFN: m.mockAppResourcesGetter,
				newAppVersionGetter: func(s string) (versionGetter, error) {
					return m.mockAppVersionGetter, nil
				},
				newSvcUpdater:     func(f func(*session.Session) serviceUpdater) {},
				endpointGetter:    m.mockEndpointGetter,
				identity:          m.mockIdentity,
				targetApp:         tc.inApp,
				targetEnvironment: tc.inEnvironment,
				unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.RequestDrivenWebService{
						Workload: manifest.Workload{
							Name: aws.String(mockSvcName),
						},
						RequestDrivenWebServiceConfig: manifest.RequestDrivenWebServiceConfig{
							ImageConfig: manifest.ImageWithPort{
								Image: manifest.Image{
									Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
								},
								Port: aws.Uint16(80),
							},
							RequestDrivenWebServiceHttpConfig: manifest.RequestDrivenWebServiceHttpConfig{
								Alias: aws.String(tc.inAlias),
							},
						},
					}, nil
				},
				uploadOpts: &uploadCustomResourcesOpts{
					uploader: m.mockUploader,
					newS3Uploader: func() (Uploader, error) {
						return nil, nil
					},
				},
			}

			_, gotErr := opts.stackConfiguration(mockAddonsURL)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestSvcDeployOpts_stackConfiguration_worker(t *testing.T) {
	mockError := errors.New("some error")
	topic, _ := deploy.NewTopic("arn:aws:sns:us-west-2:0123456789012:mockApp-mockEnv-topicSvc-givesdogs", "mockApp", "mockEnv", "topicSvc")
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockSvcName   = "mockSvc"
		mockAddonsURL = "mockAddonsURL"
	)
	tests := map[string]struct {
		inAlias        string
		inApp          *config.Application
		inEnvironment  *config.Environment
		inBuildRequire bool

		mockWorkspace          func(m *mocks.MockwsSvcDirReader)
		mockAppResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockAppVersionGetter   func(m *mocks.MockversionGetter)
		mockEndpointGetter     func(m *mocks.MockendpointGetter)
		mockDeployStore        func(m *mocks.MockdeployedEnvironmentLister)

		wantErr error
	}{
		"fail to read service manifest": {
			mockWorkspace: func(m *mocks.MockwsSvcDirReader) {
				m.EXPECT().ReadServiceManifest(mockSvcName).Return(nil, mockError)
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			mockAppVersionGetter:   func(m *mocks.MockversionGetter) {},
			mockEndpointGetter:     func(m *mocks.MockendpointGetter) {},
			mockDeployStore:        func(m *mocks.MockdeployedEnvironmentLister) {},
			wantErr:                fmt.Errorf("read service %s manifest file: %w", mockSvcName, mockError),
		},
		"fail to get deployed topics": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mockWorkspace: func(m *mocks.MockwsSvcDirReader) {
				m.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			mockAppVersionGetter:   func(m *mocks.MockversionGetter) {},
			mockEndpointGetter: func(m *mocks.MockendpointGetter) {
				m.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			mockDeployStore: func(m *mocks.MockdeployedEnvironmentLister) {
				m.EXPECT().ListSNSTopics(mockAppName, mockEnvName).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get SNS topics for app mockApp and environment mockEnv: %w", mockError),
		},
		"success": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mockWorkspace: func(m *mocks.MockwsSvcDirReader) {
				m.EXPECT().ReadServiceManifest(mockSvcName).Return([]byte{}, nil)
			},
			mockAppResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			mockAppVersionGetter:   func(m *mocks.MockversionGetter) {},
			mockEndpointGetter: func(m *mocks.MockendpointGetter) {
				m.EXPECT().ServiceDiscoveryEndpoint().Return("mockEnv.mockApp.local", nil)
			},
			mockDeployStore: func(m *mocks.MockdeployedEnvironmentLister) {
				m.EXPECT().ListSNSTopics(mockAppName, mockEnvName).Return([]deploy.Topic{
					*topic,
				}, nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockwsSvcDirReader(ctrl)
			mockAppResourcesGetter := mocks.NewMockappResourcesGetter(ctrl)
			mockAppVersionGetter := mocks.NewMockversionGetter(ctrl)
			mockEndpointGetter := mocks.NewMockendpointGetter(ctrl)
			mockDeployStore := mocks.NewMockdeployedEnvironmentLister(ctrl)
			tc.mockWorkspace(mockWorkspace)
			tc.mockAppResourcesGetter(mockAppResourcesGetter)
			tc.mockAppVersionGetter(mockAppVersionGetter)
			tc.mockEndpointGetter(mockEndpointGetter)
			tc.mockDeployStore(mockDeployStore)

			opts := deploySvcOpts{
				deployWkldVars: deployWkldVars{
					name:    mockSvcName,
					appName: mockAppName,
					envName: mockEnvName,
				},
				ws:            mockWorkspace,
				buildRequired: tc.inBuildRequire,
				appCFN:        mockAppResourcesGetter,
				newAppVersionGetter: func(s string) (versionGetter, error) {
					return mockAppVersionGetter, nil
				},
				newSvcUpdater:     func(f func(*session.Session) serviceUpdater) {},
				endpointGetter:    mockEndpointGetter,
				snsTopicGetter:    mockDeployStore,
				targetApp:         tc.inApp,
				targetEnvironment: tc.inEnvironment,
				unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.WorkerService{
						Workload: manifest.Workload{
							Name: aws.String(mockSvcName),
						},
						WorkerServiceConfig: manifest.WorkerServiceConfig{
							ImageConfig: manifest.ImageWithHealthcheck{
								Image: manifest.Image{
									Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
								},
							},
						},
					}, nil
				},
			}

			_, gotErr := opts.stackConfiguration(mockAddonsURL)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
