// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Package ssm implements CRUD operations for Package, Environment and
Pipeline configuration. This configuration contains the archer projects
a customer has, and the environments and pipelines associated with each
project.
*/
package ssm

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// Parameter name formats for resources in a project. Projects are laid out in SSM
// based on path - each parameter's key has a certain format, and you can have
// heirarchies based on that format. Projects are at the root of the hierarchy.
// Searching SSM for all parameters with the `rootProjectPath` key will give you
// all the project keys, for example.

// current schema Version for Projects
const schemaVersion = "1.0"

// schema formats supported in current schemaVersion. NOTE: May change to map in the future.
const (
	rootProjectPath  = "/archer/"
	fmtProjectPath   = "/archer/%s"
	rootEnvParamPath = "/archer/%s/environments/"
	fmtEnvParamPath  = "/archer/%s/environments/%s" // path for an environment in a project
	rootAppParamPath = "/archer/%s/applications/"
	fmtAppParamPath  = "/archer/%s/applications/%s" // path for an application in a project
)

type identityService interface {
	Get() (identity.Caller, error)
}

// SSM store is in charge of fetching and creating Projects, Environment and Pipeline
// configuration in SSM.
type SSM struct {
	systemManager ssmiface.SSMAPI
	identity      identityService
	sessionRegion string
}

// NewStore returns a Store allowing you to query or create Projects or Environments.
func NewStore() (*SSM, error) {
	sess, err := session.Default()

	if err != nil {
		return nil, err
	}

	return &SSM{
		systemManager: ssm.New(sess),
		identity:      identity.New(sess),
		sessionRegion: *sess.Config.Region,
	}, nil
}

// CreateProject instantiates a new project, validates its uniqueness and stores it in SSM.
func (s *SSM) CreateProject(project *archer.Project) error {
	projectPath := fmt.Sprintf(fmtProjectPath, project.Name)
	project.Version = schemaVersion

	data, err := marshal(project)
	if err != nil {
		return fmt.Errorf("serializing project %s: %w", project.Name, err)
	}

	_, err = s.systemManager.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(projectPath),
		Description: aws.String("An ECS-CLI Project"),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return &store.ErrProjectAlreadyExists{
					ProjectName: project.Name,
				}
			}
		}
		return fmt.Errorf("create project %s: %w", project.Name, err)
	}
	return nil
}

// GetProject fetches a project by name. If it can't be found, return a ErrNoSuchProject
func (s *SSM) GetProject(projectName string) (*archer.Project, error) {
	projectPath := fmt.Sprintf(fmtProjectPath, projectName)
	projectParam, err := s.systemManager.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(projectPath),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				account, region := s.getCallerAccountAndRegion()
				return nil, &store.ErrNoSuchProject{
					ProjectName: projectName,
					AccountID:   account,
					Region:      region,
				}
			}
		}
		return nil, fmt.Errorf("get project %s: %w", projectName, err)
	}

	if projectParam.Parameter.Value == nil {

	}
	var project archer.Project
	if err := json.Unmarshal([]byte(*projectParam.Parameter.Value), &project); err != nil {
		return nil, fmt.Errorf("read details for project %s: %w", projectName, err)
	}
	return &project, nil
}

// ListProjects returns the list of existing projects in the customer's account and region.
func (s *SSM) ListProjects() ([]*archer.Project, error) {
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

// CreateEnvironment instantiates a new environment within an existing project. Returns ErrEnvironmentAlreadyExists
// if the environment already exists in the project.
func (s *SSM) CreateEnvironment(environment *archer.Environment) error {
	if _, err := s.GetProject(environment.Project); err != nil {
		return err
	}

	environmentPath := fmt.Sprintf(fmtEnvParamPath, environment.Project, environment.Name)
	data, err := marshal(environment)
	if err != nil {
		return fmt.Errorf("serializing environment %s: %w", environment.Name, err)
	}

	paramOutput, err := s.systemManager.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(environmentPath),
		Description: aws.String(fmt.Sprintf("The %s deployment stage", environment.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return &store.ErrEnvironmentAlreadyExists{
					EnvironmentName: environment.Name,
					ProjectName:     environment.Project}
			}
		}
		return fmt.Errorf("create environment %s in project %s: %w", environment.Name, environment.Project, err)
	}

	log.Printf("Created Environment with version %v", *paramOutput.Version)
	return nil

}

