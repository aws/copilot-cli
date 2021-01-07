// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines pipeline deployment resources.
package deploy

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

// NOTE: this is duplicated from validate.go
var ghRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

// NOTE: 'region' is not currently parsed out as a Source property, but this enables that possibility.
var ccRepoExp = regexp.MustCompile(`(https:\/\/(?P<region>.+)(.console.aws.amazon.com\/codesuite\/codecommit\/repositories\/)(?P<repo>.+)(\/browse))`)

const (
	fmtInvalidRepo = "unable to locate the repository URL from the properties: %+v"
)

// CreatePipelineInput represents the fields required to deploy a pipeline.
type CreatePipelineInput struct {
	// Name of the application this pipeline belongs to
	AppName string

	// Name of the pipeline
	Name string

	// The source code provider for this pipeline
	Source *Source

	// The stages of the pipeline. The order of stages in this list
	// will be the order we deploy to.
	Stages []PipelineStage

	// A list of artifact buckets and corresponding KMS keys that will
	// be used in this pipeline.
	ArtifactBuckets []ArtifactBucket

	// AdditionalTags are labels applied to resources under the application.
	AdditionalTags map[string]string
}

// ArtifactBucket represents an S3 bucket used by the CodePipeline to store
// intermediate artifacts produced by the pipeline.
type ArtifactBucket struct {
	// The name of the S3 bucket.
	BucketName string

	// The ARN of the KMS key used to en/decrypt artifacts stored in this bucket.
	KeyArn string
}

// Region parses out the region from the ARN of the KMS key associated with
// the artifact bucket.
func (a *ArtifactBucket) Region() (string, error) {
	// We assume the bucket and the key are in the same AWS region.
	parsedArn, err := arn.Parse(a.KeyArn)
	if err != nil {
		return "", fmt.Errorf("failed to parse region out of key ARN: %s, error: %w",
			a.BucketName, err)
	}
	return parsedArn.Region, nil
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	// The name of the source code provider. For example, "GitHub"
	ProviderName string

	// Contains provider-specific configurations, such as:
	// "repository": "aws/amazon-ecs-cli-v2"
	// "githubPersonalAccessTokenSecretId": "heyyo"
	Properties map[string]interface{}
}

// GitHubPersonalAccessTokenSecretID returns the ID of the secret in the
// Secrets manager, which stores the GitHub Personal Access token if the
// provider is "GitHub". Otherwise, it returns the detected provider.
func (s *Source) GitHubPersonalAccessTokenSecretID() (string, error) {
	id := ""
	var ok bool
	if s.ProviderName == manifest.GithubProviderName {
		secretID, exists := s.Properties[manifest.GithubSecretIdKeyName]
		if !exists {
			return "", errors.New("the GitHub token secretID is not configured")
		}

		id, ok = secretID.(string)
		if !ok {
			return "", fmt.Errorf("unable to locate the GitHub token secretID from %v", secretID)
		}
	}

	return id, nil
}

// parseOwnerAndRepo parses the owner (if GitHub is the provider) and repo name from the repo URL, which was formatted and assigned in cli/pipeline_init.go.
func (s *Source) parseOwnerAndRepo(provider string) (string, string, error) {
	var (
		// NOTE: this is duplicated from validate.go
		// Ex: https://github.com/koke/grit
		ghRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)
		// NOTE: 'region' is not currently parsed out as a Source property, but this enables that possibility.
		// Ex: https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample/browse
		ccRepoExp = regexp.MustCompile(`(https:\/\/(?P<region>.+)(.console.aws.amazon.com\/codesuite\/codecommit\/repositories\/)(?P<repo>.+)(\/browse))`)
	)
	var repoExp *regexp.Regexp
	if provider == manifest.GithubProviderName {
		repoExp = ghRepoExp
	}
	if provider == manifest.CodeCommitProviderName {
		repoExp = ccRepoExp
	}
	url, exists := s.Properties["repository"]
	if !exists {
		return "", "", fmt.Errorf("unable to locate the repository from the properties: %+v", s.Properties)
	}
	urlStr, ok := url.(string)
	if !ok {
		return "", "", fmt.Errorf(fmtInvalidRepo, url)
	}

	match := repoExp.FindStringSubmatch(urlStr)
	if len(match) == 0 {
		return "", "", fmt.Errorf(fmtInvalidRepo, urlStr)
	}

	matches := make(map[string]string)
	for i, name := range repoExp.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}
	owner := ""
	if provider == manifest.GithubProviderName {
		owner = matches["owner"]
	}
	return owner, matches["repo"], nil
}

// Repository returns the repository portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "amazon-ecs-cli-v2".
func (s *Source) Repository() (string, error) {
	if s.ProviderName != manifest.GithubProviderName && s.ProviderName != manifest.CodeCommitProviderName {
		return "", fmt.Errorf("invalid provider: %s", s.ProviderName)
	}
	_, repo, err := s.parseOwnerAndRepo(s.ProviderName)
	if err != nil {
		return "", err
	}
	return repo, nil
}

// Owner returns the repository owner portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "aws".
func (s *Source) Owner() (string, error) {
	owner := "N/A"
	var err error
	if s.ProviderName == manifest.GithubProviderName {
		owner, _, err = s.parseOwnerAndRepo(s.ProviderName)
		if err != nil {
			return "", err
		}
	}
	return owner, nil
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Config Environment the pipeline
// is deploying to, the containerized services that will be deployed, and
// test commands, if the user has opted to add any.
type PipelineStage struct {
	*AssociatedEnvironment
	LocalWorkloads   []string
	RequiresApproval bool
	TestCommands     []string
}

// WorkloadTemplatePath returns the full path to the workload CFN template
// built during the build stage.
func (s *PipelineStage) WorkloadTemplatePath(wlName string) string {
	return fmt.Sprintf(WorkloadCfnTemplateNameFormat, wlName, s.Name)
}

// WorkloadTemplateConfigurationPath returns the full path to the workload CFN
// template configuration file built during the build stage.
func (s *PipelineStage) WorkloadTemplateConfigurationPath(wlName string) string {
	return fmt.Sprintf(WorkloadCfnTemplateConfigurationNameFormat,
		wlName, s.Name,
	)
}

// AssociatedEnvironment defines the necessary information a pipeline stage
// needs for an Config Environment.
type AssociatedEnvironment struct {
	// Name of the environment, must be unique within an application.
	// This is also the name of the pipeline stage.
	Name string

	// The region this environment is stored in.
	Region string

	// AccountID of the account this environment is stored in.
	AccountID string
}
