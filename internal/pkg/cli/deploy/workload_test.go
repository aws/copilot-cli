// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cloudformation0 "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/afero"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkcfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/override"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/syncbuffer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type endpointGetterDouble struct {
	ServiceDiscoveryEndpointFn func() (string, error)
}

func (d *endpointGetterDouble) ServiceDiscoveryEndpoint() (string, error) {
	return d.ServiceDiscoveryEndpointFn()
}

type deployMocks struct {
	mockRepositoryService      *mocks.MockrepositoryService
	mockEndpointGetter         *mocks.MockendpointGetter
	mockSpinner                *mocks.Mockspinner
	mockSNSTopicsLister        *mocks.MocksnsTopicsLister
	mockServiceDeployer        *mocks.MockserviceDeployer
	mockServiceForceUpdater    *mocks.MockserviceForceUpdater
	mockAddons                 *mocks.MockstackBuilder
	mockUploader               *mocks.Mockuploader
	mockAppVersionGetter       *mocks.MockversionGetter
	mockEnvVersionGetter       *mocks.MockversionGetter
	mockFileSystem             afero.Fs
	mockValidator              *mocks.MockaliasCertValidator
	mockLabeledTermPrinter     *mocks.MockLabeledTermPrinter
	mockdockerEngineRunChecker *mocks.MockdockerEngineRunChecker
}

type mockTemplateFS struct {
	read func(path string) (*template.Content, error)
}

// Read implements the template.Reader interface.
func (fs *mockTemplateFS) Read(path string) (*template.Content, error) {
	return fs.read(path)
}

func fakeTemplateFS() *mockTemplateFS {
	return &mockTemplateFS{
		read: func(path string) (*template.Content, error) {
			return &template.Content{
				Buffer: bytes.NewBufferString("fake content"),
			}, nil
		},
	}
}

type mockEndpointGetter struct {
	endpoint string
	err      error
}

// ServiceDiscoveryEndpoint implements the endpointGetter interface.
func (m *mockEndpointGetter) ServiceDiscoveryEndpoint() (string, error) {
	return m.endpoint, m.err
}

type mockEnvVersionGetter struct {
	version string
	err     error
}

// Version implements the envVersionGetter interface.
func (m *mockEnvVersionGetter) Version() (string, error) {
	return m.version, m.err
}

type mockTopicLister struct {
	topics []deploy.Topic
	err    error
}

// ListSNSTopics implements the snsTopicsLister interface.
func (m *mockTopicLister) ListSNSTopics(_, _ string) ([]deploy.Topic, error) {
	return m.topics, m.err
}

type mockWorkloadMft struct {
	fileName        string
	dockerBuildArgs map[string]*manifest.DockerBuildArgs
	workloadName    string
	customEnvFiles  map[string]string
}

func (m *mockWorkloadMft) EnvFiles() map[string]string {
	if m.customEnvFiles != nil {
		return m.customEnvFiles
	}
	return map[string]string{
		m.workloadName: m.fileName,
	}
}

func (m *mockWorkloadMft) BuildArgs(rootDirectory string) (map[string]*manifest.DockerBuildArgs, error) {
	return m.dockerBuildArgs, nil
}

func (m *mockWorkloadMft) ContainerPlatform() string {
	return "mockContainerPlatform"
}

// stubCloudFormationStack implements the cloudformation.StackConfiguration interface.
type stubCloudFormationStack struct{}

func (s *stubCloudFormationStack) StackName() string {
	return "demo"
}
func (s *stubCloudFormationStack) Template() (string, error) {
	return `
Resources:
  Queue:
    Type: AWS::SQS::Queue`, nil
}
func (s *stubCloudFormationStack) Parameters() ([]*sdkcfn.Parameter, error) {
	return []*sdkcfn.Parameter{}, nil
}
func (s *stubCloudFormationStack) Tags() []*sdkcfn.Tag {
	return []*sdkcfn.Tag{}
}
func (s *stubCloudFormationStack) SerializedParameters() (string, error) {
	return "", nil
}