// GetEnvironment gets an environment belonging to a particular project by name. If no environment is found
// it returns ErrNoSuchEnvironment.
func (s *SSM) GetEnvironment(projectName string, environmentName string) (*archer.Environment, error) {
	environmentPath := fmt.Sprintf(fmtEnvParamPath, projectName, environmentName)
	environmentParam, err := s.systemManager.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(environmentPath),
	})

	if err != nil {
		return nil, fmt.Errorf("get environment %s in project %s: %w", environmentName, projectName, err)
	}

	if environmentParam.Parameter.Value == nil {
		return nil, &store.ErrNoSuchEnvironment{
			ProjectName:     projectName,
			EnvironmentName: environmentName,
		}
	}

	var env archer.Environment
	err = json.Unmarshal([]byte(*environmentParam.Parameter.Value), &env)
	if err != nil {
		return nil, fmt.Errorf("read details for environment %s in project %s: %w", environmentName, projectName, err)
	}
	return &env, nil
}

// ListEnvironments returns all environments belonging to a particular project.
func (s *SSM) ListEnvironments(projectName string) ([]*archer.Environment, error) {
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
	return environments, nil
}

// CreateApplication instantiates a new application within an existing project. Returns ErrApplicationAlreadyExists
// if the application already exists in the project.
func (s *SSM) CreateApplication(app *archer.Application) error {
	if _, err := s.GetProject(app.Project); err != nil {
		return err
	}

	applicationPath := fmt.Sprintf(fmtAppParamPath, app.Project, app.Name)
	data, err := marshal(app)
	if err != nil {
		return fmt.Errorf("serializing application %s: %w", app.Name, err)
	}

	paramOutput, err := s.systemManager.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(applicationPath),
		Description: aws.String(fmt.Sprintf("ECS-CLI v2 Application %s", app.Name)),
		Type:        aws.String(ssm.ParameterTypeString),
		Value:       aws.String(data),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterAlreadyExists:
				return &store.ErrApplicationAlreadyExists{
					ApplicationName: app.Name,
					ProjectName:     app.Project}
			}
		}
		return fmt.Errorf("create application %s in project %s: %w", app.Name, app.Project, err)
	}

	log.Printf("Created Application with version %v", *paramOutput.Version)
	return nil

}

// GetApplication gets an application belonging to a particular project by name. If no app is found
// it returns ErrNoSuchApplication.
func (s *SSM) GetApplication(projectName, appName string) (*archer.Application, error) {
	appPath := fmt.Sprintf(fmtAppParamPath, projectName, appName)
	appParam, err := s.systemManager.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(appPath),
	})

	if err != nil {
		return nil, fmt.Errorf("get application %s in project %s: %w", appName, projectName, err)
	}

	if appParam.Parameter.Value == nil {
		return nil, &store.ErrNoSuchApplication{
			ProjectName:     projectName,
			ApplicationName: appName,
		}
	}

	var app archer.Application
	err = json.Unmarshal([]byte(*appParam.Parameter.Value), &app)
	if err != nil {
		return nil, fmt.Errorf("read details for application %s in project %s: %w", appName, projectName, err)
	}
	return &app, nil
}

// ListApplications returns all applications belonging to a particular project.
func (s *SSM) ListApplications(projectName string) ([]*archer.Application, error) {
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

func (s *SSM) listParams(path string) ([]*string, error) {
	var serializedParams []*string

	var nextToken *string = nil
	for {
		params, err := s.systemManager.GetParametersByPath(&ssm.GetParametersByPathInput{
			Path:      aws.String(path),
			Recursive: aws.Bool(false),
			NextToken: nextToken,
		})

		if err != nil {
			return nil, err
		}

		for _, param := range params.Parameters {
			serializedParams = append(serializedParams, param.Value)
		}

		nextToken = params.NextToken
		if nextToken == nil {
			break
		}
	}
	return serializedParams, nil
}

// Retrieves the caller's Account ID with a best effort. If it fails to fetch the Account ID,
// this returns "unknown".
func (s *SSM) getCallerAccountAndRegion() (string, string) {
	identity, err := s.identity.Get()
	region := s.sessionRegion
	if err != nil {
		log.Printf("Failed to get caller's Account ID %v", err)
		return "unknown", region
	}
	return identity.Account, region
}

func marshal(e interface{}) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
