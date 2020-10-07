// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/lnquy/cron"
)

const (
	every         = "@every %s"
	rate          = "Rate"
	fixedSchedule = "Fixed Schedule"

	custom  = "Custom"
	hourly  = "Hourly"
	daily   = "Daily"
	weekly  = "Weekly"
	monthly = "Monthly"
	yearly  = "Yearly"
)

var scheduleTypes = []string{
	rate,
	fixedSchedule,
}

var presetSchedules = []string{
	custom,
	hourly,
	daily,
	weekly,
	monthly,
	yearly,
}

var (
	ratePrompt = "How long would you like to wait between executions?"
	rateHelp   = `You can specify the time as a duration string. (For example, 2m, 1h30m, 24h)`

	schedulePrompt = "What schedule would you like to use?"
	scheduleHelp   = `Predefined schedules run at midnight or the top of the hour.
For example, "Daily" runs at midnight. "Weekly" runs at midnight on Mondays.`
	customSchedulePrompt = "What custom cron schedule would you like to use?"
	customScheduleHelp   = `Custom schedules can be defined using the following cron:
Minute | Hour | Day of Month | Month | Day of Week
For example: 0 17 ? * MON-FRI (5 pm on weekdays)
             0 0 1 */3 * (on the first of the month, quarterly)`
	humanReadableCronConfirmPrompt = "Would you like to use this schedule?"
	humanReadableCronConfirmHelp   = `Confirm whether the schedule looks right to you.
(Y)es will continue execution. (N)o will allow you to input a different schedule.`
)

// Prompter wraps the methods to ask for inputs from the terminal.
type Prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.Option) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.Option) (string, error)
	MultiSelect(message, help string, options []string, promptOpts ...prompt.Option) ([]string, error)
	Confirm(message, help string, promptOpts ...prompt.Option) (bool, error)
}

// AppEnvLister wraps methods to list apps and envs in config store.
type AppEnvLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListApplications() ([]*config.Application, error)
}

// ConfigSvcLister wraps the method to list svcs in config store.
type ConfigSvcLister interface {
	ListServices(appName string) ([]*config.Workload, error)
}

// ConfigLister wraps config store listing methods.
type ConfigLister interface {
	AppEnvLister
	ConfigSvcLister
}

// WsWorkloadLister wraps the method to get workloads in current workspace.
type WsWorkloadLister interface {
	ServiceNames() ([]string, error)
	JobNames() ([]string, error)
}

// WsWorkloadDockerfileLister wraps methods to get workload names and Dockerfiles from the workspace.
type WsWorkloadDockerfileLister interface {
	WsWorkloadLister
	ListDockerfiles() ([]string, error)
}

// DeployStoreClient wraps methods of deploy store.
type DeployStoreClient interface {
	ListDeployedServices(appName string, envName string) ([]string, error)
	IsServiceDeployed(appName string, envName string, svcName string) (bool, error)
}

// Select prompts users to select the name of an application or environment.
type Select struct {
	prompt Prompter
	lister ConfigLister
}

// ConfigSelect is an application and environment selector, but can also choose a service from the config store.
type ConfigSelect struct {
	*Select
	svcLister ConfigSvcLister
}

// WorkspaceSelect  is an application and environment selector, but can also choose a service from the workspace.
type WorkspaceSelect struct {
	*Select
	wlLister WsWorkloadDockerfileLister
}

// DeploySelect is a service and environment selector from the deploy store.
type DeploySelect struct {
	*Select
	deployStoreSvc DeployStoreClient
	svc            string
	env            string
}