func mockEnvFilePath(path string) string {
	return fmt.Sprintf("%s/%s/%s/%s.env", "manual", "env-files", path, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
}

func TestWorkloadDeployer_UploadArtifacts(t *testing.T) {
	const (
		mockName            = "mockWkld"
		mockEnvName         = "test"
		mockAppName         = "press"
		mockURI             = "mockRepoURI"
		mockWorkspacePath   = "."
		mockEnvFile         = "foo.env"
		mockS3Bucket        = "mockBucket"
		mockAddonsS3URL     = "https://mockS3DomainName/mockPath"
		mockBadEnvFileS3URL = "badURL"
		mockEnvFileS3URL    = "https://stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf.s3.us-west-2.amazonaws.com/manual/1638391936/env"
		mockEnvFileS3URL2   = "https://stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf.s3.us-west-2.amazonaws.com/manual/1638391936/envbar"
		mockEnvFileS3ARN    = "arn:aws:s3:::stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/env"
		mockEnvFileS3ARN2   = "arn:aws:s3:::stackset-demo-infrastruc-pipelinebuiltartifactbuc-11dj7ctf52wyf/manual/1638391936/envbar"
	)
	var mockEnvFilesOutput = map[string]string{"mockWkld": mockEnvFileS3ARN}
	mockResources := &stack.AppRegionalResources{
		S3Bucket: mockS3Bucket,
	}
	mockAddonPath := fmt.Sprintf("%s/%s/%s/%s.yml", "manual", "addons", mockName, "1307990e6ba5ca145eb35e99182a9bec46531bc54ddf656a602c780fa0240dee")
	mockError := errors.New("some error")
	type artifactsUploader interface {
		UploadArtifacts() (*UploadArtifactsOutput, error)
	}
	tests := map[string]struct {
		inEnvFile         string
		customEnvFiles    map[string]string
		inRegion          string
		inMockUserTag     string
		inMockGitTag      string
		inDockerBuildArgs map[string]*manifest.DockerBuildArgs

		mock                func(t *testing.T, m *deployMocks)
		mockServiceDeployer func(deployer *workloadDeployer) artifactsUploader
		customResourcesFunc customResourcesFunc

		wantAddonsURL     string
		wantEnvFileARNs   map[string]string
		wantImages        map[string]ContainerImageIdentifier
		wantBuildRequired bool
		wantErr           error
	}{
		"error if docker engine is not running": {
			inMockUserTag: "v1.0",
			inDockerBuildArgs: map[string]*manifest.DockerBuildArgs{
				"mockWkld": {
					Dockerfile: aws.String("mockDockerfile"),
					Context:    aws.String("mockContext"),
				},
			},
			mock: func(t *testing.T, m *deployMocks) {
				m.mockdockerEngineRunChecker.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},
			wantErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"error if failed to build and push image": {
			inMockUserTag: "v1.0",
			inDockerBuildArgs: map[string]*manifest.DockerBuildArgs{
				"mockWkld": {
					Dockerfile: aws.String("mockDockerfile"),
					Context:    aws.String("mockContext"),
				},
			},
			mock: func(t *testing.T, m *deployMocks) {
				m.mockdockerEngineRunChecker.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.mockRepositoryService.EXPECT().Login().Return(mockURI, nil)
				m.mockRepositoryService.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					URI:        mockURI,
					Dockerfile: "mockDockerfile",
					Context:    "mockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{"latest", "v1.0"},
					Labels: map[string]string{
						"com.aws.copilot.image.builder":        "copilot-cli",
						"com.aws.copilot.image.container.name": "mockWkld",
					},
				}, gomock.Any()).Return("", mockError)
			},
			wantErr: fmt.Errorf("build and push the image \"mockWkld\": some error"),
		},
		"build and push image with usertag successfully": {
			inMockUserTag: "v1.0",
			inMockGitTag:  "gitTag",
			inDockerBuildArgs: map[string]*manifest.DockerBuildArgs{
				"mockWkld": {
					Dockerfile: aws.String("mockDockerfile"),
					Context:    aws.String("mockContext"),
				},
			},
			mock: func(t *testing.T, m *deployMocks) {
				m.mockdockerEngineRunChecker.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.mockRepositoryService.EXPECT().Login().Return(mockURI, nil)
				m.mockRepositoryService.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					URI:        mockURI,
					Dockerfile: "mockDockerfile",
					Context:    "mockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{"latest", "v1.0"},
					Labels: map[string]string{
						"com.aws.copilot.image.builder":        "copilot-cli",
						"com.aws.copilot.image.container.name": "mockWkld",
					},
				}, gomock.Any()).Return("mockDigest", nil)
				m.mockAddons = nil
			},
			wantImages: map[string]ContainerImageIdentifier{
				mockName: {
					Digest:            "mockDigest",
					CustomTag:         "v1.0",
					GitShortCommitTag: "gitTag",
					RepoTags: []string{
						"mockRepoURI:latest",
						"mockRepoURI:v1.0",
					},
				},
			},
		},
		"build and push image with gitshortcommit successfully": {
			inMockGitTag: "gitTag",
			inDockerBuildArgs: map[string]*manifest.DockerBuildArgs{
				"mockWkld": {
					Dockerfile: aws.String("mockDockerfile"),
					Context:    aws.String("mockContext"),
				},
			},
			mock: func(t *testing.T, m *deployMocks) {
				m.mockdockerEngineRunChecker.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.mockRepositoryService.EXPECT().Login().Return(mockURI, nil)
				m.mockRepositoryService.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					URI:        mockURI,
					Dockerfile: "mockDockerfile",
					Context:    "mockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{"latest", "gitTag"},
					Labels: map[string]string{
						"com.aws.copilot.image.builder":        "copilot-cli",
						"com.aws.copilot.image.container.name": "mockWkld",
					},
				}, gomock.Any()).Return("mockDigest", nil)
				m.mockAddons = nil
			},
			wantImages: map[string]ContainerImageIdentifier{
				mockName: {
					Digest:            "mockDigest",
					GitShortCommitTag: "gitTag",
					RepoTags: []string{
						"mockRepoURI:gitTag",
						"mockRepoURI:latest",
					},
				},
			},
		},
		"build and push sidecar container images only with git tag Successfully": {
			inDockerBuildArgs: map[string]*manifest.DockerBuildArgs{
				"nginx": {
					Dockerfile: aws.String("sidecarMockDockerfile"),
					Context:    aws.String("sidecarMockContext"),
				},
				"logging": {
					Dockerfile: aws.String("web/Dockerfile"),
					Context:    aws.String("Users/bowie"),
				},
			},
			inMockGitTag: "gitTag",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockdockerEngineRunChecker.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.mockRepositoryService.EXPECT().Login().Return(mockURI, nil)
				m.mockRepositoryService.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					URI:        mockURI,
					Dockerfile: "sidecarMockDockerfile",
					Context:    "sidecarMockContext",
					Platform:   "mockContainerPlatform",
					Tags:       []string{fmt.Sprintf("nginx-%s", "latest"), fmt.Sprintf("nginx-%s", "gitTag")},
					Labels: map[string]string{
						"com.aws.copilot.image.builder":        "copilot-cli",
						"com.aws.copilot.image.container.name": "nginx",
					},
				}, gomock.Any()).Return("sidecarMockDigest1", nil)
				m.mockRepositoryService.EXPECT().BuildAndPush(gomock.Any(), &dockerengine.BuildArguments{
					URI:        mockURI,
					Dockerfile: "web/Dockerfile",
					Context:    "Users/bowie",
					Platform:   "mockContainerPlatform",
					Tags:       []string{"logging-latest", fmt.Sprintf("logging-%s", "gitTag")},
					Labels: map[string]string{
						"com.aws.copilot.image.builder":        "copilot-cli",
						"com.aws.copilot.image.container.name": "logging",
					},
				}, gomock.Any()).Return("sidecarMockDigest2", nil)
				m.mockLabeledTermPrinter.EXPECT().IsDone().Return(true).AnyTimes()
				m.mockLabeledTermPrinter.EXPECT().Print().AnyTimes()
				m.mockAddons = nil
			},
			wantImages: map[string]ContainerImageIdentifier{
				"nginx": {
					Digest:            "sidecarMockDigest1",
					GitShortCommitTag: "gitTag",
					RepoTags: []string{
						"mockRepoURI:nginx-gitTag",
						"mockRepoURI:nginx-latest",
					},
				},
				"logging": {
					Digest:            "sidecarMockDigest2",
					GitShortCommitTag: "gitTag",
					RepoTags: []string{
						"mockRepoURI:logging-gitTag",
						"mockRepoURI:logging-latest",
					},
				},
			},
		},
		"should retrieve Load Balanced Web Service custom resource URLs": {
			mock: func(t *testing.T, m *deployMocks) {
				// Ignore addon uploads.
				m.mockAddons = nil

				// Ensure all custom resources were uploaded.
				crs, err := customresource.LBWS(fakeTemplateFS())
				require.NoError(t, err)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			customResourcesFunc: func(fs template.Reader) ([]*customresource.CustomResource, error) {
				return customresource.LBWS(fs)
			},
			mockServiceDeployer: func(deployer *workloadDeployer) artifactsUploader {
				return &lbWebSvcDeployer{
					svcDeployer: &svcDeployer{
						workloadDeployer: deployer,
					},
				}
			},
		},
		"should retrieve Backend Service custom resource URLs": {
			mock: func(t *testing.T, m *deployMocks) {
				// Ignore addon uploads.
				m.mockAddons = nil

				// Ensure all custom resources were uploaded.
				crs, err := customresource.Backend(fakeTemplateFS())
				require.NoError(t, err)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			customResourcesFunc: func(fs template.Reader) ([]*customresource.CustomResource, error) {
				return customresource.Backend(fs)
			},
			mockServiceDeployer: func(deployer *workloadDeployer) artifactsUploader {
				return &backendSvcDeployer{
					svcDeployer: &svcDeployer{
						workloadDeployer: deployer,
					},
				}
			},
		},
		"should retrieve Worker Service custom resource URLs": {
			mock: func(t *testing.T, m *deployMocks) {
				// Ignore addon uploads.
				m.mockAddons = nil

				// Ensure all custom resources were uploaded.
				crs, err := customresource.Worker(fakeTemplateFS())
				require.NoError(t, err)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			customResourcesFunc: func(fs template.Reader) ([]*customresource.CustomResource, error) {
				return customresource.Worker(fs)
			},
			mockServiceDeployer: func(deployer *workloadDeployer) artifactsUploader {
				return &workerSvcDeployer{
					svcDeployer: &svcDeployer{
						workloadDeployer: deployer,
					},
				}
			},
		},
		"should retrieve Request-Driven Web Service custom resource URLs": {
			mock: func(t *testing.T, m *deployMocks) {
				// Ignore addon uploads.
				m.mockAddons = nil

				// Ensure all custom resources were uploaded.
				crs, err := customresource.RDWS(fakeTemplateFS())
				require.NoError(t, err)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			customResourcesFunc: func(fs template.Reader) ([]*customresource.CustomResource, error) {
				return customresource.RDWS(fs)
			},
			mockServiceDeployer: func(deployer *workloadDeployer) artifactsUploader {
				return &rdwsDeployer{
					svcDeployer: &svcDeployer{
						workloadDeployer: deployer,
					},
				}
			},
		},
		"should retrieve Scheduled Job custom resource URLs": {
			mock: func(t *testing.T, m *deployMocks) {
				// Ignore addon uploads.
				m.mockAddons = nil

				// Ensure all custom resources were uploaded.
				crs, err := customresource.ScheduledJob(fakeTemplateFS())
				require.NoError(t, err)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, gomock.Any(), gomock.Any()).DoAndReturn(func(_, key string, _ io.Reader) (url string, err error) {
					for _, cr := range crs {
						if strings.Contains(key, strings.ToLower(cr.Name())) {
							return "", nil
						}
					}
					return "", errors.New("did not match any custom resource")
				}).Times(len(crs))
			},
			customResourcesFunc: func(fs template.Reader) ([]*customresource.CustomResource, error) {
				return customresource.ScheduledJob(fs)
			},
			mockServiceDeployer: func(deployer *workloadDeployer) artifactsUploader {
				return &jobDeployer{
					workloadDeployer: deployer,
				}
			},
		},
		"error if fail to read env file": {
			inEnvFile: mockEnvFile,
			mock:      func(t *testing.T, m *deployMocks) {},
			wantErr:   fmt.Errorf("read env file foo.env: open foo.env: file does not exist"),
		},
		"successfully share one env file between containers": {
			customEnvFiles: map[string]string{"nginx": mockEnvFile, mockName: mockEnvFile},
			inRegion:       "us-west-2",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).Return(mockEnvFileS3URL, nil)
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("", nil)
				m.mockUploader.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockAddonsS3URL, nil)
			},
			wantEnvFileARNs: map[string]string{"nginx": mockEnvFileS3ARN, mockName: mockEnvFileS3ARN},
			wantAddonsURL:   mockAddonsS3URL,
		},
		"upload multiple env files": {
			customEnvFiles: map[string]string{"nginx": mockEnvFile, mockName: "bar.env"},
			inRegion:       "us-west-2",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).Return(mockEnvFileS3URL, nil)
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, "bar.env"))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath("bar.env"), gomock.Any()).Return(mockEnvFileS3URL2, nil)
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("", nil)
				m.mockUploader.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockAddonsS3URL, nil)
			},
			wantEnvFileARNs: map[string]string{"nginx": mockEnvFileS3ARN, mockName: mockEnvFileS3ARN2},
			wantAddonsURL:   mockAddonsS3URL,
		},
		"success with no env files present but logging sidecar exists": {
			inRegion:       "us-west-2",
			customEnvFiles: map[string]string{manifest.FirelensContainerName: "", mockName: ""},
			mock: func(t *testing.T, m *deployMocks) {
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("", nil)
				m.mockUploader.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockAddonsS3URL, nil)
			},
			wantEnvFileARNs: nil,
			wantAddonsURL:   mockAddonsS3URL,
		},
		"error if fail to put env file to s3 bucket": {
			inEnvFile: mockEnvFile,
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).
					Return("", mockError)
			},
			wantErr: fmt.Errorf("put env file foo.env artifact to bucket mockBucket: some error"),
		},
		"error if fail to parse s3 url": {
			inEnvFile: mockEnvFile,
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).
					Return(mockBadEnvFileS3URL, nil)

			},
			wantErr: fmt.Errorf("parse s3 url: cannot parse S3 URL badURL into bucket name and key"),
		},
		"error if fail to find the partition": {
			inEnvFile: mockEnvFile,
			inRegion:  "sun-south-0",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).
					Return(mockEnvFileS3URL, nil)
			},
			wantErr: fmt.Errorf("find the partition for region sun-south-0"),
		},
		"should push addons template to S3 bucket": {
			inEnvFile: mockEnvFile,
			inRegion:  "us-west-2",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockFileSystem.Create(filepath.Join(mockWorkspacePath, mockEnvFile))
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockEnvFilePath(mockEnvFile), gomock.Any()).
					Return(mockEnvFileS3URL, nil)
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("some data", nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockAddonPath, gomock.Any()).
					Return(mockAddonsS3URL, nil)
			},

			wantAddonsURL:   mockAddonsS3URL,
			wantEnvFileARNs: mockEnvFilesOutput,
		},
		"should return error if fail to upload to S3 bucket": {
			inRegion: "us-west-2",
			mock: func(t *testing.T, m *deployMocks) {
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("some data", nil)
				m.mockUploader.EXPECT().Upload(mockS3Bucket, mockAddonPath, gomock.Any()).
					Return("", mockError)
			},

			wantErr: fmt.Errorf("put addons artifact to bucket mockBucket: some error"),
		},
		"should return empty url if the service doesn't have any addons and env files": {
			mock: func(t *testing.T, m *deployMocks) {
				m.mockAddons = nil
			},
		},
		"should fail if packaging addons fails": {
			mock: func(t *testing.T, m *deployMocks) {
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(mockError)
			},
			wantErr: fmt.Errorf("package addons: %w", mockError),
		},
		"should fail if addons template can't be created": {
			mock: func(t *testing.T, m *deployMocks) {
				m.mockAddons.EXPECT().Package(gomock.Any()).Return(nil)
				m.mockAddons.EXPECT().Template().Return("", mockError)
			},
			wantErr: fmt.Errorf("retrieve addons template: %w", mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockUploader:               mocks.NewMockuploader(ctrl),
				mockAddons:                 mocks.NewMockstackBuilder(ctrl),
				mockRepositoryService:      mocks.NewMockrepositoryService(ctrl),
				mockFileSystem:             afero.NewMemMapFs(),
				mockLabeledTermPrinter:     mocks.NewMockLabeledTermPrinter(ctrl),
				mockdockerEngineRunChecker: mocks.NewMockdockerEngineRunChecker(ctrl),
			}
			tc.mock(t, m)

			crFn := tc.customResourcesFunc
			if crFn == nil {
				crFn = func(fs template.Reader) ([]*customresource.CustomResource, error) {
					return nil, nil
				}
			}
			wkldDeployer := &workloadDeployer{
				name: mockName,
				env: &config.Environment{
					Name:   mockEnvName,
					Region: tc.inRegion,
				},
				app: &config.Application{
					Name: mockAppName,
				},
				resources: mockResources,
				image: ContainerImageIdentifier{
					CustomTag:         tc.inMockUserTag,
					GitShortCommitTag: tc.inMockGitTag,
				},
				workspacePath: mockWorkspacePath,
				mft: &mockWorkloadMft{
					workloadName:    mockName,
					fileName:        tc.inEnvFile,
					customEnvFiles:  tc.customEnvFiles,
					dockerBuildArgs: tc.inDockerBuildArgs,
				},
				fs:              m.mockFileSystem,
				s3Client:        m.mockUploader,
				docker:          m.mockdockerEngineRunChecker,
				repository:      m.mockRepositoryService,
				templateFS:      fakeTemplateFS(),
				overrider:       new(override.Noop),
				customResources: crFn,
				labeledTermPrinter: func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) LabeledTermPrinter {
					return m.mockLabeledTermPrinter
				},
			}
			if m.mockAddons != nil {
				wkldDeployer.addons = m.mockAddons
			}
			var deployer artifactsUploader
			deployer = &lbWebSvcDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: wkldDeployer,
				},
			}
			if tc.mockServiceDeployer != nil {
				deployer = tc.mockServiceDeployer(wkldDeployer)
			}

			got, gotErr := deployer.UploadArtifacts()

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantAddonsURL, got.AddonsURL)
				require.Equal(t, tc.wantEnvFileARNs, got.EnvFileARNs)
				require.Equal(t, tc.wantImages, got.ImageDigests)
			}
		})
	}
}

