// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/template/artifactpath"
	"golang.org/x/mod/semver"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/spf13/pflag"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/aws-sdk-go/aws/arn"

	awscloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/dustin/go-humanize/english"
	"github.com/google/shlex"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	appEnvOptionNone      = "None (run in default VPC)"
	defaultDockerfilePath = "Dockerfile"
	imageTagLatest        = "latest"
	shortTaskIDLength     = 8
)

const (
	envFileExt = ".env"

	fmtTaskRunEnvUploadStart    = "Uploading env file to S3: %s"
	fmtTaskRunEnvUploadFailed   = "Failed to upload your env file to S3: %s\n"
	fmtTaskRunEnvUploadComplete = "Successfully uploaded your env file to S3: %s\n"
)

const (
	workloadTypeJob     = "job"
	workloadTypeSvc     = "svc"
	workloadTypeInvalid = "invalid"
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

var (
	taskSecretsPermissionPrompt     = "Do you grant permission to the ECS/Fargate agent for these secrets?"
	taskSecretsPermissionPromptHelp = "ECS/Fargate agent needs the permissions in order to fetch the secrets and inject them into your container."
)

type runTaskVars struct {
	count  int
	cpu    int
	memory int

	groupName string

	image                 string
	dockerfilePath        string
	dockerfileContextPath string
	dockerfileBuildArgs   map[string]string
	imageTag              string

	taskRole      string
	executionRole string
	cluster       string

	subnets                     []string
	securityGroups              []string
	env                         string
	appName                     string
	useDefaultSubnetsAndCluster bool

	envVars                  map[string]string
	envFile                  string
	secrets                  map[string]string
	acknowledgeSecretsAccess bool
	command                  string
	entrypoint               string
	resourceTags             map[string]string

	follow                bool
	generateCommandTarget string

	os   string
	arch string
}

type runTaskOpts struct {
	runTaskVars
	isDockerfileSet bool
	nFlag           int

	// Interfaces to interact with dependencies.
	fs      afero.Fs
	store   store
	sel     appEnvSelector
	spinner progress
	prompt  prompter

	// Fields below are configured at runtime.
	deployer             taskDeployer
	repository           repositoryService
	runner               taskRunner
	eventsWriter         eventsWriter
	defaultClusterGetter defaultClusterGetter
	publicIPGetter       publicIPGetter

	provider          sessionProvider
	sess              *session.Session
	targetEnvironment *config.Environment

	// Configurer functions.
	configureRuntimeOpts func() error
	configureRepository  func() error
	// NOTE: configureEventsWriter is only called when tailing logs (i.e. --follow is specified)
	configureEventsWriter func(tasks []*task.Task)

	configureECSServiceDescriber func(session *session.Session) ecs.ECSServiceDescriber
	configureServiceDescriber    func(session *session.Session) ecs.ServiceDescriber
	configureJobDescriber        func(session *session.Session) ecs.JobDescriber
	configureUploader            func(session *session.Session) uploader

	// Functions to generate a task run command.
	runTaskRequestFromECSService func(client ecs.ECSServiceDescriber, cluster, service string) (*ecs.RunTaskRequest, error)
	runTaskRequestFromService    func(client ecs.ServiceDescriber, app, env, svc string) (*ecs.RunTaskRequest, error)
	runTaskRequestFromJob        func(client ecs.JobDescriber, app, env, job string) (*ecs.RunTaskRequest, error)

	// Cached variables.
	ssmParamSecrets         map[string]string
	secretsManagerSecrets   map[string]string
	envFileARN              string
	envCompatibilityChecker func(app, env string) (versionCompatibilityChecker, error)
}

func newTaskRunOpts(vars runTaskVars) (*runTaskOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("task run"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	prompter := prompt.New()
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	opts := runTaskOpts{
		runTaskVars: vars,

		fs:                    &afero.Afero{Fs: afero.NewOsFs()},
		store:                 store,
		prompt:                prompter,
		sel:                   selector.NewAppEnvSelector(prompter, store),
		spinner:               termprogress.NewSpinner(log.DiagnosticWriter),
		provider:              sessProvider,
		secretsManagerSecrets: make(map[string]string),
		ssmParamSecrets:       make(map[string]string),
	}

	opts.configureRuntimeOpts = func() error {
		opts.runner, err = opts.configureRunner()
		if err != nil {
			return fmt.Errorf("configure task runner: %w", err)
		}
		opts.deployer = cloudformation.New(opts.sess, cloudformation.WithProgressTracker(os.Stderr))
		opts.defaultClusterGetter = awsecs.New(opts.sess)
		opts.publicIPGetter = ec2.New(opts.sess)
		return nil
	}

	opts.configureRepository = func() error {
		repoName := fmt.Sprintf(deploy.FmtTaskECRRepoName, opts.groupName)
		opts.repository = repository.New(ecr.New(opts.sess), repoName)
		return nil
	}

	opts.configureEventsWriter = func(tasks []*task.Task) {
		opts.eventsWriter = logging.NewTaskClient(opts.sess, opts.groupName, tasks)
	}

	opts.configureECSServiceDescriber = func(session *session.Session) ecs.ECSServiceDescriber {
		return awsecs.New(session)
	}
	opts.configureServiceDescriber = func(session *session.Session) ecs.ServiceDescriber {
		return ecs.New(session)
	}
	opts.configureJobDescriber = func(session *session.Session) ecs.JobDescriber {
		return ecs.New(session)
	}
	opts.configureUploader = func(session *session.Session) uploader {
		return s3.New(session)
	}
	opts.envCompatibilityChecker = func(app, env string) (versionCompatibilityChecker, error) {
		envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         app,
			Env:         env,
			ConfigStore: opts.store,
		})
		if err != nil {
			return nil, fmt.Errorf("new environment compatibility checker: %v", err)
		}
		return envDescriber, nil
	}

	opts.runTaskRequestFromECSService = ecs.RunTaskRequestFromECSService
	opts.runTaskRequestFromService = ecs.RunTaskRequestFromService
	opts.runTaskRequestFromJob = ecs.RunTaskRequestFromJob
	return &opts, nil
}

