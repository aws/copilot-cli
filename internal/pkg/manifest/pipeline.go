// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/fatih/structs"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/templates"
)

const (
	GithubProviderName    = "GitHub"
	GithubSecretIdKeyName = "access_token_secret"
)

// Provider defines a source of the artifacts
// that will be built and deployed via a pipeline
type Provider interface {
	fmt.Stringer
	Name() string
	Properties() map[string]interface{}
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

// GitHubProperties contain information for configuring a Github
// source provider.
type GitHubProperties struct {
	// use tag from https://godoc.org/github.com/fatih/structs#example-Map--Tags
	// to specify the name of the field in the output properties

	// An example for OwnerAndRepository would be: "aws/amazon-ecs-cli-v2"
	OwnerAndRepository    string `structs:"repository" yaml:"repository"`
	Branch                string `structs:"branch" yaml:"branch"`
	GithubSecretIdKeyName string `structs:"access_token_secret" yaml:"access_token_secret` // TODO fix naming
}

// NewProvider creates a source provider based on the type of
// the provided provider-specific configurations
func NewProvider(configs interface{}) (Provider, error) {
	switch props := configs.(type) {
	case *GitHubProperties:
		return &githubProvider{
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
	// Ver1 is the current schema major version of the pipeline.yml file.
	Ver1 PipelineSchemaMajorVersion = iota + 1
)

// PipelineManifest contains information that defines the relationship
// and deployment ordering of your environments.
type PipelineManifest struct {
	// Name of the pipeline
	Name    string                     `yaml:"name"`
	Version PipelineSchemaMajorVersion `yaml:"version"`
	Source  *Source                    `yaml:"source"`
	Stages  []PipelineStage            `yaml:"stages"`
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	ProviderName string                 `yaml:"provider"`
	Properties   map[string]interface{} `yaml:"properties"`
}

// App defines configurations related to how an app should be deployed, verified
// in the pipeline
type App struct {
	IntegTestBuildspecPath string `structs:"integrationTestBuildspec" yaml:"integrationTestBuildspec"`
}

// Properties return the configurations as a map
func (a App) Properties() map[string]interface{} {
	return structs.Map(a)
}

// PipelineStage represents a stage in the pipeline manifest
type PipelineStage struct {
	Name string         `yaml:"name`
	Apps map[string]App `yaml:"apps"`
}

// CreatePipeline returns a pipeline manifest object.
func CreatePipeline(
	pipelineName string,
	provider Provider,
	stageNames []string,
	apps []archer.Manifest) (*PipelineManifest, error) {

	// TODO: #221 Do more validations
	if len(stageNames) == 0 {
		return nil, fmt.Errorf("a pipeline %s can not be created without a deployment stage",
			pipelineName)
	}

	if len(apps) == 0 {
		return nil, fmt.Errorf("a pipeline %s can not be created without any app to deploy",
			pipelineName)
	}

	appsInPipeline := make(map[string]App)
	for _, app := range apps {
		appsInPipeline[app.AppName()] = App{
			IntegTestBuildspecPath: app.IntegTestBuildspecPath(),
		}
	}

	stages := make([]PipelineStage, 0, len(stageNames))
	for _, name := range stageNames {
		stages = append(stages, PipelineStage{
			Name: name,
			Apps: appsInPipeline,
		})
	}

	return &PipelineManifest{
		Name:    pipelineName,
		Version: Ver1,
		Source: &Source{
			ProviderName: provider.Name(),
			Properties:   provider.Properties(),
		},
		Stages: stages,
	}, nil
}

// Marshal serializes the pipeline manifest object into byte array that
// represents the pipeline.yml document.
func (m *PipelineManifest) Marshal() ([]byte, error) {
	box := templates.Box()
	content, err := box.FindString("cicd/pipeline.yml")
	if err != nil {
		return nil, err
	}
	tpl, err := template.New("pipelineTemplate").Parse(content)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, *m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalPipeline deserializes the YAML input stream into a pipeline
// manifest object. It returns an error if any issue occurs during
// deserialization or the YAML input contains invalid fields.
func UnmarshalPipeline(in []byte) (*PipelineManifest, error) {
	pm := PipelineManifest{}
	err := yaml.Unmarshal(in, &pm)
	if err != nil {
		return nil, err
	}

	var version PipelineSchemaMajorVersion
	if version, err = validateVersion(&pm); err != nil {
		return nil, err
	}

	// TODO: #221 Do more validations
	switch version {
	case Ver1:
		return &pm, nil
	}
	// we should never reach here, this is just to make the compiler happy
	return nil, errors.New("unexpected error occurs while unmarshalling pipeline.yml")
}

func validateVersion(pm *PipelineManifest) (PipelineSchemaMajorVersion, error) {
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
