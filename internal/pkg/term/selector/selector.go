// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"errors"
	"fmt"
	"strings"

	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
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

	pipelineEscapeOpt = "[No additional environments]"

	fmtCopilotTaskGroup = "copilot-%s"

	fmtTopicSlug = "%s (%s)"
)

const (
	// dockerfilePromptUseCustom is the option for using Dockerfile with custom path.
	dockerfilePromptUseCustom = "Enter custom path for your Dockerfile"
	// DockerfilePromptUseImage is the option for using existing image instead of Dockerfile.
	DockerfilePromptUseImage = "Use an existing image instead"

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

// Prompter wraps the methods to ask for inputs from the terminal.
type Prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.PromptConfig) (string, error)
	MultiSelect(message, help string, options []string, promptOpts ...prompt.PromptConfig) ([]string, error)
	Confirm(message, help string, promptOpts ...prompt.PromptConfig) (bool, error)
}

// AppEnvLister wraps methods to list apps and envs in config store.
type AppEnvLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListApplications() ([]*config.Application, error)
}

// ConfigWorkloadLister wraps the method to list workloads in config store.
type ConfigWorkloadLister interface {
	ListServices(appName string) ([]*config.Workload, error)
	ListJobs(appName string) ([]*config.Workload, error)
	ListWorkloads(appName string) ([]*config.Workload, error)
}

// ConfigLister wraps config store listing methods.
type ConfigLister interface {
	AppEnvLister
	ConfigWorkloadLister
}

// WsWorkloadLister wraps the method to get workloads in current workspace.
type WsWorkloadLister interface {
	ServiceNames() ([]string, error)
	JobNames() ([]string, error)
	WorkloadNames() ([]string, error)
}

// WorkspaceRetriever wraps methods to get workload names, app names, and Dockerfiles from the workspace.
type WorkspaceRetriever interface {
	WsWorkloadLister
	Summary() (*workspace.Summary, error)
	ListDockerfiles() ([]string, error)
}

// DeployStoreClient wraps methods of deploy store.
type DeployStoreClient interface {
	ListDeployedServices(appName string, envName string) ([]string, error)
	ListDeployedJobs(appName, envName string) ([]string, error)
	IsServiceDeployed(appName string, envName string, svcName string) (bool, error)
	IsJobDeployed(appName, envName, jobName string) (bool, error)
	ListDeployedSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

// TaskStackDescriber wraps cloudformation client methods to describe task stacks
type TaskStackDescriber interface {
	ListDefaultTaskStacks() ([]deploy.TaskStackInfo, error)
	ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error)
}

// TaskLister wraps methods of listing tasks.
type TaskLister interface {
	ListActiveAppEnvTasks(opts ecs.ListActiveAppEnvTasksOpts) ([]*awsecs.Task, error)
	ListActiveDefaultClusterTasks(filter ecs.ListTasksFilter) ([]*awsecs.Task, error)
}

// Select prompts users to select the name of an application or environment.
type Select struct {
	prompt Prompter
	config ConfigLister
}

// ConfigSelect is an application and environment selector, but can also choose a service from the config store.
type ConfigSelect struct {
	*Select
	workloadLister ConfigWorkloadLister
}

// WorkspaceSelect  is an application and environment selector, but can also choose a service from the workspace.
type WorkspaceSelect struct {
	*Select
	ws      WorkspaceRetriever
	appName string
}

// DeploySelect is a service and environment selector from the deploy store.
type DeploySelect struct {
	*Select
	deployStoreSvc DeployStoreClient
	svc            string
	env            string
	filters        []DeployedServiceFilter
}

// CFTaskSelect is a selector based on CF methods to get deployed one off tasks.
type CFTaskSelect struct {
	*Select
	cfStore        TaskStackDescriber
	app            string
	env            string
	defaultCluster bool
}

