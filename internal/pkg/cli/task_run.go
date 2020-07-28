// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"io"
	"time"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	appEnvOptionNone      = "None (run in default VPC)"
	defaultDockerfilePath  = "Dockerfile"
	imageTagLatest         = "latest"
	numCWLogsCallsPerRound = 10
)

const (
	fmtRepoName         = "copilot-%s"
	fmtImageURI         = "%s:%s"
	fmtTaskLogGroupName = "/copilot/%s"
)

var (
	errNumNotPositive = errors.New("number of tasks must be positive")
	errCpuNotPositive = errors.New("CPU units must be positive")
	errMemNotPositive = errors.New("memory must be positive")
)

var (
	taskRunAppPrompt       = fmt.Sprintf("In which %s would you like to run this %s?", color.Emphasize("application"), color.Emphasize("task"))
	taskRunEnvPrompt       = fmt.Sprintf("In which %s would you like to run this %s?", color.Emphasize("environment"), color.Emphasize("task"))
	taskRunGroupNamePrompt = fmt.Sprintf("What would you like to %s your task group?", color.Emphasize("name"))

	taskRunAppPromptHelp = fmt.Sprintf(`Task will be deployed to the selected application. 
Select %s to run the task in your default VPC instead of any existing application.`, color.Emphasize(appEnvOptionNone))
	taskRunEnvPromptHelp = fmt.Sprintf(`Task will be deployed to the selected environment.
Select %s to run the task in your default VPC instead of any existing environment.`, color.Emphasize(appEnvOptionNone))
	taskRunGroupNamePromptHelp = `The group name of the task. Tasks with the same group name share the same 
set of resources, including CloudFormation stack, CloudWatch log group, 
task definition and ECR repository.`
)

type runTaskVars struct {
	*GlobalOpts
	count  int
	cpu    int
	memory int

	groupName string

	image          string
	dockerfilePath string
	imageTag       string

	taskRole      string
	executionRole string

	subnets        []string
	securityGroups []string
	env            string

	envVars map[string]string
	command string

	follow bool
}

type runTaskOpts struct {
	runTaskVars

	// Interfaces to interact with dependencies.
	fs      afero.Fs
	store   store
	sel     appEnvSelector
	spinner progress

	// Fields below are configured at runtime.
	deployer          taskDeployer
	repository        repositoryService
	runner            taskRunner
	sess              *awssession.Session
	targetEnvironment *config.Environment

	taskDescriber taskDescriber
	cwLogsGetter  cwlogService
	logWriter     io.Writer

	configureSession     func() error
	configureRuntimeOpts func() error
	configureRepository  func() error
}

func newTaskRunOpts(vars runTaskVars) (*runTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	opts := runTaskOpts{
		runTaskVars: vars,

		fs:      &afero.Afero{Fs: afero.NewOsFs()},
		store:   store,
		sel:     selector.NewSelect(vars.prompt, store),
		spinner: termprogress.NewSpinner(),
	}

	opts.configureRuntimeOpts = func() error {
		opts.runner = opts.configureRunner()
		opts.deployer = cloudformation.New(opts.sess)

		opts.taskDescriber = ecs.New(opts.sess)
		opts.cwLogsGetter = cloudwatchlogs.New(opts.sess)
		opts.logWriter = log.OutputWriter

		opts.configureRepository = opts.repositoryConfigurer()
		return nil
	}

	return &opts, nil
}

func (o *runTaskOpts) repositoryConfigurer() func() error {
	return func() error {
		repoName := fmt.Sprintf(fmtRepoName, o.groupName)
		registry := ecr.New(o.sess)
		repository, err := repository.New(repoName, registry)
		if err != nil {
			return fmt.Errorf("initialize repository %s: %w", repoName, err)
		}
		o.repository = repository
		return nil
	}
}

