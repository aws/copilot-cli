// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Provider defines a source of the artifacts
// that will be built and deployed via a pipeline
type Provider interface {
	fmt.Stringer
	Name() string
	Properties() map[string]interface{}
}

type githubProvider struct {
	properties *GithubProperties
}

func (p *githubProvider) Name() string {
	return "github"
}

func (p *githubProvider) String() string {
	return "github"
}

func (p *githubProvider) Properties() map[string]interface{} {
	return structs.Map(p.properties)
}

// GithubProperties contain information for configuring a Github
// source provider.
type GithubProperties struct {
	// use tag from https://godoc.org/github.com/fatih/structs#example-Map--Tags
	// to specify the name of the field in the output properties
	Repository string `structs:"repository" yaml:"repository"`
	Branch     string `structs:"branch" yaml:"branch"`
}

// NewProvider creates a source provider based on the type of
// the provided provider-specific configurations
func NewProvider(configs interface{}) (Provider, error) {
	switch props := configs.(type) {
	case *GithubProperties:
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

// pipelineManifest contains information that defines the relationship
// and deployment ordering of your environments.
type pipelineManifest struct {
	Version PipelineSchemaMajorVersion `yaml:"version"`
	Source  *Source                    `yaml:"source"`
	Stages  []PipelineStage            `yaml:"stages"`
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	ProviderName string                 `yaml:"provider"`
	Properties   map[string]interface{} `yaml:"properties"`
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Archer environment the pipeline
// is deloying to and the containerized applications that will be deployed.
type PipelineStage struct {
	*AssociatedEnvironment `yaml:",inline"`
	Applications           []*archer.Application `yaml:"-"`
}

// AssociatedEnvironment defines the necessary information a pipline stage
// needs for an Archer environment.
type AssociatedEnvironment struct {
	Project   string `yaml:"-"`    // Name of the project this environment belongs to.
	Name      string `yaml:"name"` // Name of the environment, must be unique within a project.
	Region    string `yaml:"-"`    // Name of the region this environment is stored in.
	AccountID string `yaml:"-"`    // Account ID of the account this environment is stored in.
	Prod      bool   `yaml:"-"`    // Whether or not this environment is a production environment.
}

// CreatePipeline returns a pipeline manifest object.
func CreatePipeline(provider Provider, stages ...PipelineStage) (archer.Manifest, error) {
	// TODO: #221 Do more validations
	var defaultStages []PipelineStage
	if len(stages) == 0 {
		defaultStages = []PipelineStage{
			{
				AssociatedEnvironment: &AssociatedEnvironment{
					Name: "test",
				},
			},
			{
				AssociatedEnvironment: &AssociatedEnvironment{
					Name: "prod",
				},
			},
		}
	}

	return &pipelineManifest{
		Version: Ver1,
		Source: &Source{
			ProviderName: provider.Name(),
			Properties:   provider.Properties(),
		},
		Stages: append(defaultStages, stages...),
	}, nil
}

// Marshal serializes the pipeline manifest object into byte array that
// represents the pipeline.yml document.
func (m *pipelineManifest) Marshal() ([]byte, error) {
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
func UnmarshalPipeline(in []byte) (archer.Manifest, error) {
	pm := pipelineManifest{}
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

func validateVersion(pm *pipelineManifest) (PipelineSchemaMajorVersion, error) {
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

// CFNTemplate serializes the manifest object into a CloudFormation template.
func (m *pipelineManifest) CFNTemplate() (string, error) {
	// TODO: #223 Generate CFN template for the archer pipeline
	return "", nil
}
