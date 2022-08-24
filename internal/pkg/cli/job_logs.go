// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	jobAppNamePrompt = "Which application does your job belong to?"

	jobLogNamePrompt     = "Which job's logs would you like to show?"
	jobLogNameHelpPrompt = "The logs of the indicated deployed job will be shown."

	defaultJobLogExecutionLimit = 1
)

type jobLogsVars struct {
	wkldLogsVars

	includeStateMachineLogs bool // Whether to include the logs from the state machine log streams
	last                    int  // The number of previous executions of the state machine to show.
}

type jobLogsOpts struct {
	jobLogsVars
	wkldLogOpts

	// Cached variables.
	targetEnv *config.Environment
}

func newJobLogOpts(vars jobLogsVars) (*jobLogsOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("job logs"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	deployStore, err := deploy.NewStore(sessProvider, configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	opts := &jobLogsOpts{
		jobLogsVars: vars,
		wkldLogOpts: wkldLogOpts{
			w:           log.OutputWriter,
			configStore: configStore,
			deployStore: deployStore,
			sel:         selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		},
	}
	opts.initRuntimeClients = func() error {
		env, err := opts.getTargetEnv()
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		opts.logsSvc = logging.NewJobLogger(&logging.NewWorkloadLoggerOpts{
			Sess: sess,
			App:  opts.appName,
			Env:  opts.envName,
			Name: opts.name,
		})
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by flags are invalid.
func (o *jobLogsOpts) Validate() error {
	if o.appName != "" {
		if _, err := o.configStore.GetApplication(o.appName); err != nil {
			return err
		}
		if o.envName != "" {
			if _, err := o.configStore.GetEnvironment(o.appName, o.envName); err != nil {
				return err
			}
		}
		if o.name != "" {
			if _, err := o.configStore.GetJob(o.appName, o.name); err != nil {
				return err
			}
		}
	}

	if o.since != 0 && o.humanStartTime != "" {
		return errors.New("only one of --since or --start-time may be used")
	}

	if o.humanEndTime != "" && o.follow {
		return errors.New("only one of --follow or --end-time may be used")
	}

	if o.since != 0 {
		if o.since < 0 {
			return fmt.Errorf("--since must be greater than 0")
		}
		// round up to the nearest second
		o.startTime = parseSince(o.since)
	}

	if o.humanStartTime != "" {
		startTime, err := parseRFC3339(o.humanStartTime)
		if err != nil {
			return fmt.Errorf(`invalid argument %s for "--start-time" flag: %w`, o.humanStartTime, err)
		}
		o.startTime = aws.Int64(startTime)
	}

	if o.humanEndTime != "" {
		endTime, err := parseRFC3339(o.humanEndTime)
		if err != nil {
			return fmt.Errorf(`invalid argument %s for "--end-time" flag: %w`, o.humanEndTime, err)
		}
		o.endTime = aws.Int64(endTime)
	}

	if o.limit != 0 && (o.limit < cwGetLogEventsLimitMin || o.limit > cwGetLogEventsLimitMax) {
		return fmt.Errorf("--limit %d is out-of-bounds, value must be between %d and %d", o.limit, cwGetLogEventsLimitMin, cwGetLogEventsLimitMax)
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *jobLogsOpts) Ask() error {
	if err := o.validateOrAskApp(); err != nil {
		return err
	}
	return o.validateAndAskJobEnvName()
}

// Execute outputs logs of the job.
func (o *jobLogsOpts) Execute() error {
	if err := o.initRuntimeClients(); err != nil {
		return err
	}
	eventsWriter := logging.WriteHumanLogs
	if o.shouldOutputJSON {
		eventsWriter = logging.WriteJSONLogs
	}
	var limit *int64
	if o.limit != 0 {
		limit = aws.Int64(int64(o.limit))
	}

	// By default, only display the logs of the last execution of the job.
	logStreamLimit := defaultJobLogExecutionLimit
	if o.last != 0 {
		logStreamLimit = o.last
	}

	err := o.logsSvc.WriteLogEvents(logging.WriteLogEventsOpts{
		Follow:                  o.follow,
		Limit:                   limit,
		EndTime:                 o.endTime,
		StartTime:               o.startTime,
		TaskIDs:                 o.taskIDs,
		OnEvents:                eventsWriter,
		LogStreamLimit:          logStreamLimit,
		IncludeStateMachineLogs: o.includeStateMachineLogs,
	})
	if err != nil {
		return fmt.Errorf("write log events for job %s: %w", o.name, err)
	}
	return nil
}

func (o *jobLogsOpts) getTargetEnv() (*config.Environment, error) {
	if o.targetEnv != nil {
		return o.targetEnv, nil
	}
	env, err := o.configStore.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, err
	}
	o.targetEnv = env
	return o.targetEnv, nil
}

func (o *jobLogsOpts) validateOrAskApp() error {
	if o.appName != "" {
		_, err := o.configStore.GetApplication(o.appName)
		return err
	}
	app, err := o.sel.Application(jobAppNamePrompt, wkldAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *jobLogsOpts) validateAndAskJobEnvName() error {
	if o.envName != "" {
		if _, err := o.getTargetEnv(); err != nil {
			return err
		}
	}
	if o.name != "" {
		if _, err := o.configStore.GetJob(o.appName, o.name); err != nil {
			return err
		}
	}
	deployedJob, err := o.sel.DeployedJob(jobLogNamePrompt, jobLogNameHelpPrompt, o.appName, selector.WithEnv(o.envName), selector.WithName(o.name))
	if err != nil {
		return fmt.Errorf("select deployed jobs for application %s: %w", o.appName, err)
	}
	o.name = deployedJob.Name
	o.envName = deployedJob.Env
	return nil
}

// buildJobLogsCmd builds the command for displaying job logs in an application.
func buildJobLogsCmd() *cobra.Command {
	vars := jobLogsVars{}
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Displays logs of a deployed job.",
		Example: `
  Displays logs of the job "my-job" in environment "test".
  /code $ copilot job logs -n my-job -e test
  Displays logs in the last hour.
  /code $ copilot job logs --since 1h
  Displays logs from the last execution of the job.
  /code $ copilot job logs --last 1
  Displays logs from specific task IDs.
/code $ copilot job logs --tasks 709c7ea,1de57fd
  Displays logs in real time.
  /code $ copilot job logs --follow
  Displays container logs and state machine execution logs from the last execution.
  /code $ copilot job logs --include-state-machine --last 1`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newJobLogOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.humanStartTime, startTimeFlag, "", startTimeFlagDescription)
	cmd.Flags().StringVar(&vars.humanEndTime, endTimeFlag, "", endTimeFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	cmd.Flags().DurationVar(&vars.since, sinceFlag, 0, sinceFlagDescription)
	cmd.Flags().IntVar(&vars.limit, limitFlag, 0, limitFlagDescription)
	cmd.Flags().IntVar(&vars.last, lastFlag, 1, lastFlagDescription)
	cmd.Flags().StringSliceVar(&vars.taskIDs, tasksFlag, nil, tasksLogsFlagDescription)
	cmd.Flags().BoolVar(&vars.includeStateMachineLogs, includeStateMachineLogsFlag, false, includeStateMachineLogsFlagDescription)

	// There's no way to associate a specific execution with a task without parsing the logs of every state machine invocation.
	cmd.MarkFlagsMutuallyExclusive(includeStateMachineLogsFlag, tasksFlag)
	cmd.MarkFlagsMutuallyExclusive(followFlag, lastFlag)

	return cmd
}