func NewCFTaskSelect(prompt Prompter, store ConfigLister, cf TaskStackDescriber) *CFTaskSelect {
	return &CFTaskSelect{
		Select:  NewSelect(prompt, store),
		cfStore: cf,
	}
}

// GetDeployedTaskOpts sets up optional parameters for GetDeployedTaskOpts function.
type GetDeployedTaskOpts func(*CFTaskSelect)

// TaskWithAppEnv sets up the env name for TaskSelect.
func TaskWithAppEnv(app, env string) GetDeployedTaskOpts {
	return func(in *CFTaskSelect) {
		in.app = app
		in.env = env
	}
}

// WithDefaultCluster sets up whether CFTaskSelect should use only the default cluster.
func TaskWithDefaultCluster() GetDeployedTaskOpts {
	return func(in *CFTaskSelect) {
		in.defaultCluster = true
	}
}

// TaskSelect is a Copilot running task selector.
type TaskSelect struct {
	prompt         Prompter
	lister         TaskLister
	app            string
	env            string
	defaultCluster bool
	taskGroup      string
	taskID         string
}

// NewSelect returns a selector that chooses applications or environments.
func NewSelect(prompt Prompter, store ConfigLister) *Select {
	return &Select{
		prompt: prompt,
		config: store,
	}
}

// NewConfigSelect returns a new selector that chooses applications, environments, or services from the config store.
func NewConfigSelect(prompt Prompter, store ConfigLister) *ConfigSelect {
	return &ConfigSelect{
		Select:         NewSelect(prompt, store),
		workloadLister: store,
	}
}

// NewWorkspaceSelect returns a new selector that chooses applications and environments from the config store, but
// services from the local workspace.
func NewWorkspaceSelect(prompt Prompter, store ConfigLister, ws WorkspaceRetriever) *WorkspaceSelect {
	return &WorkspaceSelect{
		Select: NewSelect(prompt, store),
		ws:     ws,
	}
}

// NewDeploySelect returns a new selector that chooses services and environments from the deploy store.
func NewDeploySelect(prompt Prompter, configStore ConfigLister, deployStore DeployStoreClient) *DeploySelect {
	return &DeploySelect{
		Select:         NewSelect(prompt, configStore),
		deployStoreSvc: deployStore,
	}
}

// NewTaskSelect returns a new selector that chooses a running task.
func NewTaskSelect(prompt Prompter, lister TaskLister) *TaskSelect {
	return &TaskSelect{
		prompt: prompt,
		lister: lister,
	}
}

// TaskOpts sets up optional parameters for Task function.
type TaskOpts func(*TaskSelect)

// WithAppEnv sets up the app name and env name for TaskSelect.
func WithAppEnv(app, env string) TaskOpts {
	return func(in *TaskSelect) {
		in.app = app
		in.env = env
	}
}

// WithDefault uses default cluster for TaskSelect.
func WithDefault() TaskOpts {
	return func(in *TaskSelect) {
		in.defaultCluster = true
	}
}

// WithTaskGroup sets up the task group name for TaskSelect.
func WithTaskGroup(taskGroup string) TaskOpts {
	return func(in *TaskSelect) {
		if taskGroup != "" {
			in.taskGroup = fmt.Sprintf(fmtCopilotTaskGroup, taskGroup)
		}
	}
}

// WithTaskID sets up the task ID for TaskSelect.
func WithTaskID(id string) TaskOpts {
	return func(in *TaskSelect) {
		in.taskID = id
	}
}

