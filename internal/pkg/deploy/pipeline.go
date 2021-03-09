// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines pipeline deployment resources.
package deploy

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/aws-sdk-go/aws/arn"
)

const (
	fmtInvalidRepo = "unable to locate the repository URL from the properties: %+v"
	// Redefining these consts from the cli package here so as to avoid an import cycle
	ghProviderName   = "GitHub"
	ghV1ProviderName = "GitHubV1"
	ccProviderName   = "CodeCommit"
	bbProviderName   = "Bitbucket"
)

var (
	// NOTE: this is duplicated from validate.go
	// Ex: https://github.com/koke/grit
	ghRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)
	// Ex: https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample/browse
	ccRepoExp = regexp.MustCompile(`(https:\/\/(?P<region>.+).console.aws.amazon.com\/codesuite\/codecommit\/repositories\/(?P<repo>.+)(\/browse))`)
	// Ex: https://repoOwner@bitbucket.org/repoOwner/repoName
	bbRepoExp = regexp.MustCompile(`(https:\/\/(.+)@bitbucket.org\/)(?P<owner>.+)\/(?P<repo>.+)`)
)

// CreatePipelineInput represents the fields required to deploy a pipeline.
type CreatePipelineInput struct {
	// Name of the application this pipeline belongs to
	AppName string

	// Name of the pipeline
	Name string

	// The source code provider for this pipeline
	Source interface{}

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

// GitHubSource defines the (GH) source of the artifacts to be built and deployed.
type GitHubV1Source struct {
	ProviderName                string
	Branch                      string
	RepositoryURL               GitHubURL
	PersonalAccessTokenSecretID string
}

// GitHubURL is the common type for repo URLs for both GitHubSource versions.
type GitHubURL string

// CodeCommitSource defines the (CC) source of the artifacts to be built and deployed.
type CodeCommitSource struct {
	ProviderName  string
	Branch        string
	RepositoryURL string
}

// BitbucketSource defines the (BB) source of the artifacts to be built and deployed.
type BitbucketSource struct {
	ProviderName  string
	Branch        string
	RepositoryURL string
}

// PipelineSourceFromManifest processes manifest info about the source based on provider type.
// The return boolean is true for CodeStar Connections sources that require a polling prompt.
func PipelineSourceFromManifest(source *manifest.Source) (interface{}, bool, error) {
	switch source.ProviderName {
	case ghV1ProviderName:
		return GitHubV1Source{
			ProviderName:                ghV1ProviderName,
			Branch:                      (source.Properties["branch"]).(string),
			RepositoryURL:               GitHubURL((source.Properties["repository"]).(string)),
			PersonalAccessTokenSecretID: (source.Properties["access_token_secret"]).(string),
		}, false, nil
	case ghProviderName:
		// If the creation of the user's pipeline manifest predates Copilot's conversion to GHv2/CSC, the provider
		// listed in the manifest will be "GitHub," not "GitHubV1." To differentiate it from the new default
		// "GitHub," which refers to v2, we check for the presence of a secret, indicating a v1 GitHub connection.
		if source.Properties["access_token_secret"] != nil {
			return GitHubV1Source{
				ProviderName:                ghV1ProviderName,
				Branch:                      (source.Properties["branch"]).(string),
				RepositoryURL:               GitHubURL((source.Properties["repository"]).(string)),
				PersonalAccessTokenSecretID: (source.Properties["access_token_secret"]).(string),
			}, false, nil
		} else {
			return GitHubSource{
				ProviderName:  ghProviderName,
				Branch:        (source.Properties["branch"]).(string),
				RepositoryURL: GitHubURL((source.Properties["repository"]).(string)),
			}, true, nil
		}
	case ccProviderName:
		return CodeCommitSource{
			ProviderName:  ccProviderName,
			Branch:        (source.Properties["branch"]).(string),
			RepositoryURL: (source.Properties["repository"]).(string),
		}, false, nil
	case bbProviderName:
		return BitbucketSource{
			ProviderName:  bbProviderName,
			Branch:        (source.Properties["branch"]).(string),
			RepositoryURL: (source.Properties["repository"]).(string),
		}, true, nil
	default:
		return nil, false, fmt.Errorf("invalid repo source provider: %s", source.ProviderName)
	}
}

type GitHubSource struct {
	ProviderName  string
	Branch        string
	RepositoryURL GitHubURL
}

// GitHubPersonalAccessTokenSecretID returns the ID of the secret in the
// Secrets manager, which stores the GitHub Personal Access token if the
// provider is "GitHubV1".
func (s *GitHubV1Source) GitHubPersonalAccessTokenSecretID() (string, error) {
	if s.PersonalAccessTokenSecretID == "" {
		return "", errors.New("the GitHub token secretID is not configured")
	}
	return s.PersonalAccessTokenSecretID, nil
}

// parse parses the owner and repo name from the GH repo URL, which was formatted and assigned in cli/pipeline_init.go.
func (url GitHubURL) parse() (owner, repo string, err error) {
	if url == "" {
		return "", "", fmt.Errorf("unable to locate the repository")
	}

	match := ghRepoExp.FindStringSubmatch(string(url))
	if len(match) == 0 {
		return "", "", fmt.Errorf(fmtInvalidRepo, url)
	}

	matches := make(map[string]string)
	for i, name := range ghRepoExp.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}
	return matches["owner"], matches["repo"], nil
}

