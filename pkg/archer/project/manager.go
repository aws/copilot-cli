// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// Manager handles actions on all of your projects.
type Manager struct {
	c ssmiface.SSMAPI
}

// NewManager returns a new manager supervising your projects using your default AWS config.
// // See https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
func NewManager() (*Manager, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	return &Manager{
		c: ssm.New(sess),
	}, nil
}

// List returns the list of your existing projects.
// If an error occurs while listing, an empty slice is returned along with the error.
func (m *Manager) List() ([]*Project, error) {
	// TODO paginate
	resp, err := m.c.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:      aws.String("archer/"),
		Recursive: aws.Bool(true),
	})
	if err != nil {
		return []*Project{}, err
	}

	projectNames := make(map[string]bool)
	for _, param := range resp.Parameters {
		projectName := strings.Split(*param.Name, "/")[1]
		projectNames[projectName] = true
	}
	var projects []*Project
	for name, _ := range projectNames {
		p, err := New(name)
		if err != nil {
			return []*Project{}, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}
