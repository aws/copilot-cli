// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy infrastructure resources.
package deploy

import (
	"fmt"
	"sort"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
)

const (
	// AppTagKey is tag key for Copilot app.
	AppTagKey = "copilot-application"
	// EnvTagKey is tag key for Copilot env.
	EnvTagKey = "copilot-environment"
	// ServiceTagKey is tag key for Copilot svc.
	ServiceTagKey = "copilot-service"
	// TaskTagKey is tag key for Copilot task.
	TaskTagKey = "copilot-task"
)

const (
	stackResourceType = "cloudformation:stack"
	snsResourceType   = "sns"

	// fmtSNSTopicNamePrefix holds the App-Env-Workload- components of a topic name
	fmtSNSTopicNamePrefix = "%s-%s-%s-"
	snsServiceName        = "sns"
)

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

// ConfigStoreClient wraps config store methods utilized by deploy store.
type ConfigStoreClient interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListWorkloads(appName string) ([]*config.Workload, error)
	GetService(appName, svcName string) (*config.Workload, error)
	GetJob(appName, jobname string) (*config.Workload, error)
}

// Store fetches information on deployed services.
type Store struct {
	configStore         ConfigStoreClient
	newRgClientFromIDs  func(string, string) (resourceGetter, error)
	newRgClientFromRole func(string, string) (resourceGetter, error)
}

// NewStore returns a new store.
func NewStore(store ConfigStoreClient) (*Store, error) {
	s := &Store{
		configStore: store,
	}
	s.newRgClientFromIDs = func(appName, envName string) (resourceGetter, error) {
		env, err := s.configStore.GetEnvironment(appName, envName)
		if err != nil {
			return nil, fmt.Errorf("get environment config %s: %w", envName, err)
		}
		sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, fmt.Errorf("create new session from env role: %w", err)
		}
		return rg.New(sess), nil
	}
	s.newRgClientFromRole = func(roleARN, region string) (resourceGetter, error) {
		sess, err := sessions.NewProvider().FromRole(roleARN, region)
		if err != nil {
			return nil, fmt.Errorf("create new session from env role: %w", err)
		}
		return rg.New(sess), nil
	}
	return s, nil
}

// ListDeployedServices returns the names of deployed services in an environment.
func (s *Store) ListDeployedServices(appName string, envName string) ([]string, error) {
	return s.listDeployedWorkloads(appName, envName, manifest.ServiceTypes)
}

// ListDeployedJobs returns the names of deployed jobs in an environment.
func (s *Store) ListDeployedJobs(appName string, envName string) ([]string, error) {
	return s.listDeployedWorkloads(appName, envName, manifest.JobTypes)
}

func (s *Store) listDeployedWorkloads(appName string, envName string, workloadType []string) ([]string, error) {
	allWorkloads, err := s.configStore.ListWorkloads(appName)
	if err != nil {
		return nil, fmt.Errorf("list all workloads in application %s: %w", appName, err)
	}
	filteredWorkloadNames := make(map[string]bool)
	for _, wkld := range allWorkloads {
		for _, t := range workloadType {
			if wkld.Type != t {
				continue
			}
			filteredWorkloadNames[wkld.Name] = true
		}
	}

	rgClient, err := s.newRgClientFromIDs(appName, envName)
	if err != nil {
		return nil, err
	}
	resources, err := rgClient.GetResourcesByTags(stackResourceType, map[string]string{
		AppTagKey: appName,
		EnvTagKey: envName,
	})
	if err != nil {
		return nil, fmt.Errorf("get resources by Copilot tags: %w", err)
	}
	var wklds []string
	for _, resource := range resources {
		name := resource.Tags[ServiceTagKey]
		if name == "" || contains(name, wklds) {
			// To avoid listing duplicate service entry in a case when service has addons stack.
			continue
		}
		if _, ok := filteredWorkloadNames[name]; ok {
			wklds = append(wklds, name)
		}
	}
	sort.Strings(wklds)
	return wklds, nil
}

