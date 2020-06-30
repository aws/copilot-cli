// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
package deploy

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	ecsServiceResourceType = "AWS::ECS::Service"
	// AppTagKey is tag key for Copilot app.
	AppTagKey = "copilot-application"
	// EnvTagKey is tag key for Copilot env.
	EnvTagKey = "copilot-environment"
	// ServiceTagKey is tag key for Copilot svc.
	ServiceTagKey = "copilot-service"
)

// Resource represents an AWS resource.
type Resource struct {
	LogicalName string
	Type        string
}

// ResourceEvent represents a status update for an AWS resource during a deployment.
type ResourceEvent struct {
	Resource
	Status       string
	StatusReason string
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]string, error)
}

type configStoreClient interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	GetService(appName, svcName string) (*config.Service, error)
}

// Store is in charge of fetching the service deployment information.
type Store struct {
	rgClient     resourceGetter
	configStore  configStoreClient
	initRgClient func(*Store, string, string) error
}

// NewStore returns a new store.
func NewStore() (*Store, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	return &Store{
		configStore: store,
		initRgClient: func(s *Store, appName, envName string) error {
			env, err := s.configStore.GetEnvironment(appName, envName)
			if err != nil {
				return fmt.Errorf("get environment %s, %w", envName, err)
			}
			p := session.NewProvider()
			sess, err := p.FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return fmt.Errorf("create new session from env role: %w", err)
			}
			s.rgClient = rg.New(sess)
			return nil
		},
	}, nil
}

// ListDeployedServices returns all the deployed service in a specific environment of an application.
func (s *Store) ListDeployedServices(appName string, envName string) ([]*config.Service, error) {
	err := s.initRgClient(s, appName, envName)
	if err != nil {
		return nil, err
	}
	svcARNs, err := s.rgClient.GetResourcesByTags(ecsServiceResourceType, map[string]string{
		AppTagKey: appName,
		EnvTagKey: envName,
	})
	if err != nil {
		return nil, fmt.Errorf("get resources by Copilot tags: %w", err)
	}
	svcs := make([]*config.Service, len(svcARNs))
	for ind, svcARN := range svcARNs {
		svcName, err := s.getServiceName(svcARN)
		if err != nil {
			return nil, err
		}
		svc, err := s.configStore.GetService(appName, svcName)
		if err != nil {
			return nil, fmt.Errorf("get service %s: %w", svcName, err)
		}
		svcs[ind] = svc
	}
	return svcs, nil
}

// ListEnvironmentsDeployedTo returns all the environment that a service is deployed in.
func (s *Store) ListEnvironmentsDeployedTo(appName string, svcName string) ([]*config.Environment, error) {
	var envsWithDeployment []*config.Environment
	envs, err := s.configStore.ListEnvironments(appName)
	if err != nil {
		return nil, fmt.Errorf("list environment for app %s: %w", appName, err)
	}
	for _, env := range envs {
		err := s.initRgClient(s, appName, env.Name)
		if err != nil {
			return nil, err
		}
		svcARNs, err := s.rgClient.GetResourcesByTags(ecsServiceResourceType, map[string]string{
			AppTagKey:     appName,
			EnvTagKey:     env.Name,
			ServiceTagKey: svcName,
		})
		if err != nil {
			return nil, fmt.Errorf("get resources by Copilot tags: %w", err)
		}
		// If no resources found, the resp length is 0.
		if len(svcARNs) != 0 {
			envsWithDeployment = append(envsWithDeployment, env)
		}
	}
	return envsWithDeployment, nil
}

// IsDeployed returns whether a service is deployed in an environment or not.
func (s *Store) IsDeployed(appName string, envName string, svcName string) (bool, error) {
	err := s.initRgClient(s, appName, envName)
	if err != nil {
		return false, err
	}
	svcARNs, err := s.rgClient.GetResourcesByTags(ecsServiceResourceType, map[string]string{
		AppTagKey:     appName,
		EnvTagKey:     envName,
		ServiceTagKey: svcName,
	})
	if err != nil {
		return false, fmt.Errorf("get resources by Copilot tags: %w", err)
	}
	if len(svcARNs) != 0 {
		return true, nil
	}
	return false, nil
}

// getServiceName gets the ECS service name given a specific ARN.
// For example: arn:aws:ecs:us-west-2:123456789012:service/my-http-service
// returns my-http-service
func (s *Store) getServiceName(svcARN string) (string, error) {
	resp, err := arn.Parse(svcARN)
	if err != nil {
		return "", fmt.Errorf("parse service ARN %s: %w", svcARN, err)
	}
	resource := strings.Split(resp.Resource, "/")
	if len(resource) != 2 {
		return "", fmt.Errorf(`cannot parse service ARN resource "%s"`, resp.Resource)
	}
	return resource[1], nil
}