func (o *runTaskOpts) configureRunner() taskRunner {
	vpcGetter := ec2.New(o.sess)
	ecsService := ecs.New(o.sess)

	if o.env != "" {
		return &task.EnvRunner{
			Count:     o.count,
			GroupName: o.groupName,

			App: o.AppName(),
			Env: o.env,

			VPCGetter:     vpcGetter,
			ClusterGetter: resourcegroups.New(o.sess),
			Starter:       ecsService,
		}
	}

	if o.subnets != nil {
		return &task.NetworkConfigRunner{
			Count:     o.count,
			GroupName: o.groupName,

			Subnets:        o.subnets,
			SecurityGroups: o.securityGroups,

			ClusterGetter: ecsService,
			Starter:       ecsService,
		}
	}

	return &task.DefaultVPCRunner{
		Count:     o.count,
		GroupName: o.groupName,

		VPCGetter:     vpcGetter,
		ClusterGetter: ecsService,
		Starter:       ecsService,
	}

}

func (o *runTaskOpts) configureSessAndEnv() error {
	var sess *awssession.Session
	var env *config.Environment

	provider := session.NewProvider()
	if o.env != "" {
		var err error
		env, err = o.targetEnv()
		if err != nil {
			return err
		}

		sess, err = provider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return fmt.Errorf("get session from role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
		}
	} else {
		var err error
		sess, err = provider.Default()
		if err != nil {
			return fmt.Errorf("get default session: %w", err)
		}
	}

	o.targetEnvironment = env
	o.sess = sess
	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *runTaskOpts) Validate() error {
	if o.count <= 0 {
		return errNumNotPositive
	}

	if o.cpu <= 0 {
		return errCpuNotPositive
	}

	if o.memory <= 0 {
		return errMemNotPositive
	}

	if o.groupName != "" {
		if err := basicNameValidation(o.groupName); err != nil {
			return err
		}
	}

	if o.image != "" && o.dockerfilePath != "" {
		return errors.New("cannot specify both image and Dockerfile path")
	}

	if o.dockerfilePath != "" {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}

	if o.env != "" && (o.subnets != nil || o.securityGroups != nil) {
		return errors.New("neither subnet nor security groups should be specified if environment is specified")
	}

	if o.appName != "" {
		if err := o.validateAppName(); err != nil {
			return err
		}
	}

	if o.env != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}

	return nil
}