// RunningTask has the user select a running task. Callers can provide either app and env names,
// or use default cluster.
func (s *TaskSelect) RunningTask(prompt, help string, opts ...TaskOpts) (*awsecs.Task, error) {
	var tasks []*awsecs.Task
	var err error
	for _, opt := range opts {
		opt(s)
	}
	filter := ecs.ListTasksFilter{
		TaskGroup:   s.taskGroup,
		TaskID:      s.taskID,
		CopilotOnly: true,
	}
	if s.defaultCluster {
		tasks, err = s.lister.ListActiveDefaultClusterTasks(filter)
		if err != nil {
			return nil, fmt.Errorf("list active tasks for default cluster: %w", err)
		}
	}
	if s.app != "" && s.env != "" {
		tasks, err = s.lister.ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
			App:             s.app,
			Env:             s.env,
			ListTasksFilter: filter,
		})
		if err != nil {
			return nil, fmt.Errorf("list active tasks in environment %s: %w", s.env, err)
		}
	}
	var taskStrList []string
	taskStrMap := make(map[string]*awsecs.Task)
	for _, task := range tasks {
		taskStr := task.String()
		taskStrList = append(taskStrList, taskStr)
		taskStrMap[taskStr] = task
	}
	if len(taskStrList) == 0 {
		return nil, fmt.Errorf("no running tasks found")
	}
	// return if only one running task found
	if len(taskStrList) == 1 {
		log.Infof("Found only one running task %s\n", color.HighlightUserInput(taskStrList[0]))
		return taskStrMap[taskStrList[0]], nil
	}
	task, err := s.prompt.SelectOne(
		prompt,
		help,
		taskStrList,
	)
	if err != nil {
		return nil, fmt.Errorf("select running task: %w", err)
	}
	return taskStrMap[task], nil
}

// GetDeployedServiceOpts sets up optional parameters for GetDeployedServiceOpts function.
type GetDeployedServiceOpts func(*DeploySelect)

// DeployedServiceFilter determines if a service should be included in the results.
type DeployedServiceFilter func(*DeployedService) (bool, error)

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

// WithFilter sets up filters for DeploySelect
func WithFilter(filter DeployedServiceFilter) GetDeployedServiceOpts {
	return func(in *DeploySelect) {
		in.filters = append(in.filters, filter)
	}
}

// WithServiceTypesFilter sets up a ServiceType filter for DeploySelect
func WithServiceTypesFilter(svcTypes []string) GetDeployedServiceOpts {

	return WithFilter(func(svc *DeployedService) (bool, error) {
		for _, svcType := range svcTypes {
			if svc.SvcType == svcType {
				return true, nil
			}
		}
		return false, nil
	})
}

// DeployedService contains the service name and environment name of the deployed service.
type DeployedService struct {
	Svc     string
	Env     string
	SvcType string
}

func (s *DeployedService) String() string {
	return fmt.Sprintf("%s (%s)", s.Svc, s.Env)
}

// Task has the user select a task. Callers can provide an environment, an app, or a "use default cluster" option
// to filter the returned tasks.
func (s *CFTaskSelect) Task(prompt, help string, opts ...GetDeployedTaskOpts) (string, error) {
	for _, opt := range opts {
		opt(s)
	}
	if s.defaultCluster && (s.env != "" || s.app != "") {
		// Error for callers
		return "", fmt.Errorf("cannot specify both default cluster and env")
	}
	if !s.defaultCluster && (s.env == "" && s.app == "") {
		return "", fmt.Errorf("must specify either app and env or default cluster")
	}

	var tasks []deploy.TaskStackInfo
	var err error
	if s.defaultCluster {
		defaultTasks, err := s.cfStore.ListDefaultTaskStacks()
		if err != nil {
			return "", fmt.Errorf("get tasks in default cluster: %w", err)
		}
		tasks = append(tasks, defaultTasks...)
	}
	if s.env != "" && s.app != "" {
		envTasks, err := s.cfStore.ListTaskStacks(s.app, s.env)
		if err != nil {
			return "", fmt.Errorf("get tasks in environment %s: %w", s.env, err)
		}
		tasks = append(tasks, envTasks...)
	}
	choices := make([]string, len(tasks))
	for n, task := range tasks {
		choices[n] = task.TaskName()
	}

	if len(choices) == 0 {
		return "", fmt.Errorf("no deployed tasks found in selected cluster")
	}
	// Return if there's only one option.
	if len(choices) == 1 {
		log.Infof("Found only one deployed task: %s\n", color.HighlightUserInput(choices[0]))
		return choices[0], nil
	}
	choice, err := s.prompt.SelectOne(prompt, help, choices)
	if err != nil {
		return "", fmt.Errorf("select task for deletion: %w", err)
	}
	return choice, nil
}

