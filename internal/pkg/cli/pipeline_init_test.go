// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

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
		inRepoURL     string
		inRepoBranch  string
		inEnvs        []string
		mockStore     func(m *mocks.Mockstore)
		mockRunner    func(m *mocks.Mockrunner)
		repoBuffer    bytes.Buffer
		branchBuffer  bytes.Buffer
		expectedError error
	}{
		"empty app name": {
			inAppName:     "",
			mockStore:     func(m *mocks.Mockstore) {},
			mockRunner:    func(m *mocks.Mockrunner) {},
			expectedError: errNoAppInWorkspace,
		},
		"invalid app name (not in workspace)": {
			inAppName: "ghost-app",
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("ghost-app").Return(nil, fmt.Errorf("get application ghost-app: some error"))
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},

			expectedError: fmt.Errorf("get application ghost-app: some error"),
		},
		"URL flag without branch flag": {
			inAppName: "my-app",
			inEnvs:    []string{"test", "prod"},
			inRepoURL: "https://github.com/badGoose/chaOS",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},

			expectedError: errors.New("must specify either both '--url' and '--git-branch' or neither"),
		},
		"branch flag without URL flag": {
			inAppName:    "my-app",
			inEnvs:       []string{"test", "prod"},
			inRepoBranch: "prod",

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},

			expectedError: errors.New("must specify either both '--url' and '--git-branch' or neither"),
		},
		"URL to unsupported repo provider": {
			inAppName:    "my-app",
			inRepoURL:    "unsupported.org/repositories/repoName",
			inRepoBranch: "main",
			inEnvs:       []string{"test"},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},

			expectedError: errors.New("must be a URL to a supported provider (GitHub, CodeCommit, Bitbucket)"),
		},
		"URL not a local git remote": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/wrongRepo (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/main"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},

			expectedError: errors.New("URL 'https://github.com/badGoose/chaOS' is not a local git remote"),
		},
		"errors if git remote results empty": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString(""),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/main"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			expectedError: errors.New("URL 'https://github.com/badGoose/chaOS' is not a local git remote"),
		},
		"error fetching URLs": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/wrongRepo (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/main"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error")).Times(1)
			},
			expectedError: errors.New("get Git remote repository info: some error"),
		},
		"branch doesn't exist in passed in repo": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/chaOS (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/prod"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},

			expectedError: errors.New("branch 'main' not found for repo 'archer'"),
		},
		"error fetching branches of given repo": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/chaOS (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/prod"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error")).Times(1)
			},

			expectedError: fmt.Errorf("get Git repo branch info: %s", "some error"),
		},
		"error parsing git branch results": {
			inAppName:    "my-app",
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/chaOS (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archerdev\nremotes/archer/prod"),

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},

			expectedError: fmt.Errorf("parse 'git branch' results: %s", "cannot parse branch name from 'remotes/archerdev'"),
		},
		"invalid environments": {
			inAppName: "my-app",
			inEnvs:    []string{"test", "prod"},

			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},
			mockRunner: func(m *mocks.Mockrunner) {},

			expectedError: errors.New("some error"),
		},
		"success with GH repo": {
			inAppName:    "my-app",
			inEnvs:       []string{"test", "prod"},
			inRepoURL:    "https://github.com/badGoose/chaOS",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://github.com/badGoose/chaOS (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/main"),

			mockStore: func(m *mocks.Mockstore) {
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
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			expectedError: nil,
		},
		"success with CC repo": {
			inAppName:    "my-app",
			inEnvs:       []string{"test", "prod"},
			inRepoURL:    "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("archer\thttps://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/main"),

			mockStore: func(m *mocks.Mockstore) {
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
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},

			expectedError: nil,
		},
		"success with Bitbucket repo": {
			inAppName:    "my-app",
			inEnvs:       []string{"test", "prod"},
			inRepoURL:    "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service.git",
			inRepoBranch: "main",
			repoBuffer:   *bytes.NewBufferString("bb\thttps://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service.git (fetch)\n"),
			branchBuffer: *bytes.NewBufferString("remotes/bb/dev\nremotes/bb/main"),

			mockStore: func(m *mocks.Mockstore) {
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
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
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
			mockRunner := mocks.NewMockrunner(ctrl)

			tc.mockStore(mockStore)
			tc.mockRunner(mockRunner)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:      tc.inAppName,
					repoURL:      tc.inRepoURL,
					repoBranch:   tc.inRepoBranch,
					environments: tc.inEnvs,
				},
				store:        mockStore,
				runner:       mockRunner,
				repoBuffer:   tc.repoBuffer,
				branchBuffer: tc.branchBuffer,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_Ask(t *testing.T) {
	githubOwner := "goodGoose"
	githubRepoName := "bhaOS"
	githubURL := "archer: git@github.com:goodGoose/bhaOS.git"
	githubReallyBadURL := "archer: reallybadGoosegithub.comNotEvenAURL"
	githubToken := "hunter2"
	codecommitRepoName := "repo-man"
	codecommitAnotherRepoName := "repo-woman"
	codecommitHTTPSURL := "archer: https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man"
	codecommitSSHURL := "archer: ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-woman"
	codecommitFedURL := "archer: codecommit::us-west-2://repo-man"
	codecommitShortURL := "archer: codecommit://repo-man"
	codecommitBadURL := "archer: git-codecommitus-west-2amazonaws.com"
	codecommitBadRegion := "archer: codecommit::us-mess-2://repo-man"
	codecommitRegion := "us-west-2"

	testCases := map[string]struct {
		inEnvironments      []string
		inRepoURL           string
		inGitHubAccessToken string
		inGitBranch         string

		mockPrompt       func(m *mocks.Mockprompter)
		mockRunner       func(m *mocks.Mockrunner)
		mockSessProvider func(m *mocks.MocksessionProvider)
		mockSelector     func(m *mocks.MockpipelineSelector)
		mockStore        func(m *mocks.Mockstore)
		repoBuffer       bytes.Buffer
		branchBuffer     bytes.Buffer

		expectedEnvironments      []string
		expectedRepoURL           string
		expectedRepoName          string
		expectedRepoBranch        string
		expectedGitHubOwner       string
		expectedGitHubAccessToken string
		expectedCodeCommitRegion  string
		expectedError             error
	}{
		"no flags, prompts for all input, success case for GitHub": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: githubToken,
			inGitBranch:         "",

			repoBuffer:   *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n remotes/archer/test\nremotes/archer/prod"),
			branchBuffer: *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/prod"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubURL, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoURL:           githubURL,
			expectedGitHubOwner:       githubOwner,
			expectedRepoName:          githubRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedRepoBranch:        "dev",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"no flags, success case for CodeCommit": {
			inEnvironments: []string{},
			inRepoURL:      "",
			repoBuffer:     *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\thttps://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man (fetch)\narcher\tssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-woman (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),
			branchBuffer:   *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/prod"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(codecommitSSHURL, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(1)

			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},

			expectedRepoURL:          codecommitSSHURL,
			expectedRepoName:         codecommitAnotherRepoName,
			expectedRepoBranch:       "dev",
			expectedCodeCommitRegion: codecommitRegion,
			expectedEnvironments:     []string{"test", "prod"},
			expectedError:            nil,
		},
		"returns error if fail to list environments": {
			inEnvironments: []string{},

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockStore:        func(m *mocks.Mockstore) {},
			mockRunner:       func(m *mocks.Mockrunner) {},
			mockPrompt:       func(m *mocks.Mockprompter) {},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("select environments: some error"),
		},

		"returns error if fail to select URL": {
			inRepoURL:      "",
			inEnvironments: []string{},
			repoBuffer:     *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("select URL: some error"),
		},
		"returns error if fail to select branch": {
			inRepoURL:      "",
			inEnvironments: []string{},
			repoBuffer:     *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),
			branchBuffer:   *bytes.NewBufferString("remotes/archer/dev\nremotes/archer/prod"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubURL, nil).Times(1)
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)

			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("select branch: some error"),
		},
		"returns error if select invalid URL": {
			inRepoURL:      "",
			inEnvironments: []string{},
			repoBuffer:     *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://bitbub.com/badGoose/chaOS (push)\n"),
			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("archer: https://bitbub.com/badGoose/chaOS", nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedError: fmt.Errorf("must be a URL to a supported provider (GitHub, CodeCommit, Bitbucket)"),
		},
		"returns error if fail to parse GitHub URL": {
			repoBuffer: *bytes.NewBufferString("archer\treallybadGoosegithub.comNotEvenAURL (fetch)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), []string{githubReallyBadURL}, gomock.Any()).Return(githubReallyBadURL, nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("unable to parse the GitHub repository owner and name from reallybadGoosegithub.comNotEvenAURL: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error if fail to parse repo name out of CodeCommit URL": {
			inEnvironments:      []string{},
			inGitHubAccessToken: "",
			repoBuffer:          *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(codecommitBadURL, nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoName:     "",
			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("unknown CodeCommit URL format: git-codecommitus-west-2amazonaws.com"),
		},
		"returns error if fail to parse region out of CodeCommit URL": {
			repoBuffer: *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(codecommitBadRegion, nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoURL:      codecommitHTTPSURL,
			expectedRepoName:     codecommitRepoName,
			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("unable to parse the AWS region from codecommit::us-mess-2://repo-man"),
		},
		"returns error if fail to retrieve default session": {
			repoBuffer: *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-east-1",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(codecommitShortURL, nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(nil, errors.New("some error"))
			},

			expectedRepoName:     "",
			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("retrieve default session: some error"),
		},
		"returns error if repo region is not app's region": {
			repoBuffer: *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-east-1",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(codecommitFedURL, nil).Times(1)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-east-1"),
					},
				}, nil)
			},

			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("repository repo-man is in us-west-2, but app my-app is in us-east-1; they must be in the same region"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)
			mocksSessProvider := mocks.NewMocksessionProvider(ctrl)
			mockSelector := mocks.NewMockpipelineSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           "my-app",
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
				},
				prompt:       mockPrompt,
				runner:       mockRunner,
				sessProvider: mocksSessProvider,
				repoBuffer:   tc.repoBuffer,
				branchBuffer: tc.branchBuffer,
				sel:          mockSelector,
				store:        mockStore,
			}

			tc.mockPrompt(mockPrompt)
			tc.mockRunner(mockRunner)
			tc.mockSessProvider(mocksSessProvider)
			tc.mockSelector(mockSelector)
			tc.mockStore(mockStore)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedRepoName, opts.repoName)
				require.Equal(t, tc.expectedGitHubOwner, opts.repoOwner)
				require.Equal(t, tc.expectedGitHubAccessToken, opts.githubAccessToken)
				require.Equal(t, tc.expectedCodeCommitRegion, opts.ccRegion)
				require.ElementsMatch(t, tc.expectedEnvironments, opts.environments)
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	buildspecExistsErr := &workspace.ErrFileExists{FileName: "/buildspec.yml"}
	manifestExistsErr := &workspace.ErrFileExists{FileName: "/pipeline.yml"}
	testCases := map[string]struct {
		inProvider     string
		inEnvironments []string
		inEnvConfigs   []*config.Environment
		inGitHubToken  string
		inRepoName     string
		inBranch       string
		inAppName      string

		mockSecretsManager          func(m *mocks.MocksecretsManager)
		mockWsWriter                func(m *mocks.MockwsPipelineWriter)
		mockParser                  func(m *templatemocks.MockParser)
		mockFileSystem              func(mockFS afero.Fs)
		mockRegionalResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockStoreSvc                func(m *mocks.Mockstore)

		expectedError error
	}{
		"creates secret and writes manifest and buildspec for GHV1 provider": {
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
		"writes manifest and buildspec for GH(v2) provider": {
			inProvider: "CodeCommit",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inRepoName: "goose",
			inBranch:   "main",
			inAppName:  "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
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
		"writes manifest and buildspec for CC provider": {
			inProvider: "CodeCommit",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inRepoName: "goose",
			inBranch:   "main",
			inAppName:  "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
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
		"writes manifest and buildspec for BB provider": {
			inProvider: "Bitbucket",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inRepoName: "goose",
			inBranch:   "dev",
			inAppName:  "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
			inProvider: "GitHubV1",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

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
					githubAccessToken: tc.inGitHubToken,
					appName:           tc.inAppName,
				},

				secretsmanager: mockSecretsManager,
				cfnClient:      mockRegionalResourcesGetter,
				store:          mockstore,
				workspace:      mockWriter,
				parser:         mockParser,
				fs:             memFs,
				envConfigs:     tc.inEnvConfigs,
				provider:       tc.inProvider,
				repoName:       tc.inRepoName,
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

func TestInitPipelineGHRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL ghRepoURL

		expectedDetails ghRepoDetails
		expectedError   error
	}{
		"successfully parses name without .git suffix": {
			inRepoURL: "https://github.com/badgoose/cli",

			expectedDetails: ghRepoDetails{
				name:  "cli",
				owner: "badgoose",
			},
			expectedError: nil,
		},
		"successfully parses repo name with .git suffix": {
			inRepoURL: "https://github.com/koke/grit.git",

			expectedDetails: ghRepoDetails{
				name:  "grit",
				owner: "koke",
			},
			expectedError: nil,
		},
		"returns an error if it is not a github URL": {
			inRepoURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever",

			expectedError: fmt.Errorf("unable to parse the GitHub repository owner and name from https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := ghRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

func TestInitPipelineCCRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL ccRepoURL

		expectedDetails ccRepoDetails
		expectedError   error
	}{
		"successfully parses https url": {
			inRepoURL: "https://git-codecommit.sa-east-1.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "sa-east-1",
			},
			expectedError: nil,
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-east-2",
			},
			expectedError: nil,
		},
		"successfully parses federated (GRC) url": {
			inRepoURL: "codecommit::us-gov-west-1://aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-gov-west-1",
			},
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := ccRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

func TestInitPipelineBBRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL bbRepoURL

		expectedDetails bbRepoDetails
		expectedError   error
	}{
		"successfully parses https url": {
			inRepoURL: "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
			expectedError: nil,
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git@bitbucket.org:huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := bbRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

func TestInitPipelineOpts_askBranch(t *testing.T) {
	testCases := map[string]struct {
		inBranchBuffer bytes.Buffer
		inBranchFlag   string

		mockRunner func(m *mocks.Mockrunner)
		mockPrompt func(m *mocks.Mockprompter)

		expectedErr error
	}{
		"returns nil if branch passed in with flag": {
			inBranchFlag:   "myBranch",
			inBranchBuffer: *bytes.NewBufferString(""),
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(0)
			},

			expectedErr: nil,
		},
		"returns nil if no error": {
			inBranchBuffer: *bytes.NewBufferString("remotes/mockRepo/main\n  remotes/mockRepo/dev"),
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(1)
			},

			expectedErr: nil,
		},
		"returns err if can't get branch info from `git branch -a -l mockRepo/*`": {
			inBranchBuffer: *bytes.NewBufferString("\"remotes/mockRepo/main\n  remotes/mockRepo/dev\""),
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(0)
			},

			expectedErr: errors.New("get Git repo branch info: some error"),
		},
		"errors if unsuccessful in parsing git branch results": {
			inBranchBuffer: *bytes.NewBufferString("badResults"),
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("dev", nil).Times(0)
			},
			expectedErr: errors.New("parse 'git branch' results: cannot parse branch name from 'badResults'"),
		},
		"errors if unsuccessful selecting branch": {
			inBranchBuffer: *bytes.NewBufferString("remotes/mockRepo/main\n  remotes/mockRepo/dev"),
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectBranchPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			expectedErr: errors.New("select branch: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRunner := mocks.NewMockrunner(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					repoBranch: tc.inBranchFlag,
				},
				runner:       mockRunner,
				prompt:       mockPrompt,
				branchBuffer: tc.inBranchBuffer,
				repoName:     "mockRepo",
			}

			tc.mockRunner(mockRunner)
			tc.mockPrompt(mockPrompt)

			// WHEN
			err := opts.askBranch()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestInitPipelineOpts_parseGitBranchResults(t *testing.T) {
	testCases := map[string]struct {
		inGitBranchResults string

		expectedBranches []string
		expectedErr      error
	}{
		"successfully returns branch names for selector given `git branch -a -l [repoName]` results": {
			inGitBranchResults: "  remotes/cc/main\n  remotes/cc/dev",
			expectedBranches:   []string{"main", "dev"},
		},
		"success with just one branch": {
			inGitBranchResults: "remotes/origin/mainline",
			expectedBranches:   []string{"mainline"},
		},
		"errors if no branches": {
			inGitBranchResults: "",
			expectedErr:        errors.New("cannot parse branch name from ''"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			actual, err := opts.parseGitBranchResults(tc.inGitBranchResults)

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expectedBranches, actual)
			}
		})
	}
}

func TestInitPipelineOpts_pipelineName(t *testing.T) {
	testCases := map[string]struct {
		inRepoName string
		inAppName  string

		expected    string
		expectedErr error
	}{
		"generates pipeline name": {
			inAppName:  "goodmoose",
			inRepoName: "repo-man",

			expected: "pipeline-goodmoose-repo-man",
		},
		"generates and truncates pipeline name if it exceeds 100 characters": {
			inAppName:  "goodmoose01234567820123456783012345678401234567850",
			inRepoName: "repo-man101234567820123456783012345678401234567850",

			expected: "pipeline-goodmoose01234567820123456783012345678401234567850-repo-man10123456782012345678301234567840",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName: tc.inAppName,
				},
				repoName: tc.inRepoName,
			}

			// WHEN
			actual := opts.pipelineName()

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}
