// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcLogAppNamePrompt     = "Which application does your service belong to?"
	svcLogAppNameHelpPrompt = "An application groups all of your services together."
	svcLogNamePrompt        = "Which service's logs would you like to show?"
	svcLogNameHelpPrompt    = "The logs of a deployed service will be shown."

	logGroupNamePattern    = "/copilot/%s-%s-%s"
	cwGetLogEventsLimitMin = 1
	cwGetLogEventsLimitMax = 10000
)

type svcLogsVars struct {
	shouldOutputJSON bool
	follow           bool
	limit            int
	svcName          string
	envName          string
	humanStartTime   string
	humanEndTime     string
	since            time.Duration
	*GlobalOpts
}

type svcLogsOpts struct {
	svcLogsVars

	// internal states
	startTime int64
	endTime   int64

	w             io.Writer
	configStore   store
	deployStore   deployedEnvironmentLister
	sel           deploySelector
	initCwLogsSvc func(*svcLogsOpts, string) error // Overriden in tests.
	cwlogsSvc     map[string]cwlogService
}

func newSvcLogOpts(vars svcLogsVars) (*svcLogsOpts, error) {
	configStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment config store: %w", err)
	}
	deployStore, err := deploy.NewStore(configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	return &svcLogsOpts{
		svcLogsVars: vars,
		w:           log.OutputWriter,
		configStore: configStore,
		deployStore: deployStore,
		sel:         selector.NewDeploySelect(vars.prompt, configStore, deployStore),
		initCwLogsSvc: func(o *svcLogsOpts, envName string) error {
			env, err := o.configStore.GetEnvironment(o.AppName(), envName)
			if err != nil {
				return fmt.Errorf("get environment: %w", err)
			}
			sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
			if err != nil {
				return err
			}
			o.cwlogsSvc[env.Name] = cloudwatchlogs.New(sess)
			return nil
		},
		cwlogsSvc: make(map[string]cwlogService),
	}, nil
}

// Validate returns an error if the values provided by flags are invalid.
func (o *svcLogsOpts) Validate() error {
	if o.AppName() != "" {
		_, err := o.configStore.GetApplication(o.AppName())
		if err != nil {
			return err
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
		o.startTime = o.parseSince()
	}

	if o.humanStartTime != "" {
		startTime, err := o.parseRFC3339(o.humanStartTime)
		if err != nil {
			return fmt.Errorf(`invalid argument %s for "--start-time" flag: %w`, o.humanStartTime, err)
		}
		o.startTime = startTime
	}

	if o.humanEndTime != "" {
		endTime, err := o.parseRFC3339(o.humanEndTime)
		if err != nil {
			return fmt.Errorf(`invalid argument %s for "--end-time" flag: %w`, o.humanEndTime, err)
		}
		o.endTime = endTime
	}

	if o.limit < cwGetLogEventsLimitMin || o.limit > cwGetLogEventsLimitMax {
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
	logGroupName := fmt.Sprintf(logGroupNamePattern, o.AppName(), o.envName, o.svcName)
	logEventsOutput := &cloudwatchlogs.LogEventsOutput{
		LastEventTime: make(map[string]int64),
	}
	if err := o.initCwLogsSvc(o, o.envName); err != nil {
		return err
	}
	var err error
	for {
		logEventsOutput, err = o.cwlogsSvc[o.envName].TaskLogEvents(logGroupName, logEventsOutput.LastEventTime, o.generateGetLogEventOpts()...)
		if err != nil {
			return err
		}
		if err := o.outputLogs(logEventsOutput.Events); err != nil {
			return err
		}
		if !o.follow {
			return nil
		}
		// for unit test.
		if logEventsOutput.LastEventTime == nil {
			return nil
		}
		time.Sleep(cloudwatchlogs.SleepDuration)
	}
}

func (o *svcLogsOpts) askApp() error {
	if o.AppName() != "" {
		return nil
	}
	app, err := o.sel.Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcLogsOpts) generateGetLogEventOpts() []cloudwatchlogs.GetLogEventsOpts {
	opts := []cloudwatchlogs.GetLogEventsOpts{
		cloudwatchlogs.WithLimit(o.limit),
	}
	if o.startTime != 0 {
		opts = append(opts, cloudwatchlogs.WithStartTime(o.startTime))
	}
	if o.endTime != 0 {
		opts = append(opts, cloudwatchlogs.WithEndTime(o.endTime))
	}
	return opts
}

func (o *svcLogsOpts) askSvcEnvName() error {
	deployedService, err := o.sel.DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, o.AppName(), selector.WithEnv(o.envName), selector.WithSvc(o.svcName))
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.AppName(), err)
	}
	o.svcName = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

func (o *svcLogsOpts) outputLogs(logs []*cloudwatchlogs.Event) error {
	if !o.shouldOutputJSON {
		for _, log := range logs {
			fmt.Fprintf(o.w, log.HumanString())
		}
		return nil
	}
	for _, log := range logs {
		data, err := log.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	}
	return nil
}

func (o *svcLogsOpts) parseSince() int64 {
	sinceSec := int64(o.since.Round(time.Second).Seconds())
	timeNow := time.Now().Add(time.Duration(-sinceSec) * time.Second)
	return timeNow.Unix() * 1000
}

func (o *svcLogsOpts) parseRFC3339(timeStr string) (int64, error) {
	startTimeTmp, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return 0, fmt.Errorf("reading time value %s: %w", timeStr, err)
	}
	return startTimeTmp.Unix() * 1000, nil
}

// BuildSvcLogsCmd builds the command for displaying service logs in an application.
func BuildSvcLogsCmd() *cobra.Command {
	vars := svcLogsVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Displays logs of a deployed service.",

		Example: `
  Displays logs of the service "my-svc" in environment "test".
  /code $ copilot svc logs -n my-svc -e test
  Displays logs in the last hour.
  /code $ copilot svc logs --since 1h
  Displays logs from 2006-01-02T15:04:05 to 2006-01-02T15:05:05.
  /code $ copilot svc logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcLogOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.humanStartTime, startTimeFlag, "", startTimeFlagDescription)
	cmd.Flags().StringVar(&vars.humanEndTime, endTimeFlag, "", endTimeFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	cmd.Flags().DurationVar(&vars.since, sinceFlag, 0, sinceFlagDescription)
	cmd.Flags().IntVar(&vars.limit, limitFlag, 10, limitFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	return cmd
}