func (o *runTaskOpts) configureRunner() (taskRunner, error) {
	vpcGetter := ec2.New(o.sess)
	ecsService := awsecs.New(o.sess)

	if o.env != "" {
		deployStore, err := deploy.NewStore(o.provider, o.store)
		if err != nil {
			return nil, fmt.Errorf("connect to copilot deploy store: %w", err)
		}

		d, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:             o.appName,
			Env:             o.env,
			ConfigStore:     o.store,
			DeployStore:     deployStore,
			EnableResources: false, // We don't need to show detailed resources.
		})
		if err != nil {
			return nil, fmt.Errorf("create describer for environment %s in application %s: %w", o.env, o.appName, err)
		}

		ecsClient := ecs.New(o.sess)
		return &task.EnvRunner{
			Count:     o.count,
			GroupName: o.groupName,

			App: o.appName,
			Env: o.env,

			SecurityGroups: o.securityGroups,

			OS: o.os,

			VPCGetter:             vpcGetter,
			ClusterGetter:         ecsClient,
			Starter:               ecsService,
			EnvironmentDescriber:  d,
			NonZeroExitCodeGetter: ecsClient,
		}, nil
	}
	return &task.ConfigRunner{
		Count:     o.count,
		GroupName: o.groupName,

		Cluster:        o.cluster,
		Subnets:        o.subnets,
		SecurityGroups: o.securityGroups,
		OS:             o.os,

		VPCGetter:             vpcGetter,
		ClusterGetter:         ecsService,
		Starter:               ecsService,
		NonZeroExitCodeGetter: ecs.New(o.sess),
	}, nil

}

