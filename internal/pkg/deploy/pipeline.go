// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
// This file defines pipeline deployment resources.
package deploy

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/graph"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

const (
	fmtInvalidRepo           = "unable to parse the repository from the URL %+v"
	fmtErrMissingProperty    = "missing `%s` in properties"
	fmtErrPropertyNotAString = "property `%s` is not a string"

	defaultPipelineBuildImage      = "aws/codebuild/amazonlinux2-x86_64-standard:4.0"
	defaultPipelineEnvironmentType = "LINUX_CONTAINER"

	// DefaultPipelineArtifactsDir is the default folder to output Copilot-generated templates.
	DefaultPipelineArtifactsDir = "infrastructure"
	// DefaultPipelineBranch is the default repository branch to use for pipeline.
	DefaultPipelineBranch = "main"
	// StageFullNamePrefix is prefix to a pipeline stage name. For example, "DeployTo-test" for a test environment stage.
	StageFullNamePrefix = "DeployTo-"
)

// Name of the environment variables injected into the CodeBuild projects that support pre/post-deployment actions.
const (
	envVarNameEnvironmentName = "COPILOT_ENVIRONMENT_NAME"
	envVarNameApplicationName = "COPILOT_APPLICATION_NAME"
)

var (
	// NOTE: this is duplicated from validate.go
	// Ex: https://github.com/koke/grit
	ghRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)
	// Ex: https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample/browse
	ccRepoExp = regexp.MustCompile(`(https:\/\/(?P<region>.+).console.aws.amazon.com\/codesuite\/codecommit\/repositories\/(?P<repo>.+)(\/browse))`)
	// Ex: https://bitbucket.org/repoOwner/repoName
	bbRepoExp = regexp.MustCompile(`(https:\/\/bitbucket.org\/)(?P<owner>.+)\/(?P<repo>.+)`)
)

// CreatePipelineInput represents the fields required to deploy a pipeline.
type CreatePipelineInput struct {
	// Name of the application this pipeline belongs to
	AppName string

	// Name of the pipeline
	Name string

	// IsLegacy should be set to true if the pipeline has been deployed using a legacy non-namespaced name; otherwise it is false.
	IsLegacy bool

	// The source code provider for this pipeline
	Source interface{}

	// The build project settings for this pipeline
	Build *Build

	// The stages of the pipeline. The order of stages in this list
	// will be the order we deploy to.
	Stages []PipelineStage

	// A list of artifact buckets and corresponding KMS keys that will
	// be used in this pipeline.
	ArtifactBuckets []ArtifactBucket

	// AdditionalTags are labels applied to resources under the application.
	AdditionalTags map[string]string

	// PermissionsBoundary is the name of an IAM policy to set a permissions boundary.
	PermissionsBoundary string

	// Version is the pipeline template version.
	Version string
}

// Build represents CodeBuild project used in the CodePipeline
// to build and test Docker image.
type Build struct {
	// The URI that identifies the Docker image to use for this build project.
	Image                    string
	EnvironmentType          string
	BuildspecPath            string
	AdditionalPolicyDocument string
	Variables                map[string]string
}

