// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// CreateApplication instantiates a new application within an existing project. Skip if
// the application already exists in the project.
func (s *Store) CreateApplication(app *archer.Application) error {
	if _, err := s.GetProject(app.Project); err != nil {
		return err
	}

	applicationPath := fmt.Sprintf(fmtAppParamPath, app.Project, app.Name)
	data, err := marshal(app)
	if err != nil {
		return fmt.Errorf("serializing application %s: %w", app.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(applicationPath),
		Description: aws.String(fmt.Sprintf("ECS-CLI v2 Application %s", app.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return nil
			}
		}
		return fmt.Errorf("create application %s in project %s: %w", app.Name, app.Project, err)
	}
	return nil
}

// GetApplication gets an application belonging to a particular project by name. If no app is found
// it returns ErrNoSuchApplication.
func (s *Store) GetApplication(projectName, appName string) (*archer.Application, error) {
	appPath := fmt.Sprintf(fmtAppParamPath, projectName, appName)
	appParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(appPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchApplication{
					ProjectName:     projectName,
					ApplicationName: appName,
				}
			}
		}
		return nil, fmt.Errorf("get application %s in project %s: %w", appName, projectName, err)
	}

	var app archer.Application
	err = json.Unmarshal([]byte(*appParam.Parameter.Value), &app)
	if err != nil {
		return nil, fmt.Errorf("read details for application %s in project %s: %w", appName, projectName, err)
	}
	return &app, nil
}

// ListApplications returns all applications belonging to a particular project.
func (s *Store) ListApplications(projectName string) ([]*archer.Application, error) {
	var applications []*archer.Application

	applicationsPath := fmt.Sprintf(rootAppParamPath, projectName)
	serializedApps, err := s.listParams(applicationsPath)
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", projectName, err)
	}
	for _, serializedApp := range serializedApps {
		var app archer.Application
		if err := json.Unmarshal([]byte(*serializedApp), &app); err != nil {
			return nil, fmt.Errorf("read application details for project %s: %w", projectName, err)
		}

		applications = append(applications, &app)
	}
	return applications, nil
}

// DeleteApplication removes an application from SSM.
// If the application does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteApplication(projectName, appName string) error {
	paramName := fmt.Sprintf(fmtAppParamPath, projectName, appName)
	_, err := s.ssmClient.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil
			}
		}
		return fmt.Errorf("delete application %s from project %s: %w", appName, projectName, err)
	}
	return nil
}