// Ask prompts the user for any required or important fields that are not provided.
func (o *runTaskOpts) Ask() error {
	if err := o.askTaskGroupName(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute deploys and runs the task.
func (o *runTaskOpts) Execute() error {
	// NOTE: all runtime options must be configured only after session is configured
	if err := o.configureSessAndEnv(); err != nil {
		return err
	}

	if err := o.configureRuntimeOpts(); err != nil {
		return err
	}

	if err := o.deployTaskResources(); err != nil {
		return err
	}

	// NOTE: repository has to be configured only after task resources are deployed
	if err := o.configureRepository(); err != nil {
		return err
	}

	// NOTE: if image is not provided, then we build the image and push to ECR repo
	if o.image == "" {
		if err := o.buildAndPushImage(); err != nil {
			return err
		}

		tag := imageTagLatest
		if o.imageTag != "" {
			tag = o.imageTag
		}
		o.image = fmt.Sprintf(fmtImageURI, o.repository.URI(), tag)

		if err := o.updateTaskResources(); err != nil {
			return err
		}
	}

	tasks, err := o.runTask()
	if err != nil {
		return err
	}

	if o.follow {
		if err := o.displayLogStream(tasks); err != nil {
			return err
		}
	}
	return nil
}

func (o *runTaskOpts) displayLogStream(tasks []*task.Task) error {
	startTime := task.EarliestStartTime(tasks)

	logGroupName := fmt.Sprintf(fmtTaskLogGroupName, o.groupName)
	logEventsOutput := &cloudwatchlogs.LogEventsOutput{
		LastEventTime: make(map[string]int64),
	}
	for {
		for i := 1; i < numCWLogsCallsPerRound; i++ {
			logEventsOutput, err := o.cwLogsGetter.TaskLogEvents(
				logGroupName,
				logEventsOutput.LastEventTime,
				cloudwatchlogs.WithStartTime(aws.TimeUnixMilli(startTime)))
			if err != nil {
				return fmt.Errorf("display log stream: %w", err)
			}

			for _, event := range logEventsOutput.Events {
				if _, err := fmt.Fprintf(o.logWriter, event.HumanString()); err != nil {
					return fmt.Errorf("write log event: %w", err)
				}
			}
			time.Sleep(cloudwatchlogs.SleepDuration)
		}

		var (
			stopped bool
			err     error
		)
		stopped, tasks, err = o.allStopped(tasks)
		if err != nil {
			return err
		}
		if stopped {
			log.Infof("%s %s stopped.\n",
				english.PluralWord(o.count, "task", ""),
				english.PluralWord(o.count, "has", "have"))
			return nil
		}
	}
}

func (o *runTaskOpts) allStopped(tasks []*task.Task) (allStopped bool, tasksNext []*task.Task, err error) {
	taskARNs := make([]string, len(tasks))
	for idx, task := range tasks {
		taskARNs[idx] = task.TaskARN
	}

	// NOTE: all tasks are deployed to the same cluster and there are at least one tasks being deployed
	cluster := tasks[0].ClusterARN

	tasksResp, err := o.taskDescriber.DescribeTasks(cluster, taskARNs)
	if err != nil {
		return false, nil, fmt.Errorf("describe tasks: %w", err)
	}

	allStopped = true
	for _, t := range tasksResp {
		if *t.LastStatus != ecs.DesiredStatusStopped {
			allStopped = false
			tasksNext = append(tasksNext, &task.Task{
				TaskARN: *t.TaskArn,
			})
		}
	}

	return allStopped, tasksNext, nil
}

func (o *runTaskOpts) runTask() ([]*task.Task, error) {
	o.spinner.Start(fmt.Sprintf("Waiting for %s to be running for %s.", english.Plural(o.count, "task", ""), o.groupName))
	tasks, err := o.runner.Run()
	if err != nil {
		o.spinner.Stop(log.Serrorf("Failed to run %s.\n", o.groupName))
		return nil, fmt.Errorf("run task %s: %w", o.groupName, err)
	}
	o.spinner.Stop(log.Ssuccessf("%s %s %s running.\n", english.PluralWord(o.count, "task", ""), o.groupName, english.Plural(o.count, "is", "are")))
	return tasks, nil
}

func (o *runTaskOpts) buildAndPushImage() error {
	var additionalTags []string
	if o.imageTag != "" {
		additionalTags = append(additionalTags, o.imageTag)
	}

	if err := o.repository.BuildAndPush(docker.New(), o.dockerfilePath, imageTagLatest, additionalTags...); err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	return nil
}

func (o *runTaskOpts) deployTaskResources() error {
	o.spinner.Start(fmt.Sprintf("Provisioning an ECR repository, a CloudWatch log group and necessary permissions for task %s.", color.HighlightUserInput(o.groupName)))
	if err := o.deploy(); err != nil {
		o.spinner.Stop(log.Serrorln("Failed to provision task resources."))
		return fmt.Errorf("provision resources for task %s: %w", o.groupName, err)
	}
	o.spinner.Stop(log.Ssuccessln("Successfully provisioned task resources."))
	return nil
}

func (o *runTaskOpts) updateTaskResources() error {
	o.spinner.Start(fmt.Sprintf("Updating image to task %s.", color.HighlightUserInput(o.groupName)))
	if err := o.deploy(); err != nil {
		o.spinner.Stop(log.Serrorln("Failed to update task resources."))
		return fmt.Errorf("update resources for task %s: %w", o.groupName, err)
	}
	o.spinner.Stop(log.Ssuccessln("Successfully updated image to task."))
	return nil
}

func (o *runTaskOpts) deploy() error {
	var deployOpts []awscloudformation.StackOption
	if o.env != "" {
		deployOpts = []awscloudformation.StackOption{awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)}
	}
	input := &deploy.CreateTaskResourcesInput{
		Name:          o.groupName,
		CPU:           o.cpu,
		Memory:        o.memory,
		Image:         o.image,
		TaskRole:      o.taskRole,
		ExecutionRole: o.executionRole,
		Command:       o.command,
		EnvVars:       o.envVars,
		App:           o.AppName(),
		Env:           o.env,
	}
	return o.deployer.DeployTask(input, deployOpts...)
}

func (o *runTaskOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	return nil
}

func (o *runTaskOpts) validateEnvName() error {
	if o.AppName() != "" {
		if _, err := o.targetEnv(); err != nil {
			return err
		}
	} else {
		return errNoAppInWorkspace
	}

	return nil
}

func (o *runTaskOpts) askTaskGroupName() error {
	if o.groupName != "" {
		return nil
	}

	// TODO during Execute: list existing tasks like in ListApplications, ask whether to use existing tasks

	groupName, err := o.prompt.Get(
		taskRunGroupNamePrompt,
		taskRunGroupNamePromptHelp,
		basicNameValidation,
		prompt.WithFinalMessage("Task group name:"))
	if err != nil {
		return fmt.Errorf("prompt get task group name: %w", err)
	}
	o.groupName = groupName
	return nil
}

func (o *runTaskOpts) askAppName() error {
	if o.AppName() != "" {
		return nil
	}

	// If the application is empty then the user wants to run in the default VPC. Do not prompt for an environment name.
	app, err := o.sel.Application(taskRunAppPrompt, taskRunAppPromptHelp, appEnvOptionNone)
	if err != nil {
		return fmt.Errorf("ask for application: %w", err)
	}

	if app == appEnvOptionNone {
		return nil
	}

	o.appName = app
	return nil
}

func (o *runTaskOpts) askEnvName() error {
	if o.env != "" {
		return nil
	}

	// If the application is empty then the user wants to run in the default VPC. Do not prompt for an environment name.
	if o.AppName() == "" || o.subnets != nil {
		return nil
	}

	env, err := o.sel.Environment(taskRunEnvPrompt, taskRunEnvPromptHelp, o.AppName(), appEnvOptionNone)
	if err != nil {
		return fmt.Errorf("ask for environment: %w", err)
	}

	if env == appEnvOptionNone {
		return nil
	}

	o.env = env
	return nil
}

func (o *runTaskOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.AppName(), o.env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s config: %w", o.env, err)
	}
	return env, nil
}