// DeployedService has the user select a deployed service. Callers can provide either a particular environment,
// a particular service to filter on, or both.
func (s *DeploySelect) DeployedService(prompt, help string, app string, opts ...GetDeployedServiceOpts) (*DeployedService, error) {
	for _, opt := range opts {
		opt(s)
	}
	var err error
	var envNames []string
	svcTypes := map[string]string{}

	// ServiceType is only utilized by the filtering functionality. No need to retrieve types if filters are not being applied
	if len(s.filters) > 0 {
		services, err := s.config.ListServices(app)
		if err != nil {
			return nil, fmt.Errorf("list services: %w", err)
		}
		for _, svc := range services {
			svcTypes[svc.Name] = svc.Type
		}
	}

	if s.env != "" {
		envNames = append(envNames, s.env)
	} else {
		envNames, err = s.retrieveEnvironments(app)
		if err != nil {
			return nil, fmt.Errorf("list environments: %w", err)
		}
	}
	svcEnvs := []*DeployedService{}
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
			svcEnv := &DeployedService{
				Svc:     svcName,
				Env:     envName,
				SvcType: svcTypes[svcName],
			}
			svcEnvs = append(svcEnvs, svcEnv)
		}
	}
	if len(svcEnvs) == 0 {
		return nil, fmt.Errorf("no deployed services found in application %s", color.HighlightUserInput(app))
	}

	if svcEnvs, err = s.filterServices(svcEnvs); err != nil {
		return nil, err
	}

	if len(svcEnvs) == 0 {
		return nil, fmt.Errorf("no matching deployed services found in application %s", color.HighlightUserInput(app))
	}
	// return if only one deployed service found
	var deployedSvc *DeployedService
	if len(svcEnvs) == 1 {
		deployedSvc = svcEnvs[0]
		if s.svc == "" && s.env == "" {
			log.Infof("Found only one deployed service %s in environment %s\n", color.HighlightUserInput(deployedSvc.Svc), color.HighlightUserInput(deployedSvc.Env))
		}
		if (s.svc != "") != (s.env != "") {
			log.Infof("Service %s found in environment %s\n", color.HighlightUserInput(deployedSvc.Svc), color.HighlightUserInput(deployedSvc.Env))
		}
		return deployedSvc, nil
	}

	svcEnvNames := make([]string, len(svcEnvs))
	svcEnvNameMap := map[string]*DeployedService{}
	for i, svc := range svcEnvs {
		svcEnvNames[i] = svc.String()
		svcEnvNameMap[svcEnvNames[i]] = svc
	}

	svcEnvName, err := s.prompt.SelectOne(
		prompt,
		help,
		svcEnvNames,
	)
	if err != nil {
		return nil, fmt.Errorf("select deployed services for application %s: %w", app, err)
	}
	deployedSvc = svcEnvNameMap[svcEnvName]

	return deployedSvc, nil
}

func (s *DeploySelect) filterServices(inServices []*DeployedService) ([]*DeployedService, error) {
	outServices := inServices
	for _, filter := range s.filters {
		if result, err := filterDeployedServices(filter, outServices); err != nil {
			return nil, err
		} else {
			outServices = result
		}
	}
	return outServices, nil
}

