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
)

type jobLogsVars struct {
	wkldLogsVars

	includeStateMachineLogs bool // Whether to include the logs from the state machine log streams
}

type jobLogsOpts struct {
	jobLogsVars

	wkldLogOpts
}

func newJobLogOpts(vars jobLogsVars) (*jobLogsOpts, error) {
	sessProvider := sessions.NewProvider(sessions.UserAgentExtras("job logs"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	deployStore, err := deploy.NewStore(configStore)
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
	opts.initLogsSvc = func() error {
		env, err := opts.configStore.GetEnvironment(opts.appName, opts.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		opts.logsSvc, err = logging.NewServiceClient(&logging.NewServiceLogsConfig{
			Sess: sess,
			App:  opts.appName,
			Env:  opts.envName,
			Svc:  opts.name,
		})
		if err != nil {
			return err
		}
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
	if err := o.askApp(); err != nil {
		return err
	}
	return nil
}

func (o *jobLogsOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(jobAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

// Execute outputs logs of the job.
func (o *jobLogsOpts) Execute() error {
	return nil
}

// buildJobLogsCmd builds the command for displaying job logs in an application.
func buildJobLogsCmd() *cobra.Command {
	vars := jobLogsVars{}
	cmd := &cobra.Command{
		Use:    "logs",
		Short:  "Displays logs of a deployed job.",
		Hidden: true,
		Example: `
  Displays logs of the job "my-job" in environment "test".
  /code $ copilot job logs -n my-job -e test
  Displays logs in the last hour.
  /code $ copilot job logs --since 1h
  Displays logs from 2006-01-02T15:04:05 to 2006-01-02T15:05:05.
  /code $ copilot job logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00
Displays logs from specific task IDs.
  /code $ copilot job logs --tasks 709c7eae05f947f6861b150372ddc443,1de57fd63c6a4920ac416d02add891b9
  Displays logs in real time.
  /code $ copilot job logs --follow
  Displays container logs and state machine execution logs from the last execution.
  /code $ copilot job logs --include-state-machine`,
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
	cmd.Flags().StringSliceVar(&vars.taskIDs, tasksFlag, nil, tasksLogsFlagDescription)
	cmd.Flags().BoolVar(&vars.includeStateMachineLogs, includeStateMachineLogsFlag, false, includeStateMachineLogsFlagDescription)
	return cmd
}
