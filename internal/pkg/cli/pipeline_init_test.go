// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/secretsmanager"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	archermocks "github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/gobuffalo/packd"
	"github.com/google/go-github/github"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestInitPipelineOpts_Ask(t *testing.T) {
	ctx := context.Background()
	githubOwner := "badGoose"
	githubRepoName := "chaOS"
	githubBadRepoName := "chaOA"
	githubRepos := []*github.Repository{
		&github.Repository{
			Name: &githubRepoName,
		},
		&github.Repository{
			Name: &githubBadRepoName,
		},
	}
	githubToken := "hunter2"
	githubBranch := "dev"
	githubBadBranch := "master"
	testCases := map[string]struct {
		inEnvironments      []string
		inGitHubOwner       string
		inGitHubRepo        string
		inGitHubAccessToken string
		inGitHubBranch      string
		inProjectEnvs       []string

		mockPrompt     func(m *climocks.Mockprompter)
		mockRepository func(m *climocks.Mockrepository)

		expectedGitHubOwner       string
		expectedGitHubRepo        string
		expectedGitHubAccessToken string
		expectedGitHubBranch      string
		expectedEnvironments      []string
		expectedError             error
	}{
		"prompts for all input": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(githubRepos, &github.Response{}, nil).Times(1)
				m.EXPECT().ListBranches(ctx, githubOwner, githubRepoName, gomock.Any()).Return([]*github.Branch{
					&github.Branch{
						Name: &githubBranch,
					},
					&github.Branch{
						Name: &githubBadBranch,
					},
				}, &github.Response{}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName, githubBadRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubBranchPrompt, gomock.Any(), []string{githubBranch, githubBadBranch}).Return(githubBranch, nil).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedGitHubBranch:      githubBranch,
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"returns error if fail to confirm adding environment": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(false, errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("confirm adding an environment: some error"),
		},
		"returns error if fail to add an environment": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to add environment: some error"),
		},
		"returns error if fail to get GitHub owner name": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("fail to get GitHub owner name: some error"),
		},
		"returns error if fail to list GitHub repos": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(nil, nil, errors.New("some error")).Times(1)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("fail to list repositories of %s: some error", githubOwner),
		},
		"returns error if fail to get GitHub repos": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(githubRepos, &github.Response{}, nil).Times(1)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName, githubBadRepoName}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to get GitHub repository: some error"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(githubRepos, &github.Response{}, nil).Times(1)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName, githubBadRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to get GitHub access token: some error"),
		},
		"returns error if fail to list branches": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(githubRepos, &github.Response{}, nil).Times(1)
				m.EXPECT().ListBranches(ctx, githubOwner, githubRepoName, gomock.Any()).Return(nil, nil, errors.New("some error"))
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName, githubBadRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to list branches for %s: Problem in getting repository information some error", githubRepoName),
		},
		"returns error if fail to get branches": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inGitHubBranch:      "",
			inProjectEnvs:       []string{"test", "prod"},

			mockRepository: func(m *climocks.Mockrepository) {
				m.EXPECT().List(ctx, githubOwner, gomock.Any()).Return(githubRepos, &github.Response{}, nil).Times(1)
				m.EXPECT().ListBranches(ctx, githubOwner, githubRepoName, gomock.Any()).Return([]*github.Branch{
					&github.Branch{
						Name: &githubBranch,
					},
					&github.Branch{
						Name: &githubBadBranch,
					},
				}, &github.Response{}, nil)
			},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(gomock.Eq(pipelineEnterGitHubOwnerPrompt), gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName, githubBadRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: chaOS"), gomock.Any()).Return(githubToken, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubBranchPrompt, gomock.Any(), []string{githubBranch, githubBadBranch}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedGitHubBranch:      "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("failed to get GitHub branch name: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := climocks.NewMockprompter(ctrl)
			mockRepo := climocks.NewMockrepository(ctrl)

			opts := &InitPipelineOpts{
				Environments:      tc.inEnvironments,
				GitHubOwner:       tc.inGitHubOwner,
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubAccessToken,
				GitHubBranch:      tc.inGitHubBranch,

				repo:    mockRepo,
				context: context.Background(),

				projectEnvs: tc.inProjectEnvs,

				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
			}

			tc.mockPrompt(mockPrompt)
			tc.mockRepository(mockRepo)

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
				require.Equal(t, tc.expectedGitHubBranch, opts.GitHubBranch)
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
		"errors if no environments initialized": {
			inProjectEnvs: []string{},
			inProjectName: "badgoose",

			expectedError: errNoEnvsInProject,
		},

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

func genApps(names ...string) []archer.Manifest {
	result := make([]archer.Manifest, 0, len(names))
	for _, name := range names {
		result = append(result, &manifest.LBFargateManifest{
			AppManifest: manifest.AppManifest{
				Name: name,
				Type: manifest.LoadBalancedWebApplication,
			},
			Image: manifest.ImageWithPort{
				AppImage: manifest.AppImage{
					Build: name,
				},
			},
		})
	}
	return result
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	const expectedIntegTestBuildSpecTemplate = "integrationTests"
	githubBranch := "dev"
	githubToken := "hunter2"
	githubRepo := "goose"
	projectName := "badgoose"
	githubOwner := "badgoose"
	mockApps := genApps("app01", "app02")

	testCases := map[string]struct {
		inEnvironments []string
		inGitHubToken  string
		inGitHubOwner  string
		inGitHubRepo   string
		inGitHubBranch string
		inProjectName  string

		mockSecretsManager func(m *archermocks.MockSecretsManager)
		mockManifestWriter func(m *archermocks.MockWorkspace)
		mockBox            func(box *packd.MemoryBox)
		mockFileSystem     func(mockFS afero.Fs)

		expectedSecretName              string
		expectManifestPath              string
		expectedBuildspecPath           string
		expectedIntegTestBuildspecPaths []string
		expectedError                   error
	}{
		"creates secret and writes manifest and buildspecs": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
				m.AddString(integTestBuildspecTemplatePath, expectedIntegTestBuildSpecTemplate)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				for _, app := range mockApps {
					mockFS.MkdirAll(filepath.Dir(app.IntegTestBuildspecPath()), 0755)
				}
			},
			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    workspace.PipelineFileName,
			expectedBuildspecPath: workspace.BuildspecFileName,
			expectedIntegTestBuildspecPaths: func() []string {
				expectedPaths := make([]string, 0, len(mockApps))
				for _, app := range mockApps {
					expectedPaths = append(expectedPaths, app.IntegTestBuildspecPath())
				}
				return expectedPaths
			}(),
			expectedError: nil,
		},
		"does not return an error if secret already exists": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
				m.AddString(integTestBuildspecTemplatePath, expectedIntegTestBuildSpecTemplate)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				for _, app := range mockApps {
					mockFS.MkdirAll(filepath.Dir(app.IntegTestBuildspecPath()), 0755)
				}
			},

			expectedSecretName:    "github-token-badgoose-goose",
			expectManifestPath:    workspace.PipelineFileName,
			expectedBuildspecPath: workspace.BuildspecFileName,
			expectedIntegTestBuildspecPaths: func() []string {
				expectedPaths := make([]string, 0, len(mockApps))
				for _, app := range mockApps {
					expectedPaths = append(expectedPaths, app.IntegTestBuildspecPath())
				}
				return expectedPaths
			}(),
			expectedError: nil,
		},
		"returns an error if buildspec template does not exist": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Times(0)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(integTestBuildspecTemplatePath, expectedIntegTestBuildSpecTemplate)
			},
			mockFileSystem: func(mockFS afero.Fs) {},
			expectedError:  fmt.Errorf("find template for %s: %w", buildspecTemplatePath, os.ErrNotExist),
		},
		"returns an error if integ test buildspec template does not exist": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
				// intentionally miss out the integration test template here
			},
			mockFileSystem: func(mockFS afero.Fs) {},
			expectedError:  fmt.Errorf("find integration test template for %s: %w", integTestBuildspecTemplatePath, os.ErrNotExist),
		},
		"returns an error if can't write buildspec": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.BuildspecFileName).Return("", errors.New("some error"))
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
				m.AddString(integTestBuildspecTemplatePath, expectedIntegTestBuildSpecTemplate)
			},
			mockFileSystem: func(mockFS afero.Fs) {},
			expectedError:  fmt.Errorf("write file %s to workspace: some error", workspace.BuildspecFileName),
		},
		"returns an error if can't write integration test buildspec": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(mockApps, nil)
				m.EXPECT().WriteFile(gomock.Any(), workspace.PipelineFileName).Return(workspace.PipelineFileName, nil)
				m.EXPECT().WriteFile([]byte("hello"), workspace.BuildspecFileName).Return(workspace.BuildspecFileName, nil)
				m.EXPECT().WriteFile(gomock.Any(), mockApps[0].IntegTestBuildspecPath()).Return("", errors.New("error writing integ test spec"))
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
				m.AddString(integTestBuildspecTemplatePath, expectedIntegTestBuildSpecTemplate)
			},
			expectedError: fmt.Errorf("write integration test buildspec %s to app %s: error writing integ test spec", mockApps[0].IntegTestBuildspecPath(), mockApps[0].AppName()),
		},
		"returns an error when retrieving local apps": {
			inEnvironments: []string{"test"},
			inGitHubOwner:  githubOwner,
			inGitHubToken:  githubToken,
			inGitHubRepo:   githubRepo,
			inGitHubBranch: githubBranch,
			inProjectName:  projectName,

			mockSecretsManager: func(m *archermocks.MockSecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockManifestWriter: func(m *archermocks.MockWorkspace) {
				m.EXPECT().Apps().Return(nil, errors.New("some error"))
			},
			mockBox: func(m *packd.MemoryBox) {
				m.AddString(buildspecTemplatePath, "hello")
			},
			mockFileSystem: func(mockFS afero.Fs) {},
			expectedError:  errors.New("could not retrieve apps in this workspace: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := archermocks.NewMockSecretsManager(ctrl)
			mockWriter := archermocks.NewMockWorkspace(ctrl)
			mockBox := packd.NewMemoryBox()

			tc.mockSecretsManager(mockSecretsManager)
			tc.mockManifestWriter(mockWriter)
			tc.mockBox(mockBox)
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(memFs)

			opts := &InitPipelineOpts{
				Environments:      tc.inEnvironments,
				GitHubOwner:       tc.inGitHubOwner,
				GitHubRepo:        tc.inGitHubRepo,
				GitHubAccessToken: tc.inGitHubToken,
				GitHubBranch:      tc.inGitHubBranch,
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
				require.Equal(t, tc.expectedIntegTestBuildspecPaths, opts.integTestBuildspecPaths)
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
			inGitHubRepo:  "goose",
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
