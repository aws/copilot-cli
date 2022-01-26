// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
)

type deployMocks struct {
	mockImageBuilderPusher      *mocks.MockImageBuilderPusher
	mockCustomResourcesUploader *mocks.MockCustomResourcesUploader
	mockEndpointGetter          *mocks.MockEndpointGetter
	mockProgress                *mocks.MockProgress
	mockPublicCIDRBlocksGetter  *mocks.MockPublicCIDRBlocksGetter
	mockSNSTopicsLister         *mocks.MockSNSTopicsLister
	mockServiceDeployer         *mocks.MockServiceDeployer
	mockServiceForceUpdater     *mocks.MockServiceForceUpdater
	mockTemplater               *mocks.MockTemplater
	mockUploader                *mocks.MockUploader
	mockVersionGetter           *mocks.MockVersionGetter
	mockWsReader                *mocks.MockWorkspaceReader
	mockInterpolator            *mocks.MockInterpolator
}

type mockWorkloadMft struct {
	fileName      string
	buildRequired bool
}

func (m *mockWorkloadMft) EnvFile() string {
	return m.fileName
}

func (m *mockWorkloadMft) BuildRequired() (bool, error) {
	return m.buildRequired, nil
}

func (m *mockWorkloadMft) BuildArgs(rootDirectory string) *manifest.DockerBuildArgs {
	return &manifest.DockerBuildArgs{
		Dockerfile: aws.String("mockDockerfile"),
		Context:    aws.String("mockContext"),
	}
}

func (m *mockWorkloadMft) ApplyEnv(envName string) (manifest.WorkloadManifest, error) {
	return m, nil
}

func (m *mockWorkloadMft) Validate() error {
	return nil
}

func (m *mockWorkloadMft) ContainerPlatform() string {
	return "mockContainerPlatform"
}

