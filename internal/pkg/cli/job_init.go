// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/lnquy/cron"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	jobInitSchedulePrompt = "How would you like to " + color.Emphasize("schedule") + " this job?"
	jobInitScheduleHelp   = `How to determine this job's schedule. "Rate" lets you define the time between 
executions and is good for jobs which need to run frequently. "Fixed Schedule"
lets you use a predefined or custom cron schedule and is good for less-frequent 
jobs or those which require specific execution schedules.`

	jobInitRatePrompt = "How long would you like to wait between executions?"
	jobInitRateHelp   = `You can specify the time as a duration string. (For example, 2m, 1h30m, 24h)`

	jobInitCronSchedulePrompt = "What schedule would you like to use?"
	jobInitCronScheduleHelp   = `Predefined schedules run at midnight or the top of the hour.
For example, "Daily" runs at midnight. "Weekly" runs at midnight on Mondays.`
	jobInitCronCustomSchedulePrompt = "What custom cron schedule would you like to use?"
	jobInitCronCustomScheduleHelp   = `Custom schedules can be defined using the following cron:
Minute | Hour | Day of Month | Month | Day of Week
For example: 0 17 ? * MON-FRI (5 pm on weekdays)
			 0 0 1 */3 * (on the first of the month, quarterly)
For more information, see: https://en.wikipedia.org/wiki/Cron#Overview`
	jobInitCronHumanReadableConfirmPrompt = "Would you like to use this schedule?"
	jobInitCronHumanReadableConfirmHelp   = `Confirm whether the schedule looks right to you.
(Y)es will continue execution. (N)o will allow you to input a different schedule.`
)

const (
	fmtAddJobToAppStart    = "Creating ECR repositories for job %s."
	fmtAddJobToAppFailed   = "Failed to create ECR repositories for job %s.\n"
	fmtAddJobToAppComplete = "Created ECR repositories for job %s.\n"
)

const (
	job = "job"

	rate          = "Rate"
	fixedSchedule = "Fixed Schedule"

	custom  = "Custom"
	hourly  = "Hourly"
	daily   = "Daily"
	weekly  = "Weekly"
	monthly = "Monthly"
	yearly  = "Yearly"

// 	fmtAddJobToAppStart    = "Creating ECR repositories for job %s."
// 	fmtAddJobToAppFailed   = "Failed to create ECR repositories for job %s.\n"
// 	fmtAddJobToAppComplete = "Created ECR repositories for job %s.\n"
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

type initJobVars struct {
	appName        string
	name           string
	dockerfilePath string
	timeout        string
	retries        int
	schedule       string
	jobType        string
}

type initJobOpts struct {
	initJobVars

	// Interfaces to interact with dependencies.
	fs          afero.Fs
	ws          svcDirManifestWriter
	store       store
	appDeployer jobDeployer
	prog        progress
	prompt      prompter

	// Outputs stored on successful actions.
	manifestPath string
}

func newInitJobOpts(vars initJobVars) (*initJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, err
	}

	return &initJobOpts{
		initJobVars: vars,

		fs:          &afero.Afero{Fs: afero.NewOsFs()},
		store:       store,
		ws:          ws,
		appDeployer: cloudformation.New(sess),
		prog:        termprogress.NewSpinner(),
		prompt:      prompt.New(),
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initJobOpts) Validate() error {
	if o.jobType != "" {
		if err := validateJobType(o.jobType); err != nil {
			return err
		}
	}
	if o.name != "" {
		if err := validateJobName(o.name); err != nil {
			return err
		}
	}
	if o.dockerfilePath != "" {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}
	if o.schedule != "" {
		if err := validateSchedule(o.schedule); err != nil {
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
	if err := o.askJobType(); err != nil {
		return err
	}
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askDockerfile(); err != nil {
		return err
	}
	if err := o.askSchedule(); err != nil {
		return err
	}
	return nil
}

// Execute writes the job's manifest file and stores the name in SSM.
func (o *initJobOpts) Execute() error {
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}

	manifestPath, err := o.createManifest()
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath

	o.prog.Start(fmt.Sprintf(fmtAddJobToAppStart, o.name))
	if err := o.appDeployer.AddJobToApp(app, o.name); err != nil {
		o.prog.Stop(log.Serrorf(fmtAddJobToAppFailed, o.name))
		return fmt.Errorf("add job %s to application %s: %w", o.name, o.appName, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtAddJobToAppComplete, o.name))

	if err := o.store.CreateJob(&config.Workload{
		App:  o.appName,
		Name: o.name,
		Type: o.jobType,
	}); err != nil {
		return fmt.Errorf("saving job %s: %w", o.name, err)
	}
	return nil
}

func (o *initJobOpts) createManifest() (string, error) {
	manifest, err := o.newJobManifest()
	if err != nil {
		return "", err
	}
	var manifestExists bool
	manifestPath, err := o.ws.WriteWorkloadManifest(manifest, o.name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", err
		}
		manifestExists = true
		manifestPath = e.FileName
	}
	manifestPath, err = relPath(manifestPath)
	if err != nil {
		return "", err
	}

	manifestMsgFmt := "Wrote the manifest for job %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for job %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.name), color.HighlightResource(manifestPath))
	log.Infoln(color.Help(fmt.Sprintf("Your manifest contains configurations like your container size and job schedule (%s).", o.schedule)))
	log.Infoln()

	return manifestPath, nil
}

func (o *initJobOpts) newJobManifest() (*manifest.ScheduledJob, error) {
	dfPath, err := o.getRelativePath()
	if err != nil {
		return nil, err
	}
	return manifest.NewScheduledJob(manifest.ScheduledJobProps{
		WorkloadProps: &manifest.WorkloadProps{
			Name:       o.name,
			Dockerfile: dfPath,
		},
		Schedule: o.schedule,
		Timeout:  o.timeout,
		Retries:  o.retries,
	}), nil
}

func (o *initJobOpts) getRelativePath() (string, error) {
	copilotDirPath, err := o.ws.CopilotDirPath()
	if err != nil {
		return "", fmt.Errorf("get copilot directory: %w", err)
	}
	wsRoot := filepath.Dir(copilotDirPath)
	absDfPath, err := filepath.Abs(o.dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	if !strings.Contains(absDfPath, wsRoot) {
		return "", fmt.Errorf("Dockerfile %s not within workspace %s", absDfPath, wsRoot)
	}
	relDfPath, err := filepath.Rel(wsRoot, absDfPath)
	if err != nil {
		return "", fmt.Errorf("find relative path from workspace root to Dockerfile: %v", err)
	}
	return relDfPath, nil
}

func (o *initJobOpts) askJobType() error {
	if o.jobType != "" {
		return nil
	}
	// short circuit since there's only one valid job type.
	o.jobType = manifest.ScheduledJobType
	return nil
}

func (o *initJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.prompt.Get(
		fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), color.HighlightUserInput(o.jobType)),
		fmt.Sprintf(fmtWkldInitNameHelpPrompt, job, o.appName),
		validateSvcName,
		prompt.WithFinalMessage("Job name:"),
	)
	if err != nil {
		return fmt.Errorf("get job name: %w", err)
	}
	o.name = name
	return nil
}

