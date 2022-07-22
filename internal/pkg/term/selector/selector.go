// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"errors"
	"fmt"
	"sort"
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
	svcWorkloadType = "service"
	jobWorkloadType = "job"

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

// Final messages displayed after prompting.
const (
	appNameFinalMessage = "Application:"
	envNameFinalMessage = "Environment:"
	svcNameFinalMsg     = "Service name:"
	jobNameFinalMsg     = "Job name:"
	deployedJobFinalMsg = "Job:"
	deployedSvcFinalMsg = "Service:"
	taskFinalMsg        = "Task:"
	workloadFinalMsg    = "Name:"
	dockerfileFinalMsg  = "Dockerfile:"
	topicFinalMsg       = "Topic subscriptions:"
	pipelineFinalMsg    = "Pipeline:"
)

var scheduleTypes = []string{
	rate,
	fixedSchedule,
}

var presetSchedules = []prompt.Option{
	{Value: custom, Hint: ""},
	{Value: hourly, Hint: "At minute 0"},
	{Value: daily, Hint: "At midnight UTC"},
	{Value: weekly, Hint: "At midnight on Sunday UTC"},
	{Value: monthly, Hint: "At midnight, first day of month UTC"},
	{Value: yearly, Hint: "At midnight, Jan 1st UTC"},
}

// prompter wraps the methods to ask for inputs from the terminal.
type prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOption(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) (value string, err error)
	MultiSelect(message, help string, options []string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) ([]string, error)
	Confirm(message, help string, promptOpts ...prompt.PromptConfig) (bool, error)
}

type appEnvLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListApplications() ([]*config.Application, error)
}

type configWorkloadLister interface {
	ListServices(appName string) ([]*config.Workload, error)
	ListJobs(appName string) ([]*config.Workload, error)
	ListWorkloads(appName string) ([]*config.Workload, error)
}

type configLister interface {
	appEnvLister
	configWorkloadLister
}

type wsWorkloadLister interface {
	ListServices() ([]string, error)
	ListJobs() ([]string, error)
	ListWorkloads() ([]string, error)
}

type wsEnvironmentsLister interface {
	ListEnvironments() ([]string, error)
}

type wsPipelinesLister interface {
	ListPipelines() ([]workspace.PipelineManifest, error)
}

// codePipelineLister lists deployed pipelines.
type codePipelineLister interface {
	ListDeployedPipelines(appName string) ([]deploy.Pipeline, error)
}

// workspaceRetriever wraps methods to get workload names, app names, and Dockerfiles from the workspace.
type workspaceRetriever interface {
	wsWorkloadLister
	wsEnvironmentsLister
	Summary() (*workspace.Summary, error)
	ListDockerfiles() ([]string, error)
}

// deployedWorkloadsRetriever retrieves information about deployed services or jobs.
type deployedWorkloadsRetriever interface {
	ListDeployedServices(appName string, envName string) ([]string, error)
	ListDeployedJobs(appName, envName string) ([]string, error)
	IsServiceDeployed(appName string, envName string, svcName string) (bool, error)
	IsJobDeployed(appName, envName, jobName string) (bool, error)
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

// taskStackDescriber wraps cloudformation client methods to describe task stacks
type taskStackDescriber interface {
	ListDefaultTaskStacks() ([]deploy.TaskStackInfo, error)
	ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error)
}

// taskLister wraps methods of listing tasks.
type taskLister interface {
	ListActiveAppEnvTasks(opts ecs.ListActiveAppEnvTasksOpts) ([]*awsecs.Task, error)
	ListActiveDefaultClusterTasks(filter ecs.ListTasksFilter) ([]*awsecs.Task, error)
}

// AppEnvSelector prompts users to select the name of an application or environment.
type AppEnvSelector struct {
	prompt       prompter
	appEnvLister appEnvLister
}

// ConfigSelector is an application and environment selector, but can also choose a service from the config store.
type ConfigSelector struct {
	*AppEnvSelector
	workloadLister configWorkloadLister
}

