// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package project provides functionality to manager projects.
package project

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/env"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

var (
	ErrNoExistingProjects = errors.New("no projects found")
)

// SSM parameter name formats for resources in a project.
const (
	fmtEnvParamName = "/archer/%s/environments/%s" // name for an environment in a project
)

// Project is a collection of environments.
type Project struct {
	Name string

	c      ssmiface.SSMAPI
	prompt terminal.Stdio
}

// New returns a new named project managing your environments using your default AWS config.
// See https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
func New(name string) (*Project, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	return &Project{
		Name: name,
		c:    ssm.New(sess),
	}, nil
}

// Ask prompts the user for an existing project name if the project's name is empty.
func (p *Project) Ask() error {
	if p.Name != "" {
		// TODO check if the project name exists.
		return nil
	}

	m, err := NewManager()
	if err != nil {
		return err
	}
	projects, err := m.List()
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return ErrNoExistingProjects
	}
	var names []string
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return survey.AskOne(&survey.Select{
		Message: "Which project do you want to create this environment in?",
		Options: names,
		Default: names[0],
	}, &p.Name, survey.WithStdio(p.prompt.In, p.prompt.Out, p.prompt.Err))
}

// AddEnv links a new environment to the project.
func (p *Project) AddEnv(e *env.Environment) error {
	data, err := e.Marshal()
	if err != nil {
		return err
	}
	_, err = p.c.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(fmt.Sprintf(fmtEnvParamName, p.Name, e.Name)),
		Description: aws.String(fmt.Sprintf("The %s deployment stage", e.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})
	// TODO check errors
	return err
}
