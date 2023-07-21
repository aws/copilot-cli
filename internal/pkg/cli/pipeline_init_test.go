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
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	templatemocks "github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineInitMocks struct {
	workspace      *mocks.MockwsPipelineIniter
	secretsmanager *mocks.MocksecretsManager
	parser         *templatemocks.MockParser
	runner         *mocks.MockexecRunner
	sessProvider   *mocks.MocksessionProvider
	cfnClient      *mocks.MockappResourcesGetter
	store          *mocks.Mockstore
	prompt         *mocks.Mockprompter
	sel            *mocks.MockpipelineEnvSelector
	pipelineLister *mocks.MockdeployedPipelineLister
}

func TestInitPipelineOpts_Ask(t *testing.T) {
	const (
		mockAppName = "my-app"
		wantedName  = "mypipe"
	)
	mockError := errors.New("some error")
	mockApp := &config.Application{
		Name: mockAppName,
	}

	githubAnotherURL := "git@github.com:goodGoose/bhaOS.git"
	githubToken := "hunter2"
	testCases := map[string]struct {
		inName              string
		inAppName           string
		inWsAppName         string
		inEnvironments      []string
		inRepoURL           string
		inGitHubAccessToken string
		inGitBranch         string
		inType              string

		setupMocks func(m pipelineInitMocks)
		buffer     bytes.Buffer

		expectedBranch string
		expectedError  error
	}{
		"empty workspace app name": {
			inWsAppName: "",

			expectedError: errNoAppInWorkspace,
		},
		"invalid app name (not in workspace)": {
			inWsAppName: "diff-app",
			inAppName:   "ghost-app",

			expectedError: errors.New("cannot specify app ghost-app because the workspace is already registered with app diff-app"),
		},
		"invalid app name": {
			inWsAppName: "ghost-app",
			inAppName:   "ghost-app",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication("ghost-app").Return(nil, mockError)
			},
			expectedError: fmt.Errorf("get application ghost-app configuration: some error"),
		},
		"returns error when repository URL is not from a supported git provider": {
			inWsAppName: mockAppName,
			inRepoURL:   "https://gitlab.company.com/group/project.git",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New("repository https://gitlab.company.com/group/project.git must be from a supported provider: GitHub, CodeCommit or Bitbucket"),
		},
		"returns error when GitHub repository URL is of unknown format": {
			inWsAppName: mockAppName,
			inRepoURL:   "thisisnotevenagithub.comrepository",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New("unable to parse the GitHub repository owner and name from thisisnotevenagithub.comrepository: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error when CodeCommit repository URL is of unknown format": {
			inWsAppName: mockAppName,
			inRepoURL:   "git-codecommitus-west-2amazonaws.com",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New("unknown CodeCommit URL format: git-codecommitus-west-2amazonaws.com"),
		},
		"returns error when CodeCommit repository contains unknown region": {
			inWsAppName: mockAppName,
			inRepoURL:   "codecommit::us-mess-2://repo-man",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New("unable to parse the AWS region from codecommit::us-mess-2://repo-man"),
		},
		"returns error when CodeCommit repository region does not match pipeline's region": {
			inWsAppName: mockAppName,
			inRepoURL:   "codecommit::us-west-2://repo-man",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.sessProvider.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-east-1"),
					},
				}, nil)
			},
			expectedError: errors.New("repository repo-man is in us-west-2, but app my-app is in us-east-1; they must be in the same region"),
		},
		"returns error when Bitbucket repository URL is of unknown format": {
			inWsAppName: mockAppName,
			inRepoURL:   "bitbucket.org",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New("unable to parse the Bitbucket repository name from bitbucket.org"),
		},
		"successfully detects local branch and sets it": {
			inWsAppName:    mockAppName,
			inRepoURL:      "git@github.com:badgoose/goose.git",
			inEnvironments: []string{"test"},
			inName:         wantedName,
			buffer:         *bytes.NewBufferString("devBranch"),
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
			expectedBranch: "devBranch",
			expectedError:  nil,
		},
		"sets 'main' as branch name if error fetching it": {
			inWsAppName:    mockAppName,
			inRepoURL:      "git@github.com:badgoose/goose.git",
			inEnvironments: []string{"test"},
			inName:         wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
			expectedBranch: "main",
			expectedError:  nil,
		},
		"invalid pipeline name": {
			inWsAppName: mockAppName,
			inName:      "1234",
			inRepoURL:   githubAnotherURL,
			inGitBranch: "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication("my-app").Return(mockApp, nil)
			},

			expectedError: fmt.Errorf("pipeline name 1234 is invalid: %w", errBasicNameRegexNotMatched),
		},
		"returns an error if fail to get pipeline name": {
			inWsAppName: mockAppName,
			inRepoURL:   githubAnotherURL,
			inGitBranch: "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("mock error"))
			},

			expectedError: fmt.Errorf("get pipeline name: mock error"),
		},
		"invalid pipeline type": {
			inWsAppName: mockAppName,
			inRepoURL:   githubAnotherURL,
			inGitBranch: "main",
			inName:      "mock-pipeline",
			inType:      "RandomType",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
			},
			expectedError: errors.New(`invalid pipeline type "RandomType"; must be one of "Workloads" or "Environments"`),
		},
		"returns an error if fail to get pipeline type": {
			inWsAppName: mockAppName,
			inRepoURL:   githubAnotherURL,
			inGitBranch: "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().Get(gomock.Eq("What would you like to name this pipeline?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedName, nil)
				m.prompt.EXPECT().SelectOption(gomock.Eq("What type of continuous delivery pipeline is this?"), gomock.Any(), gomock.Any()).
					Return("", errors.New("mock error"))
			},

			expectedError: fmt.Errorf("prompt for pipeline type: mock error"),
		},
		"prompt for pipeline name": {
			inWsAppName:    mockAppName,
			inRepoURL:      githubAnotherURL,
			inGitBranch:    "main",
			inEnvironments: []string{"prod"},
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.store.EXPECT().GetEnvironment(mockAppName, "prod").
					Return(&config.Environment{Name: "prod"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
				m.prompt.EXPECT().Get(gomock.Eq("What would you like to name this pipeline?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedName, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
		},
		"passed-in URL to unsupported repo provider": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "unsupported.org/repositories/repoName",
			inEnvironments: []string{"test"},
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},

			expectedError: errors.New("repository unsupported.org/repositories/repoName must be from a supported provider: GitHub, CodeCommit or Bitbucket"),
		},
		"passed-in invalid environments": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "https://github.com/badGoose/chaOS",
			inEnvironments: []string{"test", "prod"},
			inGitBranch:    "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(nil, mockError)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},

			expectedError: errors.New("validate environment test: some error"),
		},
		"success with GH repo with env and repoURL flags": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inEnvironments: []string{"test", "prod"},
			inRepoURL:      "https://github.com/badGoose/chaOS",
			inGitBranch:    "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
		},
		"success with CC repo with env and repoURL flags": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inEnvironments: []string{"test", "prod"},
			inRepoURL:      "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man",
			inGitBranch:    "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.sessProvider.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
		},
		"no flags, prompts for all input, success case for selecting URL": {
			inWsAppName:         mockAppName,
			inGitHubAccessToken: githubToken,
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
				m.prompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(wantedName, nil)
				m.prompt.EXPECT().SelectOption("What type of continuous delivery pipeline is this?", gomock.Any(), gomock.Any()).Return(pipelineTypeEnvironments, nil)
				m.prompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
				m.sel.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
		},
		"returns error if fail to list environments": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			inRepoURL:   githubAnotherURL,
			inGitBranch: "main",
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return(nil, nil)
				m.sel.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return(nil, errors.New("some error"))
			},

			expectedError: fmt.Errorf("select environments: some error"),
		},
		"returns error if fail to select URL": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.prompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("", mockError).Times(1)
			},

			expectedError: fmt.Errorf("select URL: some error"),
		},
		"returns error if fail to get env config": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      githubAnotherURL,
			inGitBranch:    "main",
			inEnvironments: []string{},
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return(nil, nil)
				m.sel.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "prod").Return(nil, errors.New("some error"))
			},

			expectedError: fmt.Errorf("validate environment prod: some error"),
		},
		"skip selector prompt if only one repo URL": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			inGitBranch: "main",
			buffer:      *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.prompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				m.runner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.workspace.EXPECT().ListPipelines().Return(nil, nil)
				m.sel.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.store.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := pipelineInitMocks{
				prompt:         mocks.NewMockprompter(ctrl),
				runner:         mocks.NewMockexecRunner(ctrl),
				sessProvider:   mocks.NewMocksessionProvider(ctrl),
				sel:            mocks.NewMockpipelineEnvSelector(ctrl),
				store:          mocks.NewMockstore(ctrl),
				pipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
				workspace:      mocks.NewMockwsPipelineIniter(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           tc.inAppName,
					name:              tc.inName,
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
					repoBranch:        tc.inGitBranch,
					pipelineType:      tc.inType,
				},
				wsAppName:      tc.inWsAppName,
				prompt:         mocks.prompt,
				runner:         mocks.runner,
				sessProvider:   mocks.sessProvider,
				buffer:         tc.buffer,
				sel:            mocks.sel,
				store:          mocks.store,
				pipelineLister: mocks.pipelineLister,
				workspace:      mocks.workspace,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				if tc.expectedBranch != "" {
					require.Equal(t, tc.expectedBranch, opts.repoBranch)
				}
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	const (
		wantedName             = "mypipe"
		wantedManifestFile     = "/pipelines/mypipe/manifest.yml"
		wantedManifestRelPath  = "/copilot/pipelines/mypipe/manifest.yml"
		wantedBuildspecFile    = "/pipelines/mypipe/buildspec.yml"
		wantedBuildspecRelPath = "/copilot/pipelines/mypipe/buildspec.yml"
	)

	buildspecExistsErr := &workspace.ErrFileExists{FileName: wantedBuildspecFile}
	manifestExistsErr := &workspace.ErrFileExists{FileName: wantedManifestFile}
	testCases := map[string]struct {
		inName         string
		inEnvironments []string
		inEnvConfigs   []*config.Environment
		inGitHubToken  string
		inRepoURL      string
		inBranch       string
		inAppName      string
		inType         string

		setupMocks func(m pipelineInitMocks)
		buffer     bytes.Buffer

		expectedError error
	}{
		"creates secret and writes manifest and buildspec for GHV1 provider": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
		"writes workloads pipeline manifest and buildspec for GH(v2) provider": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "git@github.com:badgoose/goose.git",
			inAppName: "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
		"writes workloads pipeline manifest and buildspec for CC provider": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/goose",
			inAppName: "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
				m.sessProvider.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},
			expectedError: nil,
		},
		"writes workloads pipeline manifest and buildspec for BB provider": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "https://huanjani@bitbucket.org/badgoose/goose.git",
			inAppName: "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
		"writes environments pipeline manifest for GH(v2) provider": {
			inName: wantedName,
			inType: pipelineTypeEnvironments,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "git@github.com:badgoose/goose.git",
			inAppName: "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(environmentsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return("", errors.New("some error"))
			},
			expectedError: errors.New("write pipeline manifest to workspace: some error"),
		},
		"returns an error if application cannot be retrieved": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(nil, errors.New("some error"))
			},
			expectedError: errors.New("get application badgoose: some error"),
		},
		"returns an error if can't get regional application resources": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return(nil, errors.New("some error"))
			},
			expectedError: fmt.Errorf("get regional application resources: some error"),
		},
		"returns an error if buildspec cannot be parsed": {
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Times(0)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return("", manifestExistsErr)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return("", buildspecExistsErr)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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
			inName: wantedName,
			inType: pipelineTypeWorkloads,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",
			setupMocks: func(m pipelineInitMocks) {
				m.secretsmanager.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
				m.workspace.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.workspace.EXPECT().Rel(wantedManifestFile).Return(wantedManifestRelPath, nil)
				m.workspace.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return("", errors.New("some error"))
				m.parser.EXPECT().Parse(workloadsPipelineBuildspecTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
				m.store.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
				m.cfnClient.EXPECT().GetRegionalAppResources(&config.Application{
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

			mocks := pipelineInitMocks{
				workspace:      mocks.NewMockwsPipelineIniter(ctrl),
				secretsmanager: mocks.NewMocksecretsManager(ctrl),
				parser:         templatemocks.NewMockParser(ctrl),
				sessProvider:   mocks.NewMocksessionProvider(ctrl),
				cfnClient:      mocks.NewMockappResourcesGetter(ctrl),
				store:          mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					name:              tc.inName,
					githubAccessToken: tc.inGitHubToken,
					appName:           tc.inAppName,
					repoBranch:        tc.inBranch,
					repoURL:           tc.inRepoURL,
					pipelineType:      tc.inType,
				},
				workspace:      mocks.workspace,
				secretsmanager: mocks.secretsmanager,
				parser:         mocks.parser,
				sessProvider:   mocks.sessProvider,
				store:          mocks.store,
				cfnClient:      mocks.cfnClient,
				buffer:         tc.buffer,
				envConfigs:     tc.inEnvConfigs,
			}

			// WHEN
			require.NoError(t, opts.parseRepoDetails())
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
koke	git://github.com/koke/grit.git (push)
https	https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (fetch)
fed	codecommit::us-west-2://aws-sample (fetch)
ssh	ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (push)
bb	https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service.git (push)`,

			expectedURLs: []string{"git@github.com:badgoose/grit", "https://github.com/badgoose/cli", "https://github.com/koke/grit", "git://github.com/koke/grit", "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "codecommit::us-west-2://aws-sample", "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service"},
		},
		"don't add to URL list if it is not a GitHub or CodeCommit or Bitbucket URL": {
			inRemoteResult: `badgoose	verybad@gitlab.com/whatever (fetch)`,

			expectedURLs: []string{},
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
		},
		"successfully parses repo name with .git suffix": {
			inRepoURL: "https://github.com/koke/grit.git",

			expectedDetails: ghRepoDetails{
				name:  "grit",
				owner: "koke",
			},
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
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-east-2",
			},
		},
		"successfully parses federated (GRC) url": {
			inRepoURL: "codecommit::us-gov-west-1://aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-gov-west-1",
			},
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
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git@bitbucket.org:huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
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
