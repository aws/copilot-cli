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

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/aws/session"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	awsSsm "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// Parameter name formats for resources in a project. Projects are laid out in SSM
// based on path - each parameter's key has a certain format, and you can have
// heirarchies based on that format. Projects are at the root of the heirarchy.
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
)

// SSM store is in charge of fetching and creating Projects, Environment and Pipeline
// configuration in SSM.
type SSM struct {
	systemManager ssmiface.SSMAPI
	tokenService  stsiface.STSAPI
	sessionRegion string
}

// NewStore returns a Store allowing you to query or create Projects or Environments.
func NewStore() (*SSM, error) {
	sess, err := session.Default()

	if err != nil {
		return nil, err
	}

	return &SSM{
		systemManager: awsSsm.New(sess),
		tokenService:  sts.New(sess),
		sessionRegion: *sess.Config.Region,
	}, nil
}

// CreateProject instantiates a new project, validates its uniqueness and stores it in SSM.
func (s *SSM) CreateProject(project *archer.Project) error {
	projectPath := fmt.Sprintf(fmtProjectPath, project.Name)
	project.Version = schemaVersion

	data, err := marshal(project)
	if err != nil {
		return err
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
		return err
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
		return nil, err
	}

	if projectParam.Parameter.Value == nil {

	}
	var project archer.Project
	err = json.Unmarshal([]byte(*projectParam.Parameter.Value), &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// ListProjects returns the list of existing projects in the customer's account and region.
func (s *SSM) ListProjects() ([]*archer.Project, error) {
	params, err := s.systemManager.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:      aws.String(rootProjectPath),
		Recursive: aws.Bool(false),
	})

	if err != nil {
		return nil, err
	}

	var projects []*archer.Project
	for _, param := range params.Parameters {
		var project archer.Project
		err := json.Unmarshal([]byte(*param.Value), &project)

		if err != nil {
			return nil, err
		}
		projects = append(projects, &project)
	}

	return projects, nil
}

// CreateEnvironment instanciates a new environment within an existing project. Returns ErrEnvironmentAlreadyExists
// if the environment already exists in the project.
func (s *SSM) CreateEnvironment(environment *archer.Environment) error {
	_, err := s.GetProject(environment.Project)
	if err != nil {
		return err
	}
	environmentPath := fmt.Sprintf(fmtEnvParamPath, environment.Project, environment.Name)
	data, err := marshal(environment)
	if err != nil {
		return err
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
		return err
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
		return nil, err
	}

	if environmentParam.Parameter.Value == nil {
		return nil, &store.ErrNoSuchEnvironment{
			ProjectName:     projectName,
			EnvironmentName: environmentName,
		}
	}

	var env archer.Environment
	err = json.Unmarshal([]byte(*environmentParam.Parameter.Value), &env)
	return &env, err
}

// ListEnvironments returns all environments belonging to a particular project.
func (s *SSM) ListEnvironments(projectName string) ([]*archer.Environment, error) {
	environmentsPath := fmt.Sprintf(rootEnvParamPath, projectName)
	params, err := s.systemManager.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:      aws.String(environmentsPath),
		Recursive: aws.Bool(false),
	})

	if err != nil {
		return nil, err
	}

	var environments []*archer.Environment
	for _, param := range params.Parameters {
		var env archer.Environment
		err := json.Unmarshal([]byte(*param.Value), &env)

		if err != nil {
			return nil, err
		}

		environments = append(environments, &env)
	}

	return environments, nil
}

// Retrieves the caller's Account ID with a best effort. If it fails to fetch the Account ID,
// this returns "unknown".
func (s *SSM) getCallerAccountAndRegion() (string, string) {
	identity, err := s.tokenService.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	region := s.sessionRegion
	if err != nil {
		log.Printf("Failed to get caller's Account ID %v", err)
		return "unknown", region
	}
	return *identity.Account, region
}

func marshal(e interface{}) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
