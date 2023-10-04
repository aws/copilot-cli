// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package selector provides functionality for users to select an application, environment, or service name.
package selector

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dustin/go-humanize/english"

	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	svcWorkloadType = "service"
	jobWorkloadType = "job"
	anyWorkloadType = "workload"

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
	staticSourceUseCustomPrompt         = "Enter custom path to your static site dir/file"
	staticSourceAnotherCustomPathPrompt = "Would you like to enter another path?"
	staticSourceAnotherCustomPathHelp   = "You may add multiple custom paths. Enter 'y' to type another."
)

// Final messages displayed after prompting.
const (
	appNameFinalMessage  = "Application:"
	envNameFinalMessage  = "Environment:"
	svcNameFinalMsg      = "Service name:"
	jobNameFinalMsg      = "Job name:"
	deployedJobFinalMsg  = "Job:"
	deployedSvcFinalMsg  = "Service:"
	deployedWkldFinalMsg = "Workload:"
	taskFinalMsg         = "Task:"
	workloadFinalMsg     = "Name:"
	dockerfileFinalMsg   = "Dockerfile:"
	topicFinalMsg        = "Topic subscriptions:"
	pipelineFinalMsg     = "Pipeline:"
	staticAssetsFinalMsg = "Source(s):"
	customPathFinalMsg   = "Custom Path to Source:"
	anotherFinalMsg      = "Another:"
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

// Prompter wraps the methods to ask for inputs from the terminal.
type Prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOption(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) (value string, err error)
	MultiSelectOptions(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) ([]string, error)
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
}

// deployedWorkloadsRetriever retrieves information about deployed services or jobs.
type deployedWorkloadsRetriever interface {
	ListDeployedServices(appName string, envName string) ([]string, error)
	ListDeployedJobs(appName, envName string) ([]string, error)
	ListDeployedWorkloads(appName, envName string) ([]string, error)
	IsServiceDeployed(appName string, envName string, svcName string) (bool, error)
	IsJobDeployed(appName, envName, jobName string) (bool, error)
	IsWorkloadDeployed(appName, envName, wkldName string) (bool, error)
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
	prompt       Prompter
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

	// Option, turned on by passing WithOnlyInitialized to NewLocalWorkloadSelector.
	onlyInitializedWorkloads bool
}

// LocalEnvironmentSelector is an application and environment selector, but can also choose an environment from the workspace.
type LocalEnvironmentSelector struct {
	*AppEnvSelector
	ws workspaceRetriever
}

// WorkspaceSelector selects from local workspace.
type WorkspaceSelector struct {
	*ConfigSelector
	prompt Prompter
	ws     workspaceRetriever
}

// WsPipelineSelector is a workspace pipeline selector.
type WsPipelineSelector struct {
	prompt Prompter
	ws     wsPipelinesLister
}

// CodePipelineSelector is a selector for deployed pipelines.
type CodePipelineSelector struct {
	prompt         Prompter
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
func NewCFTaskSelect(prompt Prompter, store configLister, cf taskStackDescriber) *CFTaskSelector {
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
	prompt         Prompter
	lister         taskLister
	app            string
	env            string
	defaultCluster bool
	taskGroup      string
	taskID         string
}

// NewAppEnvSelector returns a selector that chooses applications or environments.
func NewAppEnvSelector(prompt Prompter, store appEnvLister) *AppEnvSelector {
	return &AppEnvSelector{
		prompt:       prompt,
		appEnvLister: store,
	}
}

// NewConfigSelector returns a new selector that chooses applications, environments, or services from the config store.
func NewConfigSelector(prompt Prompter, store configLister) *ConfigSelector {
	return &ConfigSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		workloadLister: store,
	}
}

// NewLocalWorkloadSelector returns a new selector that chooses applications and environments from the config store, but
// services from the local workspace.
func NewLocalWorkloadSelector(prompt Prompter, store configLister, ws workspaceRetriever, options ...WorkloadSelectOption) *LocalWorkloadSelector {
	s := &LocalWorkloadSelector{
		ConfigSelector: NewConfigSelector(prompt, store),
		ws:             ws,
	}
	for _, opt := range options {
		opt(s)
	}
	return s
}

