// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
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
	*GlobalOpts
	Name           string
	DockerfilePath string
	Timeout        string
	Retries        int
	Schedule       string
	JobType        string
}

type initJobOpts struct {
	initJobVars

	// Interfaces to interact with dependencies.
	fs          afero.Fs
	ws          svcManifestWriter
	store       store
	appDeployer appDeployer
	prog        progress

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
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initJobOpts) Validate() error {
	if o.JobType != "" {
		if err := validateJobType(o.JobType); err != nil {
			return err
		}
	}
	if o.Name != "" {
		if err := validateJobName(o.Name); err != nil {
			return err
		}
	}
	if o.DockerfilePath != "" {
		if _, err := o.fs.Stat(o.DockerfilePath); err != nil {
			return err
		}
	}
	if o.Schedule != "" {
		if err := validateSchedule(o.Schedule); err != nil {
			return err
		}
	}
	if o.Timeout != "" {
		if err := validateTimeout(o.Timeout); err != nil {
			return err
		}
	}
	if o.Retries < 0 {
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
	return nil
}

func (o *initJobOpts) askJobType() error {
	if o.JobType != "" {
		return nil
	}
	// short circuit since there's only one valid job type.
	o.JobType = manifest.ScheduledJobType
	return nil
}

func (o *initJobOpts) askJobName() error {
	if o.Name != "" {
		return nil
	}

	name, err := o.prompt.Get(
		fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), color.HighlightUserInput(o.JobType)),
		fmt.Sprintf(fmtWkldInitNameHelpPrompt, job, o.AppName()),
		validateSvcName,
		prompt.WithFinalMessage("Job name:"),
	)
	if err != nil {
		return fmt.Errorf("get job name: %w", err)
	}
	o.Name = name
	return nil
}

func (o *initJobOpts) askDockerfile() error {
	if o.DockerfilePath != "" {
		return nil
	}
	df, err := askDockerfile(o.Name, o.fs, o.prompt)
	if err != nil {
		return err
	}
	o.DockerfilePath = df
	return nil
}

func (o *initJobOpts) askSchedule() error {
	if o.Schedule != "" {
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
	o.Schedule = fmt.Sprintf("@every %s", rateInput)
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
		o.Schedule = getPresetSchedule(cronInput)
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

	o.Schedule = customSchedule
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
			color.HighlightCode(fmt.Sprintf("copilot job deploy --name %s --env %s", o.Name, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
}

// BuildJobInitCmd builds the command for creating a new job.
func BuildJobInitCmd() *cobra.Command {
	vars := initJobVars{
		GlobalOpts: NewGlobalOpts(),
	}
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
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.JobType, jobTypeFlag, jobTypeFlagShort, "", jobTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.DockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.Schedule, scheduleFlag, scheduleFlagShort, "", scheduleFlagDescription)
	cmd.Flags().StringVar(&vars.Timeout, timeoutFlag, "", timeoutFlagDescription)
	cmd.Flags().IntVar(&vars.Retries, retriesFlag, 0, retriesFlagDescription)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}