func TestWorkloadDeployer_UploadArtifacts(t *testing.T) {
	const (
		mockName            = "mockWkld"
		mockEnvName         = "test"
		mockAppName         = "press"
		mockWorkspacePath   = "."
		mockEnvFile         = "foo.env"
		mockS3Bucket        = "mockBucket"
		mockImageTag        = "mockImageTag"
		mockAddonsS3URL     = "https://mockS3DomainName/mockPath"
		mockBadEnvFileS3URL = "badURL"
		mockEnvFileS3URL    = "https://stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf.s3.us-west-2.amazonaws.com/manual/1638391936/env"
		mockEnvFileS3ARN    = "arn:aws:s3:::stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/env"
	)
	mockEnvFilePath := fmt.Sprintf("%s/%s/%s", "manual", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", mockEnvFile)
	mockError := errors.New("some error")
	tests := map[string]struct {
		inShouldUpload  bool
		inEnvFile       string
		inBuildRequired bool
		inRegion        string

		mock func(m *deployMocks)

		wantAddonsURL     string
		wantEnvFileARN    string
		wantImageDigest   string
		wantBuildRequired bool
		wantErr           error
	}{
		"skip if do not upload artifacts": {
			inShouldUpload: false,
			mock:           func(m *deployMocks) {},
		},
		"error out if fail to read workload manifest": {
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("read manifest file for mockWkld: some error"),
		},
		"skip if build is not required and env file is not set": {
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockTemplater.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{
					WlName: "mockWkld",
				})
			},
		},
		"error out if fail to interpolate workload manifest": {
			inShouldUpload:  true,
			inBuildRequired: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", mockError)
			},
			wantErr: fmt.Errorf("interpolate environment variables for mockWkld manifest: some error"),
		},
		"error out if fail to get the workspace path": {
			inShouldUpload:  true,
			inBuildRequired: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return("", mockError)
			},
			wantErr: fmt.Errorf("get workspace path: some error"),
		},
		"error if failed to build and push image": {
			inShouldUpload:  true,
			inBuildRequired: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockImageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					Dockerfile: "mockDockerfile",
					Context:    "mockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{mockImageTag},
				}).Return("", mockError)
			},
			wantErr: fmt.Errorf("build and push image: some error"),
		},
		"build and push image successfully": {
			inShouldUpload:  true,
			inBuildRequired: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockImageBuilderPusher.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					Dockerfile: "mockDockerfile",
					Context:    "mockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{mockImageTag},
				}).Return("mockDigest", nil)
				m.mockTemplater.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{
					WlName: "mockWkld",
				})
			},
			wantImageDigest: "mockDigest",
		},
		"error if fail to put env file to s3 bucket": {
			inShouldUpload: true,
			inEnvFile:      mockEnvFile,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath, gomock.Any()).
					Return("", mockError)
			},
			wantErr: fmt.Errorf("put env file foo.env artifact to bucket mockBucket: some error"),
		},
		"error if fail to parse s3 url": {
			inEnvFile:      mockEnvFile,
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath, gomock.Any()).
					Return(mockBadEnvFileS3URL, nil)

			},
			wantErr: fmt.Errorf("parse s3 url: cannot parse S3 URL badURL into bucket name and key"),
		},
		"error if fail to find the partition": {
			inShouldUpload: true,
			inEnvFile:      mockEnvFile,
			inRegion:       "sun-south-0",
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath, gomock.Any()).
					Return(mockEnvFileS3URL, nil)
			},
			wantErr: fmt.Errorf("find the partition for region sun-south-0"),
		},
		"should push addons template to S3 bucket": {
			inEnvFile:      mockEnvFile,
			inShouldUpload: true,
			inRegion:       "us-west-2",
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockWsReader.EXPECT().Path().Return(mockWorkspacePath, nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath, gomock.Any()).
					Return(mockEnvFileS3URL, nil)
				m.mockTemplater.EXPECT().Template().Return("some data", nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, "mockWkld.addons.stack.yml", gomock.Any()).
					Return(mockAddonsS3URL, nil)
			},

			wantAddonsURL:  mockAddonsS3URL,
			wantEnvFileARN: mockEnvFileS3ARN,
		},
		"should return error if fail to upload to S3 bucket": {
			inRegion:       "us-west-2",
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockTemplater.EXPECT().Template().Return("some data", nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, "mockWkld.addons.stack.yml", gomock.Any()).
					Return("", mockError)
			},

			wantErr: fmt.Errorf("put addons artifact to bucket mockBucket: some error"),
		},
		"should return empty url if the service doesn't have any addons and env files": {
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockTemplater.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{
					WlName: "mockWkld",
				})
			},
		},
		"should fail if addons cannot be retrieved from workspace": {
			inShouldUpload: true,
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte(""), nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockTemplater.EXPECT().Template().Return("", mockError)
			},
			wantErr: fmt.Errorf("retrieve addons template: %w", mockError),
		},
	}

	for name, tc := range tests {
		fs := afero.NewMemMapFs()
		afero.WriteFile(fs, filepath.Join(mockWorkspacePath, mockEnvFile), []byte{}, 0644)
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockUploader:           mocks.NewMockUploader(ctrl),
				mockTemplater:          mocks.NewMockTemplater(ctrl),
				mockImageBuilderPusher: mocks.NewMockImageBuilderPusher(ctrl),
				mockWsReader:           mocks.NewMockWorkspaceReader(ctrl),
				mockInterpolator:       mocks.NewMockInterpolator(ctrl),
			}
			tc.mock(m)

			deployer := WorkloadDeployer{
				Name: mockName,
				Env: &config.Environment{
					Name:   mockEnvName,
					Region: tc.inRegion,
				},
				App: &config.Application{
					Name: mockAppName,
				},
				S3Bucket: mockS3Bucket,
				ImageTag: mockImageTag,
			}
			in := UploadArtifactsInput{
				Templater:             m.mockTemplater,
				ShouldUploadArtifacts: tc.inShouldUpload,
				FS:                    &afero.Afero{Fs: fs},
				Uploader:              m.mockUploader,
				ImageBuilderPusher:    m.mockImageBuilderPusher,
				WS:                    m.mockWsReader,
				NewInterpolator: func(app, env string) Interpolator {
					return m.mockInterpolator
				},
				Unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &mockWorkloadMft{
						fileName:      tc.inEnvFile,
						buildRequired: tc.inBuildRequired,
					}, nil
				},
			}

			got, gotErr := deployer.UploadArtifacts(&in)

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantAddonsURL, got.AddonsURL)
				require.Equal(t, tc.wantEnvFileARN, got.EnvFileARN)
				require.Equal(t, tc.wantImageDigest, got.ImageDigest)
			}
		})
	}
}

