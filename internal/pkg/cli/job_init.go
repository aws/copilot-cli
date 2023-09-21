// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/version"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const jobTypeHint = "Scheduled event to State Machine to Fargate"

var (
	jobInitSchedulePrompt = "How would you like to " + color.Emphasize("schedule") + " this job?"
	jobInitScheduleHelp   = `How to determine this job's schedule. "Rate" lets you define the time between 
executions and is good for jobs which need to run frequently. "Fixed Schedule"
lets you use a predefined or custom cron schedule and is good for less-frequent 
jobs or those which require specific execution schedules.`

	jobInitTypeHelp = fmt.Sprintf(`A %s is a task which is invoked on a set schedule, with optional retry logic.
To learn more see: https://git.io/JEEU4`, manifestinfo.ScheduledJobType)
)

var jobTypeHints = map[string]string{
	manifestinfo.ScheduledJobType: jobTypeHint,
}

type initJobVars struct {
	initWkldVars

	timeout  string
	retries  int
	schedule string
}

type initJobOpts struct {
	initJobVars

	// Interfaces to interact with dependencies.
	fs               afero.Fs
	store            store
	init             jobInitializer
	prompt           prompter
	dockerEngine     dockerEngine
	mftReader        manifestReader
	dockerfileSel    dockerfileSelector
	scheduleSelector scheduleSelector

	// Outputs stored on successful actions.
	manifestPath   string
	manifestExists bool
	platform       *manifest.PlatformString

	// For workspace validation.
	wsPendingCreation bool
	wsAppName         string

	initParser          func(path string) dockerfileParser
	initEnvDescriber    func(appName, envName string) (envDescriber, error)
	newAppVersionGetter func(appName string) (versionGetter, error)

	// Overridden in tests.
	templateVersion string
}

