// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// CreateProject instantiates a new project, validates its uniqueness and stores it in SSM.
func (s *Store) CreateProject(project *archer.Project) error {
	projectPath := fmt.Sprintf(fmtProjectPath, project.Name)
	project.Version = schemaVersion

	data, err := marshal(project)
	if err != nil {
		return fmt.Errorf("serializing project %s: %w", project.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(projectPath),
		Description: aws.String("An ECS-CLI Project"),
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
		return fmt.Errorf("create project %s: %w", project.Name, err)
	}
	return nil
}

// GetProject fetches a project by name. If it can't be found, return a ErrNoSuchProject
func (s *Store) GetProject(projectName string) (*archer.Project, error) {
	projectPath := fmt.Sprintf(fmtProjectPath, projectName)
	projectParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(projectPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				account, region := s.getCallerAccountAndRegion()
				return nil, &ErrNoSuchProject{
					ProjectName: projectName,
					AccountID:   account,
					Region:      region,
				}
			}
		}
		return nil, fmt.Errorf("get project %s: %w", projectName, err)
	}

	var project archer.Project
	if err := json.Unmarshal([]byte(*projectParam.Parameter.Value), &project); err != nil {
		return nil, fmt.Errorf("read details for project %s: %w", projectName, err)
	}
	return &project, nil
}

// ListProjects returns the list of existing projects in the customer's account and region.
func (s *Store) ListProjects() ([]*archer.Project, error) {
	var projects []*archer.Project
	serializedProjects, err := s.listParams(rootProjectPath)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	for _, serializedProject := range serializedProjects {
		var project archer.Project
		if err := json.Unmarshal([]byte(*serializedProject), &project); err != nil {
			return nil, fmt.Errorf("read project details: %w", err)
		}

		projects = append(projects, &project)
	}
	return projects, nil
}

// DeleteProject deletes the SSM parameter related to the project.
func (s *Store) DeleteProject(name string) error {
	paramName := fmt.Sprintf(fmtProjectPath, name)

	_, err := s.ssmClient.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})

	if err != nil {
		awserr, ok := err.(awserr.Error)
		if !ok {
			return err
		}

		if awserr.Code() == ssm.ErrCodeParameterNotFound {
			return nil
		}

		return fmt.Errorf("delete SSM param %s: %w", paramName, awserr)
	}

	return nil
}