func TestWorkloadDeployer_DeployWorkload(t *testing.T) {
	mockError := errors.New("some error")
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockName      = "mockWkld"
		mockAddonsURL = "mockAddonsURL"
		mockS3Bucket  = "mockBucket"
	)
	mockNowTime := time.Unix(1494505750, 0)
	mockBeforeTime := time.Unix(1494505743, 0)
	mockAfterTime := time.Unix(1494505756, 0)
	tests := map[string]struct {
		inAliases     manifest.Alias
		inNLB         manifest.NetworkLoadBalancerConfiguration
		inApp         *config.Application
		inEnvironment *config.Environment
		inForceDeploy bool

		mock func(m *deployMocks)

		wantErr error
	}{
		"fail to get service discovery endpoint": {
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("", mockError)
			},
			wantErr: fmt.Errorf("get service discovery endpoint: some error"),
		},
		"fail to get public CIDR blocks": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Port: aws.String("443/tcp"),
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockPublicCIDRBlocksGetter.EXPECT().PublicCIDRBlocks().Return(nil, errors.New("some error"))
			},
			wantErr: fmt.Errorf("get public CIDR blocks information from the VPC of environment mockEnv: some error"),
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: errors.New("alias specified when application is not associated with a domain"),
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("", mockError)
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v0.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf("alias is not compatible with application versions below %s", deploy.AliasLeastAppTemplateVersion),
		},
		"fail to enable nlb alias because of incompatible app version": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Port:    aws.String("80"),
				Aliases: manifest.Alias{String: aws.String("mockAlias")},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v0.0.0", nil)
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},
			wantErr: fmt.Errorf(`alias "v1.v2.mockDomain" is not supported in hosted zones managed by Copilot`),
		},
		"fail to enable nlb alias because of invalid alias": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Port:    aws.String("80"),
				Aliases: manifest.Alias{String: aws.String("v1.v2.mockDomain")},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
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
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).Return(errors.New("some error"))
			},
			wantErr: fmt.Errorf("deploy service: some error"),
		},
		"error if change set is empty but force flag is not set": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).Return(cloudformation.NewMockErrChangeSetEmpty())
			},
			wantErr: fmt.Errorf("deploy service: change set with name mockChangeSet for stack mockStack has no changes"),
		},
		"error if fail to get last update time when force an update": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).
					Return(nil)
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(time.Time{}, mockError)
			},
			wantErr: fmt.Errorf("get the last updated deployment time for mockWkld: some error"),
		},
		"skip force updating when cmd run time is after the last update time": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).
					Return(nil)
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockAfterTime, nil)
			},
		},
		"error if fail to force an update": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockProgress.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).Return(mockError)
				m.mockProgress.EXPECT().Stop(log.Serrorf(fmtForceUpdateSvcFailed, mockName, mockEnvName, mockError))
			},
			wantErr: fmt.Errorf("force an update for service mockWkld: some error"),
		},
		"error if fail to force an update because of timeout": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockProgress.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).
					Return(&ecs.ErrWaitServiceStableTimeout{})
				m.mockProgress.EXPECT().Stop(
					log.Serror(fmt.Sprintf("%s  Run %s to check for the fail reason.\n",
						fmt.Sprintf(fmtForceUpdateSvcFailed, mockName, mockEnvName, &ecs.ErrWaitServiceStableTimeout{}),
						color.HighlightCode(fmt.Sprintf("copilot svc status --name %s --env %s", mockName, mockEnvName)))))
			},
			wantErr: fmt.Errorf("force an update for service mockWkld: max retries 0 exceeded"),
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).Return(nil)
			},
		},
		"success with force update": {
			inForceDeploy: true,
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), gomock.Any(), "mockBucket", gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockProgress.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).Return(nil)
				m.mockProgress.EXPECT().Stop(log.Ssuccessf(fmtForceUpdateSvcComplete, mockName, mockEnvName))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockWsReader:                mocks.NewMockWorkspaceReader(ctrl),
				mockInterpolator:            mocks.NewMockInterpolator(ctrl),
				mockVersionGetter:           mocks.NewMockVersionGetter(ctrl),
				mockEndpointGetter:          mocks.NewMockEndpointGetter(ctrl),
				mockServiceDeployer:         mocks.NewMockServiceDeployer(ctrl),
				mockServiceForceUpdater:     mocks.NewMockServiceForceUpdater(ctrl),
				mockProgress:                mocks.NewMockProgress(ctrl),
				mockPublicCIDRBlocksGetter:  mocks.NewMockPublicCIDRBlocksGetter(ctrl),
				mockCustomResourcesUploader: mocks.NewMockCustomResourcesUploader(ctrl),
			}
			tc.mock(m)

			deployer := WorkloadDeployer{
				Name:     mockName,
				App:      tc.inApp,
				Env:      tc.inEnvironment,
				S3Bucket: mockS3Bucket,
			}

			_, gotErr := deployer.DeployWorkload(&DeployWorkloadInput{
				ForceNewUpdate: tc.inForceDeploy,
				WS:             m.mockWsReader,
				NewInterpolator: func(app, env string) Interpolator {
					return m.mockInterpolator
				},
				Unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.LoadBalancedWebService{
						Workload: manifest.Workload{
							Name: aws.String(mockName),
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
							RoutingRule: manifest.RoutingRuleConfigOrBool{
								RoutingRuleConfiguration: manifest.RoutingRuleConfiguration{
									Path:  aws.String("/"),
									Alias: tc.inAliases,
								},
							},
							NLBConfig: tc.inNLB,
						},
					}, nil
				},
				ServiceDeployer: m.mockServiceDeployer,
				NewSvcUpdater:   func(f func(*session.Session) ServiceForceUpdater) {},
				NewAppVersionGetter: func(s string) (VersionGetter, error) {
					return m.mockVersionGetter, nil
				},
				PublicCIDRBlocksGetter: m.mockPublicCIDRBlocksGetter,
				ServiceForceUpdater:    m.mockServiceForceUpdater,
				EndpointGetter:         m.mockEndpointGetter,
				Spinner:                m.mockProgress,
				UploadOpts: &UploadCustomResourcesOpts{
					uploader: m.mockCustomResourcesUploader,
					newS3Uploader: func() (Uploader, error) {
						return nil, nil
					},
				},
				now: func() time.Time {
					return mockNowTime
				},
			})

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

