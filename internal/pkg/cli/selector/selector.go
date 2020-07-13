// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

// Prompter wraps the method to select an option from a list of options.
type Prompter interface {
	SelectOne(message, help string, options []string, promptOpts ...prompt.Option) (string, error)
}

// AppEnvLister wraps methods to list apps and envs in config store.
type AppEnvLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListApplications() ([]*config.Application, error)
}

// ConfigSvcLister wraps the method to list svcs in config store.
type ConfigSvcLister interface {
	ListServices(appName string) ([]*config.Service, error)
}

// ConfigLister wraps config store listing methods.
type ConfigLister interface {
	AppEnvLister
	ConfigSvcLister
}

// WsSvcLister wraps the method to get svcs in current workspace.
type WsSvcLister interface {
	ServiceNames() ([]string, error)
}

// DeployStoreClient wraps methods of deploy store.
type DeployStoreClient interface {
	ListDeployedServices(appName string, envName string) ([]string, error)
	IsServiceDeployed(appName string, envName string, svcName string) (bool, error)
}

// Select prompts users to select the name of an application or environment.
type Select struct {
	prompt Prompter
	lister AppEnvLister
}

// ConfigSelect is an application and environment selector, but can also choose a service from the config store.
type ConfigSelect struct {
	*Select
	svcLister ConfigSvcLister
}

// WorkspaceSelect  is an application and environment selector, but can also choose a service from the workspace.
type WorkspaceSelect struct {
	*Select
	svcLister WsSvcLister
}

// DeploySelect is a service and environment selector from the deploy store.
type DeploySelect struct {
	*Select
	deployStoreSvc DeployStoreClient
	svc            string
	env            string
}

// NewSelect returns a selector that chooses applications or environments.
func NewSelect(prompt Prompter, store AppEnvLister) *Select {
	return &Select{
		prompt: prompt,
		lister: store,
	}
}

// NewConfigSelect returns a new selector that chooses applications, environments, or services from the config store.
func NewConfigSelect(prompt Prompter, store ConfigLister) *ConfigSelect {
	return &ConfigSelect{
		Select:    NewSelect(prompt, store),
		svcLister: store,
	}
}

// NewWorkspaceSelect returns a new selector that chooses applications and environments from the config store, but
// services from the local workspace.
func NewWorkspaceSelect(prompt Prompter, store AppEnvLister, ws WsSvcLister) *WorkspaceSelect {
	return &WorkspaceSelect{
		Select:    NewSelect(prompt, store),
		svcLister: ws,
	}
}

// NewDeploySelect returns a new selector that chooses services and environments from the deploy store.
func NewDeploySelect(prompt Prompter, configStore AppEnvLister, deployStore DeployStoreClient) *DeploySelect {
	return &DeploySelect{
		Select:         NewSelect(prompt, configStore),
		deployStoreSvc: deployStore,
	}
}

// GetDeployedServiceOpts sets up optional parameters for GetDeployedServiceOpts function.
type GetDeployedServiceOpts func(*DeploySelect)

// WithSvc sets up the svc name for DeploySelect.
func WithSvc(svc string) GetDeployedServiceOpts {
	return func(in *DeploySelect) {
		in.svc = svc
	}
}

// WithEnv sets up the env name for DeploySelect.
func WithEnv(env string) GetDeployedServiceOpts {
	return func(in *DeploySelect) {
		in.env = env
	}
}

// DeployedService contains the service name and environment name of the deployed service.
type DeployedService struct {
	Svc string
	Env string
}

func (s *DeployedService) String() string {
	return fmt.Sprintf("%s (%s)", s.Svc, s.Env)
}

// DeployedService has the user select a deployed service. Callers can provide either a particular environment,
// a particular service to filter on, or both.
func (s *DeploySelect) DeployedService(prompt, help string, app string, opts ...GetDeployedServiceOpts) (*DeployedService, error) {
	for _, opt := range opts {
		opt(s)
	}
	var envNames []string
	var err error
	if s.env != "" {
		envNames = append(envNames, s.env)
	} else {
		envNames, err = s.retrieveEnvironments(app)
		if err != nil {
			return nil, fmt.Errorf("list environments: %w", err)
		}
	}
	svcEnvs := make(map[string]DeployedService)
	var svcEnvNames []string
	for _, envName := range envNames {
		var svcNames []string
		if s.svc != "" {
			deployed, err := s.deployStoreSvc.IsServiceDeployed(app, envName, s.svc)
			if err != nil {
				return nil, fmt.Errorf("check if service %s is deployed in environment %s: %w", s.svc, envName, err)
			}
			if !deployed {
				continue
			}
			svcNames = append(svcNames, s.svc)
		} else {
			svcNames, err = s.deployStoreSvc.ListDeployedServices(app, envName)
			if err != nil {
				return nil, fmt.Errorf("list deployed service for environment %s: %w", envName, err)
			}
		}
		for _, svcName := range svcNames {
			svcEnv := DeployedService{
				Svc: svcName,
				Env: envName,
			}
			svcEnvName := svcEnv.String()
			svcEnvs[svcEnvName] = svcEnv
			svcEnvNames = append(svcEnvNames, svcEnvName)
		}
	}
	if len(svcEnvNames) == 0 {
		return nil, fmt.Errorf("no deployed services found in application %s", color.HighlightUserInput(app))
	}
	// return if only one deployed service found
	var deployedSvc DeployedService
	if len(svcEnvNames) == 1 {
		deployedSvc = svcEnvs[svcEnvNames[0]]
		log.Infof("Only found one deployed service %s in environment %s\n", color.HighlightUserInput(deployedSvc.Svc), color.HighlightUserInput(deployedSvc.Env))
		return &deployedSvc, nil
	}
	svcEnvName, err := s.prompt.SelectOne(
		prompt,
		help,
		svcEnvNames,
	)
	if err != nil {
		return nil, fmt.Errorf("select deployed services for application %s: %w", app, err)
	}
	deployedSvc = svcEnvs[svcEnvName]

	return &deployedSvc, nil
}

