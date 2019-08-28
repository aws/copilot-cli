// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package project provides functionality to manager projects.
package project

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// SSM parameter name formats for resources in a project.
const (
	projectsParamPath = "/archer/"                   // path for finding projects
	fmtEnvParamName   = "/archer/%s/environments/%s" // name for an environment in a project
)

type ssmAPI interface {
	PutParameter(*ssm.PutParameterInput) (*ssm.PutParameterOutput, error)
}

// Project is a collection of environments.
type Project struct {
	Name string

	c ssmAPI
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
