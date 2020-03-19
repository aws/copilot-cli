// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// CreateEnvironment instantiates a new environment within an existing project. Skip if
// the environment already exists in the project.
func (s *Store) CreateEnvironment(environment *archer.Environment) error {
	if _, err := s.GetProject(environment.Project); err != nil {
		return err
	}

	environmentPath := fmt.Sprintf(fmtEnvParamPath, environment.Project, environment.Name)
	data, err := marshal(environment)
	if err != nil {
		return fmt.Errorf("serializing environment %s: %w", environment.Name, err)
	}

	_, err = s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(environmentPath),
		Description: aws.String(fmt.Sprintf("The %s deployment stage", environment.Name)),
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
		return fmt.Errorf("create environment %s in project %s: %w", environment.Name, environment.Project, err)
	}
	return nil
}

// GetEnvironment gets an environment belonging to a particular project by name. If no environment is found
// it returns ErrNoSuchEnvironment.
func (s *Store) GetEnvironment(projectName string, environmentName string) (*archer.Environment, error) {
	environmentPath := fmt.Sprintf(fmtEnvParamPath, projectName, environmentName)
	environmentParam, err := s.ssmClient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(environmentPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				return nil, &ErrNoSuchEnvironment{
					ProjectName:     projectName,
					EnvironmentName: environmentName,
				}
			}
		}
		return nil, fmt.Errorf("get environment %s in project %s: %w", environmentName, projectName, err)
	}

	var env archer.Environment
	err = json.Unmarshal([]byte(*environmentParam.Parameter.Value), &env)
	if err != nil {
		return nil, fmt.Errorf("read details for environment %s in project %s: %w", environmentName, projectName, err)
	}
	return &env, nil
}

// ListEnvironments returns all environments belonging to a particular project.
func (s *Store) ListEnvironments(projectName string) ([]*archer.Environment, error) {
	var environments []*archer.Environment

	environmentsPath := fmt.Sprintf(rootEnvParamPath, projectName)
	serializedEnvs, err := s.listParams(environmentsPath)
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", projectName, err)
	}
	for _, serializedEnv := range serializedEnvs {
		var env archer.Environment
		if err := json.Unmarshal([]byte(*serializedEnv), &env); err != nil {
			return nil, fmt.Errorf("read environment details for project %s: %w", projectName, err)
		}

		environments = append(environments, &env)
	}
	// non-prod env before prod env. sort by alphabetically if same
	sort.SliceStable(environments, func(i, j int) bool { return environments[i].Name < environments[i].Name })
	sort.SliceStable(environments, func(i, j int) bool { return !environments[i].Prod })
	return environments, nil
}

// DeleteEnvironment removes an environment from SSM.
// If the environment does not exist in the store or is successfully deleted then returns nil. Otherwise, returns an error.
func (s *Store) DeleteEnvironment(projectName, environmentName string) error {
	paramName := fmt.Sprintf(fmtEnvParamPath, projectName, environmentName)
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
		return fmt.Errorf("delete environment %s from project %s: %w", environmentName, projectName, err)
	}
	return nil
}