// Service fetches all services in the workspace and then prompts the user to select one.
func (s *WorkspaceSelect) Service(prompt, help string) (string, error) {
	serviceNames, err := s.retrieveWorkspaceServices()
	if err != nil {
		return "", fmt.Errorf("list services: %w", err)
	}
	if len(serviceNames) == 1 {
		log.Infof("Only found one service in workspace, defaulting to: %s\n", color.HighlightUserInput(serviceNames[0]))
		return serviceNames[0], nil
	}

	selectedServiceName, err := s.prompt.SelectOne(prompt, help, serviceNames)
	if err != nil {
		return "", fmt.Errorf("select local service: %w", err)
	}
	return selectedServiceName, nil
}

// Service fetches all services in an app and prompts the user to select one.
func (s *ConfigSelect) Service(prompt, help, app string) (string, error) {
	services, err := s.retrieveServices(app)
	if err != nil {
		return "", fmt.Errorf("get services for app %s: %w", app, err)
	}
	if len(services) == 0 {
		log.Infof("Couldn't find any services associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(app),
			color.HighlightCode("copilot svc init"))
		return "", fmt.Errorf("no services found in app %s", app)
	}
	if len(services) == 1 {
		log.Infof("Only found one service, defaulting to: %s\n", color.HighlightUserInput(services[0]))
		return services[0], nil
	}
	selectedAppName, err := s.prompt.SelectOne(prompt, help, services)
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedAppName, nil
}

// Environment fetches all the environments in an app and prompts the user to select one.
func (s *Select) Environment(prompt, help, app string) (string, error) {
	envs, err := s.retrieveEnvironments(app)
	if err != nil {
		return "", fmt.Errorf("get environments for app %s from metadata store: %w", app, err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(app),
			color.HighlightCode("copilot env init"))
		return "", fmt.Errorf("no environments found in app %s", app)
	}
	if len(envs) == 1 {
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(envs[0]))
		return envs[0], nil
	}
	selectedEnvName, err := s.prompt.SelectOne(prompt, help, envs)
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

// EnvironmentWithNone fetches all the environments in an app and prompts the user to select one of the environments or None.
func (s *Select) EnvironmentWithNone(prompt, help, app string) (string, error) {
	envs, err := s.retrieveEnvironments(app)
	if err != nil {
		return "", err
	}

	if len(envs) == 0 {
		log.Infof("No environment found associated with app %s, defaulting to %s (task will run in your default VPC)\n",
			color.HighlightUserInput(app), color.Emphasize(config.EnvNameNone))
		return config.EnvNameNone, nil
	}

	envs = append(envs, config.EnvNameNone)

	selectedEnvName, err := s.prompt.SelectOne(prompt, help, envs)
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

// Application fetches all the apps in an account/region and prompts the user to select one.
func (s *Select) Application(prompt, help string) (string, error) {
	appNames, err := s.retrieveApps()
	if err != nil {
		return "", err
	}
	if len(appNames) == 0 {
		log.Infof("Couldn't find any applications in this region and account. Try initializing one with %s\n",
			color.HighlightCode("copilot app init"))
		return "", fmt.Errorf("no apps found")

	}

	if len(appNames) == 1 {
		log.Infof("Only found one application, defaulting to: %s\n", color.HighlightUserInput(appNames[0]))
		return appNames[0], nil
	}

	app, err := s.prompt.SelectOne(prompt, help, appNames)
	if err != nil {
		return "", fmt.Errorf("select application: %w", err)
	}
	return app, nil
}

func (s *Select) retrieveApps() ([]string, error) {
	apps, err := s.lister.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}
	return appNames, nil
}

func (s *Select) retrieveEnvironments(app string) ([]string, error) {
	envs, err := s.lister.ListEnvironments(app)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	envsNames := make([]string, len(envs))
	for ind, env := range envs {
		envsNames[ind] = env.Name
	}
	return envsNames, nil
}

func (s *ConfigSelect) retrieveServices(app string) ([]string, error) {
	services, err := s.svcLister.ListServices(app)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	serviceNames := make([]string, len(services))
	for ind, service := range services {
		serviceNames[ind] = service.Name
	}
	return serviceNames, nil
}

func (s *WorkspaceSelect) retrieveWorkspaceServices() ([]string, error) {
	localServiceNames, err := s.svcLister.ServiceNames()
	if err != nil {
		return nil, err
	}
	if len(localServiceNames) == 0 {
		return nil, errors.New("no services found in workspace")
	}
	return localServiceNames, nil
}