// LocalWorkloadSelector is an application and environment selector, but can also choose a service from the workspace.
type LocalWorkloadSelector struct {
	*ConfigSelector
	ws workspaceRetriever
}

// LocalEnvironmentSelector is an application and environment selector, but can also choose an environment from the workspace.
type LocalEnvironmentSelector struct {
	*AppEnvSelector
	ws workspaceRetriever
}

// WorkspaceSelector selects from local workspace.
type WorkspaceSelector struct {
	prompt prompter
	ws     workspaceRetriever
}

// WsPipelineSelector is a workspace pipeline selector.
type WsPipelineSelector struct {
	prompt prompter
	ws     wsPipelinesLister
}

// CodePipelineSelector is a selector for deployed pipelines.
type CodePipelineSelector struct {
	prompt         prompter
	pipelineLister codePipelineLister
}

// AppPipelineSelector is a selector for deployed pipelines and apps.
type AppPipelineSelector struct {
	*AppEnvSelector
	*CodePipelineSelector
}

// DeploySelector is a service and environment selector from the deploy store.
type DeploySelector struct {
	*ConfigSelector
	deployStoreSvc deployedWorkloadsRetriever
	name           string
	env            string
	filters        []DeployedWorkloadFilter
}

// CFTaskSelector is a selector based on CF methods to get deployed one off tasks.
type CFTaskSelector struct {
	*AppEnvSelector
	cfStore        taskStackDescriber
	app            string
	env            string
	defaultCluster bool
}

// NewCFTaskSelect constructs a CFTaskSelector.
func NewCFTaskSelect(prompt prompter, store configLister, cf taskStackDescriber) *CFTaskSelector {
	return &CFTaskSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		cfStore:        cf,
	}
}

// GetDeployedTaskOpts sets up optional parameters for GetDeployedTaskOpts function.
type GetDeployedTaskOpts func(*CFTaskSelector)

// TaskWithAppEnv sets up the env name for TaskSelector.
func TaskWithAppEnv(app, env string) GetDeployedTaskOpts {
	return func(in *CFTaskSelector) {
		in.app = app
		in.env = env
	}
}

// TaskWithDefaultCluster sets up whether CFTaskSelector should use only the default cluster.
func TaskWithDefaultCluster() GetDeployedTaskOpts {
	return func(in *CFTaskSelector) {
		in.defaultCluster = true
	}
}

// TaskSelector is a Copilot running task selector.
type TaskSelector struct {
	prompt         prompter
	lister         taskLister
	app            string
	env            string
	defaultCluster bool
	taskGroup      string
	taskID         string
}

// NewAppEnvSelector returns a selector that chooses applications or environments.
func NewAppEnvSelector(prompt prompter, store appEnvLister) *AppEnvSelector {
	return &AppEnvSelector{
		prompt:       prompt,
		appEnvLister: store,
	}
}

// NewConfigSelector returns a new selector that chooses applications, environments, or services from the config store.
func NewConfigSelector(prompt prompter, store configLister) *ConfigSelector {
	return &ConfigSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		workloadLister: store,
	}
}

// NewLocalWorkloadSelector returns a new selector that chooses applications and environments from the config store, but
// services from the local workspace.
func NewLocalWorkloadSelector(prompt prompter, store configLister, ws workspaceRetriever) *LocalWorkloadSelector {
	return &LocalWorkloadSelector{
		ConfigSelector: NewConfigSelector(prompt, store),
		ws:             ws,
	}
}

// NewLocalEnvironmentSelector returns a new selector that chooses applications from the config store, but an environment
// from the local workspace.
func NewLocalEnvironmentSelector(prompt prompter, store configLister, ws workspaceRetriever) *LocalEnvironmentSelector {
	return &LocalEnvironmentSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		ws:             ws,
	}
}

// NewWorkspaceSelector returns a new selector that prompts for local information.
func NewWorkspaceSelector(prompt prompter, ws workspaceRetriever) *WorkspaceSelector {
	return &WorkspaceSelector{
		prompt: prompt,
		ws:     ws,
	}
}

