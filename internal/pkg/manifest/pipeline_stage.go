// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Archer Environment the pipeline
// is deloying to and the containerized applications that will be deployed.
type PipelineStage struct {
	*associatedEnvironment `yaml:",inline"`
	LocalApplications      []string `yaml:"-"`
}

// associatedEnvironment defines the necessary information a pipline stage
// needs for an Archer Environment.
type associatedEnvironment struct {
	// Name of the Archer project this environment belongs to
	ProjectName string `yaml:"-"`

	// Name of the environment, must be unique within a project.
	// This is also the name of the pipeline stage.
	Name string `yaml:"name"`
	// T region this environment is stored in.
	Region string `yaml:"-"`
	// AccountID of the account this environment is stored in.
	AccountID string `yaml:"-"`
	// Whether or not this environment is a production environment.
	Prod bool `yaml:"-"`
}

// associatedApp defines the name of an Archer application that will be
// deploye to certain pipeline stage.
type associatedApp struct {
	Name string
}

var fetchAssociatedEnvAndApps func(stageName string) (*associatedEnvironment, []string, error) = func(stageName string) (*associatedEnvironment, []string, error) {
	// TODO: #239 Centralize the initialization of SSM and workspace into
	// a common package
	s, err := ssm.NewStore()
	if err != nil {
		return nil, nil, err
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, nil, err
	}

	summary, err := ws.Summary()
	if err != nil {
		return nil, nil, err
	}

	env, err := s.GetEnvironment(summary.ProjectName, stageName)
	if err != nil {
		return nil, nil, err
	}

	apps, err := ws.AppNames()
	if err != nil {
		return nil, nil, err
	}
	return &associatedEnvironment{
		ProjectName: summary.ProjectName,
		Name:        stageName,
		Region:      env.Region,
		AccountID:   env.AccountID,
		Prod:        env.Prod,
	}, apps, nil
}

// NewPipelineStage creates a pipeline stage that maps to the corresponding
// environment named after the provided `stageName`.
func NewPipelineStage(stageName string) (*PipelineStage, error) {
	env, apps, err := fetchAssociatedEnvAndApps(stageName)
	if err != nil {
		return nil, fmt.Errorf("failed to create the pipeline stage %s, error: %w",
			stageName, err)
	}

	return &PipelineStage{
		associatedEnvironment: env,
		LocalApplications:     apps,
	}, nil
}

// UnmarshalYAML implements the go-yaml Unmarshaler interface for the type,
// PipelineStage, such that the associated environment and
// local applications will be populated after unmarshalling from a pipeline
// manifest file.
func (p *PipelineStage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// have to use an aux type instead of the PipelineStage type. Otherwise,
	// goyaml will try to recursively call `UnmarshalYAML` which leads to
	// stackoverflow
	var aux struct {
		// Name of the environment, must be unique within a project.
		// This is also the name of the pipeline stage.
		Name string `yaml:"name"`
	}
	err := unmarshal(&aux) // parse out the name of the stage
	if err != nil {
		return err
	}

	// TODO: #221 Do more validations
	env, apps, err := fetchAssociatedEnvAndApps(aux.Name)
	if err != nil {
		return fmt.Errorf("failed to unmarshal a pipeline stage %s, error: %w",
			aux.Name, err)
	}

	p.associatedEnvironment = env
	p.LocalApplications = apps
	return nil
}