type deployRDSvcMocks struct {
	mockWorkspace      *mocks.MockWorkspaceReader
	mockVersionGetter  *mocks.MockVersionGetter
	mockEndpointGetter *mocks.MockEndpointGetter
	mockUploader       *mocks.MockCustomResourcesUploader
	mockInterpolator   *mocks.MockInterpolator
}

func TestSvcDeployOpts_rdWebServiceStackConfiguration(t *testing.T) {
	const (
		mockAppName   = "mockApp"
		mockEnvName   = "mockEnv"
		mockName      = "mockWkld"
		mockAddonsURL = "mockAddonsURL"
		mockBucket    = "mockBucket"
	)
	tests := map[string]struct {
		inAlias       string
		inApp         *config.Application
		inEnvironment *config.Environment

		mock func(m *deployRDSvcMocks)

		wantAlias string
		wantErr   error
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
			},

			wantErr: errors.New("alias specified when application is not associated with a domain"),
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
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
				m.mockWorkspace.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockUploader.EXPECT().UploadRequestDrivenWebServiceCustomResources(gomock.Any()).Return(map[string]string{
					"mockResource2": "mockURL2",
				}, nil)
			},
			wantAlias: "v1.mockDomain",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployRDSvcMocks{
				mockWorkspace:      mocks.NewMockWorkspaceReader(ctrl),
				mockVersionGetter:  mocks.NewMockVersionGetter(ctrl),
				mockEndpointGetter: mocks.NewMockEndpointGetter(ctrl),
				mockUploader:       mocks.NewMockCustomResourcesUploader(ctrl),
				mockInterpolator:   mocks.NewMockInterpolator(ctrl),
			}
			tc.mock(m)

			deployer := WorkloadDeployer{
				Name:     mockName,
				App:      tc.inApp,
				Env:      tc.inEnvironment,
				S3Bucket: mockBucket,
			}

			got, gotErr := deployer.stackConfiguration(&DeployWorkloadInput{
				WS: m.mockWorkspace,
				NewInterpolator: func(app, env string) Interpolator {
					return m.mockInterpolator
				},
				Unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.RequestDrivenWebService{
						Workload: manifest.Workload{
							Name: aws.String(mockName),
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
				NewSvcUpdater: func(f func(*session.Session) ServiceForceUpdater) {},
				NewAppVersionGetter: func(s string) (VersionGetter, error) {
					return m.mockVersionGetter, nil
				},
				EndpointGetter: m.mockEndpointGetter,
				UploadOpts: &UploadCustomResourcesOpts{
					uploader: m.mockUploader,
					newS3Uploader: func() (Uploader, error) {
						return nil, nil
					},
				},
				AddonsURL: mockAddonsURL,
			})

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantAlias, got.rdSvcAlias)
			}
		})
	}
}

