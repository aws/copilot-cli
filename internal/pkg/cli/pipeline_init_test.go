// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
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

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestInitPipelineOpts_Ask(t *testing.T) {
	githubOwner := "bad-goose"
	githubBadOwner := "//github.com/badGoose"
	githubRepoName := "cha-os"
	githubBadRepoName := "/github.com/bad-goose/cha-os"
	githubToken := "hunter2"
	testCases := map[string]struct {
		inEnvironments      []string
		inGitHubOwner       string
		inGitHubRepo        string
		inGitHubAccessToken string
		inProjectEnvs       []string
		inOwners            []string
		inRepos             []string

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
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubOwnerPrompt, gomock.Any(), []string{githubOwner}).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: cha-os"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedGitHubRepo:        githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"prompts for all input with bad owner and repo name": {
			inEnvironments:      []string{},
			inGitHubOwner:       "",
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubBadOwner},
			inRepos:             []string{githubBadRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(pipelineGetGitHubOwnerPrompt, gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().Get(pipelineGetGitHubRepoPrompt, gomock.Any(), gomock.Any()).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: cha-os"), gomock.Any()).Return(githubToken, nil).Times(1)
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
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

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
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

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
		"returns error if fail to select GitHub owner name": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubOwnerPrompt, gomock.Any(), []string{githubOwner}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("get GitHub owner name: some error"),
		},
		"returns error if fail to get GitHub owner name": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubBadOwner},
			inRepos:             []string{githubBadRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(pipelineGetGitHubOwnerPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("get GitHub owner name: some error"),
		},
		"returns error if fail to select GitHub repos": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubOwnerPrompt, gomock.Any(), []string{githubOwner}).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName}).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("get GitHub repository: some error"),
		},
		"returns error if fail to get GitHub repos": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubBadOwner},
			inRepos:             []string{githubBadRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().Get(pipelineGetGitHubOwnerPrompt, gomock.Any(), gomock.Any()).Return(githubOwner, nil).Times(1)
				m.EXPECT().Get(pipelineGetGitHubRepoPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedGitHubRepo:        "",
			expectedGitHubAccessToken: "",
			expectedEnvironments:      []string{},
			expectedError:             fmt.Errorf("get GitHub repository: some error"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inGitHubRepo:        "",
			inGitHubAccessToken: "",
			inProjectEnvs:       []string{"test", "prod"},
			inOwners:            []string{githubOwner},
			inRepos:             []string{githubRepoName},

			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(pipelineAddEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().Confirm(pipelineAddMoreEnvPrompt, gomock.Any()).Return(true, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectEnvPrompt, gomock.Any(), []string{"prod"}).Return("prod", nil).Times(1)

				m.EXPECT().SelectOne(pipelineSelectGitHubOwnerPrompt, gomock.Any(), []string{githubOwner}).Return(githubOwner, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectGitHubRepoPrompt, gomock.Any(), []string{githubRepoName}).Return(githubRepoName, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository: cha-os"), gomock.Any()).Return("", errors.New("some error")).Times(1)
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
				owners:      tc.inOwners,
				repos:       tc.inRepos,

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

		expectedOwners []string
		expectedRepos  []string
	}{
		"matches format": {
			inRemoteResult: `efekarakus	git@github.com:efekarakus/grit.git (fetch)
efekarakus	https://github.com/karakuse/cli.git (fetch)
origin	https://github.com/koke/grit (fetch)
koke	git://github.com/koke/grit.git (push)`,

			expectedOwners: []string{"efekarakus", "karakuse", "koke"},
			expectedRepos:  []string{"grit", "cli"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &InitPipelineOpts{}

			// WHEN
			owners, repos := opts.parseGitRemoteResult(tc.inRemoteResult)

			// THEN
			require.ElementsMatch(t, tc.expectedOwners, owners)
			require.ElementsMatch(t, tc.expectedRepos, repos)
		})
	}
}
