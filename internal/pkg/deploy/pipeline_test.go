// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestPipelineSourceFromManifest(t *testing.T) {
	testCases := map[string]struct {
		mfSource             *manifest.Source
		expectedDeploySource interface{}
		expectedShouldPrompt bool
		expectedErr          error
	}{
		"transforms GitHubV1 source": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":              "test",
					"repository":          "some/repository/URL",
					"access_token_secret": "secretiveSecret",
				},
			},
			expectedDeploySource: &GitHubV1Source{
				ProviderName:                manifest.GithubV1ProviderName,
				Branch:                      "test",
				RepositoryURL:               "some/repository/URL",
				PersonalAccessTokenSecretID: "secretiveSecret",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"error out if using GitHubV1 while not having access token secret": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("missing `access_token_secret` in properties"),
		},
		"error out if a string property is not a string": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": []int{1, 2, 3},
				},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("property `repository` is not a string"),
		},
		"transforms GitHub (v2) source without existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &GitHubSource{
				ProviderName:  manifest.GithubProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: true,
			expectedErr:          nil,
		},
		"transforms GitHub (v2) source with existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"branch":         "test",
					"repository":     "some/repository/URL",
					"connection_arn": "barnARN",
				},
			},
			expectedDeploySource: &GitHubSource{
				ProviderName:  manifest.GithubProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
				ConnectionARN: "barnARN",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"transforms Bitbucket source without existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.BitbucketProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &BitbucketSource{
				ProviderName:  manifest.BitbucketProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: true,
			expectedErr:          nil,
		},
		"transforms Bitbucket source with existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.BitbucketProviderName,
				Properties: map[string]interface{}{
					"branch":         "test",
					"repository":     "some/repository/URL",
					"connection_arn": "yarnARN",
				},
			},
			expectedDeploySource: &BitbucketSource{
				ProviderName:  manifest.BitbucketProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
				ConnectionARN: "yarnARN",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"transforms CodeCommit source": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties: map[string]interface{}{
					"branch":                 "test",
					"repository":             "some/repository/URL",
					"output_artifact_format": "testFormat",
				},
			},
			expectedDeploySource: &CodeCommitSource{
				ProviderName:         manifest.CodeCommitProviderName,
				Branch:               "test",
				RepositoryURL:        "some/repository/URL",
				OutputArtifactFormat: "testFormat",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"use default branch `main` if branch is not configured": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties: map[string]interface{}{
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &CodeCommitSource{
				ProviderName:  manifest.CodeCommitProviderName,
				Branch:        "main",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"error out if repository is not configured": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties:   map[string]interface{}{},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("missing `repository` in properties"),
		},
		"errors if user changed provider name in manifest to unsupported source": {
			mfSource: &manifest.Source{
				ProviderName: "BitCommitHubBucket",
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: nil,
			expectedShouldPrompt: false,
			expectedErr:          errors.New("invalid repo source provider: BitCommitHubBucket"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			source, shouldPrompt, err := PipelineSourceFromManifest(tc.mfSource)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedDeploySource, source, "mismatched source")
				require.Equal(t, tc.expectedShouldPrompt, shouldPrompt, "mismatched bool for prompting")
			}
		})
	}
}

func TestPipelineBuild_Init(t *testing.T) {
	const (
		defaultImage   = "aws/codebuild/amazonlinux2-x86_64-standard:3.0"
		defaultEnvType = "LINUX_CONTAINER"
	)

	testCases := map[string]struct {
		mfBuild       *manifest.Build
		mfDirPath     string
		expectedBuild Build
	}{
		"set default image and env type if not specified in manifest; override default if buildspec path in manifest": {
			mfBuild: &manifest.Build{
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "some/path",
			},
		},
		"set image according to manifest": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/standard:3.0",
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           "aws/codebuild/standard:3.0",
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "some/path",
			},
		},
		"set image according to manifest (ARM based)": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				EnvironmentType: "ARM_CONTAINER",
				BuildspecPath:   "some/path",
			},
		},
		"by default convert legacy manifest path to buildspec path": {
			mfDirPath: "copilot/",
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "copilot/buildspec.yml",
			},
		},
		"by default convert non-legacy manifest path to buildspec path": {
			mfDirPath: "copilot/pipelines/my-pipeline/",
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "copilot/pipelines/my-pipeline/buildspec.yml",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var build Build
			build.Init(tc.mfBuild, tc.mfDirPath)
			require.Equal(t, tc.expectedBuild, build, "mismatched build")
		})
	}
}