func newInitJobOpts(vars initJobVars) (*initJobOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	p := sessions.ImmutableProvider(sessions.UserAgentExtras("job init"))
	sess, err := p.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(sess), ssm.New(sess), aws.StringValue(sess.Config.Region))
	jobInitter := &initialize.WorkloadInitializer{
		Store:    store,
		Ws:       ws,
		Prog:     termprogress.NewSpinner(log.DiagnosticWriter),
		Deployer: cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr)),
	}

	prompter := prompt.New()
	dockerfileSel, err := selector.NewDockerfileSelector(prompter, fs)
	if err != nil {
		return nil, err
	}
	return &initJobOpts{
		initJobVars: vars,

		fs:               fs,
		store:            store,
		init:             jobInitter,
		prompt:           prompter,
		dockerfileSel:    dockerfileSel,
		scheduleSelector: selector.NewStaticSelector(prompter),
		dockerEngine:     dockerengine.New(exec.NewCmd()),
		mftReader:        ws,
		initParser: func(path string) dockerfileParser {
			return dockerfile.New(fs, path)
		},
		initEnvDescriber: func(appName string, envName string) (envDescriber, error) {
			envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
				App:         appName,
				Env:         envName,
				ConfigStore: store,
			})
			if err != nil {
				return nil, err
			}
			return envDescriber, nil
		},
		newAppVersionGetter: func(appName string) (versionGetter, error) {
			return describe.NewAppDescriber(appName)
		},
		wsAppName:       tryReadingAppName(),
		templateVersion: version.LatestTemplateVersion(),
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initJobOpts) Validate() error {
	// If this app is pending creation, we'll skip validation.
	if !o.wsPendingCreation {
		if err := validateWorkspaceApp(o.wsAppName, o.appName, o.store); err != nil {
			return err
		}
		o.appName = o.wsAppName
	}
	if o.dockerfilePath != "" && o.image != "" {
		return fmt.Errorf("--%s and --%s cannot be specified together", dockerFileFlag, imageFlag)
	}
	if o.dockerfilePath != "" {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}
	if o.timeout != "" {
		if err := validateTimeout(o.timeout); err != nil {
			return err
		}
	}
	if o.retries < 0 {
		return errors.New("number of retries must be non-negative")
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initJobOpts) Ask() error {
	if o.wkldType != "" {
		if err := validateJobType(o.wkldType); err != nil {
			return err
		}
	} else {
		if err := o.askJobType(); err != nil {
			return err
		}
	}
	if o.name == "" {
		if err := o.askJobName(); err != nil {
			return err
		}
	}
	if err := validateJobName(o.name); err != nil {
		return err
	}
	if err := o.validateDuplicateJob(); err != nil {
		return err
	}
	if !o.wsPendingCreation {
		localMft, err := o.mftReader.ReadWorkloadManifest(o.name)
		if err == nil {
			jobType, err := localMft.WorkloadType()
			if err != nil {
				return fmt.Errorf(`read "type" field for job %s from local manifest: %w`, o.name, err)
			}
			if o.wkldType != jobType {
				return fmt.Errorf("manifest file for job %s exists with a different type %s", o.name, jobType)
			}
			log.Infof("Manifest file for job %s already exists. Skipping configuration.\n", o.name)
			o.manifestExists = true
			return nil
		}
		var errNotFound *workspace.ErrFileNotExists
		if !errors.As(err, &errNotFound) {
			return fmt.Errorf("read manifest file for job %s: %w", o.name, err)
		}
	}
	dfSelected, err := o.askDockerfile()
	if err != nil {
		return err
	}
	if !dfSelected {
		if err := o.askImage(); err != nil {
			return err
		}
	}
	if o.schedule == "" {
		if err := o.askSchedule(); err != nil {
			return err
		}
	}
	if err := validateSchedule(o.schedule); err != nil {
		return err
	}
	return nil
}

// envsWithPrivateSubnetsOnly returns the list of environments names deployed that contains only private subnets.
func envsWithPrivateSubnetsOnly(store store, initEnvDescriber func(string, string) (envDescriber, error), appName string) ([]string, error) {
	envs, err := store.ListEnvironments(appName)
	if err != nil {
		return nil, fmt.Errorf("list environments for application %s: %w", appName, err)
	}
	var privateOnlyEnvs []string
	for _, env := range envs {
		envDescriber, err := initEnvDescriber(appName, env.Name)
		if err != nil {
			return nil, err
		}
		mft, err := envDescriber.Manifest()
		if err != nil {
			return nil, fmt.Errorf("read the manifest used to deploy environment %s: %w", env.Name, err)
		}
		envConfig, err := manifest.UnmarshalEnvironment(mft)
		if err != nil {
			return nil, fmt.Errorf("unmarshal the manifest used to deploy environment %s: %w", env.Name, err)
		}
		subnets := envConfig.Network.VPC.Subnets

		if len(subnets.Public) == 0 && len(subnets.Private) != 0 {
			privateOnlyEnvs = append(privateOnlyEnvs, env.Name)
		}
	}
	return privateOnlyEnvs, err
}

// Execute writes the job's manifest file, creates an ECR repo, and stores the name in SSM.
func (o *initJobOpts) Execute() error {
	if !o.allowAppDowngrade {
		appVersionGetter, err := o.newAppVersionGetter(o.appName)
		if err != nil {
			return err
		}
		if err := validateAppVersion(appVersionGetter, o.appName, o.templateVersion); err != nil {
			return err
		}
	}
	// Check for a valid healthcheck and add it to the opts.
	var hc manifest.ContainerHealthCheck
	var err error
	if o.dockerfilePath != "" {
		hc, err = parseHealthCheck(o.initParser(o.dockerfilePath))
		if err != nil {
			log.Warningf("Cannot parse the HEALTHCHECK instruction from the Dockerfile: %v\n", err)
		}
	}
	// If the user passes in an image, their docker engine isn't necessarily running, and we can't do anything with the platform because we're not building the Docker image.
	if o.image == "" && !o.manifestExists {
		platform, err := legitimizePlatform(o.dockerEngine, o.wkldType)
		if err != nil {
			return err
		}
		if platform != "" {
			o.platform = &platform
		}
	}
	envs, err := envsWithPrivateSubnetsOnly(o.store, o.initEnvDescriber, o.appName)
	if err != nil {
		return err
	}
	manifestPath, err := o.init.Job(&initialize.JobProps{
		WorkloadProps: initialize.WorkloadProps{
			App:            o.appName,
			Name:           o.name,
			Type:           o.wkldType,
			DockerfilePath: o.dockerfilePath,
			Image:          o.image,
			Platform: manifest.PlatformArgsOrString{
				PlatformString: o.platform,
			},
			PrivateOnlyEnvironments: envs,
		},

		Schedule:    o.schedule,
		HealthCheck: hc,
		Timeout:     o.timeout,
		Retries:     o.retries,
	})
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *initJobOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your job to a %s environment.",
			color.HighlightCode(fmt.Sprintf("copilot job deploy --name %s --env %s", o.name, defaultEnvironmentName)),
			defaultEnvironmentName),
	})
	return nil
}