// NewSelect returns a selector that chooses applications or environments.
func NewSelect(prompt Prompter, store ConfigLister) *Select {
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
func NewWorkspaceSelect(prompt Prompter, store ConfigLister, ws WsWorkloadDockerfileLister) *WorkspaceSelect {
	return &WorkspaceSelect{
		Select:   NewSelect(prompt, store),
		wlLister: ws,
	}
}

// NewDeploySelect returns a new selector that chooses services and environments from the deploy store.
func NewDeploySelect(prompt Prompter, configStore ConfigLister, deployStore DeployStoreClient) *DeploySelect {
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
func (s *WorkspaceSelect) Service(msg, help string, app string) (string, error) {
	wsServiceNames, err := s.retrieveWorkspaceServices()
	if err != nil {
		return "", fmt.Errorf("retrieve services from workspace: %w", err)
	}
	storeServiceNames, err := s.Select.lister.ListServices(app)
	if err != nil {
		return "", fmt.Errorf("retrieve services from store: %w", err)
	}
	serviceNames := filterWlsByName(storeServiceNames, wsServiceNames)
	if len(serviceNames) == 0 {
		return "", errors.New("no services found")
	}
	if len(serviceNames) == 1 {
		log.Infof("Only found one service, defaulting to: %s\n", color.HighlightUserInput(serviceNames[0]))
		return serviceNames[0], nil
	}

	selectedServiceName, err := s.prompt.SelectOne(msg, help, serviceNames, prompt.WithFinalMessage("Service name:"))
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedServiceName, nil
}

// Job fetches all jobs in the workspace and then prompts the user to select one.
func (s *WorkspaceSelect) Job(msg, help string, app string) (string, error) {
	wsJobNames, err := s.retrieveWorkspaceJobs()
	if err != nil {
		return "", fmt.Errorf("retrieve jobs from workspace: %w", err)
	}
	storeJobNames, err := s.Select.lister.ListServices(app)
	if err != nil {
		return "", fmt.Errorf("retrieve jobs from store: %w", err)
	}
	jobNames := filterWlsByName(storeJobNames, wsJobNames)
	if len(jobNames) == 0 {
		return "", errors.New("no jobs found")
	}
	if len(jobNames) == 1 {
		log.Infof("Only found one job, defaulting to: %s\n", color.HighlightUserInput(jobNames[0]))
		return jobNames[0], nil
	}

	selectedJobName, err := s.prompt.SelectOne(msg, help, jobNames, prompt.WithFinalMessage("Job name:"))
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil
}

func filterWlsByName(wls []*config.Workload, wantedNames []string) []string {
	isWanted := make(map[string]bool)
	for _, name := range wantedNames {
		isWanted[name] = true
	}
	var filtered []string
	for _, wl := range wls {
		if _, ok := isWanted[wl.Name]; !ok {
			continue
		}
		filtered = append(filtered, wl.Name)
	}
	return filtered
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
func (s *Select) Environment(prompt, help, app string, additionalOpts ...string) (string, error) {
	envs, err := s.retrieveEnvironments(app)
	if err != nil {
		return "", fmt.Errorf("get environments for app %s from metadata store: %w", app, err)
	}

	envs = append(envs, additionalOpts...)
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

// Application fetches all the apps in an account/region and prompts the user to select one.
func (s *Select) Application(prompt, help string, additionalOpts ...string) (string, error) {
	appNames, err := s.retrieveApps()
	if err != nil {
		return "", err
	}

	appNames = append(appNames, additionalOpts...)
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
	localServiceNames, err := s.wlLister.ServiceNames()
	if err != nil {
		return nil, err
	}
	return localServiceNames, nil
}

func (s *WorkspaceSelect) retrieveWorkspaceJobs() ([]string, error) {
	localJobNames, err := s.wlLister.JobNames()
	if err != nil {
		return nil, err
	}
	return localJobNames, nil
}

// Dockerfile asks the user to select from a list of Dockerfiles in the current
// directory or one level down. If no dockerfiles are found, it asks for a custom path.
func (s *WorkspaceSelect) Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pathValidator prompt.ValidatorFunc) (string, error) {
	dockerfiles, err := s.wlLister.ListDockerfiles()
	// If Dockerfiles are found in the current directory or subdirectory one level down, ask the user to select one.
	var sel string
	if err == nil {
		sel, err = s.prompt.SelectOne(
			selPrompt,
			selHelp,
			dockerfiles,
			prompt.WithFinalMessage("Dockerfile:"),
		)
		if err != nil {
			return "", fmt.Errorf("select Dockerfile: %w", err)
		}
		return sel, nil
	}

	var notExistErr *workspace.ErrDockerfileNotFound
	if !errors.As(err, &notExistErr) {
		return "", err
	}
	// If no Dockerfiles were found, prompt user for custom path.
	sel, err = s.prompt.Get(
		notFoundPrompt,
		notFoundHelp,
		pathValidator,
		prompt.WithFinalMessage("Dockerfile:"))
	if err != nil {
		return "", fmt.Errorf("get custom Dockerfile path: %w", err)
	}
	return sel, nil
}

// Schedule asks the user to select either a rate, preset cron, or custom cron.
func (s *WorkspaceSelect) Schedule(scheduleTypePrompt, scheduleTypeHelp string, scheduleValidator, rateValidator prompt.ValidatorFunc) (string, error) {
	scheduleType, err := s.prompt.SelectOne(
		scheduleTypePrompt,
		scheduleTypeHelp,
		scheduleTypes,
		prompt.WithFinalMessage("Schedule type:"),
	)
	if err != nil {
		return "", fmt.Errorf("get schedule type: %w", err)
	}
	switch scheduleType {
	case rate:
		return s.askRate(rateValidator)
	case fixedSchedule:
		return s.askCron(scheduleValidator)
	default:
		return "", fmt.Errorf("unrecognized schedule type %s", scheduleType)
	}
}

func (s *WorkspaceSelect) askRate(rateValidator prompt.ValidatorFunc) (string, error) {
	rateInput, err := s.prompt.Get(
		ratePrompt,
		rateHelp,
		rateValidator,
		prompt.WithFinalMessage("Rate:"),
	)
	if err != nil {
		return "", fmt.Errorf("get schedule rate: %w", err)
	}
	return fmt.Sprintf(every, rateInput), nil
}

func (s *WorkspaceSelect) askCron(scheduleValidator prompt.ValidatorFunc) (string, error) {
	cronInput, err := s.prompt.SelectOne(
		schedulePrompt,
		scheduleHelp,
		presetSchedules,
		prompt.WithFinalMessage("Fixed Schedule:"),
	)
	if err != nil {
		return "", fmt.Errorf("get preset schedule: %w", err)
	}
	if cronInput != custom {
		return presetScheduleToDefinitionString(cronInput), nil
	}
	var customSchedule, humanCron string
	cronDescriptor, err := cron.NewDescriptor()
	if err != nil {
		return "", fmt.Errorf("get custom schedule: %w", err)
	}
	for {
		customSchedule, err = s.prompt.Get(
			customSchedulePrompt,
			customScheduleHelp,
			scheduleValidator,
			prompt.WithDefaultInput("0 * * * *"),
			prompt.WithFinalMessage("Custom Schedule:"),
		)
		if err != nil {
			return "", fmt.Errorf("get custom schedule: %w", err)
		}

		// Break if the customer has specified an easy to read cron definition string
		if strings.HasPrefix(customSchedule, "@") {
			break
		}

		humanCron, err = cronDescriptor.ToDescription(customSchedule, cron.Locale_en)
		if err != nil {
			return "", fmt.Errorf("convert cron to human string: %w", err)
		}

		log.Infoln(fmt.Sprintf("Your job will run at the following times: %s", humanCron))

		ok, err := s.prompt.Confirm(
			humanReadableCronConfirmPrompt,
			humanReadableCronConfirmHelp,
		)
		if err != nil {
			return "", fmt.Errorf("confirm cron schedule: %w", err)
		}
		if ok {
			break
		}
	}

	return customSchedule, nil
}

func presetScheduleToDefinitionString(input string) string {
	return fmt.Sprintf("@%s", strings.ToLower(input))
}
