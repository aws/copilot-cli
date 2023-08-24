// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/fatih/structs"
	"gopkg.in/yaml.v3"
)

// Valid source providers for Copilot Pipelines.
const (
	GithubProviderName     = "GitHub"
	GithubV1ProviderName   = "GitHubV1"
	CodeCommitProviderName = "CodeCommit"
	BitbucketProviderName  = "Bitbucket"
)

const pipelineManifestPath = "cicd/pipeline.yml"

// PipelineProviders is the list of all available source integrations.
var PipelineProviders = []string{
	GithubProviderName,
	CodeCommitProviderName,
	BitbucketProviderName,
}

// Provider defines a source of the artifacts
// that will be built and deployed via a pipeline
type Provider interface {
	fmt.Stringer
	Name() string
	Properties() map[string]interface{}
}

type githubV1Provider struct {
	properties *GitHubV1Properties
}

func (p *githubV1Provider) Name() string {
	return GithubV1ProviderName
}
func (p *githubV1Provider) String() string {
	return GithubProviderName
}
func (p *githubV1Provider) Properties() map[string]interface{} {
	return structs.Map(p.properties)
}

type githubProvider struct {
	properties *GitHubProperties
}

func (p *githubProvider) Name() string {
	return GithubProviderName
}
func (p *githubProvider) String() string {
	return GithubProviderName
}
func (p *githubProvider) Properties() map[string]interface{} {
	return structs.Map(p.properties)
}

type codecommitProvider struct {
	properties *CodeCommitProperties
}

func (p *codecommitProvider) Name() string {
	return CodeCommitProviderName
}
func (p *codecommitProvider) String() string {
	return CodeCommitProviderName
}
func (p *codecommitProvider) Properties() map[string]interface{} {
	return structs.Map(p.properties)
}

type bitbucketProvider struct {
	properties *BitbucketProperties
}

func (p *bitbucketProvider) Name() string {
	return BitbucketProviderName
}
func (p *bitbucketProvider) String() string {
	return BitbucketProviderName
}
func (p *bitbucketProvider) Properties() map[string]interface{} {
	return structs.Map(p.properties)
}

// GitHubV1Properties contain information for configuring a Githubv1
// source provider.
type GitHubV1Properties struct {
	// use tag from https://godoc.org/github.com/fatih/structs#example-Map--Tags
	// to specify the name of the field in the output properties
	RepositoryURL         string `structs:"repository" yaml:"repository"`
	Branch                string `structs:"branch" yaml:"branch"`
	GithubSecretIdKeyName string `structs:"access_token_secret" yaml:"access_token_secret"`
}

// GitHubProperties contains information for configuring a GitHubv2
// source provider.
type GitHubProperties struct {
	RepositoryURL string `structs:"repository" yaml:"repository"`
	Branch        string `structs:"branch" yaml:"branch"`
}

// BitbucketProperties contains information for configuring a Bitbucket
// source provider.
type BitbucketProperties struct {
	RepositoryURL string `structs:"repository" yaml:"repository"`
	Branch        string `structs:"branch" yaml:"branch"`
}

// CodeCommitProperties contains information for configuring a CodeCommit
// source provider.
type CodeCommitProperties struct {
	RepositoryURL string `structs:"repository" yaml:"repository"`
	Branch        string `structs:"branch" yaml:"branch"`
}

// NewProvider creates a source provider based on the type of
// the provided provider-specific configurations
func NewProvider(configs interface{}) (Provider, error) {
	switch props := configs.(type) {
	case *GitHubV1Properties:
		return &githubV1Provider{
			properties: props,
		}, nil
	case *GitHubProperties:
		return &githubProvider{
			properties: props,
		}, nil
	case *CodeCommitProperties:
		return &codecommitProvider{
			properties: props,
		}, nil
	case *BitbucketProperties:
		return &bitbucketProvider{
			properties: props,
		}, nil
	default:
		return nil, &ErrUnknownProvider{unknownProviderProperties: props}
	}
}

// PipelineSchemaMajorVersion is the major version number
// of the pipeline manifest schema
type PipelineSchemaMajorVersion int

const (
	// Ver1 is the current schema major version of the pipelines/*/manifest.yml file.
	Ver1 PipelineSchemaMajorVersion = iota + 1
)

