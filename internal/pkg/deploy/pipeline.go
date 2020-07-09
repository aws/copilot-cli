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

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

// NOTE: this is duplicated from validate.go
var githubRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

const (
	fmtInvalidGitHubRepo = "unable to locate the repository from the properties: %+v"
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
// provider is "GitHub". Otherwise, it returns an error.
func (s *Source) GitHubPersonalAccessTokenSecretID() (string, error) {
	// TODO type check if properties are GitHubProperties?
	secretID, exists := s.Properties[manifest.GithubSecretIdKeyName]
	if !exists {
		return "", errors.New("the GitHub token secretID is not configured")
	}

	id, ok := secretID.(string)
	if !ok {
		return "", fmt.Errorf("unable to locate the GitHub token secretID from %v", secretID)
	}

	if s.ProviderName != manifest.GithubProviderName {
		return "", fmt.Errorf("failed attempt to retrieve GitHub token from a non-GitHub provider")
	}

	return id, nil
}

type ownerAndRepo struct {
	owner string
	repo  string
}

func (s *Source) parseOwnerAndRepo() (*ownerAndRepo, error) {
	if s.ProviderName != manifest.GithubProviderName {
		return nil, fmt.Errorf("invalid provider: %s", s.ProviderName)
	}
	ownerAndRepoI, exists := s.Properties["repository"]
	if !exists {
		return nil, fmt.Errorf("unable to locate the repository from the properties: %+v", s.Properties)
	}
	ownerAndRepoStr, ok := ownerAndRepoI.(string)
	if !ok {
		return nil, fmt.Errorf(fmtInvalidGitHubRepo, ownerAndRepoI)
	}

	match := githubRepoExp.FindStringSubmatch(ownerAndRepoStr)
	if len(match) == 0 {
		return nil, fmt.Errorf(fmtInvalidGitHubRepo, ownerAndRepoStr)
	}

	matches := make(map[string]string)
	for i, name := range githubRepoExp.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}

	return &ownerAndRepo{
		owner: matches["owner"],
		repo:  matches["repo"],
	}, nil
}

// Repository returns the repository portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "amazon-ecs-cli-v2".
func (s *Source) Repository() (string, error) {
	oAndR, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", err
	}
	return oAndR.repo, nil
}

// Owner returns the repository owner portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "aws".
func (s *Source) Owner() (string, error) {
	oAndR, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", err
	}
	return oAndR.owner, nil
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Config Environment the pipeline
// is deploying to, the containerized services that will be deployed, and
// test commands, if the user has opted to add any.
type PipelineStage struct {
	*AssociatedEnvironment
	LocalServices []string
	TestCommands  []string
}

// ServiceTemplatePath returns the full path to the service CFN template
// built during the build stage.
func (s *PipelineStage) ServiceTemplatePath(svcName string) string {
	return fmt.Sprintf(config.ServiceCfnTemplateNameFormat, svcName)
}

// ServiceTemplateConfigurationPath returns the full path to the service CFN
// template configuration file built during the build stage.
func (s *PipelineStage) ServiceTemplateConfigurationPath(svcName string) string {
	return fmt.Sprintf(config.ServiceCfnTemplateConfigurationNameFormat,
		svcName, s.Name,
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

	// Whether or not this environment is a production environment.
	Prod bool
}
