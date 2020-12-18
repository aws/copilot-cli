// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	templatemocks "github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestInitPipelineOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inrepoURL     string
		inEnvs        []string
		setupMocks    func(m *mocks.Mockstore)
		expectedError error
	}{
		"empty app name": {
			inAppName:     "",
			setupMocks:    func(m *mocks.Mockstore) {},
			expectedError: errNoAppInWorkspace,
		},
		"invalid app name (not in workspace)": {
			inAppName: "ghost-app",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("ghost-app").Return(nil, fmt.Errorf("get application ghost-app: some error"))
			},
			expectedError: fmt.Errorf("get application ghost-app: some error"),
		},
		"invalid URL": {
			inAppName: "my-app",
			inrepoURL: "somethingdotcom",
			inEnvs:    []string{"test"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			expectedError: errDomainInvalid,
		},
		"non-GH repo provider": {
			inAppName: "my-app",
			inrepoURL: "bitbucket.org/repositories/repoName",
			inEnvs:    []string{"test"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			expectedError: errors.New("Copilot currently accepts only URLs to GitHub repository sources"),
		},
		"invalid environments": {
			inAppName: "my-app",
			inrepoURL: "https://github.com/badGoose/chaOS",
			inEnvs:    []string{"test", "prod"},

			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},

			expectedError: errors.New("some error"),
		},
		"success": {
			inAppName: "my-app",
			inEnvs:    []string{"test", "prod"},
			inrepoURL: "https://github.com/badGoose/chaOS",

			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
			},

			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			tc.setupMocks(mockStore)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:      tc.inAppName,
					repoURL:      tc.inrepoURL,
					environments: tc.inEnvs,
				},
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestInitPipelineOpts_Ask(t *testing.T) {
	githubOwner := "badGoose"
	githubRepoName := "chaOS"
	githubAnotherOwner := "goodGoose"
	githubAnotherRepoName := "bhaOS"
	githubURL := "https://github.com/badGoose/chaOS"
	githubAnotherURL := "git@github.com:goodGoose/bhaOS.git"
	githubReallyBadURL := "reallybadGoosegithub.comNotEvenAURL"
	githubToken := "hunter2"
	testCases := map[string]struct {
		inEnvironments      []string
		inRepoURL           string
		inGitHubAccessToken string
		inGitBranch         string

		mockStore  func(m *mocks.Mockstore)
		mockPrompt func(m *mocks.Mockprompter)
		mockRunner func(m *mocks.Mockrunner)
		buffer     bytes.Buffer

		expectedEnvironments      []string
		expectedRepoURL           string
		expectedGitHubOwner       string
		expectedGitHubRepo        string
		expectedGitHubAccessToken string
		expectedGitBranch         string
		expectedError             error
	}{
		"no flags, prompts for all input": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineInitAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository bhaOS:"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedRepoURL:           githubAnotherURL,
			expectedGitHubOwner:       githubAnotherOwner,
			expectedGitHubRepo:        githubAnotherRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedGitBranch:         "main",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"returns error if fail to list environments": {
			inEnvironments: []string{},

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return(nil, errors.New("some error"))
			},
			mockRunner: func(m *mocks.Mockrunner) {},
			mockPrompt: func(m *mocks.Mockprompter) {},

			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("list environments for application my-app: some error"),
		},
		"returns error if fail to confirm adding environment": {
			inEnvironments: []string{},

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(false, errors.New("some error")).Times(1)
			},

			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("confirm adding an environment: some error"),
		},
		"returns error if fail to add an environment": {
			inEnvironments: []string{},

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("", errors.New("some error")).Times(1)
			},

			expectedError: fmt.Errorf("add environment: some error"),
		},
		"returns error if fail to select GitHub URL": {
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineInitAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("select GitHub URL: some error"),
		},
		"returns error if fail to parse GitHub URL": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\treallybadGoosegithub.comNotEvenAURL (fetch)\n"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineInitAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), []string{githubReallyBadURL}).Return(githubReallyBadURL, nil).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("unable to parse the GitHub repository owner and name from reallybadGoosegithub.comNotEvenAURL: please pass the repository URL with the format `--github-url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineInitAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineInitAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubURLPrompt, gomock.Any(), gomock.Any()).Return(githubURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository chaOS:"), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "main",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("get GitHub access token: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           "my-app",
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
				},
				envs:   []*config.Environment{},
				prompt: mockPrompt,
				store:  mockStore,
				runner: mockRunner,
				buffer: tc.buffer,
			}

			tc.mockPrompt(mockPrompt)
			tc.mockStore(mockStore)
			tc.mockRunner(mockRunner)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedGitHubOwner, opts.githubOwner)
				require.Equal(t, tc.expectedGitHubRepo, opts.githubRepo)
				require.Equal(t, tc.expectedGitHubAccessToken, opts.githubAccessToken)
				require.ElementsMatch(t, tc.expectedEnvironments, opts.environments)
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	buildspecExistsErr := &workspace.ErrFileExists{FileName: "/buildspec.yml"}
	manifestExistsErr := &workspace.ErrFileExists{FileName: "/pipeline.yml"}
	testCases := map[string]struct {
		inEnvironments []string
		inGitHubToken  string
		inGitHubRepo   string
		inGitBranch    string
		inAppName      string
		inAppEnvs      []*config.Environment

		mockSecretsManager          func(m *mocks.MocksecretsManager)
		mockWsWriter                func(m *mocks.MockwsPipelineWriter)
		mockParser                  func(m *templatemocks.MockParser)
		mockFileSystem              func(mockFS afero.Fs)
		mockRegionalResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockStoreSvc                func(m *mocks.Mockstore)

		expectedError error
	}{
		"creates secret and writes manifest and buildspecs": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("/buildspec.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedError: nil,
		},
		"does not return an error if secret already exists": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("/buildspec.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},

			expectedError: nil,
		},
		"returns an error if can't write manifest": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("", errors.New("some error"))
			},
			mockParser:                  func(m *templatemocks.MockParser) {},
			mockStoreSvc:                func(m *mocks.Mockstore) {},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			expectedError:               errors.New("write pipeline manifest to workspace: some error"),
		},
		"returns an error if application cannot be retrieved": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(nil, errors.New("some error"))
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			expectedError:               errors.New("get application badgoose: some error"),
		},
		"returns an error if can't get regional application resources": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return(nil, errors.New("some error"))
			},
			expectedError: fmt.Errorf("get regional application resources: some error"),
		},
		"returns an error if buildspec cannot be parsed": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Times(0)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedError: errors.New("some error"),
		},
		"does not return an error if buildspec and manifest already exists": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("", manifestExistsErr)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", buildspecExistsErr)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			expectedError: nil,
		},
		"returns an error if can't write buildspec": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "goose",
			inGitBranch:    "dev",
			inAppName:      "badgoose",
			inAppEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", errors.New("some error"))
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
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

			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockWriter := mocks.NewMockwsPipelineWriter(ctrl)
			mockParser := templatemocks.NewMockParser(ctrl)
			mockRegionalResourcesGetter := mocks.NewMockappResourcesGetter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)

			tc.mockSecretsManager(mockSecretsManager)
			tc.mockWsWriter(mockWriter)
			tc.mockParser(mockParser)
			tc.mockRegionalResourcesGetter(mockRegionalResourcesGetter)
			tc.mockStoreSvc(mockstore)
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					environments:      tc.inEnvironments,
					githubRepo:        tc.inGitHubRepo,
					githubAccessToken: tc.inGitHubToken,
					gitBranch:         tc.inGitBranch,
					appName:           tc.inAppName,
				},

				secretsmanager: mockSecretsManager,
				cfnClient:      mockRegionalResourcesGetter,
				store:          mockstore,
				workspace:      mockWriter,
				parser:         mockParser,
				fs:             memFs,
				envs:           tc.inAppEnvs,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_createPipelineName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubRepo string
		inAppName    string
		inAppOwner   string

		expected string
	}{
		"matches repo name": {
			inGitHubRepo: "goose",
			inAppName:    "badgoose",
			inAppOwner:   "david",

			expected: "pipeline-badgoose-david-goose",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					githubRepo:  tc.inGitHubRepo,
					appName:     tc.inAppName,
					githubOwner: tc.inAppOwner,
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

func TestInitPipelineOpts_parseOwnerRepoName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubURL string

		expectedOwner string
		expectedRepo  string
		expectedError error
	}{
		"matches repo name without .git suffix": {
			inGitHubURL: "https://github.com/badgoose/cli",

			expectedOwner: "badgoose",
			expectedRepo:  "cli",
			expectedError: nil,
		},
		"matches repo name with .git suffix": {
			inGitHubURL: "https://github.com/koke/grit.git",

			expectedOwner: "koke",
			expectedRepo:  "grit",
			expectedError: nil,
		},
		"returns an error if it is not a github URL": {
			inGitHubURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever",

			expectedOwner: "",
			expectedRepo:  "",
			expectedError: fmt.Errorf("unable to parse the GitHub repository owner and name from https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever: please pass the repository URL with the format `--github-url https://github.com/{owner}/{repositoryName}`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			owner, repo, err := opts.parseOwnerRepoName(tc.inGitHubURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedOwner, owner)
				require.Equal(t, tc.expectedRepo, repo)
			}
		})
	}
}

func TestInitPipelineOpts_getEnvConfig(t *testing.T) {
	testCases := map[string]struct {
		inAppName         string
		inEnvironmentName string
		inEnvs            []*config.Environment

		expectedEnvironment *config.Environment
		expectedError       error
	}{
		"happy case": {
			inAppName:         "badgoose",
			inEnvironmentName: "test",
			inEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			expectedEnvironment: &config.Environment{
				Name: "test",
			},
			expectedError: nil,
		},
		"returns an error if an environment is not found": {
			inAppName:         "badgoose",
			inEnvironmentName: "badenv",
			inEnvs: []*config.Environment{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},

			expectedEnvironment: nil,
			expectedError:       fmt.Errorf("environment badenv in application badgoose is not found"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName: tc.inAppName,
				},
				envs: tc.inEnvs,
			}

			// WHEN
			env, err := opts.getEnvConfig(tc.inEnvironmentName)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedEnvironment, env)
			}
		})
	}
}