func TestSvcDeployOpts_stackConfiguration_worker(t *testing.T) {
	mockError := errors.New("some error")
	topic, _ := deploy.NewTopic("arn:aws:sns:us-west-2:0123456789012:mockApp-mockEnv-mockwkld-givesdogs", "mockApp", "mockEnv", "mockwkld")
	const (
		mockAppName = "mockApp"
		mockEnvName = "mockEnv"
		mockName    = "mockwkld"
		mockBucket  = "mockBucket"
	)
	mockTopics := []manifest.TopicSubscription{
		{
			Name:    aws.String("givesdogs"),
			Service: aws.String("mockwkld"),
		},
	}
	tests := map[string]struct {
		inAlias        string
		inApp          *config.Application
		inEnvironment  *config.Environment
		inBuildRequire bool

		mock func(m *deployMocks)

		wantErr             error
		wantedSubscriptions []manifest.TopicSubscription
	}{
		"fail to get deployed topics": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockSNSTopicsLister.EXPECT().ListSNSTopics(mockAppName, mockEnvName).Return(nil, mockError)
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
			mock: func(m *deployMocks) {
				m.mockWsReader.EXPECT().ReadWorkloadManifest(mockName).Return([]byte{}, nil)
				m.mockInterpolator.EXPECT().Interpolate("").Return("", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockEnv.mockApp.local", nil)
				m.mockSNSTopicsLister.EXPECT().ListSNSTopics(mockAppName, mockEnvName).Return([]deploy.Topic{
					*topic,
				}, nil)
			},
			wantedSubscriptions: mockTopics,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &deployMocks{
				mockWsReader:        mocks.NewMockWorkspaceReader(ctrl),
				mockEndpointGetter:  mocks.NewMockEndpointGetter(ctrl),
				mockSNSTopicsLister: mocks.NewMockSNSTopicsLister(ctrl),
				mockInterpolator:    mocks.NewMockInterpolator(ctrl),
			}
			tc.mock(m)

			deployer := WorkloadDeployer{
				Name:     mockName,
				App:      tc.inApp,
				Env:      tc.inEnvironment,
				S3Bucket: mockBucket,
			}

			got, gotErr := deployer.stackConfiguration(&DeployWorkloadInput{
				WS: m.mockWsReader,
				NewInterpolator: func(app, env string) Interpolator {
					return m.mockInterpolator
				},
				Unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &manifest.WorkerService{
						Workload: manifest.Workload{
							Name: aws.String(mockName),
						},
						WorkerServiceConfig: manifest.WorkerServiceConfig{
							ImageConfig: manifest.ImageWithHealthcheck{
								Image: manifest.Image{
									Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
								},
							},
							Subscribe: manifest.SubscribeConfig{
								Topics: mockTopics,
							},
						},
					}, nil
				},
				SNSTopicsLister: m.mockSNSTopicsLister,
				NewSvcUpdater:   func(f func(*session.Session) ServiceForceUpdater) {},
				NewAppVersionGetter: func(s string) (VersionGetter, error) {
					return m.mockVersionGetter, nil
				},
				EndpointGetter: m.mockEndpointGetter,
			})

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.ElementsMatch(t, tc.wantedSubscriptions, got.subscriptions)
			}
		})
	}
}
