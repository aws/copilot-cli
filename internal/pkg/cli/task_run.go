// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	envOptionNone         = "None (run in default VPC)"
	defaultDockerfilePath = "Dockerfile"
	imageTagLatest        = "latest"
)

const (
	fmtRepoName = "copilot-%s"
	fmtImageURI = "%s:%s"
)

var (
	errNumNotPositive = errors.New("number of tasks must be positive")
	errCpuNotPositive = errors.New("CPU units must be positive")
	errMemNotPositive = errors.New("memory must be positive")
)

var (
	fmtTaskRunEnvPrompt       = fmt.Sprintf("In which %s would you like to run this %s?", color.Emphasize("environment"), color.Emphasize("task"))
	fmtTaskRunGroupNamePrompt = fmt.Sprintf("What would you like to %s your task group?", color.Emphasize("name"))

	taskRunEnvPromptHelp = fmt.Sprintf("Task will be deployed to the selected environment. "+
		"Select %s to run the task in your default VPC instead of any existing environment.", color.Emphasize(envOptionNone))
	taskRunGroupNamePromptHelp = "The group name of the task. Tasks with the same group name share the same set of resources, including CloudFormation stack, CloudWatch log group, task definition and ECR repository."
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

	taskRole string

	subnets        []string
	securityGroups []string
	env            string

	envVars map[string]string
	command string
}

type runTaskOpts struct {
	runTaskVars

	// Interfaces to interact with dependencies.
	fs      afero.Fs
	store   store
	sel     appEnvSelector
	spinner progress

	deployer taskDeployer

	// Fields below are configured at runtime.
	repository repositoryService
	configureRuntimeOpts func() error
}

func newTaskRunOpts(vars runTaskVars) (*runTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	provider := session.NewProvider()
	sess, err := provider.Default()
	if err != nil {
		return nil, fmt.Errorf("get default session: %w", err)
	}

	opts := runTaskOpts{
		runTaskVars: vars,

		fs:      &afero.Afero{Fs: afero.NewOsFs()},
		store:   store,
		sel:     selector.NewSelect(vars.prompt, store),
		spinner: termprogress.NewSpinner(),

		deployer: cloudformation.New(sess),
	}

	opts.configureRuntimeOpts = func() error {
		if err := opts.configureRepository(provider); err != nil {
			return err
		}
		// TODO: configure runner
		return nil
	}

	return &opts, nil
}

func (o *runTaskOpts) configureRepository(provider sessionProvider) error {
	var registry repository.Registry
	if o.env != "" {
		env, err := o.targetEnv()
		if err != nil {
			return err
		}

		sess, err := provider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return fmt.Errorf("get session from role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
		}

		registry = ecr.New(sess)
	} else {
		sess, err := provider.Default()
		if err != nil {
			return fmt.Errorf("get default session: %w", err)
		}
		registry = ecr.New(sess)
	}

	repoName := fmt.Sprintf(fmtRepoName, o.groupName)
	repository, err := repository.New(repoName, registry)
	if err != nil {
		return fmt.Errorf("initiate repository %s: %w", repoName, err)
	}

	o.repository = repository
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
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute deploys and runs the task.
func (o *runTaskOpts) Execute() error {
	if o.dockerfilePath == "" {
		o.dockerfilePath = defaultDockerfilePath
	}

	if err := o.deployTaskResources(); err != nil {
		return err
	}

	// NOTE: repository has to be configured only after task resources are deployed
	if err := o.configureRuntimeOpts(); err != nil {
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

	_, err := o.runTask()
	if err != nil {
		return err
	}

	return nil
}

func (o *runTaskOpts) runTask() ([]string, error) {
	// TODO: run tasks
	return nil, nil
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
	return o.deployer.DeployTask(&deploy.CreateTaskResourcesInput{
		Name:     o.groupName,
		CPU:      o.cpu,
		Memory:   o.memory,
		Image:    o.image,
		TaskRole: o.taskRole,
		Command:  o.command,
		EnvVars:  o.envVars,
	})
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
		fmtTaskRunGroupNamePrompt,
		taskRunGroupNamePromptHelp,
		basicNameValidation,
		prompt.WithFinalMessage("Task group name:"))
	if err != nil {
		return fmt.Errorf("prompt get task group name: %w", err)
	}
	o.groupName = groupName
	return nil
}

func (o *runTaskOpts) askEnvName() error {
	if o.env != "" {
		return nil
	}

	// NOTE: if the subnets are not provided, we are not in any workspace and app flag is not specified, use the "None" environment.
	if o.AppName() == "" || o.subnets != nil {
		return nil
	}

	env, err := o.sel.Environment(fmtTaskRunEnvPrompt, taskRunEnvPromptHelp, o.AppName(), envOptionNone)
	if err != nil {
		return fmt.Errorf("ask for environment: %w", err)
	}

	if env == envOptionNone {
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
		Short: "Run a one-off task",
		Long:  `Run a one-off task with configurations such as cpu-units, memory, image, etc.`,
		Example: `
Run a task with default setting. You will be prompted to specify a task group name and an environment for the tasks to run in.
/code $ copilot task run
Run a task named "db-migrate" in the "test" environment under the current workspace.
/code $ copilot task run -n db-migrate --env test
Starts 4 tasks with 2GB memory, run a particular image, and run with a particular task role.
/code $ copilot task run --num 4 --memory 2048 --image=python --task-role migrate-exec-role
Run a task with environment variables.
/code $ copilot task run --env-vars name=myName,user=myUser
Run a task with subnets and security groups.
/code $ copilot task run --subnets subnet-123,subnet-456 --subnets subnet-789 --security-groups sg-123,sg-456
Run a task with a command.
/code $ copilot task run --command "python migrate-script.py" 
`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskRunOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
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
	cmd.Flags().StringVar(&vars.dockerfilePath, dockerFileFlag, "", dockerFileFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", taskImageTagFlagDescription)

	cmd.Flags().StringVar(&vars.taskRole, taskRoleFlag, "", taskRoleFlagDescription)

	cmd.Flags().StringVar(&vars.appName, appFlag, "", appFlagDescription)
	cmd.Flags().StringVar(&vars.env, envFlag, "", envFlagDescription)
	cmd.Flags().StringSliceVar(&vars.subnets, subnetsFlag, nil, subnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.securityGroups, securityGroupsFlag, nil, securityGroupsFlagDescription)

	cmd.Flags().StringToStringVar(&vars.envVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.command, commandFlag, "", commandFlagDescription)

	return cmd
}