// NewLocalEnvironmentSelector returns a new selector that chooses applications from the config store, but an environment
// from the local workspace.
func NewLocalEnvironmentSelector(prompt Prompter, store configLister, ws workspaceRetriever) *LocalEnvironmentSelector {
	return &LocalEnvironmentSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		ws:             ws,
	}
}

// NewWorkspaceSelector returns a new selector that prompts for local information.
func NewWorkspaceSelector(prompt Prompter, ws workspaceRetriever) *WorkspaceSelector {
	return &WorkspaceSelector{
		prompt: prompt,
		ws:     ws,
	}
}

// NewWsPipelineSelector returns a new selector with pipelines from the local workspace.
func NewWsPipelineSelector(prompt Prompter, ws wsPipelinesLister) *WsPipelineSelector {
	return &WsPipelineSelector{
		prompt: prompt,
		ws:     ws,
	}
}

// NewAppPipelineSelector returns new selectors with deployed pipelines and apps.
func NewAppPipelineSelector(prompt Prompter, store configLister, lister codePipelineLister) *AppPipelineSelector {
	return &AppPipelineSelector{
		AppEnvSelector: NewAppEnvSelector(prompt, store),
		CodePipelineSelector: &CodePipelineSelector{
			prompt:         prompt,
			pipelineLister: lister,
		},
	}
}

// NewDeploySelect returns a new selector that chooses services and environments from the deploy store.
func NewDeploySelect(prompt Prompter, configStore configLister, deployStore deployedWorkloadsRetriever) *DeploySelector {
	return &DeploySelector{
		ConfigSelector: NewConfigSelector(prompt, configStore),
		deployStoreSvc: deployStore,
	}
}