func TestWorkloadDeployer_DeployWorkload(t *testing.T) {
	mockError := errors.New("some error")
	const (
		mockAppName  = "mockApp"
		mockEnvName  = "mockEnv"
		mockName     = "mockWkld"
		mockS3Bucket = "mockBucket"
	)
	mockMultiAliases := []manifest.AdvancedAlias{
		{
			Alias: aws.String("example.com"),
		},
		{
			Alias: aws.String("foobar.com"),
		},
	}
	mockAlias := []manifest.AdvancedAlias{
		{
			Alias: aws.String("mockAlias"),
		},
	}
	mockCertARNs := []string{"mockCertARN"}
	mockCDNCertARN := "mockCDNCertARN"
	mockResources := &stack.AppRegionalResources{
		S3Bucket: mockS3Bucket,
	}
	mockNowTime := time.Unix(1494505750, 0)
	mockBeforeTime := time.Unix(1494505743, 0)
	mockAfterTime := time.Unix(1494505756, 0)
	tests := map[string]struct {
		inAliases         manifest.Alias
		inNLB             manifest.NetworkLoadBalancerConfiguration
		inApp             *config.Application
		inEnvironment     *config.Environment
		inForceDeploy     bool
		inDisableRollback bool
		inRedirectToHTTPS *bool

		// Cached variables.
		inEnvironmentConfig func() *manifest.Environment

		mock func(m *deployMocks)

		wantErr error
	}{
		"fail to get service discovery endpoint": {
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("", mockError)
			},
			wantErr: fmt.Errorf("get service discovery endpoint: some error"),
		},
		"fail to get env version": {
			inEnvironment: &config.Environment{
				Name: mockEnvName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("", errors.New("some error"))
			},
			wantErr: fmt.Errorf(`get version of environment "mockEnv": some error`),
		},
		"fail if alias is not specified with env has imported certs": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				return envConfig
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf(`validate ALB runtime configuration for "http": cannot deploy service mockWkld without "alias" to environment mockEnv with certificate imported`),
		},
		"fail if http redirect to https configured without custom domain": {
			inRedirectToHTTPS: aws.Bool(true),
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				return &manifest.Environment{}
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf(`validate ALB runtime configuration for "http": cannot configure http to https redirect without having a domain associated with the app "mockApp" or importing any certificates in env "mockEnv"`),
		},
		"cannot specify alias hosted zone when no certificates are imported in the env": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-east-1",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				return envConfig
			},
			inAliases: manifest.Alias{
				AdvancedAliases: []manifest.AdvancedAlias{
					{
						Alias:      aws.String("example.com"),
						HostedZone: aws.String("mockHostedZone1"),
					},
					{
						Alias:      aws.String("foobar.com"),
						HostedZone: aws.String("mockHostedZone2"),
					},
				},
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": cannot specify alias hosted zones [mockHostedZone1 mockHostedZone2] when no certificates are imported in environment \"mockEnv\""),
		},
		"cannot specify alias hosted zone when cdn is enabled": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-east-1",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				envConfig.CDNConfig.Config.Certificate = aws.String(mockCDNCertARN)
				return envConfig
			},
			inAliases: manifest.Alias{
				AdvancedAliases: []manifest.AdvancedAlias{
					{
						Alias:      aws.String("example.com"),
						HostedZone: aws.String("mockHostedZone1"),
					},
					{
						Alias:      aws.String("foobar.com"),
						HostedZone: aws.String("mockHostedZone2"),
					},
				},
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": cannot specify alias hosted zones when cdn is enabled in environment \"mockEnv\""),
		},
		"fail to validate certificate aliases": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				return envConfig
			},
			inAliases: manifest.Alias{
				AdvancedAliases: mockMultiAliases,
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, mockCertARNs).Return(mockError)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": validate aliases against the imported public ALB certificate for env mockEnv: some error"),
		},
		"fail to validate cdn certificate aliases": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-east-1",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				envConfig.CDNConfig.Config.Certificate = aws.String(mockCDNCertARN)
				return envConfig
			},
			inAliases: manifest.Alias{
				AdvancedAliases: mockMultiAliases,
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, mockCertARNs).Return(nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, []string{mockCDNCertARN}).Return(mockError)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": validate aliases against the imported CDN certificate for env mockEnv: some error"),
		},
		"alias used while app is not associated with a domain": {
			inAliases: manifest.Alias{AdvancedAliases: mockAlias},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: errors.New(`validate ALB runtime configuration for "http": cannot specify "alias" when application is not associated with a domain and env mockEnv doesn't import one or more certificates`),
		},
		"nlb alias used while app is not associated with a domain": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Listener: manifest.NetworkLoadBalancerListener{
					Port: aws.String("80"),
				},
				Aliases: manifest.Alias{AdvancedAliases: mockAlias},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: errors.New("cannot specify nlb.alias when application is not associated with a domain"),
		},
		"nlb alias used while env has imported certs": {
			inAliases: manifest.Alias{AdvancedAliases: mockAlias},
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Listener: manifest.NetworkLoadBalancerListener{
					Port: aws.String("80"),
				},
				Aliases: manifest.Alias{AdvancedAliases: mockAlias},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				return envConfig
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"mockAlias"}, mockCertARNs).Return(nil).Times(2)
			},
			wantErr: errors.New("cannot specify nlb.alias when env mockEnv imports one or more certificates"),
		},
		"fail to get app version": {
			inAliases: manifest.Alias{AdvancedAliases: mockAlias},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("", mockError)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": alias not supported: get version for app %q: %w", mockAppName, mockError),
		},
		"fail to enable https alias because of incompatible app version": {
			inAliases: manifest.Alias{AdvancedAliases: mockAlias},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v0.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf("validate ALB runtime configuration for \"http\": alias not supported: app version must be >= %s", version.AppTemplateMinAlias),
		},
		"fail to enable nlb alias because of incompatible app version": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Listener: manifest.NetworkLoadBalancerListener{
					Port: aws.String("80"),
				},
				Aliases: manifest.Alias{AdvancedAliases: mockAlias},
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
				m.mockAppVersionGetter.EXPECT().Version().Return("v0.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf("alias not supported: app version must be >= %s", version.AppTemplateMinAlias),
		},
		"fail to enable https alias because of invalid alias": {
			inAliases: manifest.Alias{AdvancedAliases: []manifest.AdvancedAlias{
				{Alias: aws.String("v1.v2.mockDomain")},
			}},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf(`validate ALB runtime configuration for "http": validate 'alias': alias "v1.v2.mockDomain" is not supported in hosted zones managed by Copilot`),
		},
		"fail to enable nlb alias because of invalid alias": {
			inNLB: manifest.NetworkLoadBalancerConfiguration{
				Listener: manifest.NetworkLoadBalancerListener{
					Port: aws.String("80"),
				},
				Aliases: manifest.Alias{AdvancedAliases: []manifest.AdvancedAlias{
					{Alias: aws.String("v1.v2.mockDomain")},
				}},
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
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil)
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
			},
			wantErr: fmt.Errorf(`validate 'nlb.alias': alias "v1.v2.mockDomain" is not supported in hosted zones managed by Copilot`),
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(errors.New("some error"))
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(cloudformation.NewMockErrChangeSetEmpty())
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).Return(mockError)
				m.mockSpinner.EXPECT().Stop(log.Serrorf(fmtForceUpdateSvcFailed, mockName, mockEnvName, mockError))
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).
					Return(&ecs.ErrWaitServiceStableTimeout{})
				m.mockSpinner.EXPECT().Stop(
					log.Serror(fmt.Sprintf("%s  Run %s to check for the fail reason.\n",
						fmt.Sprintf(fmtForceUpdateSvcFailed, mockName, mockEnvName, &ecs.ErrWaitServiceStableTimeout{}),
						color.HighlightCode(fmt.Sprintf("copilot svc status --name %s --env %s", mockName, mockEnvName)))))
			},
			wantErr: fmt.Errorf("force an update for service mockWkld: max retries 0 exceeded"),
		},
		"skip validating": {
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(nil)
			},
		},
		"success": {
			inAliases: manifest.Alias{
				AdvancedAliases: mockMultiAliases,
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				return envConfig
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, mockCertARNs).Return(nil).Times(2)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(nil)
			},
		},
		"success with http redirect disabled and alb certs imported": {
			inRedirectToHTTPS: aws.Bool(false),
			inAliases: manifest.Alias{
				AdvancedAliases: mockMultiAliases,
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.HTTPConfig.Public.Certificates = mockCertARNs
				return envConfig
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, mockCertARNs).Return(nil).Times(2)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(nil)
			},
		},
		"success with only cdn certs imported": {
			inAliases: manifest.Alias{
				AdvancedAliases: mockMultiAliases,
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				envConfig.CDNConfig.Config.Certificate = aws.String(mockCDNCertARN)
				return envConfig
			},
			inApp: &config.Application{
				Name: mockAppName,
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockValidator.EXPECT().ValidateCertAliases([]string{"example.com", "foobar.com"}, []string{mockCDNCertARN}).Return(nil).Times(2)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(nil)
			},
		},
		"success with http redirect disabled and domain imported": {
			inRedirectToHTTPS: aws.Bool(false),
			inAliases: manifest.Alias{
				AdvancedAliases: []manifest.AdvancedAlias{
					{
						Alias: aws.String("hi.mockDomain"),
					},
				},
			},
			inEnvironment: &config.Environment{
				Name:   mockEnvName,
				Region: "us-west-2",
			},
			inEnvironmentConfig: func() *manifest.Environment {
				envConfig := &manifest.Environment{}
				return envConfig
			},
			inApp: &config.Application{
				Name:   mockAppName,
				Domain: "mockDomain",
			},
			mock: func(m *deployMocks) {
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockAppVersionGetter.EXPECT().Version().Return("v1.0.0", nil).Times(2)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).Return(nil)
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
				m.mockEndpointGetter.EXPECT().ServiceDiscoveryEndpoint().Return("mockApp.local", nil)
				m.mockEnvVersionGetter.EXPECT().Version().Return("v1.42.0", nil)
				m.mockServiceDeployer.EXPECT().DeployService(gomock.Any(), "mockBucket", false, gomock.Any()).
					Return(cloudformation.NewMockErrChangeSetEmpty())
				m.mockServiceForceUpdater.EXPECT().LastUpdatedAt(mockAppName, mockEnvName, mockName).
					Return(mockBeforeTime, nil)
				m.mockSpinner.EXPECT().Start(fmt.Sprintf(fmtForceUpdateSvcStart, mockName, mockEnvName))
				m.mockServiceForceUpdater.EXPECT().ForceUpdateService(mockAppName, mockEnvName, mockName).Return(nil)
				m.mockSpinner.EXPECT().Stop(log.Ssuccessf(fmtForceUpdateSvcComplete, mockName, mockEnvName))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployMocks{
				mockAppVersionGetter:    mocks.NewMockversionGetter(ctrl),
				mockEnvVersionGetter:    mocks.NewMockversionGetter(ctrl),
				mockEndpointGetter:      mocks.NewMockendpointGetter(ctrl),
				mockServiceDeployer:     mocks.NewMockserviceDeployer(ctrl),
				mockServiceForceUpdater: mocks.NewMockserviceForceUpdater(ctrl),
				mockSpinner:             mocks.NewMockspinner(ctrl),
				mockValidator:           mocks.NewMockaliasCertValidator(ctrl),
			}
			tc.mock(m)

			if tc.inEnvironmentConfig == nil {
				tc.inEnvironmentConfig = func() *manifest.Environment {
					return &manifest.Environment{}
				}
			}
			deployer := lbWebSvcDeployer{
				svcDeployer: &svcDeployer{
					workloadDeployer: &workloadDeployer{
						name:             mockName,
						app:              tc.inApp,
						env:              tc.inEnvironment,
						envConfig:        tc.inEnvironmentConfig(),
						resources:        mockResources,
						deployer:         m.mockServiceDeployer,
						endpointGetter:   m.mockEndpointGetter,
						spinner:          m.mockSpinner,
						envVersionGetter: m.mockEnvVersionGetter,
						overrider:        new(override.Noop),
					},
					newSvcUpdater: func(f func(*session.Session) serviceForceUpdater) serviceForceUpdater {
						return m.mockServiceForceUpdater
					},
					now: func() time.Time {
						return mockNowTime
					},
				},
				appVersionGetter: m.mockAppVersionGetter,
				newAliasCertValidator: func(region *string) aliasCertValidator {
					return m.mockValidator
				},
				lbMft: &manifest.LoadBalancedWebService{
					Workload: manifest.Workload{
						Name: aws.String(mockName),
					},
					LoadBalancedWebServiceConfig: manifest.LoadBalancedWebServiceConfig{
						ImageConfig: manifest.ImageWithPortAndHealthcheck{
							ImageWithPort: manifest.ImageWithPort{
								Image: manifest.Image{
									ImageLocationOrBuild: manifest.ImageLocationOrBuild{
										Build: manifest.BuildArgsOrString{BuildString: aws.String("/Dockerfile")},
									},
								},
								Port: aws.Uint16(80),
							},
						},
						HTTPOrBool: manifest.HTTPOrBool{
							HTTP: manifest.HTTP{
								Main: manifest.RoutingRule{
									Path:            aws.String("/"),
									Alias:           tc.inAliases,
									RedirectToHTTPS: tc.inRedirectToHTTPS,
								},
								AdditionalRoutingRules: []manifest.RoutingRule{
									{
										Path:            aws.String("/admin"),
										Alias:           tc.inAliases,
										RedirectToHTTPS: tc.inRedirectToHTTPS,
									},
								},
							},
						},
						NLBConfig: tc.inNLB,
					},
				},
				newStack: func() cloudformation0.StackConfiguration {
					return new(stubCloudFormationStack)
				},
			}

			_, gotErr := deployer.DeployWorkload(&DeployWorkloadInput{
				Options: Options{
					ForceNewUpdate:  tc.inForceDeploy,
					DisableRollback: tc.inDisableRollback,
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

func TestUploadArtifacts(t *testing.T) {
	d := &workloadDeployer{}
	errFunc := func(out *UploadArtifactsOutput) error {
		return errors.New("test error")
	}
	noErrFunc := func(out *UploadArtifactsOutput) error {
		out.AddonsURL = "an addons url"
		return nil
	}

	out, err := d.uploadArtifacts(noErrFunc, errFunc)
	require.EqualError(t, err, "test error")
	require.Nil(t, out)

	out, err = d.uploadArtifacts(errFunc, noErrFunc)
	require.EqualError(t, err, "test error")
	require.Nil(t, out)

	out, err = d.uploadArtifacts(noErrFunc)
	require.NoError(t, err)
	require.Equal(t, &UploadArtifactsOutput{AddonsURL: "an addons url"}, out)

}

type deployDiffMocks struct {
	mockDeployedTmplGetter *mocks.MockdeployedTemplateGetter
}

func TestWorkloadDeployer_DeployDiff(t *testing.T) {
	testCases := map[string]struct {
		inTemplate string
		setUpMocks func(m *deployDiffMocks)
		wanted     string
		checkErr   func(t *testing.T, gotErr error)
	}{
		"error getting the deployed template": {
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.
					EXPECT().Template(gomock.Eq(stack.NameForWorkload("mockApp", "mockEnv", "mockSvc"))).
					Return("", errors.New("some error"))
			},
			checkErr: func(t *testing.T, gotErr error) {
				require.EqualError(t, gotErr, `retrieve the deployed template for "mockSvc": some error`)
			},
		},
		"error parsing the diff against the deployed template": {
			inTemplate: `!!!???what a weird template`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(stack.NameForWorkload("mockApp", "mockEnv", "mockSvc"))).
					Return("wow such template", nil)
			},
			checkErr: func(t *testing.T, gotErr error) {
				require.ErrorContains(t, gotErr, `parse the diff against the deployed "mockSvc" in environment "mockEnv"`)
			},
		},
		"get the correct diff": {
			inTemplate: `peace: and love`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(stack.NameForWorkload("mockApp", "mockEnv", "mockSvc"))).
					Return("peace: und Liebe", nil)
			},
			wanted: `~ peace: und Liebe -> and love
`,
		},
		"get the correct diff when there is no deployed diff": {
			inTemplate: `peace: and love`,
			setUpMocks: func(m *deployDiffMocks) {
				m.mockDeployedTmplGetter.EXPECT().
					Template(gomock.Eq(stack.NameForWorkload("mockApp", "mockEnv", "mockSvc"))).
					Return("", &cloudformation.ErrStackNotFound{})
			},
			wanted: `+ peace: and love
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &deployDiffMocks{
				mockDeployedTmplGetter: mocks.NewMockdeployedTemplateGetter(ctrl),
			}
			tc.setUpMocks(m)
			deployer := workloadDeployer{
				name: "mockSvc",
				app: &config.Application{
					Name: "mockApp",
				},
				env: &config.Environment{
					Name: "mockEnv",
				},
				tmplGetter: m.mockDeployedTmplGetter,
			}
			got, gotErr := deployer.DeployDiff(tc.inTemplate)
			if tc.checkErr != nil {
				tc.checkErr(t, gotErr)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}
