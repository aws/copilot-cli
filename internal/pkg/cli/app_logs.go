// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	applicationLogProjectNamePrompt     = "Which project does your application belong to?"
	applicationLogProjectNameHelpPrompt = "A project groups all of your applications together."
	applicationLogAppNamePrompt         = "Which application's logs would you like to show?"
	applicationLogAppNameHelpPrompt     = "The logs of a deployed application will be shown."

	logGroupNamePattern    = "/ecs/%s-%s-%s"
	cwGetLogEventsLimitMin = 1
	cwGetLogEventsLimitMax = 10000
)

type appLogsVars struct {
	shouldOutputJSON bool
	follow           bool
	limit            int
	appName          string
	envName          string
	humanStartTime   string
	humanEndTime     string
	since            time.Duration
	*GlobalOpts
}

type appLogsOpts struct {
	appLogsVars

	// internal states
	startTime int64
	endTime   int64

	w             io.Writer
	storeSvc      storeReader
	initCwLogsSvc func(*appLogsOpts, *archer.Environment) error // Overriden in tests.
	cwlogsSvc     map[string]cwlogService
}

func newAppLogOpts(vars appLogsVars) (*appLogsOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}

	return &appLogsOpts{
		appLogsVars: vars,
		w:           log.OutputWriter,
		storeSvc:    ssmStore,
		initCwLogsSvc: func(o *appLogsOpts, env *archer.Environment) error {
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

// Validate returns an error if the values provided by the user are invalid.
func (o *appLogsOpts) Validate() error {
	if o.ProjectName() != "" {
		_, err := o.storeSvc.GetApplication(o.ProjectName())
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
func (o *appLogsOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppEnvName()
}

// Execute shows the applications through the prompt.
func (o *appLogsOpts) Execute() error {
	logGroupName := fmt.Sprintf(logGroupNamePattern, o.ProjectName(), o.envName, o.appName)
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

func (o *appLogsOpts) askProject() error {
	if o.ProjectName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjectNames()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		return fmt.Errorf("no project found: run %s please", color.HighlightCode("project init"))
	}
	proj, err := o.prompt.SelectOne(
		applicationLogProjectNamePrompt,
		applicationLogProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("select projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *appLogsOpts) generateGetLogEventOpts() []cloudwatchlogs.GetLogEventsOpts {
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

func (o *appLogsOpts) askAppEnvName() error {
	var appNames []string
	var envs []*archer.Environment
	var err error
	if o.appName == "" {
		appNames, err = o.retrieveAllAppNames()
		if err != nil {
			return err
		}
		if len(appNames) == 0 {
			return fmt.Errorf("no applications found in project %s", color.HighlightUserInput(o.ProjectName()))
		}
	} else {
		app, err := o.storeSvc.GetService(o.ProjectName(), o.appName)
		if err != nil {
			return fmt.Errorf("get application: %w", err)
		}
		appNames = []string{app.Name}
	}

	if o.envName == "" {
		envs, err = o.storeSvc.ListEnvironments(o.ProjectName())
		if err != nil {
			return fmt.Errorf("list environments: %w", err)
		}
		if len(envs) == 0 {
			return fmt.Errorf("no environments found in project %s", color.HighlightUserInput(o.ProjectName()))
		}
	} else {
		env, err := o.storeSvc.GetEnvironment(o.ProjectName(), o.envName)
		if err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
		envs = []*archer.Environment{env}
	}

	appEnvs := make(map[string]appEnv)
	var appEnvNames []string
	for _, appName := range appNames {
		for _, env := range envs {
			if err := o.initCwLogsSvc(o, env); err != nil {
				return err
			}
			deployed, err := o.cwlogsSvc[env.Name].LogGroupExists(fmt.Sprintf(logGroupNamePattern, o.ProjectName(), env.Name, appName))
			if err != nil {
				return fmt.Errorf("check if the log group exists: %w", err)
			}
			if !deployed {
				continue
			}
			appEnv := appEnv{
				appName: appName,
				envName: env.Name,
			}
			appEnvName := appEnv.String()
			appEnvs[appEnvName] = appEnv
			appEnvNames = append(appEnvNames, appEnvName)
		}
	}
	if len(appEnvNames) == 0 {
		return fmt.Errorf("no deployed applications found in project %s", color.HighlightUserInput(o.ProjectName()))
	}

	// return if only one deployed app found
	if len(appEnvNames) == 1 {
		log.Infof("Only found one deployed app, defaulting to: %s\n", color.HighlightUserInput(appEnvNames[0]))
		o.appName = appEnvs[appEnvNames[0]].appName
		o.envName = appEnvs[appEnvNames[0]].envName

		return nil
	}

	appEnvName, err := o.prompt.SelectOne(
		fmt.Sprintf(applicationLogAppNamePrompt),
		applicationLogAppNameHelpPrompt,
		appEnvNames,
	)
	if err != nil {
		return fmt.Errorf("select deployed applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appEnvs[appEnvName].appName
	o.envName = appEnvs[appEnvName].envName

	return nil
}

func (o *appLogsOpts) retrieveProjectNames() ([]string, error) {
	projs, err := o.storeSvc.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *appLogsOpts) retrieveAllAppNames() ([]string, error) {
	apps, err := o.storeSvc.ListServices(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}

	return appNames, nil
}

func (o *appLogsOpts) outputLogs(logs []*cloudwatchlogs.Event) error {
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

func (o *appLogsOpts) parseSince() int64 {
	sinceSec := int64(o.since.Round(time.Second).Seconds())
	timeNow := time.Now().Add(time.Duration(-sinceSec) * time.Second)
	return timeNow.Unix() * 1000
}

func (o *appLogsOpts) parseRFC3339(timeStr string) (int64, error) {
	startTimeTmp, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return 0, fmt.Errorf("reading time value %s: %w", timeStr, err)
	}
	return startTimeTmp.Unix() * 1000, nil
}

// BuildAppLogsCmd builds the command for displaying application logs in a project.
func BuildAppLogsCmd() *cobra.Command {
	vars := appLogsVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Displays logs of a deployed application.",

		Example: `
  Displays logs of the application "my-app" in environment "test"
	/code $ ecs-preview app logs -n my-app -e test
  Displays logs in the last hour
	/code $ ecs-preview app logs --since 1h
  Displays logs from 2006-01-02T15:04:05 to 2006-01-02T15:05:05
	/code $ ecs-preview app logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newAppLogOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.humanStartTime, startTimeFlag, "", startTimeFlagDescription)
	cmd.Flags().StringVar(&vars.humanEndTime, endTimeFlag, "", endTimeFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.follow, followFlag, false, followFlagDescription)
	cmd.Flags().DurationVar(&vars.since, sinceFlag, 0, sinceFlagDescription)
	cmd.Flags().IntVar(&vars.limit, limitFlag, 10, limitFlagDescription)
	cmd.Flags().StringVarP(&vars.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	return cmd
}
