// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"testing"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/secretsmanager"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	archermocks "github.com/aws/amazon-ecs-cli-v2/mocks"

	"github.com/gobuffalo/packd"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const githubRepoURL = "https://github.com/badGoose/chaOS.git"
const githubRepoName = "https://github.com/badGoose/chaOS"
const githubToken = "hunter2"
const githubBranch = "dev"

func TestInitPipelineOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inEnvironments      []string
		inGitHubRepo        string
		inGitHubAccessToken string
		inGitBranch         string
		inProjectEnvs       []string

		mockPrompt func(m *climocks.Mockprompter)

		expectedGitHubRepo        string
		expectedGitHubAccessToken string
		expectedGitBranch         string
		expectedEnvironments      []string
		expectedError             error
	}{
		"prompts for all input": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubRepoPrompt), gomock.Any(), gomock.Any()).Return(githubRepoURL, nil).Times(1)
				m.EXPECT().Get(gomock.Eq(pipelineEnterGitBranchPrompt), gomock.Any(), gomock.Any(), gomock.Any()).Return(githubBranch, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: https://github.com/badGoose/chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedGitBranch:         githubBranch,
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"returns error if fail to confirm adding environment": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(false, errors.New("some error")).Times(1)
			},

			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("confirm adding an environment: some error"),
		},
		"returns error if fail to add an environment": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to add environment: some error"),
		},
		"returns error if fail to get GitHub repo": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubRepoPrompt), gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to get GitHub repository: some error"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubRepoPrompt), gomock.Any(), gomock.Any()).Return(githubRepoURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: https://github.com/badGoose/chaOS"), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: "",
			expectedGitBranch:         "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to get GitHub access token: some error"),
		},
		"returns error if fail to get GitHub branch name": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			inProjectEnvs:       []string{"test", "prod"},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubRepoPrompt), gomock.Any(), gomock.Any()).Return(githubRepoURL, nil).Times(1)
				m.EXPECT().Get(gomock.Eq(pipelineEnterGitBranchPrompt), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: https://github.com/badGoose/chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedGitBranch:         "",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("failed to get git branch name: some error"),
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
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubAccessToken,
				GitBranch:         tc.inGitBranch,

				projectEnvs: tc.inProjectEnvs,

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
				require.Equal(t, tc.expectedGitHubRepo, opts.GitHubRepo)
				require.Equal(t, tc.expectedGitHubAccessToken, opts.GitHubAccessToken)
				require.Equal(t, tc.expectedGitBranch, opts.GitBranch)
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

func TestInitPipelineOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inEnvironments []string
		inGitHubToken  string
		inGitHubRepo   string
		inGitBranch    string
		inProjectName  string

		mockSecretsManager func(m *archermocks.MockSecretsManager)
		mockManifestWriter func(m *archermocks.MockManifestIO)
		mockBox            func(box *packd.MemoryBox)
		mockFileSystem     func(mockFS afero.Fs)

		expectedSecretName    string
		expectManifestPath    string
		expectedBuildspecPath string
		expectedError         error
	}{
		"creates secret and writes manifest and buildspecs": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "https://github.com/badgoose/goose",
			inGitBranch:    githubBranch,
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockManifestIO) {
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    workspace.PipelineFileName,
			expectedBuildspecPath: workspace.BuildspecFileName,
			expectedError:         nil,
		},
		"does not return an error if secret already exists": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "https://github.com/badgoose/goose",
			inGitBranch:    githubBranch,
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockManifestWriter: func(m *archermocks.MockManifestIO) {
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    workspace.PipelineFileName,
			expectedBuildspecPath: workspace.BuildspecFileName,
			expectedError:         nil,
		},
		"returns an error if buildspec template does not exist": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "https://github.com/badgoose/goose",
			inGitBranch:    githubBranch,
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockManifestIO) {
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Times(0)
			},
			mockBox: func(m *packd.MemoryBox) {
			},
			expectedError: fmt.Errorf("find template for %s: %w", buildspecTemplatePath, os.ErrNotExist),
		},
		"returns an error if can't write buildspec": {
			inEnvironments: []string{"test"},
			inGitHubToken:  "hunter2",
			inGitHubRepo:   "https://github.com/badgoose/goose",
			inGitBranch:    githubBranch,
			inProjectName:  "badgoose",

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockManifestIO) {
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Return("", errors.New("some error"))
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
			},
			expectedError: fmt.Errorf("write file %s to workspace: some error", workspace.BuildspecFileName),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := archermocks.NewMockSecretsManager(ctrl)
			mockWriter := archermocks.NewMockManifestIO(ctrl)
			mockBox := packd.NewMemoryBox()

			tc.mockSecretsManager(mockSecretsManager)
			tc.mockManifestWriter(mockWriter)
			tc.mockBox(mockBox)
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}

			opts := &InitPipelineOpts{
				Environments:      tc.inEnvironments,
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubToken,
				GitBranch:         tc.inGitBranch,
				secretsmanager:    mockSecretsManager,
				workspace:         mockWriter,
				box:               mockBox,
				fsUtils:           memFs,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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

// Tests for helpers
func TestInitPipelineOpts_getRepoName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubRepo string
		expected     string
	}{
		"matches repo name": {
			inGitHubRepo: "https://github.com/bad/goose",
			expected:     "goose",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &InitPipelineOpts{
				GitHubRepo: tc.inGitHubRepo,
			}

			// WHEN
			actual := opts.getRepoName()

			// THEN
			require.Equal(t, tc.expected, actual)
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
			inGitHubRepo:  "https://github.com/bad/goose",
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
		inGitHubRepo  string
		inProjectName string

		expected string
	}{
		"matches repo name": {
			inGitHubRepo:  "https://github.com/bad/goose",
			inProjectName: "badgoose",

			expected: "pipeline-badgoose-goose",
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
			actual := opts.createPipelineName()

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}
