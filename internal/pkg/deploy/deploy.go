// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
package deploy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

// CreateEnvironmentInput represents the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	Project                  string // Name of the project this environment belongs to.
	Name                     string // Name of the environment, must be unique within a project.
	Prod                     bool   // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool   // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string // The Principal ARN of the tools account.
}

const (
	GithubProviderName    = "GitHub"
	GithubSecretIdKeyName = "githubPersonalAccessTokenSecretId"
)

// CreateEnvironmentResponse holds the created environment on successful deployment.
// Otherwise, the environment is set to nil and a descriptive error is returned.
type CreateEnvironmentResponse struct {
	Env *archer.Environment
	Err error
}

// CreatePipelineInput represents the fields required to deploy a pipeline.
type CreatePipelineInput struct {
	// Name of the project this pipeline belongs to
	ProjectName string
	// Name of the pipeline
	Name string
	// The source code provider for this pipeline
	Source *Source
	// The stages of the pipeline. The order of stages in this list
	// will be the order we deploy to
	Stages []PipelineStage
	// A list of artifact buckets and corresponding KMS keys that will
	// be used in this pipeline.
	ArtifactBuckets []ArtifactBucket
}

// ArtifactBucket represents an S3 bucket used by the CodePipeline to store
// intermediate artifacts produced by the pipeline.
type ArtifactBucket struct {
	// The ARN of the S3 bucket.
	BucketArn string
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
			a.BucketArn, err)
	}
	return parsedArn.Region, nil
}

// BucketName parses out the name of the bucket from its ARN.
func (a *ArtifactBucket) BucketName() (string, error) {
	parsedArn, err := arn.Parse(a.BucketArn)
	if err != nil {
		return "", fmt.Errorf("failed to parse the name of the bucket out of bucket ARN: %s, error: %w",
			a.BucketArn, err)
	}
	return parsedArn.Resource, nil
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

// GitHubPersonalAccessTokenSecretID returns the ID of the secret in the Secrets manager,
// which stores the GitHub OAuth token if the provider is "GitHub". Otherwise,
// it returns an error.
func (s *Source) GitHubPersonalAccessTokenSecretID() (string, error) {
	secretID, exists := s.Properties[GithubSecretIdKeyName]
	if !exists {
		return "", errors.New("the GitHub token secretID is not configured")
	}

	id, ok := secretID.(string)
	if !ok {
		return "", fmt.Errorf("unable to locate the GitHub token secretID from %v", secretID)
	}

	if s.ProviderName != GithubProviderName {
		return "", fmt.Errorf("failed attempt to retrieve GitHub token from a non-GitHub provider")
	}

	return id, nil
}

type ownerAndRepo struct {
	owner string
	repo  string
}

func (s *Source) parseOwnerAndRepo() (*ownerAndRepo, error) {
	if s.ProviderName != GithubProviderName {
		return nil, fmt.Errorf("invalid provider: %s", s.ProviderName)
	}
	ownerAndRepoI, exists := s.Properties["repository"]
	if !exists {
		return nil, fmt.Errorf("unable to locate the repository from the properties: %+v", s.Properties)
	}
	ownerAndRepoStr, ok := ownerAndRepoI.(string)
	if !ok {
		return nil, fmt.Errorf("unable to locate the repository from the properties: %+v", ownerAndRepoI)
	}

	result := strings.Split(ownerAndRepoStr, "/")
	if len(result) != 2 {
		return nil, fmt.Errorf("unable to locate the repository from the properties: %s", ownerAndRepoStr)
	}
	return &ownerAndRepo{
		owner: result[0],
		repo:  result[1],
	}, nil
}

// Repository returns the repository portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "amazon-ecs-cli-v2"
func (s *Source) Repository() (string, error) {
	oAndR, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", err
	}
	return oAndR.repo, nil
}

// Owner returns the repository owner portion. For example,
// given "aws/amazon-ecs-cli-v2", this function returns "aws"
func (s *Source) Owner() (string, error) {
	oAndR, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", err
	}
	return oAndR.repo, nil
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Archer Environment the pipeline
// is deloying to and the containerized applications that will be deployed.
type PipelineStage struct {
	*AssociatedEnvironment
	LocalApplications []string
}

// AssociatedEnvironment defines the necessary information a pipline stage
// needs for an Archer Environment.
type AssociatedEnvironment struct {
	// Name of the environment, must be unique within a project.
	// This is also the name of the pipeline stage.
	Name string
	// The region this environment is stored in.
	Region string
	// AccountID of the account this environment is stored in.
	AccountID string
	// Whether or not this environment is a production environment.
	Prod bool
}

// CreateLBFargateAppInput holds the fields required to deploy a load-balanced AWS Fargate application.
type CreateLBFargateAppInput struct {
	App      *manifest.LBFargateManifest
	Env      *archer.Environment
	ImageTag string
}

// Resource represents an AWS resource.
type Resource struct {
	LogicalName string
	Type        string
}

// ResourceEvent represents a status update for an AWS resource during a deployment.
type ResourceEvent struct {
	Resource
	Status       string
	StatusReason string
}