// NewWsPipelineSelector returns a new selector with pipelines from the local workspace.
func NewWsPipelineSelector(prompt prompter, ws wsPipelinesLister) *WsPipelineSelector {
	return &WsPipelineSelector{
		prompt: prompt,
		ws:     ws,
	}
}

// NewAppPipelineSelector returns new selectors with deployed pipelines and apps.
func NewAppPipelineSelector(prompt prompter, store configLister, lister codePipelineLister) *AppPipelineSelector {
	return &AppPipelineSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		CodePipelineSelector: &CodePipelineSelector{
			prompt:         prompt,
			pipelineLister: lister,
		},
	}
}

// NewDeploySelect returns a new selector that chooses services and environments from the deploy store.
func NewDeploySelect(prompt prompter, configStore configLister, deployStore deployedWorkloadsRetriever) *DeploySelector {
	return &DeploySelector{
		ConfigSelector: NewConfigSelector(prompt, configStore),
		deployStoreSvc: deployStore,
	}
}

// NewTaskSelector returns a new selector that chooses a running task.
func NewTaskSelector(prompt prompter, lister taskLister) *TaskSelector {
	return &TaskSelector{
		prompt: prompt,
		lister: lister,
	}
}

// TaskOpts sets up optional parameters for Task function.
type TaskOpts func(*TaskSelector)

// WithAppEnv sets up the app name and env name for TaskSelector.
func WithAppEnv(app, env string) TaskOpts {
	return func(in *TaskSelector) {
		in.app = app
		in.env = env
	}
}

// WithDefault uses default cluster for TaskSelector.
func WithDefault() TaskOpts {
	return func(in *TaskSelector) {
		in.defaultCluster = true
	}
}

// WithTaskGroup sets up the task group name for TaskSelector.
func WithTaskGroup(taskGroup string) TaskOpts {
	return func(in *TaskSelector) {
		if taskGroup != "" {
			in.taskGroup = fmt.Sprintf(fmtCopilotTaskGroup, taskGroup)
		}
	}
}

// WithTaskID sets up the task ID for TaskSelector.
func WithTaskID(id string) TaskOpts {
	return func(in *TaskSelector) {
		in.taskID = id
	}
}

// RunningTask has the user select a running task. Callers can provide either app and env names,
// or use default cluster.
func (s *TaskSelector) RunningTask(msg, help string, opts ...TaskOpts) (*awsecs.Task, error) {
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
		msg,
		help,
		taskStrList,
		prompt.WithFinalMessage(taskFinalMsg),
	)
	if err != nil {
		return nil, fmt.Errorf("select running task: %w", err)
	}
	return taskStrMap[task], nil
}

// GetDeployedServiceOpts sets up optional parameters for GetDeployedServiceOpts function.
type GetDeployedWorkloadOpts func(*DeploySelector)

// DeployedWorkloadFilter determines if a service or job should be included in the results.
type DeployedWorkloadFilter func(*DeployedWorkload) (bool, error)

// WithName sets up the wkld name for DeploySelector.
func WithName(name string) GetDeployedWorkloadOpts {
	return func(in *DeploySelector) {
		in.name = name
	}
}

// WithEnv sets up the env name for DeploySelector.
func WithEnv(env string) GetDeployedWorkloadOpts {
	return func(in *DeploySelector) {
		in.env = env
	}
}

// WithWkldFilter sets up filters for DeploySelector
func WithWkldFilter(filter DeployedWorkloadFilter) GetDeployedWorkloadOpts {
	return func(in *DeploySelector) {
		in.filters = append(in.filters, filter)
	}
}

// WithServiceTypesFilter sets up a ServiceType filter for DeploySelector
func WithServiceTypesFilter(svcTypes []string) GetDeployedWorkloadOpts {
	return WithWkldFilter(func(svc *DeployedWorkload) (bool, error) {
		for _, svcType := range svcTypes {
			if svc.Type == svcType {
				return true, nil
			}
		}
		return false, nil
	})
}

// DeployedWorkload contains the name and environment name of the deployed workload.
type DeployedWorkload struct {
	Name string
	Env  string
	Type string
}