// Init populates the fields in Build by parsing the manifest file's "build" section.
func (b *Build) Init(mfBuild *manifest.Build, mfDirPath string) error {
	image := defaultPipelineBuildImage
	environmentType := defaultPipelineEnvironmentType
	path := filepath.Join(mfDirPath, "buildspec.yml")
	if mfBuild != nil && mfBuild.Image != "" {
		image = mfBuild.Image
	}
	if mfBuild != nil && mfBuild.Buildspec != "" {
		path = mfBuild.Buildspec
	}
	if strings.Contains(image, "aarch64") {
		environmentType = "ARM_CONTAINER"
	}
	if mfBuild != nil && !mfBuild.AdditionalPolicy.Document.IsZero() {
		additionalPolicy, err := yaml.Marshal(&mfBuild.AdditionalPolicy.Document)
		if err != nil {
			return fmt.Errorf("marshal `additional_policy.PolicyDocument` in pipeline manifest: %v", err)
		}
		b.AdditionalPolicyDocument = strings.TrimSpace(string(additionalPolicy))
	}
	b.Image = image
	b.EnvironmentType = environmentType
	b.BuildspecPath = filepath.ToSlash(path) // Buildspec path must be with '/' because CloudFormation expects forward-slash separated file path.

	return nil
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

// GitHubV1Source defines the source of the artifacts to be built and deployed. This version uses personal access tokens
// and is not recommended. https://docs.aws.amazon.com/codepipeline/latest/userguide/update-github-action-connections.html
type GitHubV1Source struct {
	ProviderName                string
	Branch                      string
	RepositoryURL               GitHubURL
	PersonalAccessTokenSecretID string
}

// GitHubSource (version 2) defines the source of the artifacts to be built and deployed. This version uses CodeStar
// Connections to authenticate access to the remote repo.
type GitHubSource struct {
	ProviderName         string
	Branch               string
	RepositoryURL        GitHubURL
	ConnectionARN        string
	OutputArtifactFormat string
}

// GitHubURL is the common type for repo URLs for both GitHubSource versions:
// GitHubV1 (w/ access tokens) and GitHub (V2 w CodeStar Connections).
type GitHubURL string

// CodeCommitSource defines the (CC) source of the artifacts to be built and deployed.
type CodeCommitSource struct {
	ProviderName         string
	Branch               string
	RepositoryURL        string
	OutputArtifactFormat string
}

// BitbucketSource defines the (BB) source of the artifacts to be built and deployed.
type BitbucketSource struct {
	ProviderName         string
	Branch               string
	RepositoryURL        string
	ConnectionARN        string
	OutputArtifactFormat string
}

func convertRequiredProperty(properties map[string]interface{}, key string) (string, error) {
	v, ok := properties[key]
	if !ok {
		return "", fmt.Errorf(fmtErrMissingProperty, key)
	}
	vStr, ok := v.(string)
	if !ok {
		return "", fmt.Errorf(fmtErrPropertyNotAString, key)
	}
	return vStr, nil
}

func convertOptionalProperty(properties map[string]interface{}, key string, defaultValue string) (string, error) {
	v, ok := properties[key]
	if !ok {
		return defaultValue, nil
	}
	vStr, ok := v.(string)
	if !ok {
		return "", fmt.Errorf(fmtErrPropertyNotAString, key)
	}
	return vStr, nil
}

// PipelineSourceFromManifest processes manifest info about the source based on provider type.
// The return boolean is true for CodeStar Connections sources that require a polling prompt.
func PipelineSourceFromManifest(mfSource *manifest.Source) (source interface{}, shouldPrompt bool, err error) {
	branch, err := convertOptionalProperty(mfSource.Properties, "branch", DefaultPipelineBranch)
	if err != nil {
		return nil, false, err
	}
	repository, err := convertRequiredProperty(mfSource.Properties, "repository")
	if err != nil {
		return nil, false, err
	}
	outputFormat, err := convertOptionalProperty(mfSource.Properties, "output_artifact_format", "")
	if err != nil {
		return nil, false, err
	}
	switch mfSource.ProviderName {
	case manifest.GithubV1ProviderName:
		token, err := convertRequiredProperty(mfSource.Properties, "access_token_secret")
		if err != nil {
			return nil, false, err
		}
		return &GitHubV1Source{
			ProviderName:                manifest.GithubV1ProviderName,
			Branch:                      branch,
			RepositoryURL:               GitHubURL(repository),
			PersonalAccessTokenSecretID: token,
		}, false, nil
	case manifest.GithubProviderName:
		// If the creation of the user's pipeline manifest predates Copilot's conversion to GHv2/CSC, the provider
		// listed in the manifest will be "GitHub," not "GitHubV1." To differentiate it from the new default
		// "GitHub," which refers to v2, we check for the presence of a secret, indicating a v1 GitHub connection.
		if mfSource.Properties["access_token_secret"] != nil {
			return &GitHubV1Source{
				ProviderName:                manifest.GithubV1ProviderName,
				Branch:                      branch,
				RepositoryURL:               GitHubURL(repository),
				PersonalAccessTokenSecretID: (mfSource.Properties["access_token_secret"]).(string),
			}, false, nil
		} else {
			// If an existing CSC connection is being used, don't prompt to update connection from 'PENDING' to 'AVAILABLE'.
			connection, ok := mfSource.Properties["connection_arn"]
			repo := &GitHubSource{
				ProviderName:         manifest.GithubProviderName,
				Branch:               branch,
				RepositoryURL:        GitHubURL(repository),
				OutputArtifactFormat: outputFormat,
			}
			if !ok {
				return repo, true, nil
			}
			repo.ConnectionARN = connection.(string)
			return repo, false, nil
		}
	case manifest.CodeCommitProviderName:
		return &CodeCommitSource{
			ProviderName:         manifest.CodeCommitProviderName,
			Branch:               branch,
			RepositoryURL:        repository,
			OutputArtifactFormat: outputFormat,
		}, false, nil
	case manifest.BitbucketProviderName:
		// If an existing CSC connection is being used, don't prompt to update connection from 'PENDING' to 'AVAILABLE'.
		connection, ok := mfSource.Properties["connection_arn"]
		repo := &BitbucketSource{
			ProviderName:         manifest.BitbucketProviderName,
			Branch:               branch,
			RepositoryURL:        repository,
			OutputArtifactFormat: outputFormat,
		}
		if !ok {
			return repo, true, nil
		}
		repo.ConnectionARN = connection.(string)
		return repo, false, nil
	default:
		return nil, false, fmt.Errorf("invalid repo source provider: %s", mfSource.ProviderName)
	}
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

// Connection returns the ARN correlated with a ConnectionName in the pipeline manifest.
func (s *BitbucketSource) Connection() string {
	return s.ConnectionARN
}

// Connection returns the ARN correlated with a ConnectionName in the pipeline manifest.
func (s *GitHubSource) Connection() string {
	return s.ConnectionARN
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

// ConnectionName generates a recognizable string by which the connection may be identified.
func (s *BitbucketSource) ConnectionName() (string, error) {
	owner, repo, err := s.parseOwnerAndRepo()
	if err != nil {
		return "", fmt.Errorf("parse owner and repo to generate connection name: %w", err)
	}
	return formatConnectionName(owner, repo), nil
}

// ConnectionName generates a recognizable string by which the connection may be identified.
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

type associatedEnvironment struct {
	// Name of the environment, must be unique within an application.
	// This is also the name of the pipeline stage.
	Name string

	// The region this environment is created in.
	Region string

	// AppName represents the application name the environment is part of.
	AppName string

	// AccountID of the account this environment is stored in.
	AccountID string
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Config Environment the pipeline
// is deploying to, the containerized services that will be deployed, and
// test commands, if the user has opted to add any.
type PipelineStage struct {
	*associatedEnvironment
	requiresApproval  bool
	testCommands      []string
	execRoleARN       string
	envManagerRoleARN string
	preDeployments    manifest.PrePostDeployments
	deployments       manifest.Deployments
	postDeployments   manifest.PrePostDeployments
}

// Init populates the fields in PipelineStage against a target environment,
// the user's manifest config, and any local workload names.
func (stg *PipelineStage) Init(env *config.Environment, mftStage *manifest.PipelineStage, workloads []string) {
	stg.associatedEnvironment = &associatedEnvironment{
		AppName:   env.App,
		Name:      mftStage.Name,
		Region:    env.Region,
		AccountID: env.AccountID,
	}
	deployments := mftStage.Deployments
	if len(deployments) == 0 {
		// Transform local workloads into the manifest.Deployments format if the manifest doesn't have any deployment config.
		deployments = make(manifest.Deployments)
		for _, workload := range workloads {
			deployments[workload] = nil
		}
	}
	stg.preDeployments = mftStage.PreDeployments
	stg.deployments = deployments
	stg.postDeployments = mftStage.PostDeployments
	stg.requiresApproval = mftStage.RequiresApproval
	stg.testCommands = mftStage.TestCommands
	stg.execRoleARN = env.ExecutionRoleARN
	stg.envManagerRoleARN = env.ManagerRoleARN
}

// Name returns the stage's name.
func (stg *PipelineStage) Name() string {
	return stg.associatedEnvironment.Name
}

// FullName returns the stage's full name.
func (stg *PipelineStage) FullName() string {
	return StageFullNamePrefix + stg.associatedEnvironment.Name
}

// Approval returns a manual approval action for the stage.
// If the stage does not require approval, then returns nil.
func (stg *PipelineStage) Approval() *ManualApprovalAction {
	if !stg.requiresApproval {
		return nil
	}
	return &ManualApprovalAction{
		name: stg.associatedEnvironment.Name,
	}
}

// Region returns the AWS region name, such as "us-west-2", where the deployments will occur.
func (stg *PipelineStage) Region() string {
	return stg.associatedEnvironment.Region
}

// ExecRoleARN returns the IAM role assumed by CloudFormation to create or update resources defined in a template.
func (stg *PipelineStage) ExecRoleARN() string {
	return stg.execRoleARN
}

// EnvManagerRoleARN returns the IAM role used to create or update CloudFormation stacks in an environment.
func (stg *PipelineStage) EnvManagerRoleARN() string {
	return stg.envManagerRoleARN
}

// PreDeployments returns a list of pre-deployment actions for the pipeline stage.
func (stg *PipelineStage) PreDeployments() ([]PrePostDeployAction, error) {
	if len(stg.preDeployments) == 0 {
		return nil, nil
	}
	var prevActions []orderedRunner
	if approval := stg.Approval(); approval != nil {
		prevActions = append(prevActions, approval)
	}

	var actionGraphNodes []actionGraphNode
	for name, action := range stg.preDeployments {
		var depends_on []string
		if action != nil && action.DependsOn != nil {
			depends_on = action.DependsOn
		}
		actionGraphNodes = append(actionGraphNodes, actionGraphNode{
			name:       name,
			depends_on: depends_on,
		})
	}
	topo, err := graph.TopologicalOrder(stg.buildActionsGraph(actionGraphNodes))
	if err != nil {
		return nil, fmt.Errorf("find an ordering for deployments: %v", err)
	}

	var actions []PrePostDeployAction
	for name, conf := range stg.preDeployments {
		actions = append(actions, PrePostDeployAction{
			name: name,
			action: action{
				prevActions: prevActions,
			},
			Build: Build{
				Image:           defaultPipelineBuildImage,
				EnvironmentType: defaultPipelineEnvironmentType,
				BuildspecPath:   conf.BuildspecPath,
				Variables: map[string]string{
					envVarNameApplicationName: stg.AppName,
					envVarNameEnvironmentName: stg.associatedEnvironment.Name,
				},
			},
			ranker: topo,
		})
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name() < actions[j].Name()
	})
	return actions, nil
}

// Deployments returns a list of deploy actions for the pipeline.
func (stg *PipelineStage) Deployments() ([]DeployAction, error) {
	if len(stg.deployments) == 0 {
		return nil, nil
	}
	var prevActions []orderedRunner
	if approval := stg.Approval(); approval != nil {
		prevActions = append(prevActions, approval)
	}
	preDeployActions, err := stg.PreDeployments()
	if err != nil {
		return nil, err
	}
	for i := range preDeployActions {
		prevActions = append(prevActions, &preDeployActions[i])
	}

	var actionGraphNodes []actionGraphNode
	for name, action := range stg.deployments {
		var depends_on []string
		if action != nil && action.DependsOn != nil {
			depends_on = action.DependsOn
		}
		actionGraphNodes = append(actionGraphNodes, actionGraphNode{
			name:       name,
			depends_on: depends_on,
		})
	}
	topo, err := graph.TopologicalOrder(stg.buildActionsGraph(actionGraphNodes))
	if err != nil {
		return nil, fmt.Errorf("find an ordering for deployments: %v", err)
	}

	var actions []DeployAction
	for name, conf := range stg.deployments {
		actions = append(actions, DeployAction{
			action: action{
				prevActions: prevActions,
			},
			name:     name,
			envName:  stg.associatedEnvironment.Name,
			appName:  stg.AppName,
			override: conf,
			ranker:   topo,
		})
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name() < actions[j].Name()
	})
	return actions, nil
}

// PostDeployments returns a list of post-deployment actions for the pipeline stage.
func (stg *PipelineStage) PostDeployments() ([]PrePostDeployAction, error) {
	if len(stg.postDeployments) == 0 {
		return nil, nil
	}

	var prevActions []orderedRunner
	if approval := stg.Approval(); approval != nil {
		prevActions = append(prevActions, approval)
	}
	preDeployActions, err := stg.PreDeployments()
	if err != nil {
		return nil, err
	}
	for i := range preDeployActions {
		prevActions = append(prevActions, &preDeployActions[i])
	}
	deployActions, err := stg.Deployments()
	if err != nil {
		return nil, err
	}
	for i := range deployActions {
		prevActions = append(prevActions, &deployActions[i])
	}

	var actionGraphNodes []actionGraphNode
	for name, action := range stg.postDeployments {
		var depends_on []string
		if action != nil && action.DependsOn != nil {
			depends_on = action.DependsOn
		}
		actionGraphNodes = append(actionGraphNodes, actionGraphNode{
			name:       name,
			depends_on: depends_on,
		})
	}

	topo, err := graph.TopologicalOrder(stg.buildActionsGraph(actionGraphNodes))
	if err != nil {
		return nil, fmt.Errorf("find an ordering for deployments: %v", err)
	}

	var actions []PrePostDeployAction
	for name, conf := range stg.postDeployments {
		actions = append(actions, PrePostDeployAction{
			name: name,
			action: action{
				prevActions: prevActions,
			},
			Build: Build{
				Image:           defaultPipelineBuildImage,
				EnvironmentType: defaultPipelineEnvironmentType,
				BuildspecPath:   conf.BuildspecPath,
				Variables: map[string]string{
					envVarNameApplicationName: stg.AppName,
					envVarNameEnvironmentName: stg.associatedEnvironment.Name,
				},
			},
			ranker: topo,
		})
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name() < actions[j].Name()
	})
	return actions, nil
}

// Test returns a test for the stage.
// If the stage does not have any test commands, then returns nil.
func (stg *PipelineStage) Test() (*TestCommandsAction, error) {
	if len(stg.testCommands) == 0 {
		return nil, nil
	}

	var prevActions []orderedRunner
	if approval := stg.Approval(); approval != nil {
		prevActions = append(prevActions, approval)
	}
	preDeployActions, err := stg.PreDeployments()
	if err != nil {
		return nil, err
	}
	for i := range preDeployActions {
		prevActions = append(prevActions, &preDeployActions[i])
	}
	deployActions, err := stg.Deployments()
	if err != nil {
		return nil, err
	}
	for i := range deployActions {
		prevActions = append(prevActions, &deployActions[i])
	}

	return &TestCommandsAction{
		action: action{
			prevActions: prevActions,
		},
		commands: stg.testCommands,
	}, nil
}

type actionGraphNode struct {
	name       string
	depends_on []string
}

func (stg *PipelineStage) buildActionsGraph(rankables []actionGraphNode) *graph.Graph[string] {
	var names []string
	for _, r := range rankables {
		names = append(names, r.name)
	}
	digraph := graph.New(names...)

	for _, r := range rankables {
		if r.depends_on == nil {
			continue
		}
		for _, dependency := range r.depends_on {
			digraph.Add(graph.Edge[string]{
				From: dependency, // Dependency must be completed before name.
				To:   r.name,
			})
		}
	}
	return digraph
}

type orderedRunner interface {
	RunOrder() int
}

// action represents a generic CodePipeline action.
type action struct {
	prevActions []orderedRunner // The last actions to be executed immediately before this action.
}

// RunOrder returns the order in which the action should run. A higher numbers means the action is run later.
// Actions with the same RunOrder run in parallel.
func (a *action) RunOrder() int {
	max := 0
	for _, prevAction := range a.prevActions {
		if cur := prevAction.RunOrder(); cur > max {
			max = cur
		}
	}
	return max + 1
}

// ManualApprovalAction represents a stage approval action.
type ManualApprovalAction struct {
	action
	name string // Name of the stage to approve.
}

// Name returns the name of the CodePipeline approval action for the stage.
func (a *ManualApprovalAction) Name() string {
	return fmt.Sprintf("ApprovePromotionTo-%s", a.name)
}

type ranker interface {
	Rank(name string) (int, bool)
}

// DeployAction represents a CodePipeline action of category "Deploy" for a cloudformation stack.
type DeployAction struct {
	action

	name     string
	envName  string
	appName  string
	override *manifest.Deployment // User defined settings over Copilot's defaults.

	ranker ranker // Interface to rank this deployment action against others in the same stage.
}

// Name returns the name of the CodePipeline deploy action for a workload.
func (a *DeployAction) Name() string {
	return fmt.Sprintf("CreateOrUpdate-%s-%s", a.name, a.envName)
}

// StackName returns the name of the workload stack to create or update.
func (a *DeployAction) StackName() string {
	if a.override != nil && a.override.StackName != "" {
		return a.override.StackName
	}
	return fmt.Sprintf("%s-%s-%s", a.appName, a.envName, a.name)
}

// TemplatePath returns the path of the CloudFormation template file generated during the build phase.
func (a *DeployAction) TemplatePath() string {
	if a.override != nil && a.override.TemplatePath != "" {
		return a.override.TemplatePath
	}

	// Use path.Join instead of filepath to join with "/" instead of OS-specific file separators.
	return path.Join(DefaultPipelineArtifactsDir, fmt.Sprintf(WorkloadCfnTemplateNameFormat, a.name, a.envName))
}

// TemplateConfigPath returns the path of the CloudFormation template config file generated during the build phase.
func (a *DeployAction) TemplateConfigPath() string {
	if a.override != nil && a.override.TemplateConfig != "" {
		return a.override.TemplateConfig
	}

	// Use path.Join instead of filepath to join with "/" instead of OS-specific file separators.
	return path.Join(DefaultPipelineArtifactsDir, fmt.Sprintf(WorkloadCfnTemplateConfigurationNameFormat, a.name, a.envName))
}

// RunOrder returns the order in which the action should run.
func (a *DeployAction) RunOrder() int {
	rank, _ := a.ranker.Rank(a.name) // The deployment is guaranteed to be in the ranker.
	return a.action.RunOrder() /* baseline */ + rank
}

// TestCommandsAction represents a CodePipeline action of category "Test" to validate deployments.
type TestCommandsAction struct {
	action
	commands []string
}

// Name returns the name of the test action.
func (a *TestCommandsAction) Name() string {
	return "TestCommands"
}

// Commands returns the list commands to run part of the test action.
func (a *TestCommandsAction) Commands() []string {
	return a.commands
}

// PrePostDeployAction represents a CodePipeline action of category "Build" backed by a CodeBuild project.
type PrePostDeployAction struct {
	action
	Build
	name   string
	ranker ranker // Interface to rank this deployment action against others in the same stage.
}

// Name returns the name of the action.
func (p *PrePostDeployAction) Name() string {
	return p.name
}

// RunOrder returns the order in which the action should run.
func (p *PrePostDeployAction) RunOrder() int {
	rank, _ := p.ranker.Rank(p.name) // The deployment is guaranteed to be in the ranker.
	return p.action.RunOrder() /* baseline */ + rank
}
