// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package project provides functionality to manager projects.
package project

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

var (
	ErrNoExistingProjects = errors.New("no projects found")
)

// SSM parameter name formats for resources in a project.
const (
	fmtProjectsParamPath = "/archer/"                   // path for finding projects
	fmtProjectParamName  = "/archer/%s"                 // name for a project
	fmtEnvParamName      = "/archer/%s/environments/%s" // name for an environment in a project
)

// Project is a collection of environments.
type Project struct {
	Name string

	c ssmiface.SSMAPI
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
