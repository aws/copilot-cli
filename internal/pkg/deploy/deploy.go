// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
package deploy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	// AppTagKey is tag key for Copilot app.
	AppTagKey = "copilot-application"
	// EnvTagKey is tag key for Copilot env.
	EnvTagKey = "copilot-environment"
	// ServiceTagKey is tag key for Copilot svc.
	ServiceTagKey = "copilot-service"
)

const (
	ecsServiceResourceType = "AWS::ECS::Service"
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

type tagGetter interface {
	GetTags(arn string) (map[string]string, error)
}

// ConfigStoreClient wraps config store methods utilized by deploy store.
type ConfigStoreClient interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	GetService(appName, svcName string) (*config.Service, error)
}

// Store fetches information on deployed services.
type Store struct {
	rgClient              resourceGetter
	configStore           ConfigStoreClient
	ecsClient             tagGetter
	newClientFromIDs      func(string, string) error
	newClientFromRole     func(string, string) error
	newEcsServiceFromRole func(string, string) error
}

// NewStore returns a new store.
func NewStore(store ConfigStoreClient) (*Store, error) {
	s := &Store{
		configStore: store,
	}
	s.newClientFromIDs = func(appName, envName string) error {
		env, err := s.configStore.GetEnvironment(appName, envName)
		if err != nil {
			return fmt.Errorf("get environment config %s: %w", envName, err)
		}
		sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return fmt.Errorf("create new session from env role: %w", err)
		}
		s.ecsClient = ecs.New(sess)
		s.rgClient = rg.New(sess)
		return nil
	}
	s.newClientFromRole = func(roleARN, region string) error {
		sess, err := session.NewProvider().FromRole(roleARN, region)
		if err != nil {
			return fmt.Errorf("create new session from env role: %w", err)
		}
		s.ecsClient = ecs.New(sess)
		s.rgClient = rg.New(sess)
		return nil
	}
	return s, nil
}

// ListDeployedServices returns the names of deployed services in an environment part of an application.
func (s *Store) ListDeployedServices(appName string, envName string) ([]string, error) {
	err := s.newClientFromIDs(appName, envName)
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
	svcs := make([]string, len(svcARNs))
	for ind, svcARN := range svcARNs {
		svcName, err := s.getServiceName(svcARN, appName, envName)
		if err != nil {
			return nil, err
		}
		svc, err := s.configStore.GetService(appName, svcName)
		if err != nil {
			return nil, fmt.Errorf("get service %s: %w", svcName, err)
		}
		svcs[ind] = svc.Name
	}
	return svcs, nil
}

// ListEnvironmentsDeployedTo returns all the environment that a service is deployed in.
func (s *Store) ListEnvironmentsDeployedTo(appName string, svcName string) ([]string, error) {
	envs, err := s.configStore.ListEnvironments(appName)
	if err != nil {
		return nil, fmt.Errorf("list environment for app %s: %w", appName, err)
	}
	var envsWithDeployment []string
	for _, env := range envs {
		err := s.newClientFromRole(env.ManagerRoleARN, env.Region)
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
			envsWithDeployment = append(envsWithDeployment, env.Name)
		}
	}
	return envsWithDeployment, nil
}

// IsDeployed returns whether a service is deployed in an environment or not.
func (s *Store) IsDeployed(appName string, envName string, svcName string) (bool, error) {
	err := s.newClientFromIDs(appName, envName)
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

// // getServiceName gets the ECS service name given a specific ARN.
// // For example: arn:aws:ecs:us-west-2:123456789012:service/my-app-test-Cluster-cdgG2k6XIBtM/my-app-test-my-svc-Service-WLYA7MGACV1F
// // returns my-service
// func (s *Store) getServiceName(svcARN string) (string, error) {
// 	tags, err := s.ecsClient.GetTags(svcARN)
// 	if err != nil {
// 		return "", fmt.Errorf("get tags for ECS service: %w", err)
// 	}
// 	if _, ok := tags[ServiceTagKey]; !ok {
// 		return "", fmt.Errorf("service with ARN %s is not tagged with %s", svcARN, ServiceTagKey)
// 	}
// 	return tags[ServiceTagKey], nil
// }

// getServiceName gets the ECS service name given a specific ARN.
// For example: arn:aws:ecs:us-west-2:123456789012:service/my-app-test-Cluster-cdgG2k6XIBtM/my-app-test-my-svc-Service-WLYA7MGACV1F
// returns my-service
func (s *Store) getServiceName(svcARN, appName, envName string) (string, error) {
	appEnvName := fmt.Sprintf("%s-%s", appName, envName)
	re := regexp.MustCompile(fmt.Sprintf("%s-[a-zA-Z0-9-]+-Service", appEnvName))
	match := re.FindAllString(svcARN, -1)
	if len(match) != 1 {
		return "", fmt.Errorf("cannot parse service ARN %s to get service name", svcARN)
	}
	return strings.TrimSuffix(strings.TrimPrefix(match[0], appEnvName+"-"), "-Service"), nil
}
