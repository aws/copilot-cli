// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

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
	svcLogNamePrompt     = "Which service's logs would you like to show?"
	svcLogNameHelpPrompt = "The logs of a deployed service will be shown."

	cwGetLogEventsLimitMin = 1
	cwGetLogEventsLimitMax = 10000
)

type wkldLogsVars struct {
	shouldOutputJSON bool
	follow           bool
	limit            int
	name             string
	envName          string
	appName          string
	humanStartTime   string
	humanEndTime     string
	taskIDs          []string
	since            time.Duration
	logGroup         string
}

type svcLogsOpts struct {
	wkldLogsVars
	wkldLogOpts
}

type wkldLogOpts struct {
	// internal states
	startTime *int64
	endTime   *int64

	w           io.Writer
	configStore store
	deployStore deployedEnvironmentLister
	sel         deploySelector
	logsSvc     logEventsWriter
	initLogsSvc func() error // Overriden in tests.
}

func newSvcLogOpts(vars wkldLogsVars) (*svcLogsOpts, error) {
	sessProvider := sessions.NewProvider()
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	opts := &svcLogsOpts{
		wkldLogsVars: vars,
		wkldLogOpts: wkldLogOpts{
			w:           log.OutputWriter,
			configStore: configStore,
			deployStore: deployStore,
			sel:         selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		},
	}
	opts.initLogsSvc = func() error {
		env, err := configStore.GetEnvironment(opts.appName, opts.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		workload, err := configStore.GetWorkload(opts.appName, opts.name)
		if err != nil {
			return fmt.Errorf("get workload: %w", err)
		}
		sess, err := sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		opts.logsSvc, err = logging.NewServiceClient(&logging.NewServiceLogsConfig{
			App:         opts.appName,
			Env:         opts.envName,
			Svc:         opts.name,
			Sess:        sess,
			LogGroup:    opts.logGroup,
			WkldType:    workload.Type,
			TaskIDs:     opts.taskIDs,
			ConfigStore: configStore,
		})
		if err != nil {
			return err
		}
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by flags are invalid.
func (o *svcLogsOpts) Validate() error {
	if o.appName != "" {
		if _, err := o.configStore.GetApplication(o.appName); err != nil {
			return err
		}
		if o.name != "" {
			if _, err := o.configStore.GetService(o.appName, o.name); err != nil {
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
func (o *svcLogsOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcEnvName()
}

// Execute outputs logs of the service.
func (o *svcLogsOpts) Execute() error {
	if err := o.initLogsSvc(); err != nil {
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
	err := o.logsSvc.WriteLogEvents(logging.WriteLogEventsOpts{
		Follow:    o.follow,
		Limit:     limit,
		EndTime:   o.endTime,
		StartTime: o.startTime,
		TaskIDs:   o.taskIDs,
		OnEvents:  eventsWriter,
	})
	if err != nil {
		return fmt.Errorf("write log events for service %s: %w", o.name, err)
	}
	return nil
}

func (o *svcLogsOpts) askApp() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(svcAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcLogsOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, o.appName, selector.WithEnv(o.envName), selector.WithSvc(o.name))
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.appName, err)
	}
	o.name = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

func parseSince(since time.Duration) *int64 {
	sinceSec := int64(since.Round(time.Second).Seconds())
	timeNow := time.Now().Add(time.Duration(-sinceSec) * time.Second)
	return aws.Int64(timeNow.UnixMilli())
}

func parseRFC3339(timeStr string) (int64, error) {
	startTimeTmp, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return 0, fmt.Errorf("reading time value %s: %w", timeStr, err)
	}
	return startTimeTmp.UnixMilli(), nil
}

// buildSvcLogsCmd builds the command for displaying service logs in an application.
func buildSvcLogsCmd() *cobra.Command {
	vars := wkldLogsVars{}
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Displays logs of a deployed service.",

		Example: `
  Displays logs of the service "my-svc" in environment "test".
  /code $ copilot svc logs -n my-svc -e test
  Displays logs in the last hour.
  /code $ copilot svc logs --since 1h
  Displays logs from 2006-01-02T15:04:05 to 2006-01-02T15:05:05.
  /code $ copilot svc logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00
  Displays logs from specific task IDs.
  /code $ copilot svc logs --tasks 709c7eae05f947f6861b150372ddc443,1de57fd63c6a4920ac416d02add891b9
  Displays logs in real time.
  /code $ copilot svc logs --follow
  Display logs from specific log group.
  /code $ copilot svc logs --log-group system`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcLogOpts(vars)
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
	cmd.Flags().StringVar(&vars.logGroup, logGroupFlag, "", logGroupFlagDescription)
	return cmd
}
