// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type parametersByPathGetter interface {
	GetParametersByPath(*ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error)
}

// Store handles actions on all of your projects.
type Store struct {
	c parametersByPathGetter
}

// NewStore returns a new manager supervising your projects using your default AWS config.
// See https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
func NewStore() (*Store, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		c: ssm.New(sess),
	}, nil
}

// List returns the list of your existing projects.
// If an error occurs while listing, a nil slice is returned along with the error.
func (m *Store) List() ([]*Project, error) {
	// TODO paginate
	resp, err := m.c.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path: aws.String(projectsParamPath),
	})
	if err != nil {
		return nil, err
	}

	projectNames := make(map[string]bool)
	for _, param := range resp.Parameters {
		projectName := strings.Split(*param.Name, "/")[2]
		projectNames[projectName] = true
	}
	var projects []*Project
	for name, _ := range projectNames {
		p, err := New(name)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}