func TestParseOwnerAndRepo(t *testing.T) {
	testCases := map[string]struct {
		src            *GitHubSource
		expectedErrMsg *string
		expectedOwner  string
		expectedRepo   string
	}{
		"missing repository property": {
			src: &GitHubSource{
				RepositoryURL: "",
				Branch:        "main",
			},
			expectedErrMsg: aws.String("unable to locate the repository"),
		},
		"valid GH repository property": {
			src: &GitHubSource{
				RepositoryURL: "chicken/wings",
			},
			expectedErrMsg: nil,
			expectedOwner:  "chicken",
			expectedRepo:   "wings",
		},
		"valid full GH repository name": {
			src: &GitHubSource{
				RepositoryURL: "https://github.com/badgoose/chaOS",
			},
			expectedErrMsg: nil,
			expectedOwner:  "badgoose",
			expectedRepo:   "chaOS",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			owner, repo, err := tc.src.RepositoryURL.parse()
			if tc.expectedErrMsg != nil {
				require.Contains(t, err.Error(), *tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedOwner, owner, "mismatched owner")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}

func TestPipelineStage_Init(t *testing.T) {
	var stg PipelineStage
	stg.Init(&config.Environment{
		Name:             "test",
		App:              "badgoose",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		ManagerRoleARN:   "arn:aws:iam::123456789012:role/badgoose-test-EnvManagerRole",
		ExecutionRoleARN: "arn:aws:iam::123456789012:role/badgoose-test-CFNExecutionRole",
	}, &manifest.PipelineStage{
		Name:             "test",
		RequiresApproval: true,
		TestCommands:     []string{"make test", "echo \"made test\""},
	}, []string{"frontend", "backend"})

	t.Run("stage name matches the environment's name", func(t *testing.T) {
		require.Equal(t, "test", stg.Name())
	})
	t.Run("stage region matches the environment's region", func(t *testing.T) {
		require.Equal(t, "us-west-2", stg.Region())
	})
	t.Run("stage env manager role ARN matches the environment's config", func(t *testing.T) {
		require.Equal(t, "arn:aws:iam::123456789012:role/badgoose-test-EnvManagerRole", stg.EnvManagerRoleARN())
	})
	t.Run("stage exec role ARN matches the environment's config", func(t *testing.T) {
		require.Equal(t, "arn:aws:iam::123456789012:role/badgoose-test-CFNExecutionRole", stg.ExecRoleARN())
	})
	t.Run("number of expected deployments match", func(t *testing.T) {
		require.Equal(t, 2, len(stg.Deployments()))
	})
	t.Run("stage test commands match manifest input", func(t *testing.T) {
		require.ElementsMatch(t, []string{"make test", `echo "made test"`}, stg.TestCommands())
	})
	t.Run("stage test commands order should come after deployments", func(t *testing.T) {
		require.Equal(t, 3, stg.TestCommandsOrder())
	})
	t.Run("manual approval button", func(t *testing.T) {
		require.NotNil(t, stg.Approval(), "should require approval action for stages when the manifest requires it")

		stg := PipelineStage{}
		require.Nil(t, stg.Approval(), "should return nil by default")
	})
}

func TestManualApprovalAction_Name(t *testing.T) {
	action := ManualApprovalAction{
		name: "test",
	}

	require.Equal(t, "ApprovePromotionTo-test", action.Name())
}

func TestManualApprovalAction_RunOrder(t *testing.T) {
	action := ManualApprovalAction{}

	require.Equal(t, 1, action.RunOrder(), "approval actions should always run first in the stage")
}

func TestWorkloadDeployAction_Name(t *testing.T) {
	action := WorkloadDeployAction{
		name:    "frontend",
		envName: "test",
	}

	require.Equal(t, "CreateOrUpdate-frontend-test", action.Name())
}

func TestWorkloadDeployAction_StackName(t *testing.T) {
	action := WorkloadDeployAction{
		name:    "frontend",
		envName: "test",
		appName: "phonetool",
	}

	require.Equal(t, "phonetool-test-frontend", action.StackName())
}

func TestWorkloadDeployAction_RunOrder(t *testing.T) {
	testCases := map[string]struct {
		in     WorkloadDeployAction
		wanted int
	}{
		"action has a preceding manual approval": {
			in: WorkloadDeployAction{
				action: action{
					prevActions: []orderedRunner{&ManualApprovalAction{}},
				},
				name: "frontend",
			},
			wanted: 2,
		},
		"action does not require a manual approval": {
			in: WorkloadDeployAction{
				name: "frontend",
			},
			wanted: 1,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.RunOrder())
		})
	}
}

func TestWorkloadDeployAction_TemplatePath(t *testing.T) {
	testCases := map[string]struct {
		in     WorkloadDeployAction
		wanted string
	}{
		"default location for workload templates": {
			in: WorkloadDeployAction{
				name:    "frontend",
				envName: "test",
			},
			wanted: "infrastructure/frontend-test.stack.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.TemplatePath())
		})
	}
}

func TestWorkloadDeployAction_TemplateConfigPath(t *testing.T) {
	testCases := map[string]struct {
		in     WorkloadDeployAction
		wanted string
	}{
		"default location for workload template configs": {
			in: WorkloadDeployAction{
				name:    "frontend",
				envName: "test",
			},
			wanted: "infrastructure/frontend-test.params.json",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.TemplateConfigPath())
		})
	}
}

func TestParseRepo(t *testing.T) {
	testCases := map[string]struct {
		src           *CodeCommitSource
		expectedErr   error
		expectedOwner string
		expectedRepo  string
	}{
		"missing repository property": {
			src: &CodeCommitSource{
				RepositoryURL: "",
			},
			expectedErr: errors.New("unable to locate the repository"),
		},
		"unable to parse repository name from URL": {
			src: &CodeCommitSource{
				RepositoryURL: "https://hahahaha.wrong.URL/repositories/wings/browse",
			},
			expectedErr:   errors.New("unable to parse the repository from the URL https://hahahaha.wrong.URL/repositories/wings/browse"),
			expectedOwner: "",
			expectedRepo:  "wings",
		},
		"valid full CC repository name": {
			src: &CodeCommitSource{
				RepositoryURL: "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/wings/browse",
			},
			expectedOwner: "",
			expectedRepo:  "wings",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			repo, err := tc.src.parseRepo()
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}