// Pipeline contains information that defines the relationship
// and deployment ordering of your environments.
type Pipeline struct {
	// Name of the pipeline
	Name    string                     `yaml:"name"`
	Version PipelineSchemaMajorVersion `yaml:"version"`
	Source  *Source                    `yaml:"source"`
	Build   *Build                     `yaml:"build"`
	Stages  []PipelineStage            `yaml:"stages"`

	parser template.Parser
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	ProviderName string                 `yaml:"provider"`
	Properties   map[string]interface{} `yaml:"properties"`
}

// Build defines the build project to build and test image.
type Build struct {
	Image            string `yaml:"image"`
	Buildspec        string `yaml:"buildspec,omitempty"`
	AdditionalPolicy struct {
		Document yaml.Node `yaml:"PolicyDocument,omitempty"`
	} `yaml:"additional_policy,omitempty"`
}

// PipelineStage represents a stage in the pipeline manifest
type PipelineStage struct {
	Name             string             `yaml:"name"`
	RequiresApproval bool               `yaml:"requires_approval,omitempty"`
	TestCommands     []string           `yaml:"test_commands,omitempty"`
	Deployments      Deployments        `yaml:"deployments,omitempty"`
	PreDeployments   PrePostDeployments `yaml:"pre_deployments,omitempty"`
	PostDeployments  PrePostDeployments `yaml:"post_deployments,omitempty"`
}

// Deployments represent a directed graph of cloudformation deployments.
type Deployments map[string]*Deployment

// Deployment is a cloudformation stack deployment configuration.
type Deployment struct {
	StackName      string   `yaml:"stack_name"`
	TemplatePath   string   `yaml:"template_path"`
	TemplateConfig string   `yaml:"template_config"`
	DependsOn      []string `yaml:"depends_on"`
}

// PrePostDeployments represent a directed graph of cloudformation deployments.
type PrePostDeployments map[string]*PrePostDeployment

// PrePostDeployment is the config for a pre- or post-deployment action backed by CodeBuild.
type PrePostDeployment struct {
	BuildspecPath string   `yaml:"buildspec"`
	DependsOn     []string `yaml:"depends_on"`
}

// NewPipeline returns a pipeline manifest object.
func NewPipeline(pipelineName string, provider Provider, stages []PipelineStage) (*Pipeline, error) {
	// TODO: #221 Do more validations
	if len(stages) == 0 {
		return nil, fmt.Errorf("a pipeline %s can not be created without a deployment stage",
			pipelineName)
	}

	return &Pipeline{
		Name:    pipelineName,
		Version: Ver1,
		Source: &Source{
			ProviderName: provider.Name(),
			Properties:   provider.Properties(),
		},
		Stages: stages,

		parser: template.New(),
	}, nil
}

// MarshalBinary serializes the pipeline manifest object into byte array that
// represents the pipeline.yml document.
func (m *Pipeline) MarshalBinary() ([]byte, error) {
	content, err := m.parser.Parse(pipelineManifestPath, *m)
	if err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

// UnmarshalPipeline deserializes the YAML input stream into a pipeline
// manifest object. It returns an error if any issue occurs during
// deserialization or the YAML input contains invalid fields.
func UnmarshalPipeline(in []byte) (*Pipeline, error) {
	pm := Pipeline{}
	err := yaml.Unmarshal(in, &pm)
	if err != nil {
		return nil, err
	}

	var version PipelineSchemaMajorVersion
	if version, err = validateVersion(&pm); err != nil {
		return nil, err
	}
	switch version {
	case Ver1:
		return &pm, nil
	}
	// we should never reach here, this is just to make the compiler happy
	return nil, errors.New("unexpected error occurs while unmarshalling manifest.yml")
}

// IsCodeStarConnection indicates to the manifest if this source requires a CSC connection.
func (s Source) IsCodeStarConnection() bool {
	switch s.ProviderName {
	case GithubProviderName:
		return true
	case BitbucketProviderName:
		return true
	default:
		return false
	}
}

func validateVersion(pm *Pipeline) (PipelineSchemaMajorVersion, error) {
	switch pm.Version {
	case Ver1:
		return Ver1, nil
	default:
		return pm.Version,
			&ErrInvalidPipelineManifestVersion{
				invalidVersion: pm.Version,
			}
	}
}
