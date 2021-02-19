// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/google/shlex"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	appEnvOptionNone      = "None (run in default VPC)"
	defaultDockerfilePath = "Dockerfile"
	imageTagLatest        = "latest"
)

const (
	fmtImageURI = "%s:%s"
)

var (
	errNumNotPositive = errors.New("number of tasks must be positive")
	errCPUNotPositive = errors.New("CPU units must be positive")
	errMemNotPositive = errors.New("memory must be positive")
)

var (
	taskRunAppPrompt = fmt.Sprintf("In which %s would you like to run this %s?", color.Emphasize("application"), color.Emphasize("task"))
	taskRunEnvPrompt = fmt.Sprintf("In which %s would you like to run this %s?", color.Emphasize("environment"), color.Emphasize("task"))

	taskRunAppPromptHelp = fmt.Sprintf(`Task will be deployed to the selected application. 
Select %s to run the task in your default VPC instead of any existing application.`, color.Emphasize(appEnvOptionNone))
	taskRunEnvPromptHelp = fmt.Sprintf(`Task will be deployed to the selected environment.
Select %s to run the task in your default VPC instead of any existing environment.`, color.Emphasize(appEnvOptionNone))
)

type runTaskVars struct {
	count  int
	cpu    int
	memory int

	groupName string

	image          string
	dockerfilePath string
	imageTag       string

	taskRole      string
	executionRole string

	subnets           []string
	securityGroups    []string
	env               string
	appName           string
	useDefaultSubnets bool

	envVars      map[string]string
	command      string
	resourceTags map[string]string

	follow bool
}

type runTaskOpts struct {
	runTaskVars
	isDockerfileSet bool

	// Interfaces to interact with dependencies.
	fs      afero.Fs
	store   store
	sel     appEnvSelector
	spinner progress

	// Fields below are configured at runtime.
	deployer             taskDeployer
	repository           repositoryService
	runner               taskRunner
	eventsWriter         eventsWriter
	defaultClusterGetter defaultClusterGetter

	sess              *session.Session
	targetEnvironment *config.Environment

	// Configurer methods.
	configureRuntimeOpts func() error
	configureRepository  func() error
	// NOTE: configureEventsWriter is only called when tailing logs (i.e. --follow is specified)
	configureEventsWriter func(tasks []*task.Task)
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
		sel:     selector.NewSelect(prompt.New(), store),
		spinner: termprogress.NewSpinner(log.DiagnosticWriter),
	}

	opts.configureRuntimeOpts = func() error {
		opts.runner = opts.configureRunner()
		opts.deployer = cloudformation.New(opts.sess)
		opts.defaultClusterGetter = awsecs.New(opts.sess)
		return nil
	}

	opts.configureRepository = func() error {
		repoName := fmt.Sprintf(deploy.FmtTaskECRRepoName, opts.groupName)
		registry := ecr.New(opts.sess)
		repository, err := repository.New(repoName, registry)
		if err != nil {
			return fmt.Errorf("initialize repository %s: %w", repoName, err)
		}
		opts.repository = repository
		return nil
	}

	opts.configureEventsWriter = func(tasks []*task.Task) {
		opts.eventsWriter = logging.NewTaskClient(opts.sess, opts.groupName, tasks)
	}
	return &opts, nil
}

func (o *runTaskOpts) configureRunner() taskRunner {
	vpcGetter := ec2.New(o.sess)
	ecsService := awsecs.New(o.sess)

	if o.env != "" {
		return &task.EnvRunner{
			Count:     o.count,
			GroupName: o.groupName,

			App: o.appName,
			Env: o.env,

			VPCGetter:     vpcGetter,
			ClusterGetter: ecs.New(o.sess),
			Starter:       ecsService,
		}
	}

	return &task.NetworkConfigRunner{
		Count:     o.count,
		GroupName: o.groupName,

		Subnets:        o.subnets,
		SecurityGroups: o.securityGroups,

		VPCGetter:     vpcGetter,
		ClusterGetter: ecsService,
		Starter:       ecsService,
	}

}