// parseRepo parses the region (not returned) and repo name from the CC repo URL, which was formatted and assigned in cli/pipeline_init.go.
func (s *CodeCommitSource) parseRepo() (string, error) {
	// NOTE: 'region' is not currently parsed out as a Source property, but this enables that possibility.
	if s.RepositoryURL == "" {
		return "", fmt.Errorf("unable to locate the repository")
	}
	match := ccRepoExp.FindStringSubmatch(s.RepositoryURL)
	if len(match) == 0 {
		return "", fmt.Errorf(fmtInvalidRepo, s.RepositoryURL)
	}

	matches := make(map[string]string)
	for i, name := range ccRepoExp.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}

	return matches["repo"], nil
}

// parseOwnerAndRepo parses the owner and repo name from the BB repo URL, which was formatted and assigned in cli/pipeline_init.go.
func (s *BitbucketSource) parseOwnerAndRepo() (owner, repo string, err error) {
	if s.RepositoryURL == "" {
		return "", "", fmt.Errorf("unable to locate the repository")
	}

	match := bbRepoExp.FindStringSubmatch(s.RepositoryURL)
	if len(match) == 0 {
		return "", "", fmt.Errorf(fmtInvalidRepo, s.RepositoryURL)
	}

	matches := make(map[string]string)
	for i, name := range bbRepoExp.SubexpNames() {
		if i != 0 && name != "" {
			matches[name] = match[i]
		}
	}
	return matches["owner"], matches["repo"], nil
}

// ConnectionName generates a string of maximum length 32 to be used as a CodeStar Connections ConnectionName.
// If there is a duplicate ConnectionName generated by CFN, the previous one is replaced. (Duplicate names
// generated by the aws cli don't have to be unique for some reason.)
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-codestarconnections-connection.html#cfn-codestarconnections-connection-connectionname
const (
	maxOwnerLength    = 5
	maxRepoLength     = 18
	fmtConnectionName = "copilot-%s-%s"
)

func (s *BitbucketSource) ConnectionName() (string, error) {
	owner, repo, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", fmt.Errorf("parse owner and repo to generate connection name: %w", err)
	}
	return formatConnectionName(owner, repo), nil
}

func (s *GitHubSource) ConnectionName() (string, error) {
	owner, repo, err := s.RepositoryURL.parse()
	if err != nil {
		return "", fmt.Errorf("parse owner and repo to generate connection name: %w", err)
	}
	return formatConnectionName(owner, repo), nil
}

func formatConnectionName(owner, repo string) string {
	if len(owner) > maxOwnerLength {
		owner = owner[:maxOwnerLength]
	}
	if len(repo) > maxRepoLength {
		repo = repo[:maxRepoLength]
	}
	return fmt.Sprintf(fmtConnectionName, owner, repo)
}

// Repository returns the repository portion. For example,
// given "aws/amazon-copilot", this function returns "amazon-copilot".
func (s *GitHubV1Source) Repository() (string, error) {
	_, repo, err := s.RepositoryURL.parse()
	if err != nil {
		return "", err
	}
	return repo, nil
}

// Repository returns the repository portion. For CodeStar Connections,
// this needs to be in the format "some-user/my-repo."
func (s *BitbucketSource) Repository() (string, error) {
	owner, repo, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", owner, repo), nil
}

// Repository returns the repository portion. For CodeStar Connections,
// this needs to be in the format "some-user/my-repo."
func (s *GitHubSource) Repository() (string, error) {
	owner, repo, err := s.RepositoryURL.parse()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", owner, repo), nil
}

// Repository returns the repository portion. For example,
// given "aws/amazon-copilot", this function returns "amazon-copilot".
func (s *CodeCommitSource) Repository() (string, error) {
	repo, err := s.parseRepo()
	if err != nil {
		return "", err
	}
	return repo, nil
}

// Owner returns the repository owner portion. For example,
// given "aws/amazon-copilot", this function returns "aws".
func (s *GitHubSource) Owner() (string, error) {
	owner, _, err := s.RepositoryURL.parse()
	if err != nil {
		return "", err
	}
	return owner, nil
}

// Owner returns the repository owner portion. For example,
// given "aws/amazon-copilot", this function returns "aws".
func (s *GitHubV1Source) Owner() (string, error) {
	owner, _, err := s.RepositoryURL.parse()
	if err != nil {
		return "", err
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