// Service fetches all services in the workspace and then prompts the user to select one.
func (s *WorkspaceSelect) Service(msg, help string) (string, error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsServiceNames, err := s.retrieveWorkspaceServices()
	if err != nil {
		return "", fmt.Errorf("retrieve services from workspace: %w", err)
	}
	storeServiceNames, err := s.Select.config.ListServices(summary.Application)
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
func (s *WorkspaceSelect) Job(msg, help string) (string, error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsJobNames, err := s.retrieveWorkspaceJobs()
	if err != nil {
		return "", fmt.Errorf("retrieve jobs from workspace: %w", err)
	}
	storeJobNames, err := s.Select.config.ListJobs(summary.Application)
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

// Workload fetches all jobs and services in an app and prompts the user to select one.
func (s *WorkspaceSelect) Workload(msg, help string) (wl string, err error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsWlNames, err := s.retrieveWorkspaceWorkloads()
	if err != nil {
		return "", fmt.Errorf("retrieve jobs and services from workspace: %w", err)
	}
	storeWls, err := s.Select.config.ListWorkloads(summary.Application)
	if err != nil {
		return "", fmt.Errorf("retrieve jobs and services from store: %w", err)
	}
	wlNames := filterWlsByName(storeWls, wsWlNames)
	if len(wlNames) == 0 {
		return "", errors.New("no jobs or services found")
	}
	if len(wlNames) == 1 {
		log.Infof("Only found one workload, defaulting to: %s\n", color.HighlightUserInput(wlNames[0]))
		return wlNames[0], nil
	}
	selectedWlName, err := s.prompt.SelectOne(msg, help, wlNames, prompt.WithFinalMessage("Name: "))
	if err != nil {
		return "", fmt.Errorf("select workload: %w", err)
	}
	return selectedWlName, nil
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
		return "", err
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
	selectedSvcName, err := s.prompt.SelectOne(prompt, help, services)
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedSvcName, nil
}

// Job fetches all jobs in an app and prompts the user to select one.
func (s *ConfigSelect) Job(prompt, help, app string) (string, error) {
	jobs, err := s.retrieveJobs(app)
	if err != nil {
		return "", err
	}
	if len(jobs) == 0 {
		log.Infof("Couldn't find any jobs associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(app),
			color.HighlightCode("copilot job init"))
		return "", fmt.Errorf("no jobs found in app %s", app)
	}
	if len(jobs) == 1 {
		log.Infof("Only found one job, defaulting to: %s\n", color.HighlightUserInput(jobs[0]))
		return jobs[0], nil
	}
	selectedJobName, err := s.prompt.SelectOne(prompt, help, jobs)
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil
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

// Environments fetches all the environments in an app and prompts the user to select one OR MORE.
// The List of options decreases as envs are chosen. Chosen envs displayed above with the finalMsg.
func (s *Select) Environments(prompt, help, app string, finalMsgFunc func(int) prompt.PromptConfig) ([]string, error) {
	envs, err := s.retrieveEnvironments(app)
	if err != nil {
		return nil, fmt.Errorf("get environments for app %s from metadata store: %w", app, err)
	}
	if len(envs) == 0 {
		log.Infof("Couldn't find any environments associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(app),
			color.HighlightCode("copilot env init"))
		return nil, fmt.Errorf("no environments found in app %s", app)
	}

	envs = append(envs, pipelineEscapeOpt)
	var selectedEnvs []string
	usedEnvs := make(map[string]bool)

	for i := 1; i < len(envs); i++ {
		var availableEnvs []string
		for _, env := range envs {
			// Check if environment has already been added to pipeline
			if _, ok := usedEnvs[env]; !ok {
				availableEnvs = append(availableEnvs, env)
			}
		}

		selectedEnv, err := s.prompt.SelectOne(prompt, help, availableEnvs, finalMsgFunc(i))
		if err != nil {
			return nil, fmt.Errorf("select environments: %w", err)
		}
		if selectedEnv == pipelineEscapeOpt {
			break
		}
		selectedEnvs = append(selectedEnvs, selectedEnv)

		usedEnvs[selectedEnv] = true
	}
	return selectedEnvs, nil
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
	apps, err := s.config.ListApplications()
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
	envs, err := s.config.ListEnvironments(app)
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
	services, err := s.workloadLister.ListServices(app)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	serviceNames := make([]string, len(services))
	for ind, service := range services {
		serviceNames[ind] = service.Name
	}
	return serviceNames, nil
}

func (s *ConfigSelect) retrieveJobs(app string) ([]string, error) {
	jobs, err := s.workloadLister.ListJobs(app)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	jobNames := make([]string, len(jobs))
	for ind, job := range jobs {
		jobNames[ind] = job.Name
	}
	return jobNames, nil
}

func (s *WorkspaceSelect) retrieveWorkspaceServices() ([]string, error) {
	localServiceNames, err := s.ws.ServiceNames()
	if err != nil {
		return nil, err
	}
	return localServiceNames, nil
}

func (s *WorkspaceSelect) retrieveWorkspaceJobs() ([]string, error) {
	localJobNames, err := s.ws.JobNames()
	if err != nil {
		return nil, err
	}
	return localJobNames, nil
}

func (s *WorkspaceSelect) retrieveWorkspaceWorkloads() ([]string, error) {
	localWlNames, err := s.ws.WorkloadNames()
	if err != nil {
		return nil, err
	}
	return localWlNames, nil
}

// Dockerfile asks the user to select from a list of Dockerfiles in the current
// directory or one level down. If no dockerfiles are found, it asks for a custom path.
func (s *WorkspaceSelect) Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pathValidator prompt.ValidatorFunc) (string, error) {
	dockerfiles, err := s.ws.ListDockerfiles()
	if err != nil {
		return "", fmt.Errorf("list Dockerfiles: %w", err)
	}
	var sel string
	dockerfiles = append(dockerfiles, []string{dockerfilePromptUseCustom, DockerfilePromptUseImage}...)
	sel, err = s.prompt.SelectOne(
		selPrompt,
		selHelp,
		dockerfiles,
		prompt.WithFinalMessage("Dockerfile:"),
	)
	if err != nil {
		return "", fmt.Errorf("select Dockerfile: %w", err)
	}
	if sel != dockerfilePromptUseCustom {
		return sel, nil
	}
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
		prompt.WithDefaultInput("1h30m"),
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

func filterDeployedServices(filter DeployedServiceFilter, inServices []*DeployedService) ([]*DeployedService, error) {
	outServices := []*DeployedService{}
	for _, svc := range inServices {
		if include, err := filter(svc); err != nil {
			return nil, err
		} else if include {
			outServices = append(outServices, svc)
		}
	}
	return outServices, nil
}

// Topics asks the user to select from all Copilot-managed SNS topics and returns the ARNs
// of those topics.
func (s *DeploySelect) Topics(prompt, help, app, env string) ([]string, error) {

	topics, err := s.deployStoreSvc.ListDeployedSNSTopics(app, env)
	if err != nil {
		return nil, fmt.Errorf("list SNS topics: %w", err)
	}
	if len(topics) == 0 {
		return nil, nil
	}
	// Create the list of options and the map from option to full topic specification.
	var topicSlugs []string
	topicMap := make(map[string]*deploy.Topic)
	for _, t := range topics {
		n, err := t.Name()
		if err != nil {
			continue
		}
		// A slug is the human string printed out as an option.
		// In this case, it's "orders (api)" to denote the "orders"
		// topic published by the "api" service.
		slug := fmt.Sprintf(fmtTopicSlug, n, t.Wkld)
		topicSlugs = append(topicSlugs, slug)
		topicMap[slug] = &t
	}

	selectedTopics, err := s.prompt.MultiSelect(prompt, help, topicSlugs)
	if err != nil {
		return nil, fmt.Errorf("select SNS topics: %w", err)
	}

	// Get the ARNs from the topic slugs again.
	var topicARNs []string
	for _, t := range selectedTopics {
		topicARNs = append(topicARNs, topicMap[t].ARN)
	}
	return topicARNs, nil
}