// NewTaskSelector returns a new selector that chooses a running task.
func NewTaskSelector(prompt Prompter, lister taskLister) *TaskSelector {
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

// GetDeployedWorkloadOpts sets up optional parameters for GetDeployedWorkloadOpts function.
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

// DeployedWorkload has the user select a deployed workload. Callers can provide either a particular environment,
// a particular workload to filter on, or both.
func (s *DeploySelector) DeployedWorkload(msg, help string, app string, opts ...GetDeployedWorkloadOpts) (*DeployedWorkload, error) {
	wkld, err := s.deployedWorkload(anyWorkloadType, msg, help, app, opts...)
	if err != nil {
		return nil, err
	}
	return &DeployedWorkload{
		Name: wkld.Name,
		Env:  wkld.Env,
		Type: wkld.Type,
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
	case anyWorkloadType:
		isWorkloadDeployed = s.deployStoreSvc.IsWorkloadDeployed
		listDeployedWorkloads = s.deployStoreSvc.ListDeployedWorkloads
		finalMessage = deployedWkldFinalMsg
	default:
		return nil, fmt.Errorf("unrecognized workload type %s", workloadType)

	}

	var err error
	var envNames []string
	wkldTypes := map[string]string{}
	workloads, err := s.workloadLister.ListWorkloads(app)
	if err != nil {
		return nil, fmt.Errorf("list %ss: %w", workloadType, err)
	}
	for _, wkld := range workloads {
		wkldTypes[wkld.Name] = wkld.Type
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
		switch {
		case s.name == "" && s.env == "":
			log.Infof("Found only one deployed %s %s in environment %s\n", workloadType, color.HighlightUserInput(deployedWkld.Name), color.HighlightUserInput(deployedWkld.Env))
		case s.name != "" && s.env == "":
			log.Infof("%s found only in environment %s\n", color.HighlightUserInput(deployedWkld.Name), color.HighlightUserInput(deployedWkld.Env))
		case s.name == "" && s.env != "":
			log.Infof("Only the %s %s is found in the environment %s\n", deployedWkld.Type, color.HighlightUserInput(deployedWkld.Name), color.HighlightUserInput(deployedWkld.Env))
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
		return nil, err
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
	options, err := s.getWorkloadSelectOptions(svcWorkloadType)
	if err != nil {
		return "", err
	}

	if len(options) == 1 {
		log.Infof("Found only one service, defaulting to: %s\n", color.HighlightUserInput(options[0].Value))
		return options[0].Value, nil
	}
	selectedSvcName, err := s.prompt.SelectOption(msg, help, options, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select service: %w", err)
	}
	return selectedSvcName, nil
}

// Job fetches all jobs in the workspace and then prompts the user to select one.
func (s *LocalWorkloadSelector) Job(msg, help string) (string, error) {
	options, err := s.getWorkloadSelectOptions(jobWorkloadType)
	if err != nil {
		return "", err
	}

	if len(options) == 1 {
		log.Infof("Found only one job, defaulting to: %s\n", color.HighlightUserInput(options[0].Value))
		return options[0].Value, nil
	}
	selectedJobName, err := s.prompt.SelectOption(msg, help, options, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil

}

func (s *LocalWorkloadSelector) getWorkloadSelectOptions(workloadType string) ([]prompt.Option, error) {
	pluralNounString := english.PluralWord(2, workloadType, "")

	summary, err := s.ws.Summary()
	if err != nil {
		return nil, fmt.Errorf("read workspace summary: %w", err)
	}
	wsWlNames, err := s.retrieveWorkspaceWorkloads(workloadType)
	if err != nil {
		return nil, fmt.Errorf("retrieve %s from workspace: %w", pluralNounString, err)
	}

	storeWls, err := s.retrieveStoreWorkloads(summary.Application, workloadType)
	if err != nil {
		return nil, fmt.Errorf("retrieve %s from store: %w", pluralNounString, err)
	}

	var options []prompt.Option

	// Get the list of initialized workloads that are present in the workspace
	initializedLocalWorkloads := filterWlsByName(storeWls, wsWlNames)
	for _, wl := range initializedLocalWorkloads {
		options = append(options, prompt.Option{Value: wl})
	}

	// Return early if we're only looking for config store workloads.
	if s.onlyInitializedWorkloads {
		if len(initializedLocalWorkloads) == 0 {
			return nil, fmt.Errorf("no %s found", pluralNounString)
		}
		return options, nil
	}

	// If there are no local workload names, error out.
	if len(wsWlNames) == 0 {
		return nil, fmt.Errorf("no %s found in workspace", pluralNounString)
	}
	// Get the list of un-initialized workloads that are present in the workspace and add them to options.
	unInitializedLocalWorkloads := filterOutItems(wsWlNames, initializedLocalWorkloads, func(a string) string { return a })
	for _, wl := range unInitializedLocalWorkloads {
		options = append(options, prompt.Option{
			Value: wl,
			Hint:  "uninitialized",
		})
	}
	return options, nil
}

// WorkloadSelectOption represents an option for customizing LocalWorkloadSelector's behavior.
type WorkloadSelectOption func(selector *LocalWorkloadSelector)

// OnlyInitializedWorkloads modifies LocalWorkloadSelector to show only the initialized workloads in the workspace,
// ignoring uninitialized workloads with local manifests.
var OnlyInitializedWorkloads WorkloadSelectOption = func(s *LocalWorkloadSelector) {
	s.onlyInitializedWorkloads = true
}

// Workloads fetches all jobs and services in a workspace and prompts the user to select one or more.
// It can optionally select only initialized workloads which exist in the app (by passing the
// OnlyInitializedWorkloads option to NewLocalWorkloadSelector) or list all workloads for which
// there are manifests in the workspace (default).
func (s *LocalWorkloadSelector) Workloads(msg, help string) ([]string, error) {
	options, err := s.getWorkloadSelectOptions(anyWorkloadType)
	if err != nil {
		return nil, err
	}
	if len(options) == 1 {
		log.Infof("Found only one workload, defaulting to: %s\n", color.HighlightUserInput(options[0].Value))
		return []string{options[0].Value}, nil
	}

	selectedWlNames, err := s.prompt.MultiSelectOptions(msg, help, options, prompt.WithFinalMessage("Names:"))
	if err != nil {
		return nil, fmt.Errorf("select workloads: %w", err)
	}
	return selectedWlNames, nil
}

// Workload fetches all jobs and services in a workspace and prompts the user to select one.
// It can optionally select only initialized workloads which exist in the app (by passing the
// OnlyInitializedWorkloads option to NewLocalWorkloadSelector) or list all workloads for which
// there are manifests in the workspace (default).
func (s *LocalWorkloadSelector) Workload(msg, help string) (wl string, err error) {
	options, err := s.getWorkloadSelectOptions(anyWorkloadType)
	if err != nil {
		return "", err
	}
	if len(options) == 1 {
		log.Infof("Found only one workload, defaulting to: %s\n", color.HighlightUserInput(options[0].Value))
		return options[0].Value, nil
	}

	selectedWlName, err := s.prompt.SelectOption(msg, help, options, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select workload: %w", err)
	}
	return selectedWlName, nil
}

// filterOutItems is a generic function to return the subset of allItems which does not include the items specified in
// unwantedItems. stringFunc is a function which maps the unwantedItem type T to a string value. For example, one can
// convert a struct of type *config.Workload to a string by passing
//
//	func(w *config.Workload) string { return w.Name }
//
// as the stringFunc parameter.
func filterOutItems[T any](allItems []string, unwantedItems []T, stringFunc func(T) string) []string {
	isUnwanted := make(map[string]bool)
	for _, item := range unwantedItems {
		isUnwanted[stringFunc(item)] = true
	}
	var filtered []string
	for _, str := range allItems {
		if isUnwanted[str] {
			continue
		}
		filtered = append(filtered, str)
	}
	return filtered
}

// LocalEnvironment fetches all environments belong to the app in the workspace and prompts the user to select one.
func (s *LocalEnvironmentSelector) LocalEnvironment(msg, help string) (string, error) {
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
		log.Infof("Found only one environment, defaulting to: %s\n", color.HighlightUserInput(filteredEnvNames[0]))
		return filteredEnvNames[0], nil
	}
	selectedEnvName, err := s.prompt.SelectOne(msg, help, filteredEnvNames, prompt.WithFinalMessage(workloadFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select environment: %w", err)
	}
	return selectedEnvName, nil
}

func filterEnvsByName(envs []*config.Environment, wantedNames []string) []string {
	return filterItemsByStrings(wantedNames, envs, func(e *config.Environment) string { return e.Name })
}

func filterWlsByName(wls []*config.Workload, wantedNames []string) []string {
	return filterItemsByStrings(wantedNames, wls, func(w *config.Workload) string { return w.Name })
}

// filterItemsByStrings is a generic function to return the subset of wantedStrings that exists in possibleItems.
// stringFunc is a method to convert the generic item type (T) to a string; for example, one can convert a struct of type
// *config.Workload to a string by passing
//
//	func(w *config.Workload) string { return w.Name }.
//
// Likewise, filterItemsByStrings can work on a list of strings by returning the unmodified item:
//
//	filterItemsByStrings(wantedStrings, stringSlice2, func(s string) string { return s })
//
// It returns a list of strings (items whose stringFunc() exists in the list of wantedStrings).
func filterItemsByStrings[T any](wantedStrings []string, possibleItems []T, stringFunc func(T) string) []string {
	m := make(map[string]bool)
	for _, item := range wantedStrings {
		m[item] = true
	}
	res := make([]string, 0, len(wantedStrings))
	for _, item := range possibleItems {
		if m[stringFunc(item)] {
			res = append(res, stringFunc(item))
		}
	}
	return res
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
		log.Infof("Found only one pipeline; defaulting to: %s\n", color.HighlightUserInput(pipelineNames[0]))
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
		return "", &errNoServiceInApp{appName: app}
	}
	if len(services) == 1 {
		log.Infof("Found only one service, defaulting to: %s\n", color.HighlightUserInput(services[0]))
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
		return "", &errNoJobInApp{appName: app}
	}
	if len(jobs) == 1 {
		log.Infof("Found only one job, defaulting to: %s\n", color.HighlightUserInput(jobs[0]))
		return jobs[0], nil
	}
	selectedJobName, err := s.prompt.SelectOne(msg, help, jobs, prompt.WithFinalMessage(jobNameFinalMsg))
	if err != nil {
		return "", fmt.Errorf("select job: %w", err)
	}
	return selectedJobName, nil
}

// Workload fetches all workloads in an app and prompts the user to select one.
func (s *ConfigSelector) Workload(msg, help, app string) (string, error) {
	services, err := s.retrieveServices(app)
	if err != nil {
		return "", err
	}
	jobs, err := s.retrieveJobs(app)
	if err != nil {
		return "", err
	}

	workloads := append(services, jobs...)
	if len(workloads) == 0 {
		return "", &errNoWorkloadInApp{appName: app}
	}
	if len(workloads) == 1 {
		log.Infof("Found only one workload, defaulting to: %s\n", color.HighlightUserInput(workloads[0]))
		return workloads[0], nil
	}
	selectedWorkloadName, err := s.prompt.SelectOne(msg, help, workloads, prompt.WithFinalMessage("Workload name:"))
	if err != nil {
		return "", fmt.Errorf("select workload: %w", err)
	}
	return selectedWorkloadName, nil
}

// Environment fetches all the environments in an app and prompts the user to select one.
func (s *AppEnvSelector) Environment(msg, help, app string, additionalOpts ...prompt.Option) (string, error) {
	envs, err := s.retrieveEnvironments(app)
	if err != nil {
		return "", fmt.Errorf("get environments for app %s from metadata store: %w", app, err)
	}

	envOpts := make([]prompt.Option, len(envs))
	for k := range envs {
		envOpts[k] = prompt.Option{Value: envs[k]}
	}
	envOpts = append(envOpts, additionalOpts...)
	if len(envOpts) == 0 {
		log.Infof("Couldn't find any environments associated with app %s, try initializing one: %s\n",
			color.HighlightUserInput(app),
			color.HighlightCode("copilot env init"))
		return "", fmt.Errorf("no environments found in app %s", app)
	}
	if len(envOpts) == 1 {
		log.Infof("Only found one option, defaulting to: %s\n", color.HighlightUserInput(envOpts[0].Value))
		return envOpts[0].Value, nil
	}

	selectedEnvName, err := s.prompt.SelectOption(msg, help, envOpts, prompt.WithFinalMessage(envNameFinalMessage))
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
		log.Infof("Found only one application, defaulting to: %s\n", color.HighlightUserInput(appNames[0]))
		return appNames[0], nil
	}

	appNames = append(appNames, additionalOpts...)
	app, err := s.prompt.SelectOne(msg, help, appNames, prompt.WithFinalMessage(appNameFinalMessage))
	if err != nil {
		return "", fmt.Errorf("select application: %w", err)
	}
	return app, nil
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

func (s *LocalWorkloadSelector) retrieveStoreWorkloads(appName, wlType string) ([]*config.Workload, error) {
	switch wlType {
	case svcWorkloadType:
		return s.ConfigSelector.workloadLister.ListServices(appName)
	case jobWorkloadType:
		return s.ConfigSelector.workloadLister.ListJobs(appName)
	case anyWorkloadType:
		return s.ConfigSelector.workloadLister.ListWorkloads(appName)
	}
	return nil, fmt.Errorf("unrecognized workload type %s", wlType)
}

func (s *LocalWorkloadSelector) retrieveWorkspaceWorkloads(wlType string) ([]string, error) {
	switch wlType {
	case svcWorkloadType:
		return s.retrieveWorkspaceServices()
	case jobWorkloadType:
		return s.retrieveWorkspaceJobs()
	case anyWorkloadType:
		return s.ws.ListWorkloads()
	}
	return nil, fmt.Errorf("unrecognized workload type %s", wlType)
}

func (s *WsPipelineSelector) pipelinePath(pipelines []workspace.PipelineManifest, name string) string {
	for _, pipeline := range pipelines {
		if pipeline.Name == name {
			return pipeline.Path
		}
	}
	return ""
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
