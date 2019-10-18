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

// Provider represents the possible source provider
// for this pipeline.
type Provider int

// this is definitely not thread-safe but we'll deal with it
// when there's a need
var providerSpecificProperties map[Provider]interface{} = make(map[Provider]interface{})

const (
	// Github is the provider that sources from a Github repository.
	Github Provider = iota + 1
)

// GithubProperties contain information to configure a Github source provider.
type GithubProperties struct {
	// use tag from https://godoc.org/github.com/fatih/structs#example-Map--Tags
	// to specify the name of the field in the output properties
	Repository string `structs:"repository"`
}

func (p Provider) String() string {
	names := [...]string{
		"uninitialized",
		"github",
	}

	if int(p) < 0 || int(p) >= len(names) {
		return "unknown"
	}

	return names[p]
}

// ConfiguredWith allows configuring the provider with provider-specific
// properties.
func (p Provider) ConfiguredWith(newProps interface{}) error {
	if props, exists := providerSpecificProperties[p]; exists {
		return fmt.Errorf("the provider is already configured, provider: %s, new properties: %v, old properties: %v",
			p, newProps, props)
	}

	switch p {
	case Github:
		_, ok := newProps.(*GithubProperties)
		if !ok {
			return &ErrProviderPropertiesMismatch{
				provider: p,
				newProps: newProps,
			}
		}
	default:
		return errors.New("unknown provider")
	}

	providerSpecificProperties[p] = newProps
	return nil
}

func (p Provider) properties() (map[string]interface{}, error) {
	if props, exists := providerSpecificProperties[p]; exists {
		return structs.Map(props), nil
	}
	return nil, &ErrMissingProviderProperties{
		provider: p,
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
	Version      PipelineSchemaMajorVersion `yaml:"version"`
	Source       *Source                    `yaml:"source"`
	Environments []PipelineStage            `yaml:"stages"`
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	ProviderName string                 `yaml:"provider"`
	Properties   map[string]interface{} `yaml:"properties"`
}

// PipelineStage represents configuration for each deployment stage
// of an application.
type PipelineStage struct {
	Name string `yaml:"env"`
}

// CreatePipeline returns a pipeline manifest object.
func CreatePipeline(provider Provider, stages ...PipelineStage) (archer.Manifest, error) {
	// TODO: #221 Do more validations
	props, err := provider.properties()
	if err != nil {
		return nil, err
	}

	var defaultStages []PipelineStage
	if len(stages) == 0 {
		defaultStages = []PipelineStage{
			{
				Name: "Test",
			},
			{
				Name: "Prod",
			},
		}
	}

	return &PipelineManifest{
		Version: Ver1,
		Source: &Source{
			ProviderName: provider.String(),
			Properties:   props,
		},
		Environments: append(defaultStages, stages...),
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
func UnmarshalPipeline(in []byte) (archer.Manifest, error) {
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
				version: pm.Version,
			}
	}
}

// CFNTemplate serializes the manifest object into a CloudFormation template.
func (m *PipelineManifest) CFNTemplate() (string, error) {
	// TODO: #223 Generate CFN template for the archer pipeline
	return "", nil
}