func (o *initJobOpts) validateDuplicateJob() error {
	_, err := o.store.GetJob(o.appName, o.name)
	if err == nil {
		log.Errorf(`It seems like you are trying to init a job that already exists.
To recreate the job, please run:
1. %s. Note: The manifest file will not be deleted and will be used in Step 2.
If you'd prefer a new default manifest, please manually delete the existing one.
2. And then %s
`,
			color.HighlightCode(fmt.Sprintf("copilot job delete --name %s", o.name)),
			color.HighlightCode(fmt.Sprintf("copilot job init --name %s", o.name)))
		return fmt.Errorf("job %s already exists", color.HighlightUserInput(o.name))
	}

	var errNoSuchJob *config.ErrNoSuchJob
	if !errors.As(err, &errNoSuchJob) {
		return fmt.Errorf("validate if job exists: %w", err)
	}
	return nil
}

func (o *initJobOpts) askJobType() error {
	if o.wkldType != "" {
		return nil
	}
	// short circuit since there's only one valid job type.
	o.wkldType = manifestinfo.ScheduledJobType
	return nil
}

func (o *initJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}
	name, err := o.prompt.Get(
		fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), "job"),
		fmt.Sprintf(fmtWkldInitNameHelpPrompt, "job", o.appName),
		func(val interface{}) error {
			return validateJobName(val)
		},
		prompt.WithFinalMessage("Job name:"),
	)
	if err != nil {
		return fmt.Errorf("get job name: %w", err)
	}
	o.name = name
	return nil
}

func (o *initJobOpts) askImage() error {
	if o.image != "" {
		return nil
	}
	image, err := o.prompt.Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil,
		prompt.WithFinalMessage("Image:"))
	if err != nil {
		return fmt.Errorf("get image location: %w", err)
	}
	o.image = image
	return nil
}

// isDfSelected indicates if any Dockerfile is in use.
func (o *initJobOpts) askDockerfile() (isDfSelected bool, err error) {
	if o.dockerfilePath != "" || o.image != "" {
		return true, nil
	}
	if err = o.dockerEngine.CheckDockerEngineRunning(); err != nil {
		var errDaemon *dockerengine.ErrDockerDaemonNotResponsive
		switch {
		case errors.Is(err, dockerengine.ErrDockerCommandNotFound):
			log.Info("Docker command is not found; Copilot won't build from a Dockerfile.\n")
			return false, nil
		case errors.As(err, &errDaemon):
			log.Info("Docker daemon is not responsive; Copilot won't build from a Dockerfile.\n")
			return false, nil
		default:
			return false, fmt.Errorf("check if docker engine is running: %w", err)
		}
	}
	df, err := o.dockerfileSel.Dockerfile(
		fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(o.name)),
		fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(o.name)),
		wkldInitDockerfileHelpPrompt,
		wkldInitDockerfilePathHelpPrompt,
		func(v interface{}) error {
			return validatePath(afero.NewOsFs(), v)
		},
	)
	if err != nil {
		return false, fmt.Errorf("select Dockerfile: %w", err)
	}
	if df == selector.DockerfilePromptUseImage {
		return false, nil
	}
	o.dockerfilePath = df
	return true, nil
}

func (o *initJobOpts) askSchedule() error {
	schedule, err := o.scheduleSelector.Schedule(
		jobInitSchedulePrompt,
		jobInitScheduleHelp,
		validateSchedule,
		validateRate,
	)
	if err != nil {
		return fmt.Errorf("get schedule: %w", err)
	}

	o.schedule = schedule
	return nil
}

func jobTypePromptOpts() []prompt.Option {
	var options []prompt.Option
	for _, jobType := range manifestinfo.JobTypes() {
		options = append(options, prompt.Option{
			Value: jobType,
			Hint:  jobTypeHints[jobType],
		})
	}
	return options
}

// buildJobInitCmd builds the command for creating a new job.
func buildJobInitCmd() *cobra.Command {
	vars := initJobVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new scheduled job in an application.",
		Example: `
  Create a "reaper" scheduled task to run once per day.
  /code $ copilot job init --name reaper --dockerfile ./frontend/Dockerfile --schedule "every 2 hours"

  Create a "report-generator" scheduled task with retries.
  /code $ copilot job init --name report-generator --schedule "@monthly" --retries 3 --timeout 900s`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitJobOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.wkldType, jobTypeFlag, typeFlagShort, "", jobTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.schedule, scheduleFlag, scheduleFlagShort, "", scheduleFlagDescription)
	cmd.Flags().StringVar(&vars.timeout, timeoutFlag, "", timeoutFlagDescription)
	cmd.Flags().IntVar(&vars.retries, retriesFlag, 0, retriesFlagDescription)
	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().BoolVar(&vars.allowAppDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}