func (o *initJobOpts) askDockerfile() error {
	if o.dockerfilePath != "" {
		return nil
	}
	df, err := askDockerfile(o.name, o.fs, o.prompt)
	if err != nil {
		return err
	}
	o.dockerfilePath = df
	return nil
}

func (o *initJobOpts) askSchedule() error {
	if o.schedule != "" {
		return nil
	}
	scheduleType, err := o.prompt.SelectOne(
		jobInitSchedulePrompt,
		jobInitScheduleHelp,
		scheduleTypes,
		prompt.WithFinalMessage("Schedule type:"),
	)
	if err != nil {
		return fmt.Errorf("get schedule type: %w", err)
	}
	switch scheduleType {
	case rate:
		return o.askRate()
	case fixedSchedule:
		return o.askCron()
	default:
		return fmt.Errorf("unrecognized schedule type %s", scheduleType)
	}
}

func (o *initJobOpts) askRate() error {
	rateInput, err := o.prompt.Get(
		jobInitRatePrompt,
		jobInitRateHelp,
		validateRate,
		prompt.WithFinalMessage("Rate:"),
	)
	if err != nil {
		return fmt.Errorf("get schedule rate: %w", err)
	}
	o.schedule = fmt.Sprintf("@every %s", rateInput)
	return nil
}

func (o *initJobOpts) askCron() error {
	cronInput, err := o.prompt.SelectOne(
		jobInitCronSchedulePrompt,
		jobInitCronScheduleHelp,
		presetSchedules,
		prompt.WithFinalMessage("Fixed Schedule:"),
	)
	if err != nil {
		return fmt.Errorf("get preset schedule: %w", err)
	}
	if cronInput != custom {
		o.schedule = getPresetSchedule(cronInput)
		return nil
	}
	var customSchedule, humanCron string
	cronDescriptor, err := cron.NewDescriptor()
	if err != nil {
		return fmt.Errorf("get custom schedule: %w", err)
	}
	for {
		customSchedule, err = o.prompt.Get(
			jobInitCronCustomSchedulePrompt,
			jobInitCronCustomScheduleHelp,
			validateSchedule,
			prompt.WithDefaultInput("0 * * * *"),
			prompt.WithFinalMessage("Custom Schedule:"),
		)
		if err != nil {
			return fmt.Errorf("get custom schedule: %w", err)
		}

		// Break if the customer has specified an easy to read cron definition string
		if strings.HasPrefix(customSchedule, "@") {
			break
		}

		humanCron, err = cronDescriptor.ToDescription(customSchedule, cron.Locale_en)
		if err != nil {
			return fmt.Errorf("convert cron to human string: %w", err)
		}

		log.Infoln(fmt.Sprintf("Your job will run at the following times: %s", humanCron))

		ok, err := o.prompt.Confirm(
			jobInitCronHumanReadableConfirmPrompt,
			jobInitCronHumanReadableConfirmHelp,
		)
		if err != nil {
			return fmt.Errorf("confirm cron schedule: %w", err)
		}
		if ok {
			break
		}
	}

	o.schedule = customSchedule
	return nil
}

func getPresetSchedule(input string) string {
	return fmt.Sprintf("@%s", strings.ToLower(input))
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initJobOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your job to a %s environment.",
			color.HighlightCode(fmt.Sprintf("copilot job deploy --name %s --env %s", o.name, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
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
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.jobType, jobTypeFlag, jobTypeFlagShort, "", jobTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.schedule, scheduleFlag, scheduleFlagShort, "", scheduleFlagDescription)
	cmd.Flags().StringVar(&vars.timeout, timeoutFlag, "", timeoutFlagDescription)
	cmd.Flags().IntVar(&vars.retries, retriesFlag, 0, retriesFlagDescription)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}
