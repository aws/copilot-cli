// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
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
		inProjectEnvs       []string
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
			inProjectEnvs:       []string{"test", "prod"},
			inURLs:              []string{githubURL, githubBadURL},

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
			inProjectEnvs:       []string{"test", "prod"},

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
			inProjectEnvs:       []string{"test", "prod"},

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
			inProjectEnvs:       []string{"test", "prod"},
			inURLs:              []string{githubURL, githubBadURL},

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
			inProjectEnvs:       []string{"test", "prod"},
			inURLs:              []string{githubReallyBadURL},

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
			inProjectEnvs:       []string{"test", "prod"},
			inURLs:              []string{githubURL, githubBadURL},

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

			opts := &InitPipelineOpts{
				Environments:      tc.inEnvironments,
				GitHubOwner:       tc.inGitHubOwner,
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubAccessToken,

				projectEnvs: tc.inProjectEnvs,
				repoURLs:    tc.inURLs,

				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
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
		inProjectEnvs []string
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

			opts := &InitPipelineOpts{
				projectEnvs: tc.inProjectEnvs,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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

// FIXME commented out until new mocks are generated
//func TestInitPipelineOpts_Execute(t *testing.T) {
//	testCases := map[string]struct {
//		inEnvironments []string
//		inGitHubToken  string
//		inGitHubRepo   string
//		inGitBranch    string
//		inProjectName  string
//
//		mockSecretsManager func(m *archermocks.MockSecretsManager)
//		mockManifestWriter func(m *archermocks.MockManifestIO)
//		mockBox            func(box *packd.MemoryBox)
//		mockFileSystem     func(mockFS afero.Fs)
//
//		expectedSecretName    string
//		expectManifestPath    string
//		expectedBuildspecPath string
//		expectedError         error
//	}{
//		"creates secret and writes manifest and buildspecs": {
//			inEnvironments: []string{"test"},
//			inGitHubToken:  "hunter2",
//			inGitHubRepo:   "goose",
//			inGitBranch:    "dev",
//			inProjectName:  "badgoose",
//
//			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
//				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
//			},
//			mockManifestWriter: func(m *archermocks.MockManifestIO) {
//				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
//				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
//			},
//			mockBox: func(m *packd.MemoryBox) {
//				m.AddString(buildspecTemplatePath, "hello")
//			},
//			expectedSecretName:    "github-token-badgoose-goose",
//			expectManifestPath:    workspace.PipelineFileName,
//			expectedBuildspecPath: workspace.BuildspecFileName,
//			expectedError:         nil,
//		},
//		"does not return an error if secret already exists": {
//			inEnvironments: []string{"test"},
//			inGitHubToken:  "hunter2",
//			inGitHubRepo:   "goose",
//			inGitBranch:    "dev",
//			inProjectName:  "badgoose",
//
//			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
//				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
//				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
//			},
//			mockManifestWriter: func(m *archermocks.MockManifestIO) {
//				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
//				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
//			},
//			mockBox: func(m *packd.MemoryBox) {
//				m.AddString(buildspecTemplatePath, "hello")
//			},
//			expectedSecretName:    "github-token-badgoose-goose",
//			expectManifestPath:    workspace.PipelineFileName,
//			expectedBuildspecPath: workspace.BuildspecFileName,
//			expectedError:         nil,
//		},
//		"returns an error if buildspec template does not exist": {
//			inEnvironments: []string{"test"},
//			inGitHubToken:  "hunter2",
//			inGitHubRepo:   "goose",
//			inGitBranch:    "dev",
//			inProjectName:  "badgoose",
//
//			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
//				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
//			},
//			mockManifestWriter: func(m *archermocks.MockManifestIO) {
//				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
//				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Times(0)
//			},
//			mockBox: func(m *packd.MemoryBox) {
//			},
//			expectedError: fmt.Errorf("find template for %s: %w", buildspecTemplatePath, os.ErrNotExist),
//		},
//		"returns an error if can't write buildspec": {
//			inEnvironments: []string{"test"},
//			inGitHubToken:  "hunter2",
//			inGitHubRepo:   "goose",
//			inGitBranch:    "dev",
//			inProjectName:  "badgoose",
//
//			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
//				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
//			},
//			mockManifestWriter: func(m *archermocks.MockManifestIO) {
//				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
//				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Return("", errors.New("some error"))
//			},
//			mockBox: func(m *packd.MemoryBox) {
//				m.AddString(buildspecTemplatePath, "hello")
//			},
//			expectedError: fmt.Errorf("write file %s to workspace: some error", workspace.BuildspecFileName),
//		},
//	}
//
//	for name, tc := range testCases {
//		t.Run(name, func(t *testing.T) {
//			// GIVEN
//			ctrl := gomock.NewController(t)
//			defer ctrl.Finish()
//
//			mockSecretsManager := archermocks.NewMockSecretsManager(ctrl)
//			mockWriter := archermocks.NewMockManifestIO(ctrl)
//			mockBox := packd.NewMemoryBox()
//
//			tc.mockSecretsManager(mockSecretsManager)
//			tc.mockManifestWriter(mockWriter)
//			tc.mockBox(mockBox)
//			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}
//
//			opts := &InitPipelineOpts{
//				Environments:      tc.inEnvironments,
//				GitHubRepo:        tc.inGitHubRepo,
//				GitHubAccessToken: tc.inGitHubToken,
//				GitBranch:         tc.inGitBranch,
//				secretsmanager:    mockSecretsManager,
//				workspace:         mockWriter,
//				box:               mockBox,
//				fsUtils:           memFs,
//
//				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
//			}
//
//			// WHEN
//			err := opts.Execute()
//
//			// THEN
//			if tc.expectedError != nil {
//				require.EqualError(t, err, tc.expectedError.Error())
//			} else {
//				require.Nil(t, err)
//				require.Equal(t, tc.expectedSecretName, opts.secretName)
//				require.Equal(t, tc.expectManifestPath, opts.manifestPath)
//				require.Equal(t, tc.expectedBuildspecPath, opts.buildspecPath)
//			}
//		})
//	}
//}

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
			opts := &InitPipelineOpts{
				GitHubRepo: tc.inGitHubRepo,
				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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
			opts := &InitPipelineOpts{
				GitHubRepo:  tc.inGitHubRepo,
				GlobalOpts:  &GlobalOpts{projectName: tc.inProjectName},
				GitHubOwner: tc.inProjectOwner,
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
			opts := &InitPipelineOpts{}

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
