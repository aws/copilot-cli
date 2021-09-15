// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"testing"

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

func TestPipelineBuildFromManifest(t *testing.T) {
	const defaultImage = "aws/codebuild/amazonlinux2-x86_64-standard:3.0"

	testCases := map[string]struct {
		mfBuild       *manifest.Build
		expectedBuild *Build
	}{
		"set default image if not be specified in manifest": {
			mfBuild: nil,
			expectedBuild: &Build{
				Image: defaultImage,
			},
		},
		"set image according to manifest": {
			mfBuild: &manifest.Build{
				Image: "aws/codebuild/standard:3.0",
			},
			expectedBuild: &Build{
				Image: "aws/codebuild/standard:3.0",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			build := PipelineBuildFromManifest(tc.mfBuild)
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

func TestParseRepo(t *testing.T) {
	testCases := map[string]struct {
		src            *CodeCommitSource
		expectedErrMsg *string
		expectedOwner  string
		expectedRepo   string
	}{
		"missing repository property": {
			src: &CodeCommitSource{
				RepositoryURL: "",
			},
			expectedErrMsg: aws.String("unable to locate the repository"),
		},
		"valid full CC repository name": {
			src: &CodeCommitSource{
				RepositoryURL: "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/wings/browse",
			},
			expectedErrMsg: nil,
			expectedOwner:  "",
			expectedRepo:   "wings",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			repo, err := tc.src.parseRepo()
			if tc.expectedErrMsg != nil {
				require.Contains(t, err.Error(), *tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}