// String returns a string representation of the workload's name and environment.
func (w *DeployedWorkload) String() string {
	return fmt.Sprintf("%s (%s)", w.Name, w.Env)
}

// DeployedJob contains the name and environment of the deployed job.
type DeployedJob struct {
	Name string
	Env  string
}

// String returns a string representation of the job's name and environment.
func (j *DeployedJob) String() string {
	return fmt.Sprintf("%s (%s)", j.Name, j.Env)
}

// DeployedService contains the name and environment of the deployed service.
type DeployedService struct {
	Name    string
	Env     string
	SvcType string
}

// String returns a string representation of the service's name and environment.
func (s *DeployedService) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.Env)
}

// Task has the user select a task. Callers can provide an environment, an app, or a "use default cluster" option
// to filter the returned tasks.
func (s *CFTaskSelector) Task(msg, help string, opts ...GetDeployedTaskOpts) (string, error) {
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
	choice, err := s.prompt.SelectOne(msg, help, choices, prompt.WithFinalMessage(taskFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select task for deletion: %w", err)
	}
	return choice, nil
}

// DeployedJob has the user select a deployed job. Callers can provide either a particular environment,
// a particular job to filter on, or both.
func (s *DeploySelector) DeployedJob(msg, help string, app string, opts ...GetDeployedWorkloadOpts) (*DeployedJob, error) {
	j, err := s.deployedWorkload(jobWorkloadType, msg, help, app, opts...)
	if err != nil {
		return nil, err
	}
	return &DeployedJob{
		Name: j.Name,
		Env:  j.Env,
	}, nil
}

// DeployedService has the user select a deployed service. Callers can provide either a particular environment,
// a particular service to filter on, or both.
func (s *DeploySelector) DeployedService(msg, help string, app string, opts ...GetDeployedWorkloadOpts) (*DeployedService, error) {
	svc, err := s.deployedWorkload(svcWorkloadType, msg, help, app, opts...)
	if err != nil {
		return nil, err
	}
	return &DeployedService{
		Name:    svc.Name,
		Env:     svc.Env,
		SvcType: svc.Type,
	}, nil
}

func (s *DeploySelector) deployedWorkload(workloadType string, msg, help string, app string, opts ...GetDeployedWorkloadOpts) (*DeployedWorkload, error) {
	for _, opt := range opts {
		opt(s)
	}

	var isWorkloadDeployed func(string, string, string) (bool, error)
	var listDeployedWorkloads func(string, string) ([]string, error)
	var finalMessage string
	switch workloadType {
	case svcWorkloadType:
		isWorkloadDeployed = s.deployStoreSvc.IsServiceDeployed
		listDeployedWorkloads = s.deployStoreSvc.ListDeployedServices
		finalMessage = deployedSvcFinalMsg
	case jobWorkloadType:
		isWorkloadDeployed = s.deployStoreSvc.IsJobDeployed
		listDeployedWorkloads = s.deployStoreSvc.ListDeployedJobs
		finalMessage = deployedJobFinalMsg
	default:
		return nil, fmt.Errorf("unrecognized workload type %s", workloadType)
	}

	var err error
	var envNames []string
	wkldTypes := map[string]string{}
	// Type is only utilized by the filtering functionality. No need to retrieve types if filters are not being applied
	if len(s.filters) > 0 {
		workloads, err := s.workloadLister.ListWorkloads(app)
		if err != nil {
			return nil, fmt.Errorf("list %ss: %w", workloadType, err)
		}
		for _, wkld := range workloads {
			wkldTypes[wkld.Name] = wkld.Type
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
	wkldEnvs := []*DeployedWorkload{}
	for _, envName := range envNames {
		var wkldNames []string
		if s.name != "" {
			deployed, err := isWorkloadDeployed(app, envName, s.name)
			if err != nil {
				return nil, fmt.Errorf("check if %s %s is deployed in environment %s: %w", workloadType, s.name, envName, err)
			}
			if !deployed {
				continue
			}
			wkldNames = append(wkldNames, s.name)
		} else {
			wkldNames, err = listDeployedWorkloads(app, envName)
			if err != nil {
				return nil, fmt.Errorf("list deployed %ss for environment %s: %w", workloadType, envName, err)
			}
		}
		for _, wkldName := range wkldNames {
			wkldEnv := &DeployedWorkload{
				Name: wkldName,
				Env:  envName,
				Type: wkldTypes[wkldName],
			}
			wkldEnvs = append(wkldEnvs, wkldEnv)
		}
	}
	if len(wkldEnvs) == 0 {
		return nil, fmt.Errorf("no deployed %ss found in application %s", workloadType, color.HighlightUserInput(app))
	}

	if wkldEnvs, err = s.filterWorkloads(wkldEnvs); err != nil {
		return nil, err
	}

	if len(wkldEnvs) == 0 {
		return nil, fmt.Errorf("no matching deployed %ss found in application %s", workloadType, color.HighlightUserInput(app))
	}
	// return if only one deployed workload found
	var deployedWkld *DeployedWorkload
	if len(wkldEnvs) == 1 {
		deployedWkld = wkldEnvs[0]
		if s.name == "" && s.env == "" {
			log.Infof("Found only one deployed %s %s in environment %s\n", workloadType, color.HighlightUserInput(deployedWkld.Name), color.HighlightUserInput(deployedWkld.Env))
		}
		if (s.name != "") != (s.env != "") {
			log.Infof("%s %s found in environment %s\n", strings.ToTitle(workloadType), color.HighlightUserInput(deployedWkld.Name), color.HighlightUserInput(deployedWkld.Env))
		}
		return deployedWkld, nil
	}

	wkldEnvNames := make([]string, len(wkldEnvs))
	wkldEnvNameMap := map[string]*DeployedWorkload{}
	for i, svc := range wkldEnvs {
		wkldEnvNames[i] = svc.String()
		wkldEnvNameMap[wkldEnvNames[i]] = svc
	}

	wkldEnvName, err := s.prompt.SelectOne(
		msg,
		help,
		wkldEnvNames,
		prompt.WithFinalMessage(finalMessage),
	)
	if err != nil {
		return nil, fmt.Errorf("select deployed %ss for application %s: %w", workloadType, app, err)
	}
	deployedWkld = wkldEnvNameMap[wkldEnvName]

	return deployedWkld, nil
}

func (s *DeploySelector) filterWorkloads(inWorkloads []*DeployedWorkload) ([]*DeployedWorkload, error) {
	outWorkloads := inWorkloads
	for _, filter := range s.filters {
		if result, err := filterDeployedServices(filter, outWorkloads); err != nil {
			return nil, err
		} else {
			outWorkloads = result
		}
	}
	return outWorkloads, nil
}

// Service fetches all services in the workspace and then prompts the user to select one.
func (s *LocalWorkloadSelector) Service(msg, help string) (string, error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsServiceNames, err := s.retrieveWorkspaceServices()
	if err != nil {
		return "", fmt.Errorf("retrieve services from workspace: %w", err)
	}
	storeServiceNames, err := s.ConfigSelector.workloadLister.ListServices(summary.Application)
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

	selectedServiceName, err := s.prompt.SelectOne(msg, help, serviceNames, prompt.WithFinalMessage(svcNameFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedServiceName, nil
}

// Job fetches all jobs in the workspace and then prompts the user to select one.
func (s *LocalWorkloadSelector) Job(msg, help string) (string, error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsJobNames, err := s.retrieveWorkspaceJobs()
	if err != nil {
		return "", fmt.Errorf("retrieve jobs from workspace: %w", err)
	}
	storeJobNames, err := s.ConfigSelector.workloadLister.ListJobs(summary.Application)
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

	selectedJobName, err := s.prompt.SelectOne(msg, help, jobNames, prompt.WithFinalMessage(jobNameFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil
}

// Workload fetches all jobs and services in an app and prompts the user to select one.
func (s *LocalWorkloadSelector) Workload(msg, help string) (wl string, err error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsWlNames, err := s.retrieveWorkspaceWorkloads()
	if err != nil {
		return "", fmt.Errorf("retrieve jobs and services from workspace: %w", err)
	}
	storeWls, err := s.ConfigSelector.workloadLister.ListWorkloads(summary.Application)
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
	selectedWlName, err := s.prompt.SelectOne(msg, help, wlNames, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select workload: %w", err)
	}
	return selectedWlName, nil
}

// LocalEnvironment fetches all environments belong to the app in the workspace and prompts the user to select one.
func (s *LocalEnvironmentSelector) LocalEnvironment(msg, help string) (wl string, err error) {
	summary, err := s.ws.Summary()
	if err != nil {
		return "", fmt.Errorf("read workspace summary: %w", err)
	}
	wsEnvNames, err := s.ws.ListEnvironments()
	if err != nil {
		return "", fmt.Errorf("retrieve environments from workspace: %w", err)
	}
	envs, err := s.appEnvLister.ListEnvironments(summary.Application)
	if err != nil {
		return "", fmt.Errorf("retrieve environments from store: %w", err)
	}
	filteredEnvNames := filterEnvsByName(envs, wsEnvNames)
	if len(filteredEnvNames) == 0 {
		return "", ErrLocalEnvsNotFound
	}
	if len(filteredEnvNames) == 1 {
		log.Infof("Only found one environment, defaulting to: %s\n", color.HighlightUserInput(filteredEnvNames[0]))
		return filteredEnvNames[0], nil
	}
	selectedEnvName, err := s.prompt.SelectOne(msg, help, filteredEnvNames, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

func filterEnvsByName(envs []*config.Environment, wantedNames []string) []string {
	// TODO: refactor this and `filterWlsByName`  when generic supports using common struct fields: https://github.com/golang/go/issues/48522
	isWanted := make(map[string]bool)
	for _, name := range wantedNames {
		isWanted[name] = true
	}
	var filtered []string
	for _, wl := range envs {
		if _, ok := isWanted[wl.Name]; !ok {
			continue
		}
		filtered = append(filtered, wl.Name)
	}
	return filtered
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

// WsPipeline fetches all the pipelines in a workspace and prompts the user to select one.
func (s *WsPipelineSelector) WsPipeline(msg, help string) (*workspace.PipelineManifest, error) {
	pipelines, err := s.ws.ListPipelines()
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	if len(pipelines) == 0 {
		return nil, errors.New("no pipelines found")
	}
	var pipelineNames []string
	for _, pipeline := range pipelines {
		pipelineNames = append(pipelineNames, pipeline.Name)
	}
	if len(pipelineNames) == 1 {
		log.Infof("Only found one pipeline; defaulting to: %s\n", color.HighlightUserInput(pipelineNames[0]))
		return &workspace.PipelineManifest{
			Name: pipelines[0].Name,
			Path: pipelines[0].Path,
		}, nil
	}
	selectedPipeline, err := s.prompt.SelectOne(msg, help, pipelineNames, prompt.WithFinalMessage(pipelineFinalMsg))
	if err != nil {
		return nil, fmt.Errorf("select pipeline: %w", err)
	}
	return &workspace.PipelineManifest{
		Name: selectedPipeline,
		Path: s.pipelinePath(pipelines, selectedPipeline),
	}, nil
}

// DeployedPipeline fetches all the pipelines in a workspace and prompts the user to select one.
func (s *CodePipelineSelector) DeployedPipeline(msg, help, app string) (deploy.Pipeline, error) {
	pipelines, err := s.pipelineLister.ListDeployedPipelines(app)
	if err != nil {
		return deploy.Pipeline{}, fmt.Errorf("list deployed pipelines: %w", err)
	}
	if len(pipelines) == 0 {
		return deploy.Pipeline{}, errors.New("no deployed pipelines found")
	}
	if len(pipelines) == 1 {
		log.Infof("Only one deployed pipeline found; defaulting to: %s\n", color.HighlightUserInput(pipelines[0].Name))
		return pipelines[0], nil
	}

	var pipelineNames []string
	pipelineNameToInfo := make(map[string]deploy.Pipeline)
	for _, pipeline := range pipelines {
		pipelineNames = append(pipelineNames, pipeline.Name)
		pipelineNameToInfo[pipeline.Name] = pipeline
	}
	selectedPipeline, err := s.prompt.SelectOne(msg, help, pipelineNames, prompt.WithFinalMessage(pipelineFinalMsg))
	if err != nil {
		return deploy.Pipeline{}, fmt.Errorf("select pipeline: %w", err)
	}
	return pipelineNameToInfo[selectedPipeline], nil
}

// Service fetches all services in an app and prompts the user to select one.
func (s *ConfigSelector) Service(msg, help, app string) (string, error) {
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
	selectedSvcName, err := s.prompt.SelectOne(msg, help, services, prompt.WithFinalMessage(svcNameFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedSvcName, nil
}

// Job fetches all jobs in an app and prompts the user to select one.
func (s *ConfigSelector) Job(msg, help, app string) (string, error) {
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
	selectedJobName, err := s.prompt.SelectOne(msg, help, jobs, prompt.WithFinalMessage(jobNameFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil
}

// Environment fetches all the environments in an app and prompts the user to select one.
func (s *AppEnvSelector) Environment(msg, help, app string, additionalOpts ...string) (string, error) {
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

	selectedEnvName, err := s.prompt.SelectOne(msg, help, envs, prompt.WithFinalMessage(envNameFinalMessage))
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

// Environments fetches all the environments in an app and prompts the user to select one OR MORE.
// The List of options decreases as envs are chosen. Chosen envs displayed above with the finalMsg.
func (s *AppEnvSelector) Environments(prompt, help, app string, finalMsgFunc func(int) prompt.PromptConfig) ([]string, error) {
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
func (s *AppEnvSelector) Application(msg, help string, additionalOpts ...string) (string, error) {
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

	appNames = append(appNames, additionalOpts...)
	app, err := s.prompt.SelectOne(msg, help, appNames, prompt.WithFinalMessage(appNameFinalMessage))
	if err != nil {
		return "", fmt.Errorf("select application: %w", err)
	}
	return app, nil
}

// Dockerfile asks the user to select from a list of Dockerfiles in the current
// directory or one level down. If no dockerfiles are found, it asks for a custom path.
func (s *WorkspaceSelector) Dockerfile(selPrompt, notFoundPrompt, selHelp, notFoundHelp string, pathValidator prompt.ValidatorFunc) (string, error) {
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
		prompt.WithFinalMessage(dockerfileFinalMsg),
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
		prompt.WithFinalMessage(dockerfileFinalMsg))
	if err != nil {
		return "", fmt.Errorf("get custom Dockerfile path: %w", err)
	}
	return sel, nil
}

// Schedule asks the user to select either a rate, preset cron, or custom cron.
func (s *WorkspaceSelector) Schedule(scheduleTypePrompt, scheduleTypeHelp string, scheduleValidator, rateValidator prompt.ValidatorFunc) (string, error) {
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

// Topics asks the user to select from all Copilot-managed SNS topics *which are deployed
// across all environments* and returns the topic structs.
func (s *DeploySelector) Topics(promptMsg, help, app string) ([]deploy.Topic, error) {
	envs, err := s.appEnvLister.ListEnvironments(app)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	if len(envs) == 0 {
		log.Infoln("No environments are currently deployed. Skipping subscription selection.")
		return nil, nil
	}

	envTopics := make(map[string][]deploy.Topic, len(envs))
	for _, env := range envs {
		topics, err := s.deployStoreSvc.ListSNSTopics(app, env.Name)
		if err != nil {
			return nil, fmt.Errorf("list SNS topics: %w", err)
		}
		envTopics[env.Name] = topics
	}

	// Get only topics deployed in all environments.
	// Computes the intersection of the `envTopics` lists.
	overallTopics := make(map[string]deploy.Topic)
	// Initialize the list of topics.
	for _, topic := range envTopics[envs[0].Name] {
		overallTopics[topic.String()] = topic
	}
	// Then do the pairwise intersection of all other envs.
	for _, env := range envs[1:] {
		topics := envTopics[env.Name]
		overallTopics = intersect(overallTopics, topics)
	}

	if len(overallTopics) == 0 {
		log.Infoln("No SNS topics are currently deployed in all environments. You can customize subscriptions in your manifest.")
		return nil, nil
	}
	// Create the list of options.
	var topicDescriptions []string
	for t := range overallTopics {
		topicDescriptions = append(topicDescriptions, t)
	}
	// Sort descriptions by ARN, which implies sorting by workload name and then by topic name due to
	// behavior of `intersect`. That is, the `overallTopics` map is guaranteed to contain topics
	// referencing the same environment.
	sort.Slice(topicDescriptions, func(i, j int) bool {
		return overallTopics[topicDescriptions[i]].ARN() < overallTopics[topicDescriptions[j]].ARN()
	})

	selectedTopics, err := s.prompt.MultiSelect(
		promptMsg,
		help,
		topicDescriptions,
		nil,
		prompt.WithFinalMessage(topicFinalMsg),
	)
	if err != nil {
		return nil, fmt.Errorf("select SNS topics: %w", err)
	}

	// Get the topics from the topic descriptions again.
	var topics []deploy.Topic
	for _, t := range selectedTopics {
		topics = append(topics, overallTopics[t])
	}
	return topics, nil
}

func (s *AppEnvSelector) retrieveApps() ([]string, error) {
	apps, err := s.appEnvLister.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}
	return appNames, nil
}

func (s *AppEnvSelector) retrieveEnvironments(app string) ([]string, error) {
	envs, err := s.appEnvLister.ListEnvironments(app)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	envsNames := make([]string, len(envs))
	for ind, env := range envs {
		envsNames[ind] = env.Name
	}
	return envsNames, nil
}

func (s *ConfigSelector) retrieveServices(app string) ([]string, error) {
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

func (s *ConfigSelector) retrieveJobs(app string) ([]string, error) {
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

func (s *LocalWorkloadSelector) retrieveWorkspaceServices() ([]string, error) {
	localServiceNames, err := s.ws.ListServices()
	if err != nil {
		return nil, err
	}
	return localServiceNames, nil
}

func (s *LocalWorkloadSelector) retrieveWorkspaceJobs() ([]string, error) {
	localJobNames, err := s.ws.ListJobs()
	if err != nil {
		return nil, err
	}
	return localJobNames, nil
}

func (s *LocalWorkloadSelector) retrieveWorkspaceWorkloads() ([]string, error) {
	localWlNames, err := s.ws.ListWorkloads()
	if err != nil {
		return nil, err
	}
	return localWlNames, nil
}

func (s *WsPipelineSelector) pipelinePath(pipelines []workspace.PipelineManifest, name string) string {
	for _, pipeline := range pipelines {
		if pipeline.Name == name {
			return pipeline.Path
		}
	}
	return ""
}

func (s *WorkspaceSelector) askRate(rateValidator prompt.ValidatorFunc) (string, error) {
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

func (s *WorkspaceSelector) askCron(scheduleValidator prompt.ValidatorFunc) (string, error) {
	cronInput, err := s.prompt.SelectOption(
		schedulePrompt,
		scheduleHelp,
		presetSchedules,
		prompt.WithFinalMessage("Fixed schedule:"),
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
			prompt.WithFinalMessage("Custom schedule:"),
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

func filterDeployedServices(filter DeployedWorkloadFilter, inServices []*DeployedWorkload) ([]*DeployedWorkload, error) {
	outServices := []*DeployedWorkload{}
	for _, svc := range inServices {
		if include, err := filter(svc); err != nil {
			return nil, err
		} else if include {
			outServices = append(outServices, svc)
		}
	}
	return outServices, nil
}

func intersect(firstMap map[string]deploy.Topic, secondArr []deploy.Topic) map[string]deploy.Topic {
	out := make(map[string]deploy.Topic)
	for _, topic := range secondArr {
		if _, ok := firstMap[topic.String()]; ok {
			out[topic.String()] = topic
		}
	}
	return out
}