// ListSNSTopics returns a list of SNS topics deployed to the current environment and tagged with
// Copilot identifiers.
func (s *Store) ListSNSTopics(appName string, envName string) ([]Topic, error) {
	rgClient, err := s.newRgClientFromIDs(appName, envName)
	if err != nil {
		return nil, err
	}
	topics, err := rgClient.GetResourcesByTags(snsResourceType, map[string]string{
		AppTagKey: appName,
		EnvTagKey: envName,
	})

	if err != nil {
		return nil, fmt.Errorf("get SNS topics for environment %s and app %s: %w", envName, appName, err)
	}

	var out []Topic
	for _, r := range topics {
		// If the topic doesn't have a specific workload tag, don't return it.
		if _, ok := r.Tags[ServiceTagKey]; !ok {
			continue
		}

		t, err := NewTopic(r.ARN, appName, envName, r.Tags[ServiceTagKey])
		if err != nil {
			// If there's an error parsing the topic ARN, don't include it in the list of topics.
			// This includes times where the topic name does not match its tags, or the name
			// is invalid.
			switch err {
			case errInvalidARN:
				// This error indicates that the returned ARN is not parseable.
				return nil, err
			default:
				continue
			}
		}

		out = append(out, *t)
	}

	return out, nil
}

func contains(name string, names []string) bool {
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}

type result struct {
	name string
	err  error
}

func (s *Store) deployedServices(rgClient resourceGetter, app, env, svc string) result {
	resources, err := rgClient.GetResourcesByTags(stackResourceType, map[string]string{
		AppTagKey:     app,
		EnvTagKey:     env,
		ServiceTagKey: svc,
	})
	if err != nil {
		return result{err: fmt.Errorf("get resources by Copilot tags: %w", err)}
	}
	// If no resources found, the resp length is 0.
	var res result
	if len(resources) != 0 {
		res.name = env
	}
	return res
}

// ListEnvironmentsDeployedTo returns all the environment that a service is deployed in.
func (s *Store) ListEnvironmentsDeployedTo(appName string, svcName string) ([]string, error) {
	envs, err := s.configStore.ListEnvironments(appName)
	if err != nil {
		return nil, fmt.Errorf("list environment for app %s: %w", appName, err)
	}
	deployedEnv := make(chan result, len(envs))
	defer close(deployedEnv)
	for _, env := range envs {
		go func(env *config.Environment) {
			rgClient, err := s.newRgClientFromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				deployedEnv <- result{err: err}
				return
			}
			deployedEnv <- s.deployedServices(rgClient, appName, env.Name, svcName)
		}(env)
	}
	var envsWithDeployment []string
	for i := 0; i < len(envs); i++ {
		env := <-deployedEnv
		if env.err != nil {
			return nil, env.err
		}
		if env.name != "" {
			envsWithDeployment = append(envsWithDeployment, env.name)
		}
	}
	return envsWithDeployment, nil
}

// IsServiceDeployed returns whether a service is deployed in an environment or not.
func (s *Store) IsServiceDeployed(appName string, envName string, svcName string) (bool, error) {
	return s.isWorkloadDeployed(appName, envName, svcName)
}

// IsJobDeployed returnds whether a job is deployed in an environment or not by checking for a state machine.
func (s *Store) IsJobDeployed(appName, envName, jobName string) (bool, error) {
	return s.isWorkloadDeployed(appName, envName, jobName)
}

func (s *Store) isWorkloadDeployed(appName, envName, name string) (bool, error) {
	rgClient, err := s.newRgClientFromIDs(appName, envName)
	if err != nil {
		return false, err
	}
	stacks, err := rgClient.GetResourcesByTags(stackResourceType, map[string]string{
		AppTagKey:     appName,
		EnvTagKey:     envName,
		ServiceTagKey: name,
	})
	if err != nil {
		return false, fmt.Errorf("get resources by Copilot tags: %w", err)
	}
	if len(stacks) != 0 {
		return true, nil
	}
	return false, nil
}