func (o *runTaskOpts) configureSessAndEnv() error {
	var sess *session.Session
	var env *config.Environment

	if o.env != "" {
		var err error
		env, err = o.targetEnv(o.appName, o.env)
		if err != nil {
			return err
		}

		sess, err = o.provider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return fmt.Errorf("get session from role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
		}
	} else {
		var err error
		sess, err = o.provider.Default()
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
	if o.generateCommandTarget != "" {
		if o.nFlag >= 2 {
			return errors.New("cannot specify `--generate-cmd` with any other flag")
		}
	}

	if o.count <= 0 {
		return errNumNotPositive
	}

	if o.groupName != "" {
		if err := basicNameValidation(o.groupName); err != nil {
			return err
		}
	}

	if o.image != "" && o.isDockerfileSet {
		return errors.New("cannot specify both `--image` and `--dockerfile`")
	}

	if o.image != "" && o.dockerfileContextPath != "" {
		return errors.New("cannot specify both `--image` and `--build-context`")
	}

	if o.image != "" && o.dockerfileBuildArgs != nil {
		return errors.New("cannot specify both `--image` and `--build-args`")
	}

	if o.isDockerfileSet {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return fmt.Errorf("invalid `--dockerfile` path: %w", err)
		}
	}

	if o.dockerfileContextPath != "" {
		if _, err := o.fs.Stat(o.dockerfileContextPath); err != nil {
			return fmt.Errorf("invalid `--build-context` path: %w", err)
		}
	}

	if noOS, noArch := o.os == "", o.arch == ""; noOS != noArch {
		return fmt.Errorf("must specify either both `--%s` and `--%s` or neither", osFlag, archFlag)
	}
	if err := o.validatePlatform(); err != nil {
		return err
	}

	if o.cpu <= 0 {
		return errCPUNotPositive
	}

	if o.memory <= 0 {
		return errMemNotPositive
	}

	if err := o.validateFlagsWithCluster(); err != nil {
		return err
	}

	if err := o.validateFlagsWithDefaultCluster(); err != nil {
		return err
	}

	if err := o.validateFlagsWithSubnets(); err != nil {
		return err
	}

	if err := o.validateFlagsWithWindows(); err != nil {
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

	for _, value := range o.secrets {
		if !isSSM(value) && !isSecretsManager(value) {
			return fmt.Errorf("must specify a valid secrets ARN")
		}
	}

	if o.envFile != "" {
		if filepath.Ext(o.envFile) != envFileExt {
			return fmt.Errorf("environment file %s specified in --%s must have a %s file extension", o.envFile, envFileFlag, envFileExt)
		}
	}

	return nil
}

func isSSM(value string) bool {
	// For SSM parameter you can specify it as ARN or name if it exists in the same Region as the task you are launching.
	return !template.IsARNFunc(value) || strings.Contains(value, ":ssm:")
}

func isSecretsManager(value string) bool {
	return template.IsARNFunc(value) && strings.Contains(value, ":secretsmanager:")
}

func (o *runTaskOpts) getCategorizedSecrets() (map[string]string, map[string]string) {
	if len(o.ssmParamSecrets) > 0 || len(o.secretsManagerSecrets) > 0 {
		return o.secretsManagerSecrets, o.ssmParamSecrets
	}

	for name, value := range o.secrets {
		if isSSM(value) {
			o.ssmParamSecrets[name] = value
		}
		if isSecretsManager(value) {
			o.secretsManagerSecrets[name] = value
		}
	}
	return o.secretsManagerSecrets, o.ssmParamSecrets
}

func (o *runTaskOpts) confirmSecretsAccess() error {
	if o.executionRole != "" {
		return nil
	}

	if o.appName != "" || o.env != "" {
		return nil
	}

	if o.acknowledgeSecretsAccess {
		return nil
	}

	secretsManagerSecrets, ssmParamSecrets := o.getCategorizedSecrets()

	log.Info("Looks like ")

	if len(ssmParamSecrets) > 0 {
		log.Infoln("you're requesting ssm:GetParameters to the following SSM parameters:")
	} else {
		log.Infoln("you're requesting secretsmanager:GetSecretValue to the following Secrets Manager secrets:")
	}

	for _, value := range ssmParamSecrets {
		log.Infoln("* " + value)
	}

	if len(ssmParamSecrets) > 0 && len(secretsManagerSecrets) > 0 {
		log.Infoln("\nand secretsmanager:GetSecretValue to the following Secrets Manager secrets:")
	}

	for _, value := range secretsManagerSecrets {
		log.Infoln("* " + value)
	}

	secretsAccessConfirmed, err := o.prompt.Confirm(taskSecretsPermissionPrompt, taskSecretsPermissionPromptHelp)
	if err != nil {
		return fmt.Errorf("prompt to confirm secrets access: %w", err)
	}

	if !secretsAccessConfirmed {
		return errors.New("access to secrets denied")
	}

	return nil
}

func (o *runTaskOpts) validateEnvCompatibilityForGenerateJobCmd(app, env string) error {
	envStack, err := o.envCompatibilityChecker(app, env)
	if err != nil {
		return err
	}
	version, err := envStack.Version()
	if err != nil {
		return fmt.Errorf("retrieve version of environment stack %q in application %q: %v", env, app, err)
	}
	// The '--generate-cmd' flag was introduced in env v1.4.0. In env v1.8.0, EnvManagerRole took over, but
	//"states:DescribeStateMachine" permissions weren't added until 1.12.2.
	if semver.Compare(version, "v1.12.2") < 0 {
		return &errFeatureIncompatibleWithEnvironment{
			missingFeature: "task run --generate-cmd",
			envName:        env,
			curVersion:     version,
		}
	}
	return nil
}

func (o *runTaskOpts) validatePlatform() error {
	if o.os == "" {
		return nil
	}
	o.os = strings.ToUpper(o.os)
	o.arch = strings.ToUpper(o.arch)
	validPlatforms := task.ValidCFNPlatforms
	for _, validPlatform := range validPlatforms {
		if dockerengine.PlatformString(o.os, o.arch) == validPlatform {
			return nil
		}
	}
	return fmt.Errorf("platform %s is invalid; %s: %s", dockerengine.PlatformString(o.os, o.arch), english.PluralWord(len(validPlatforms), "the valid platform is", "valid platforms are"), english.WordSeries(validPlatforms, "and"))
}

func (o *runTaskOpts) validateFlagsWithCluster() error {
	if o.cluster == "" {
		return nil
	}

	if o.appName != "" {
		return fmt.Errorf("cannot specify both `--app` and `--cluster`")
	}

	if o.env != "" {
		return fmt.Errorf("cannot specify both `--env` and `--cluster`")
	}

	if o.useDefaultSubnetsAndCluster {
		return fmt.Errorf("cannot specify both `--default` and `--cluster`")
	}

	return nil
}

func (o *runTaskOpts) validateFlagsWithDefaultCluster() error {
	if !o.useDefaultSubnetsAndCluster {
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

	if o.useDefaultSubnetsAndCluster {
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

func (o *runTaskOpts) validateFlagsWithWindows() error {
	if !isWindowsOS(o.os) {
		return nil
	}
	if o.cpu < manifest.MinWindowsTaskCPU {
		return fmt.Errorf("CPU is %d, but it must be at least %d for a Windows-based task", o.cpu, manifest.MinWindowsTaskCPU)
	}
	if o.memory < manifest.MinWindowsTaskMemory {
		return fmt.Errorf("memory is %d, but it must be at least %d for a Windows-based task", o.memory, manifest.MinWindowsTaskMemory)
	}
	return nil
}

func isWindowsOS(os string) bool {
	return task.IsValidWindowsOS(os)
}

// Ask prompts the user for any required or important fields that are not provided.
func (o *runTaskOpts) Ask() error {
	if o.generateCommandTarget != "" {
		return nil
	}
	if o.shouldPromptForAppEnv() {
		if err := o.askAppName(); err != nil {
			return err
		}
		if err := o.askEnvName(); err != nil {
			return err
		}
	}
	if len(o.secrets) > 0 {
		if err := o.confirmSecretsAccess(); err != nil {
			return err
		}
	}
	return nil
}

func (o *runTaskOpts) shouldPromptForAppEnv() bool {
	// NOTE: if security groups are specified but subnets are not, then we use the default subnets with the
	// specified security groups.
	useDefault := o.useDefaultSubnetsAndCluster || (o.securityGroups != nil && o.subnets == nil && o.cluster == "")
	useConfig := o.subnets != nil || o.cluster != ""

	// if user hasn't specified that they want to use the default subnets, and that they didn't provide specific subnets
	// that they want to use, then we prompt.
	return !useDefault && !useConfig
}

// Execute deploys and runs the task.
func (o *runTaskOpts) Execute() error {
	if o.generateCommandTarget != "" {
		return o.generateCommand()
	}

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

	if o.env == "" && o.cluster == "" {
		hasDefaultCluster, err := o.defaultClusterGetter.HasDefaultCluster()
		if err != nil {
			return fmt.Errorf(`find "default" cluster to deploy the task to: %v`, err)
		}
		if !hasDefaultCluster {
			log.Errorf(
				"Looks like there is no \"default\" cluster in your region!\nPlease run %s to create the cluster first, and then re-run %s.\n",
				color.HighlightCode("aws ecs create-cluster"),
				color.HighlightCode("copilot task run"),
			)
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

	var shouldUpdate bool

	if o.envFile != "" {
		envFileARN, err := o.deployEnvFile()
		if err != nil {
			return fmt.Errorf("deploy env file %s: %w", o.envFile, err)
		}
		o.envFileARN = envFileARN

		shouldUpdate = true
	}

	// NOTE: if image is not provided, then we build the image and push to ECR repo
	if o.image == "" {
		uri, err := o.repository.Login()
		if err != nil {
			return fmt.Errorf("login to docker: %w", err)
		}

		tag := imageTagLatest
		if o.imageTag != "" {
			tag = o.imageTag
		}
		o.image = fmt.Sprintf(fmtImageURI, uri, tag)

		if err := o.buildAndPushImage(uri); err != nil {
			return err
		}

		shouldUpdate = true
	}

	if shouldUpdate {
		if err := o.updateTaskResources(); err != nil {
			return err
		}
	}

	tasks, err := o.runTask()
	if err != nil {
		if strings.Contains(err.Error(), "AccessDeniedException") && strings.Contains(err.Error(), "unable to pull secrets") && o.appName != "" && o.env != "" {
			log.Error(`It looks like your task is not able to pull the secrets.
Did you tag your secrets with the "copilot-application" and "copilot-environment" tags?
`)
		}
		return err
	}

	o.showPublicIPs(tasks)

	if o.follow {
		o.configureEventsWriter(tasks)
		if err := o.displayLogStream(); err != nil {
			return err
		}
		if err := o.runner.CheckNonZeroExitCode(tasks); err != nil {
			return err
		}
	}
	return nil
}

func (o *runTaskOpts) generateCommand() error {
	command, err := o.runTaskCommand()
	if err != nil {
		return err
	}
	cliString, err := command.CLIString()
	if err != nil {
		return err
	}
	log.Infoln(cliString)
	return nil
}

func (o *runTaskOpts) runTaskCommand() (cliStringer, error) {
	var cmd cliStringer
	if arn.IsARN(o.generateCommandTarget) {
		clusterName, serviceName, err := o.parseARN()
		if err != nil {
			return nil, err
		}
		sess, err := o.provider.Default()
		if err != nil {
			return nil, fmt.Errorf("get default session: %s", err)
		}
		return o.runTaskCommandFromECSService(sess, clusterName, serviceName)
	}
	parts := strings.Split(o.generateCommandTarget, "/")
	switch len(parts) {
	case 2:
		clusterName, serviceName := parts[0], parts[1]
		sess, err := o.provider.Default()
		if err != nil {
			return nil, fmt.Errorf("get default session: %s", err)
		}
		cmd, err = o.runTaskCommandFromECSService(sess, clusterName, serviceName)
		if err != nil {
			return nil, err
		}
	case 3:
		appName, envName, workloadName := parts[0], parts[1], parts[2]
		env, err := o.targetEnv(appName, envName)
		if err != nil {
			return nil, err
		}
		sess, err := o.provider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, fmt.Errorf("get environment session: %s", err)
		}
		cmd, err = o.runTaskCommandFromWorkload(sess, appName, envName, workloadName)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("invalid input to --generate-cmd: must be of format <cluster>/<service> or <app>/<env>/<workload>")
	}

	return cmd, nil
}

func (o *runTaskOpts) parseARN() (string, string, error) {
	svcARN, err := awsecs.ParseServiceArn(o.generateCommandTarget)
	if err != nil {
		return "", "", fmt.Errorf("parse service arn %s: %w", o.generateCommandTarget, err)
	}
	return svcARN.ClusterName(), svcARN.ServiceName(), nil
}

func (o *runTaskOpts) runTaskCommandFromECSService(sess *session.Session, clusterName, serviceName string) (cliStringer, error) {
	cmd, err := o.runTaskRequestFromECSService(o.configureECSServiceDescriber(sess), clusterName, serviceName)
	if err != nil {
		var errMultipleContainers *ecs.ErrMultipleContainersInTaskDef
		if errors.As(err, &errMultipleContainers) {
			log.Errorln("`copilot task run` does not support running more than one container.")
		}
		return nil, fmt.Errorf("generate task run command from ECS service %s: %w", clusterName+"/"+serviceName, err)
	}
	return cmd, nil
}

func (o *runTaskOpts) runTaskCommandFromWorkload(sess *session.Session, appName, envName, workloadName string) (cliStringer, error) {
	workloadType, err := o.workloadType(appName, workloadName)
	if err != nil {
		return nil, err
	}

	var cmd cliStringer
	switch workloadType {
	case workloadTypeJob:
		if err := o.validateEnvCompatibilityForGenerateJobCmd(appName, envName); err != nil {
			return nil, err
		}
		cmd, err = o.runTaskRequestFromJob(o.configureJobDescriber(sess), appName, envName, workloadName)
		if err != nil {
			return nil, fmt.Errorf("generate task run command from job %s of application %s deployed in environment %s: %w", workloadName, appName, envName, err)
		}
	case workloadTypeSvc:
		cmd, err = o.runTaskRequestFromService(o.configureServiceDescriber(sess), appName, envName, workloadName)
		if err != nil {
			return nil, fmt.Errorf("generate task run command from service %s of application %s deployed in environment %s: %w", workloadName, appName, envName, err)
		}
	}
	return cmd, nil
}

func (o *runTaskOpts) workloadType(appName, workloadName string) (string, error) {
	_, err := o.store.GetJob(appName, workloadName)
	if err == nil {
		return workloadTypeJob, nil
	}

	var errNoSuchJob *config.ErrNoSuchJob
	if !errors.As(err, &errNoSuchJob) {
		return "", fmt.Errorf("determine whether workload %s is a job: %w", workloadName, err)
	}

	_, err = o.store.GetService(appName, workloadName)
	if err == nil {
		return workloadTypeSvc, nil
	}

	var errNoSuchService *config.ErrNoSuchService
	if !errors.As(err, &errNoSuchService) {
		return "", fmt.Errorf("determine whether workload %s is a service: %w", workloadName, err)
	}

	return workloadTypeInvalid, fmt.Errorf("workload %s is neither a service nor a job", workloadName)
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

func (o *runTaskOpts) showPublicIPs(tasks []*task.Task) {
	publicIPs := make(map[string]string)
	for _, t := range tasks {
		if t.ENI == "" {
			continue
		}
		ip, err := o.publicIPGetter.PublicIP(t.ENI) // We will just not show the ip address if an error occurs.
		if err == nil {
			publicIPs[t.TaskARN] = ip
		}
	}

	if len(publicIPs) == 0 {
		return
	}

	log.Infof("%s associated with the %s %s:\n",
		english.PluralWord(len(publicIPs), "The public IP", "Public IPs"),
		english.PluralWord(len(publicIPs), "task", "tasks"),
		english.PluralWord(len(publicIPs), "is", "are"))
	for taskARN, ip := range publicIPs {
		if len(taskARN) >= shortTaskIDLength {
			taskARN = taskARN[len(taskARN)-shortTaskIDLength:]
		}
		log.Infof("- %s (for %s)\n", ip, taskARN)
	}

}

func (o *runTaskOpts) buildAndPushImage(uri string) error {
	var additionalTags []string
	if o.imageTag != "" {
		additionalTags = append(additionalTags, o.imageTag)
	}

	ctx := filepath.Dir(o.dockerfilePath)
	if o.dockerfileContextPath != "" {
		ctx = o.dockerfileContextPath
	}
	buildArgs := &dockerengine.BuildArguments{
		URI:        uri,
		Dockerfile: o.dockerfilePath,
		Context:    ctx,
		Tags:       append([]string{imageTagLatest}, additionalTags...),
		Args:       o.dockerfileBuildArgs,
	}
	buildArgsList, err := buildArgs.GenerateDockerBuildArgs(dockerengine.New(exec.NewCmd()))
	if err != nil {
		return fmt.Errorf("generate docker build args: %w", err)
	}
	log.Infof("Building your container image: docker %s\n", strings.Join(buildArgsList, " "))
	if _, err := o.repository.BuildAndPush(context.Background(), buildArgs, log.DiagnosticWriter); err != nil {
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

	var boundaryPolicy string
	if o.appName != "" {
		app, err := o.store.GetApplication(o.appName)
		if err != nil {
			return fmt.Errorf("get application: %w", err)
		}
		boundaryPolicy = app.PermissionsBoundary
	}

	secretsManagerSecrets, ssmParamSecrets := o.getCategorizedSecrets()

	entrypoint, err := shlex.Split(o.entrypoint)
	if err != nil {
		return fmt.Errorf("split entrypoint %s into tokens using shell-style rules: %w", o.entrypoint, err)
	}

	command, err := shlex.Split(o.command)
	if err != nil {
		return fmt.Errorf("split command %s into tokens using shell-style rules: %w", o.command, err)
	}

	input := &deploy.CreateTaskResourcesInput{
		Name:                  o.groupName,
		CPU:                   o.cpu,
		Memory:                o.memory,
		Image:                 o.image,
		PermissionsBoundary:   boundaryPolicy,
		TaskRole:              o.taskRole,
		ExecutionRole:         o.executionRole,
		Command:               command,
		EntryPoint:            entrypoint,
		EnvVars:               o.envVars,
		EnvFileARN:            o.envFileARN,
		SSMParamSecrets:       ssmParamSecrets,
		SecretsManagerSecrets: secretsManagerSecrets,
		OS:                    o.os,
		Arch:                  o.arch,
		App:                   o.appName,
		Env:                   o.env,
		AdditionalTags:        o.resourceTags,
	}
	return o.deployer.DeployTask(input, deployOpts...)
}

// deployEnvFileIfNeeded uploads the env file if needed, ensures that an S3 bucket is available, and returns the ARN of uploaded file.
func (o *runTaskOpts) deployEnvFile() (string, error) {
	if o.envFile == "" {
		return "", nil
	}

	info, err := o.deployer.GetTaskStack(o.groupName)
	if err != nil {
		return "", fmt.Errorf("deploy env file: %w", err)
	}

	// push env file
	o.spinner.Start(fmt.Sprintf(fmtTaskRunEnvUploadStart, color.HighlightUserInput(o.envFile)))
	envFileARN, err := o.pushEnvFileToS3(info.BucketName)
	if err != nil {
		o.spinner.Stop(log.Serrorf(fmtTaskRunEnvUploadFailed, color.HighlightUserInput(o.envFile)))
		return "", err
	}
	o.spinner.Stop(log.Ssuccessf(fmtTaskRunEnvUploadComplete, color.HighlightUserInput(o.envFile)))

	return envFileARN, nil
}

// pushEnvFileToS3 reads an env file from disk, uploads it to a unique path, and then returns the ARN of the env file.
func (o *runTaskOpts) pushEnvFileToS3(bucket string) (string, error) {
	content, err := afero.ReadFile(o.fs, o.envFile)
	if err != nil {
		return "", fmt.Errorf("read env file %s: %w", o.envFile, err)
	}
	reader := bytes.NewReader(content)

	uploader := o.configureUploader(o.sess)
	url, err := uploader.Upload(bucket, artifactpath.EnvFiles(o.envFile, content), reader)
	if err != nil {
		return "", fmt.Errorf("put env file %s artifact to bucket %s: %w", o.envFile, bucket, err)
	}
	bucket, key, err := s3.ParseURL(url)
	if err != nil {
		return "", fmt.Errorf("parse s3 url: %w", err)
	}
	// The app and environment are always within the same partition.
	partition, err := partitions.Region(aws.StringValue(o.sess.Config.Region)).Partition()
	if err != nil {
		return "", err
	}
	return s3.FormatARN(partition.ID(), fmt.Sprintf("%s/%s", bucket, key)), nil
}

func (o *runTaskOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	return nil
}

func (o *runTaskOpts) validateEnvName() error {
	if o.appName != "" {
		if _, err := o.targetEnv(o.appName, o.env); err != nil {
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

	env, err := o.sel.Environment(taskRunEnvPrompt, taskRunEnvPromptHelp, o.appName, prompt.Option{Value: appEnvOptionNone})
	if err != nil {
		return fmt.Errorf("ask for environment: %w", err)
	}

	if env == appEnvOptionNone {
		return nil
	}

	o.env = env
	return nil
}

func (o *runTaskOpts) targetEnv(appName, envName string) (*config.Environment, error) {
	env, err := o.store.GetEnvironment(appName, envName)
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
  Run a task using your local Dockerfile and display log streams after the task is running.
  You will be prompted to specify an environment for the tasks to run in.
  /code $ copilot task run
  Run a task named "db-migrate" in the "test" environment under the current workspace.
  /code $ copilot task run -n db-migrate --env test
  Run 4 tasks with 2GB memory, an existing image, and a custom task role.
  /code $ copilot task run --count 4 --memory 2048 --image=rds-migrate --task-role migrate-role
  Run a task with environment variables.
  /code $ copilot task run --env-vars name=myName,user=myUser
  Run a task using the current workspace with specific subnets and security groups.
  /code $ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
  Run a task with a command.
  /code $ copilot task run --command "python migrate-script.py"
  Run a task with Docker build args.
  /code $ copilot task run --build-args GO_VERSION=1.19"`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskRunOpts(vars)
			if err != nil {
				return err
			}
			opts.nFlag = cmd.Flags().NFlag()
			if cmd.Flags().Changed(dockerFileFlag) {
				opts.isDockerfileSet = true
			}
			return run(opts)
		}),
	}

	// add flags to comments.
	cmd.Flags().StringVarP(&vars.groupName, taskGroupNameFlag, nameFlagShort, "", taskGroupFlagDescription)

	cmd.Flags().StringVar(&vars.dockerfilePath, dockerFileFlag, defaultDockerfilePath, dockerFileFlagDescription)
	cmd.Flags().StringToStringVar(&vars.dockerfileBuildArgs, dockerFileBuildArgsFlag, nil, dockerFileBuildArgsFlagDescription)
	cmd.Flags().StringVar(&vars.dockerfileContextPath, dockerFileContextFlag, "", dockerFileContextFlagDescription)
	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", taskImageTagFlagDescription)

	cmd.Flags().StringVar(&vars.appName, appFlag, "", taskAppFlagDescription)
	cmd.Flags().StringVar(&vars.env, envFlag, "", taskEnvFlagDescription)
	cmd.Flags().StringVar(&vars.cluster, clusterFlag, "", clusterFlagDescription)
	cmd.Flags().BoolVar(&vars.acknowledgeSecretsAccess, acknowledgeSecretsAccessFlag, false, acknowledgeSecretsAccessDescription)
	cmd.Flags().StringSliceVar(&vars.subnets, subnetsFlag, nil, subnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.securityGroups, securityGroupsFlag, nil, securityGroupsFlagDescription)
	cmd.Flags().BoolVar(&vars.useDefaultSubnetsAndCluster, taskDefaultFlag, false, taskRunDefaultFlagDescription)

	cmd.Flags().IntVar(&vars.count, countFlag, 1, countFlagDescription)
	cmd.Flags().IntVar(&vars.cpu, cpuFlag, 256, cpuFlagDescription)
	cmd.Flags().IntVar(&vars.memory, memoryFlag, 512, memoryFlagDescription)
	cmd.Flags().StringVar(&vars.taskRole, taskRoleFlag, "", taskRoleFlagDescription)
	cmd.Flags().StringVar(&vars.executionRole, executionRoleFlag, "", executionRoleFlagDescription)
	cmd.Flags().StringVar(&vars.os, osFlag, "", osFlagDescription)
	cmd.Flags().StringVar(&vars.arch, archFlag, "", archFlagDescription)
	cmd.Flags().StringToStringVar(&vars.envVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.envFile, envFileFlag, "", envFileFlagDescription)
	cmd.Flags().StringToStringVar(&vars.secrets, secretsFlag, nil, secretsFlagDescription)
	cmd.Flags().StringVar(&vars.command, commandFlag, "", runCommandFlagDescription)
	cmd.Flags().StringVar(&vars.entrypoint, entrypointFlag, "", entrypointFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	cmd.Flags().StringVar(&vars.generateCommandTarget, generateCommandFlag, "", generateCommandFlagDescription)

	// group flags.
	nameFlags := pflag.NewFlagSet("Name", pflag.ContinueOnError)
	nameFlags.AddFlag(cmd.Flags().Lookup(taskGroupNameFlag))

	buildFlags := pflag.NewFlagSet("Build", pflag.ContinueOnError)
	buildFlags.AddFlag(cmd.Flags().Lookup(dockerFileFlag))
	buildFlags.AddFlag(cmd.Flags().Lookup(dockerFileBuildArgsFlag))
	buildFlags.AddFlag(cmd.Flags().Lookup(dockerFileContextFlag))
	buildFlags.AddFlag(cmd.Flags().Lookup(imageFlag))
	buildFlags.AddFlag(cmd.Flags().Lookup(imageTagFlag))

	placementFlags := pflag.NewFlagSet("Placement", pflag.ContinueOnError)
	placementFlags.AddFlag(cmd.Flags().Lookup(appFlag))
	placementFlags.AddFlag(cmd.Flags().Lookup(envFlag))
	placementFlags.AddFlag(cmd.Flags().Lookup(clusterFlag))
	placementFlags.AddFlag(cmd.Flags().Lookup(subnetsFlag))
	placementFlags.AddFlag(cmd.Flags().Lookup(securityGroupsFlag))
	placementFlags.AddFlag(cmd.Flags().Lookup(taskDefaultFlag))

	taskFlags := pflag.NewFlagSet("Task", pflag.ContinueOnError)
	taskFlags.AddFlag(cmd.Flags().Lookup(countFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(cpuFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(memoryFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(taskRoleFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(executionRoleFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(osFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(archFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(envVarsFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(envFileFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(secretsFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(commandFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(entrypointFlag))
	taskFlags.AddFlag(cmd.Flags().Lookup(resourceTagsFlag))

	utilityFlags := pflag.NewFlagSet("Utility", pflag.ContinueOnError)
	utilityFlags.AddFlag(cmd.Flags().Lookup(followFlag))
	utilityFlags.AddFlag(cmd.Flags().Lookup(generateCommandFlag))
	utilityFlags.AddFlag(cmd.Flags().Lookup(acknowledgeSecretsAccessFlag))

	// prettify help menu.
	cmd.Annotations = map[string]string{
		"name":      nameFlags.FlagUsages(),
		"build":     buildFlags.FlagUsages(),
		"placement": placementFlags.FlagUsages(),
		"task":      taskFlags.FlagUsages(),
		"utility":   utilityFlags.FlagUsages(),
	}
	cmd.SetUsageTemplate(`{{h1 "Usage"}}
{{- if .Runnable}}
{{.UseLine}}
{{- end }}

{{h1 "Name Flags"}}
{{(index .Annotations "name") | trimTrailingWhitespaces}}

{{h1 "Build Flags"}}
{{(index .Annotations "build") | trimTrailingWhitespaces}}

{{h1 "Placement Flags"}}
{{(index .Annotations "placement") | trimTrailingWhitespaces}}

{{h1 "Task Configuration Flags"}}
{{(index .Annotations "task") | trimTrailingWhitespaces}}

{{h1 "Utility Flags"}}
{{(index .Annotations "utility") | trimTrailingWhitespaces}}

{{if .HasExample }}
{{- h1 "Examples"}}
{{- code .Example}}
{{- end}}
`)
	return cmd
}