// BuildTaskRunCmd build the command for running a new task
func BuildTaskRunCmd() *cobra.Command {
	vars := runTaskVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a one-off task on Amazon ECS.",
		Example: `
Run a task using your local Dockerfile. 
You will be prompted to specify a task group name and an environment for the tasks to run in.
/code $ copilot task run
Run a task named "db-migrate" in the "test" environment under the current workspace.
/code $ copilot task run -n db-migrate --env test
Run 4 tasks with 2GB memory, an existing image, and a custom task role.
/code $ copilot task run --num 4 --memory 2048 --image=rds-migrate --task-role migrate-role
Run a task with environment variables.
/code $ copilot task run --env-vars name=myName,user=myUser
Run a task using the current workspace with specific subnets and security groups.
/code $ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
Run a task with a command.
/code $ copilot task run --command "python migrate-script.py"`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskRunOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Ask(); err != nil {
				return err
			}

			if err := opts.Execute(); err != nil {
				return err
			}
			return nil
		}),
	}

	cmd.Flags().IntVar(&vars.count, countFlag, 1, countFlagDescription)
	cmd.Flags().IntVar(&vars.cpu, cpuFlag, 256, cpuFlagDescription)
	cmd.Flags().IntVar(&vars.memory, memoryFlag, 512, memoryFlagDescription)

	cmd.Flags().StringVarP(&vars.groupName, taskGroupNameFlag, nameFlagShort, "", taskGroupFlagDescription)

	cmd.Flags().StringVar(&vars.image, imageFlag, "", imageFlagDescription)
	cmd.Flags().StringVar(&vars.dockerfilePath, dockerFileFlag, defaultDockerfilePath, dockerFileFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", taskImageTagFlagDescription)

	cmd.Flags().StringVar(&vars.taskRole, taskRoleFlag, "", taskRoleFlagDescription)
	cmd.Flags().StringVar(&vars.executionRole, executionRoleFlag, "", executionRoleFlagDescription)

	cmd.Flags().StringVar(&vars.appName, appFlag, "", appFlagDescription)
	cmd.Flags().StringVar(&vars.env, envFlag, "", envFlagDescription)
	cmd.Flags().StringSliceVar(&vars.subnets, subnetsFlag, nil, subnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.securityGroups, securityGroupsFlag, nil, securityGroupsFlagDescription)

	cmd.Flags().StringToStringVar(&vars.envVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.command, commandFlag, "", commandFlagDescription)

	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	return cmd
}
