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
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
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
	store         store
	sel           configSelector
	initCwLogsSvc func(*svcLogsOpts, *config.Environment) error // Overriden in tests.
	cwlogsSvc     map[string]cwlogService
}

func newSvcLogOpts(vars svcLogsVars) (*svcLogsOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment config store: %w", err)
	}

	return &svcLogsOpts{
		svcLogsVars: vars,
		w:           log.OutputWriter,
		store:       store,
		sel:         selector.NewConfigSelect(vars.prompt, store),
		initCwLogsSvc: func(o *svcLogsOpts, env *config.Environment) error {
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
		_, err := o.store.GetApplication(o.AppName())
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
	var svcNames []string
	var envs []*config.Environment
	var err error
	if o.svcName == "" {
		svcNames, err = o.retrieveSvcNames()
		if err != nil {
			return err
		}
		if len(svcNames) == 0 {
			return fmt.Errorf("no services found in application %s", color.HighlightUserInput(o.AppName()))
		}
	} else {
		svc, err := o.store.GetService(o.AppName(), o.svcName)
		if err != nil {
			return fmt.Errorf("get service: %w", err)
		}
		svcNames = []string{svc.Name}
	}

	if o.envName == "" {
		envs, err = o.store.ListEnvironments(o.AppName())
		if err != nil {
			return fmt.Errorf("list environments: %w", err)
		}
		if len(envs) == 0 {
			return fmt.Errorf("no environments found in application %s", color.HighlightUserInput(o.AppName()))
		}
	} else {
		env, err := o.store.GetEnvironment(o.AppName(), o.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		envs = []*config.Environment{env}
	}

	svcEnvs := make(map[string]svcEnv)
	var svcEnvNames []string
	for _, svcName := range svcNames {
		for _, env := range envs {
			if err := o.initCwLogsSvc(o, env); err != nil {
				return err
			}
			logGroup := fmt.Sprintf(logGroupNamePattern, o.AppName(), env.Name, svcName)
			deployed, err := o.cwlogsSvc[env.Name].LogGroupExists(logGroup)
			if err != nil {
				return fmt.Errorf("check if the log group %s exists: %w", logGroup, err)
			}
			if !deployed {
				continue
			}
			svcEnv := svcEnv{
				svcName: svcName,
				envName: env.Name,
			}
			svcEnvName := svcEnv.String()
			svcEnvs[svcEnvName] = svcEnv
			svcEnvNames = append(svcEnvNames, svcEnvName)
		}
	}
	if len(svcEnvNames) == 0 {
		return fmt.Errorf("no deployed services found in application %s", color.HighlightUserInput(o.AppName()))
	}

	// return if only one deployed service found
	if len(svcEnvNames) == 1 {
		o.svcName = svcEnvs[svcEnvNames[0]].svcName
		o.envName = svcEnvs[svcEnvNames[0]].envName
		log.Infof("Showing logs of service %s deployed in environment %s\n", color.HighlightUserInput(o.svcName), color.HighlightUserInput(o.envName))
		return nil
	}

	svcEnvName, err := o.prompt.SelectOne(
		fmt.Sprintf(svcLogNamePrompt),
		svcLogNameHelpPrompt,
		svcEnvNames,
	)
	if err != nil {
		return fmt.Errorf("select deployed services for application %s: %w", o.AppName(), err)
	}
	o.svcName = svcEnvs[svcEnvName].svcName
	o.envName = svcEnvs[svcEnvName].envName

	return nil
}

func (o *svcLogsOpts) retrieveSvcNames() ([]string, error) {
	svcs, err := o.store.ListServices(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list services for application %s: %w", o.AppName(), err)
	}
	svcNames := make([]string, len(svcs))
	for ind, svc := range svcs {
		svcNames[ind] = svc.Name
	}

	return svcNames, nil
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
