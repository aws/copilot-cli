// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	svcStatusAppNamePrompt     = "Which application is the service in?"
	svcStatusAppNameHelpPrompt = "An application groups all of your services together."
	svcStatusNamePrompt        = "Which service's status would you like to show?"
	svcStatusNameHelpPrompt    = "Displays the service's task status, most recent deployment and alarm statuses."
)

type svcStatusVars struct {
	*GlobalOpts
	shouldOutputJSON bool
	svcName          string
	envName          string
}

type svcStatusOpts struct {
	svcStatusVars

	w                   io.Writer
	store               store
	svcDescriber        serviceArnGetter
	statusDescriber     statusDescriber
	sel                 configSelector
	initSvcDescriber    func(*svcStatusOpts, string, string) error
	initStatusDescriber func(*svcStatusOpts) error
}

func newSvcStatusOpts(vars svcStatusVars) (*svcStatusOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}

	return &svcStatusOpts{
		svcStatusVars: vars,
		store:         store,
		w:             log.OutputWriter,
		sel:           selector.NewConfigSelect(vars.prompt, store),
		initSvcDescriber: func(o *svcStatusOpts, envName, svcName string) error {
			d, err := describe.NewServiceDescriber(o.AppName(), envName, svcName)
			if err != nil {
				return fmt.Errorf("creating service describer for application %s, environment %s, and service %s: %w", o.AppName(), envName, svcName, err)
			}
			o.svcDescriber = d
			return nil
		},
		initStatusDescriber: func(o *svcStatusOpts) error {
			d, err := describe.NewServiceStatus(o.AppName(), o.envName, o.svcName)
			if err != nil {
				return fmt.Errorf("creating status describer for service %s in application %s: %w", o.svcName, o.AppName(), err)
			}
			o.statusDescriber = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *svcStatusOpts) Validate() error {
	if o.AppName() != "" {
		if _, err := o.store.GetApplication(o.AppName()); err != nil {
			return err
		}
	}
	if o.svcName != "" {
		if _, err := o.store.GetService(o.AppName(), o.svcName); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.AppName(), o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *svcStatusOpts) Ask() error {
	if err := o.askApp(); err != nil {
		return err
	}
	return o.askSvcEnvName()
}

// Execute displays the status of the service.
func (o *svcStatusOpts) Execute() error {
	err := o.initStatusDescriber(o)
	if err != nil {
		return err
	}
	svcStatus, err := o.statusDescriber.Describe()
	if err != nil {
		return fmt.Errorf("describe status of service %s: %w", o.svcName, err)
	}
	if o.shouldOutputJSON {
		data, err := svcStatus.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, svcStatus.HumanString())
	}

	return nil
}

func (o *svcStatusOpts) askApp() error {
	if o.AppName() != "" {
		return nil
	}
	app, err := o.sel.Application(svcStatusAppNamePrompt, svcStatusAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcStatusOpts) askSvcEnvName() error {
	var err error
	svcNames := []string{o.svcName}
	if o.svcName == "" {
		svcNames, err = o.retrieveSvcNames()
		if err != nil {
			return err
		}
		if len(svcNames) == 0 {
			return fmt.Errorf("no services found in application %s", color.HighlightUserInput(o.AppName()))
		}
	}

	envNames := []string{o.envName}
	if o.envName == "" {
		envNames, err = o.retrieveEnvNames()
		if err != nil {
			return err
		}
		if len(envNames) == 0 {
			return fmt.Errorf("no environments found in application %s", color.HighlightUserInput(o.AppName()))
		}
	}

	svcEnvs := make(map[string]svcEnv)
	var svcEnvNames []string
	for _, svcName := range svcNames {
		for _, envName := range envNames {
			if err := o.initSvcDescriber(o, envName, svcName); err != nil {
				return err
			}
			_, err := o.svcDescriber.GetServiceArn()
			if err != nil {
				if describe.IsStackNotExistsErr(err) {
					continue
				}
				return fmt.Errorf("check if service %s is deployed in env %s: %w", svcName, envName, err)
			}
			svcEnv := svcEnv{
				svcName: svcName,
				envName: envName,
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
		log.Infof("Showing status of service %s deployed in environment %s\n", color.HighlightUserInput(o.svcName), color.HighlightUserInput(o.envName))
		return nil
	}

	svcEnvName, err := o.prompt.SelectOne(
		svcLogNamePrompt,
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

func (o *svcStatusOpts) retrieveSvcNames() ([]string, error) {
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

func (o *svcStatusOpts) retrieveEnvNames() ([]string, error) {
	envs, err := o.store.ListEnvironments(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list environments for application %s: %w", o.AppName(), err)
	}
	envNames := make([]string, len(envs))
	for ind, env := range envs {
		envNames[ind] = env.Name
	}

	return envNames, nil
}

// BuildSvcStatusCmd builds the command for showing the status of a deployed service.
func BuildSvcStatusCmd() *cobra.Command {
	vars := svcStatusVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Shows status of a deployed service.",
		Long:  "Shows status of a deployed service's task status, most recent deployment and alarm statuses.",

		Example: `
  Shows status of the deployed service "my-svc"
  /code $ copilot svc status -n my-svc`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcStatusOpts(vars)
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
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	return cmd
}
