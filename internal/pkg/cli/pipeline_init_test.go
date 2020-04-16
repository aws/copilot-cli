// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	archermocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/secretsmanager"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	templatemocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestInitPipelineOpts_Ask(t *testing.T) {
	githubOwner := "badGoose"
	githubRepoName := "chaOS"
	githubURL := "https://github.com/badGoose/chaOS"
	githubBadURL := "git@github.com:goodGoose/bhaOS"
	githubReallyBadURL := "reallybadGoose//notEvenAURL"
	githubToken := "hunter2"
	testCases := map[string]struct {
		inEnvironments      []string
		inGitHubOwner       string
		inGitHubRepo        string
		inGitHubAccessToken string
		inProjectEnvs       []*archer.Environment
		inURLs              []string

		mockPrompt func(m *climocks.Mockprompter)

		expectedGitHubOwner       string
		expectedGitHubRepo        string
		expectedGitHubAccessToken string
		expectedEnvironments      []string
		expectedError             error
	}{
		"prompts for all input": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},
			inURLs: []string{githubURL, githubBadURL},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), []string{githubURL, githubBadURL}).Return(githubURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"returns error if fail to confirm adding environment": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(false, errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("confirm adding an environment: some error"),
		},
		"returns error if fail to add an environment": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("add environment: some error"),
		},
		"returns error if fail to select GitHub URL": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},
			inURLs: []string{githubURL, githubBadURL},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), []string{githubURL, githubBadURL}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("select GitHub URL: some error"),
		},
		"returns error if fail to parse GitHub URL": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},
			inURLs: []string{githubReallyBadURL},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), []string{githubReallyBadURL}).Return(githubReallyBadURL, nil).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("unable to parse the GitHub repository owner and name from reallybadGoose//notEvenAURL: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs: []*archer.Environment{
				&archer.Environment{
					Name: "test",
				},
				&archer.Environment{
					Name: "prod",
				},
			},
			inURLs: []string{githubURL, githubBadURL},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), []string{githubURL, githubBadURL}).Return(githubURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("get GitHub access token: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := climocks.NewMockprompter(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					Environments:      tc.inEnvironments,
					GitHubOwner:       tc.inGitHubOwner,
					GitHubRepo:        tc.inGitHubRepo,
					GitHubAccessToken: tc.inGitHubAccessToken,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},

				projectEnvs: tc.inProjectEnvs,
				repoURLs:    tc.inURLs,
			}

			tc.mockPrompt(mockPrompt)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedGitHubOwner, opts.GitHubOwner)
				require.Equal(t, tc.expectedGitHubRepo, opts.GitHubRepo)
				require.Equal(t, tc.expectedGitHubAccessToken, opts.GitHubAccessToken)
				require.ElementsMatch(t, tc.expectedEnvironments, opts.Environments)
			}
		})
	}
}

func TestInitPipelineOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string

		expectedError error
	}{
		"invalid project name": {
			inProjectName: "",
			expectedError: errNoProjectInWorkspace,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
				},
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	fileExistsErr := &workspace.ErrFileExists{FileName: "buildspec.yml"}
	testCases := map[string]struct {
		inEnvironments []string
		inGitHubToken  string
		inGitHubRepo   string
		inGitBranch    string
		inProjectName  string

		mockSecretsManager          func(m *archermocks.MockSecretsManager)
		mockWsWriter                func(m *climocks.MockwsPipelineWriter)
		mockParser                  func(m *templatemocks.MockParser)
		mockFileSystem              func(mockFS afero.Fs)
		mockRegionalResourcesGetter func(m *archermocks.MockProjectResourceStore)
		mockStoreSvc                func(m *climocks.MockstoreReader)

		expectedSecretName    string
		expectManifestPath    string
		expectedBuildspecPath string
		expectedError         error
	}{
		"creates secret and writes manifest and buildspecs": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("buildspec.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return([]*archer.ProjectRegionalResources{
					&archer.ProjectRegionalResources{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    "pipeline.yml",
			expectedBuildspecPath: "buildspec.yml",
			expectedError:         nil,
		},
		"does not return an error if secret already exists": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("buildspec.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return([]*archer.ProjectRegionalResources{
					&archer.ProjectRegionalResources{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    "pipeline.yml",
			expectedBuildspecPath: "buildspec.yml",
			expectedError:         nil,
		},
		"returns an error if can't write manifest": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("", errors.New("some error"))
			},
			mockParser:                  func(m *templatemocks.MockParser) {},
			mockStoreSvc:                func(m *climocks.MockstoreReader) {},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {},
			expectedError:               errors.New("some error"),
		},
		"returns an error if project cannot be retrieved": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(nil, errors.New("some error"))
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {},
			expectedError:               errors.New("get project metadata badgoose: some error"),
		},
		"returns an error if can't get regional project resources": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return(nil, errors.New("some error"))
			},
			expectedError: fmt.Errorf("get regional project resources: some error"),
		},
		"returns an error if buildspec cannot be parsed": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Times(0)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return([]*archer.ProjectRegionalResources{
					&archer.ProjectRegionalResources{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedError: errors.New("some error"),
		},
		"does not return an error if buildspec already exists": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", fileExistsErr)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return([]*archer.ProjectRegionalResources{
					&archer.ProjectRegionalResources{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    "pipeline.yml",
			expectedBuildspecPath: "buildspec.yml",
			expectedError:         nil,
		},
		"returns an error if can't write buildspec": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *climocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", errors.New("some error"))
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("badgoose").Return(&archer.Project{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *archermocks.MockProjectResourceStore) {
				m.EXPECT().GetRegionalProjectResources(&archer.Project{
					Name: "badgoose",
				}).Return([]*archer.ProjectRegionalResources{
					&archer.ProjectRegionalResources{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedError: fmt.Errorf("write buildspec to workspace: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := archermocks.NewMockSecretsManager(ctrl)
			mockWriter := climocks.NewMockwsPipelineWriter(ctrl)
			mockParser := templatemocks.NewMockParser(ctrl)
			mockRegionalResourcesGetter := archermocks.NewMockProjectResourceStore(ctrl)
			mockStoreReader := climocks.NewMockstoreReader(ctrl)

			tc.mockSecretsManager(mockSecretsManager)
			tc.mockWsWriter(mockWriter)
			tc.mockParser(mockParser)
			tc.mockRegionalResourcesGetter(mockRegionalResourcesGetter)
			tc.mockStoreSvc(mockStoreReader)
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					Environments:      tc.inEnvironments,
					GitHubRepo:        tc.inGitHubRepo,
					GitHubAccessToken: tc.inGitHubToken,
					GitBranch:         tc.inGitBranch,
					GlobalOpts:        &GlobalOpts{projectName: tc.inProjectName},
				},

				secretsmanager: mockSecretsManager,
				cfnClient:      mockRegionalResourcesGetter,
				storeSvc:       mockStoreReader,
				workspace:      mockWriter,
				parser:         mockParser,
				fsUtils:        memFs,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedSecretName, opts.secretName)
				require.Equal(t, tc.expectManifestPath, opts.manifestPath)
				require.Equal(t, tc.expectedBuildspecPath, opts.buildspecPath)
			}
		})
	}
}

func TestInitPipelineOpts_createSecretName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubRepo  string
		inProjectName string

		expected string
	}{
		"matches repo name": {
			inGitHubRepo:  "goose",
			inProjectName: "badgoose",

			expected: "github-token-badgoose-goose",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					GitHubRepo: tc.inGitHubRepo,
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
				},
			}

			// WHEN
			actual := opts.createSecretName()

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestInitPipelineOpts_createPipelineName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubRepo   string
		inProjectName  string
		inProjectOwner string

		expected string
	}{
		"matches repo name": {
			inGitHubRepo:   "goose",
			inProjectName:  "badgoose",
			inProjectOwner: "david",

			expected: "pipeline-badgoose-david-goose",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					GitHubRepo:  tc.inGitHubRepo,
					GlobalOpts:  &GlobalOpts{projectName: tc.inProjectName},
					GitHubOwner: tc.inProjectOwner,
				},
			}

			// WHEN
			actual := opts.createPipelineName()

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestInitPipelineOpts_parseGitRemoteResult(t *testing.T) {
	testCases := map[string]struct {
		inRemoteResult string

		expectedURLs  []string
		expectedError error
	}{
		"matched format": {
			inRemoteResult: `badgoose	git@github.com:badgoose/grit.git (fetch)
badgoose	https://github.com/badgoose/cli.git (fetch)
origin	https://github.com/koke/grit (fetch)
koke	git://github.com/koke/grit.git (push)`,

			expectedURLs:  []string{"git@github.com:badgoose/grit", "https://github.com/badgoose/cli", "https://github.com/koke/grit", "git://github.com/koke/grit"},
			expectedError: nil,
		},
		"don't add to URL list if it is not a github URL": {
			inRemoteResult: `badgoose	verybad@gitlab.com/whatever (fetch)`,

			expectedURLs:  []string{},
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			urls, err := opts.parseGitRemoteResult(tc.inRemoteResult)
			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.ElementsMatch(t, tc.expectedURLs, urls)
			}
		})
	}
}