func (o *runTaskOpts) configureSessAndEnv() error {
	var sess *session.Session
	var env *config.Environment

	provider := sessions.NewProvider()
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
		return errCPUNotPositive
	}

	if o.memory <= 0 {
		return errMemNotPositive
	}

	if o.groupName != "" {
		if err := basicNameValidation(o.groupName); err != nil {
			return err
		}
	}

	if o.image != "" && o.isDockerfileSet {
		return errors.New("cannot specify both `--image` and `--dockerfile`")
	}

	if o.isDockerfileSet {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}

	if err := o.validateFlagsWithDefaultCluster(); err != nil {
		return err
	}

	if err := o.validateFlagsWithSubnets(); err != nil {
		return err
	}

	if err := o.validateFlagsWithSecurityGroups(); err != nil {
		return err
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

func (o *runTaskOpts) validateFlagsWithDefaultCluster() error {
	if !o.useDefaultSubnets {
		return nil
	}

	if o.subnets != nil {
		return fmt.Errorf("cannot specify both `--subnets` and `--default`")
	}

	if o.appName != "" {
		return fmt.Errorf("cannot specify both `--app` and `--default`")
	}

	if o.env != "" {
		return fmt.Errorf("cannot specify both `--env` and `--default`")
	}

	return nil
}

func (o *runTaskOpts) validateFlagsWithSubnets() error {
	if o.subnets == nil {
		return nil
	}

	if o.useDefaultSubnets {
		return fmt.Errorf("cannot specify both `--subnets` and `--default`")
	}

	if o.appName != "" {
		return fmt.Errorf("cannot specify both `--subnets` and `--app`")
	}

	if o.env != "" {
		return fmt.Errorf("cannot specify both `--subnets` and `--env`")
	}

	return nil
}

func (o *runTaskOpts) validateFlagsWithSecurityGroups() error {
	if o.securityGroups == nil {
		return nil
	}

	if o.appName != "" {
		return fmt.Errorf("cannot specify both `--security-groups` and `--app`")
	}

	if o.env != "" {
		return fmt.Errorf("cannot specify both `--security-groups` and `--env`")
	}
	return nil
}

// Ask prompts the user for any required or important fields that are not provided.
func (o *runTaskOpts) Ask() error {
	if o.shouldPromptForAppEnv() {
		if err := o.askAppName(); err != nil {
			return err
		}
		if err := o.askEnvName(); err != nil {
			return err
		}
	}
	return nil
}

func (o *runTaskOpts) shouldPromptForAppEnv() bool {
	// NOTE: if security groups are specified but subnets are not, then we use the default subnets with the
	// specified security groups.
	useDefault := o.useDefaultSubnets || (o.securityGroups != nil && o.subnets == nil)
	useConfig := o.subnets != nil

	// if user hasn't specified that they want to use the default subnets, and that they didn't provide specific subnets
	// that they want to use, then we prompt.
	return !useDefault && !useConfig
}

// Execute deploys and runs the task.
func (o *runTaskOpts) Execute() error {
	if o.groupName == "" {
		dir, err := os.Getwd()
		if err != nil {
			log.Errorf("Cannot retrieve working directory, please use --%s to specify a task group name.\n", taskGroupNameFlag)
			return fmt.Errorf("get working directory: %v", err)
		}
		o.groupName = strings.ToLower(filepath.Base(dir))
	}

	// NOTE: all runtime options must be configured only after session is configured
	if err := o.configureSessAndEnv(); err != nil {
		return err
	}

	if err := o.configureRuntimeOpts(); err != nil {
		return err
	}

	if o.env == "" {
		hasDefaultCluster, err := o.defaultClusterGetter.HasDefaultCluster()
		if err != nil {
			return fmt.Errorf(`find "default" cluster to deploy the task to: %v`, err)
		}
		if !hasDefaultCluster {
			return errors.New(`cannot find a "default" cluster to deploy the task to`)
		}
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
		o.configureEventsWriter(tasks)
		if err := o.displayLogStream(); err != nil {
			return err
		}
	}
	return nil
}

func (o *runTaskOpts) displayLogStream() error {
	if err := o.eventsWriter.WriteEventsUntilStopped(); err != nil {
		return fmt.Errorf("write events: %w", err)
	}

	log.Infof("%s %s stopped.\n",
		english.PluralWord(o.count, "Task", ""),
		english.PluralWord(o.count, "has", "have"))
	return nil
}

func (o *runTaskOpts) runTask() ([]*task.Task, error) {
	o.spinner.Start(fmt.Sprintf("Waiting for %s to be running for %s.", english.Plural(o.count, "task", ""), o.groupName))
	tasks, err := o.runner.Run()
	if err != nil {
		o.spinner.Stop(log.Serrorf("Failed to run %s.\n\n", o.groupName))
		return nil, fmt.Errorf("run task %s: %w", o.groupName, err)
	}
	o.spinner.Stop(log.Ssuccessf("%s %s %s running.\n\n", english.PluralWord(o.count, "Task", ""), o.groupName, english.PluralWord(o.count, "is", "are")))
	return tasks, nil
}

func (o *runTaskOpts) buildAndPushImage() error {
	var additionalTags []string
	if o.imageTag != "" {
		additionalTags = append(additionalTags, o.imageTag)
	}

	if err := o.repository.BuildAndPush(exec.NewDockerCommand(), &exec.BuildArguments{
		Dockerfile:     o.dockerfilePath,
		Context:        filepath.Dir(o.dockerfilePath),
		ImageTag:       imageTagLatest,
		AdditionalTags: additionalTags,
	}); err != nil {
		return fmt.Errorf("build and push image: %w", err)
	}
	return nil
}

func (o *runTaskOpts) deployTaskResources() error {
	if err := o.deploy(); err != nil {
		return fmt.Errorf("provision resources for task %s: %w", o.groupName, err)
	}
	return nil
}

func (o *runTaskOpts) updateTaskResources() error {
	if err := o.deploy(); err != nil {
		return fmt.Errorf("update resources for task %s: %w", o.groupName, err)
	}
	return nil
}

func (o *runTaskOpts) deploy() error {
	var deployOpts []awscloudformation.StackOption
	if o.env != "" {
		deployOpts = []awscloudformation.StackOption{awscloudformation.WithRoleARN(o.targetEnvironment.ExecutionRoleARN)}
	}
	command, err := shlex.Split(o.command)
	if err != nil {
		return fmt.Errorf("split command %s into tokens using shell-style rules: %w", o.command, err)
	}
	input := &deploy.CreateTaskResourcesInput{
		Name:           o.groupName,
		CPU:            o.cpu,
		Memory:         o.memory,
		Image:          o.image,
		TaskRole:       o.taskRole,
		ExecutionRole:  o.executionRole,
		Command:        command,
		EnvVars:        o.envVars,
		App:            o.appName,
		Env:            o.env,
		AdditionalTags: o.resourceTags,
	}
	return o.deployer.DeployTask(os.Stderr, input, deployOpts...)
}

func (o *runTaskOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	return nil
}

func (o *runTaskOpts) validateEnvName() error {
	if o.appName != "" {
		if _, err := o.targetEnv(); err != nil {
			return err
		}
	} else {
		return errNoAppInWorkspace
	}

	return nil
}

func (o *runTaskOpts) askAppName() error {
	if o.appName != "" {
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
	if o.appName == "" || o.subnets != nil {
		return nil
	}

	env, err := o.sel.Environment(taskRunEnvPrompt, taskRunEnvPromptHelp, o.appName, appEnvOptionNone)
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
	env, err := o.store.GetEnvironment(o.appName, o.env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s config: %w", o.env, err)
	}
	return env, nil
}

// BuildTaskRunCmd build the command for running a new task
func BuildTaskRunCmd() *cobra.Command {
	vars := runTaskVars{}
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

			if cmd.Flags().Changed(dockerFileFlag) {
				opts.isDockerfileSet = true
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

	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().StringVar(&vars.dockerfilePath, dockerFileFlag, defaultDockerfilePath, dockerFileFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", taskImageTagFlagDescription)

	cmd.Flags().StringVar(&vars.taskRole, taskRoleFlag, "", taskRoleFlagDescription)
	cmd.Flags().StringVar(&vars.executionRole, executionRoleFlag, "", executionRoleFlagDescription)

	cmd.Flags().StringVar(&vars.appName, appFlag, "", taskAppFlagDescription)
	cmd.Flags().StringVar(&vars.env, envFlag, "", taskEnvFlagDescription)
	cmd.Flags().StringSliceVar(&vars.subnets, subnetsFlag, nil, subnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.securityGroups, securityGroupsFlag, nil, securityGroupsFlagDescription)
	cmd.Flags().BoolVar(&vars.useDefaultSubnets, taskDefaultFlag, false, taskDefaultFlagDescription)

	cmd.Flags().StringToStringVar(&vars.envVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.command, commandFlag, "", commandFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	return cmd
}
