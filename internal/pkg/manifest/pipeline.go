// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/fatih/structs"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	archerCfn "github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
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
	return "Github"
}

func (p *githubProvider) String() string {
	return "Github"
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
	// Name of the project this pipeline belongs to
	ProjectName string `yaml:"-"`
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

// CreatePipeline returns a pipeline manifest object.
func CreatePipeline(pipelineName string, provider Provider, stages ...PipelineStage) (archer.Pipeline, error) {
	// TODO: #221 Do more validations
	if len(stages) == 0 {
		return nil, fmt.Errorf("a pipeline %s can not be created without a deployment stage",
			pipelineName)
	}

	var projectName = stages[0].ProjectName
	for _, s := range stages[1:] {
		if s.ProjectName != projectName {
			return nil, fmt.Errorf("failed to create a pipieline that is associated with multiple projects, found at least: [%s, %s]",
				projectName, s.ProjectName)
		}
	}

	return &pipelineManifest{
		ProjectName: stages[0].ProjectName,
		Name:        pipelineName,
		Version:     Ver1,
		Source: &Source{
			ProviderName: provider.Name(),
			Properties:   provider.Properties(),
		},
		Stages: stages,
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
func UnmarshalPipeline(in []byte) (archer.Pipeline, error) {
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

func (m *pipelineManifest) StackName() string {
	return m.ProjectName + "-" + m.Name
}

func (m *pipelineManifest) Template() (string, error) {
	return "", nil
}

func (m *pipelineManifest) Parameters() []*cloudformation.Parameter {
	return nil
}

func (m *pipelineManifest) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(archerCfn.ProjectTagKey),
			Value: aws.String(m.ProjectName),
		},
	}
}
