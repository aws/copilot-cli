// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/env"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
)

type projectLister interface {
	List() ([]*Project, error)
}

// AddEnvOpts holds the parameters needed to created a new environment under a project.
type AddEnvOpts struct {
	ProjectName string `survey:"project"`
	EnvName     string `survey:"env"`
	EnvProfile  string

	store  projectLister
	prompt terminal.Stdio
}

// NewAddEnvOpts creates a new option with a "default" AWS profile for the environment.
func NewAddEnvOpts() (*AddEnvOpts, error) {
	m, err := NewStore()
	if err != nil {
		return nil, err
	}
	return &AddEnvOpts{
		EnvProfile: "default",
		store:      m,
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}, nil
}

// Validate returns an error if the project already exists. Otherwise, returns nil.
func (opts *AddEnvOpts) Validate() error {
	if opts.ProjectName == "" {
		return nil
	}
	// TODO check if the project name exists.
	return nil
}

// Ask prompts the user for required parameters to create a new environment in a project if the user hasn't specified them.
func (opts *AddEnvOpts) Ask() error {
	var qs []*survey.Question
	if opts.ProjectName == "" {
		projects, err := opts.store.List()
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve list of existing projects")
		}
		if len(projects) == 0 {
			return ErrNoExistingProjects
		}
		var names []string
		for _, p := range projects {
			names = append(names, p.Name)
		}
		qs = append(qs, &survey.Question{
			Name: "project",
			Prompt: &survey.Select{
				Message: "Which project do you want to create this environment in?",
				Options: names,
				Default: names[0],
			},
			Validate: survey.Required,
		})
	}
	if opts.EnvName == "" {
		qs = append(qs, &survey.Question{
			Name: "env",
			Prompt: &survey.Input{
				Message: "What is your environment's name?",
			},
			Validate: survey.Required,
		})
	}
	return survey.Ask(qs, opts, survey.WithStdio(opts.prompt.In, opts.prompt.Out, opts.prompt.Err))
}

// AddEnv creates a new environment under the project.
// The environment is first linked in SSM to the project, and then the CloudFormation stack is created.
func (p *Project) AddEnv(environ *env.Environment) error {
	// 1. Add the environment to SSM
	data, err := environ.Marshal()
	if err != nil {
		return err
	}
	_, err = p.c.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(fmt.Sprintf(fmtEnvParamName, p.Name, environ.Name)),
		Description: aws.String(fmt.Sprintf("The %s deployment stage", environ.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return &ErrEnvAlreadyExists{Name: environ.Name, Project: p.Name}
			}
		}
		return err
	}
	// 2. Deploy the environment
	return environ.Deploy()
}
