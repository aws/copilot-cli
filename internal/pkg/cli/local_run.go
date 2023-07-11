// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type localRunVars struct {
	wkldName string
	appName  string
	envName  string
}

type localRunOpts struct {
	localRunVars

	store                store
	ws                   wsWlDirReader
	prompt               prompter
	deployStore          deployedEnvironmentLister
	envFeaturesDescriber func(string) (versionCompatibilityChecker, error)
}

func newLocalRunOpts(vars localRunVars) (*localRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("local run"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, store)
	if err != nil {
		return nil, err
	}

	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	prompter := prompt.New()
	opts := &localRunOpts{
		localRunVars: vars,

		prompt:      prompter,
		store:       store,
		ws:          ws,
		deployStore: deployStore,
	}
	opts.envFeaturesDescriber = func(envName string) (versionCompatibilityChecker, error) {
		envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         opts.appName,
			Env:         envName,
			ConfigStore: opts.store,
		})
		if err != nil {
			return nil, fmt.Errorf("new environment compatibility checker: %v", err)
		}
		return envDescriber, nil
	}
	return opts, nil
}

func (o *localRunOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}
	return nil
}

func (o *localRunOpts) Ask() error {
	if err := o.validateOrAskEnvName(); err != nil {
		return err
	}
	if err := o.validateOrAskWorkloadName(); err != nil {
		return err
	}
	return nil
}

func (o *localRunOpts) Execute() error {
	//TODO(varun359): Get build information from the manifest and task definition for workloads

	return nil
}

func (o *localRunOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		return o.validateEnvName()
	}
	envs, err := o.getDeployedEnvironments(o.appName)
	if err != nil {
		return err
	}
	if len(envs) == 0 {
		return fmt.Errorf("no deployed environments found in the app %s", o.appName)
	}
	if len(envs) == 1 {
		log.Infof("Only one environment found, defaulting to: %s\n", color.HighlightUserInput(envs[0]))
		o.envName = envs[0]
		return nil
	}
	selectedEnvName, err := o.prompt.SelectOne("Select an environment in which you want to test", "", envs, prompt.WithFinalMessage("Environment:"))
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = selectedEnvName
	return nil
}

func (o *localRunOpts) getDeployedEnvironments(appName string) ([]string, error) {
	envConfig, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get environments for the app %s: %w", appName, err)
	}

	var envs []string
	for _, env := range envConfig {
		envs = append(envs, env.Name)
	}

	var deployedEnvs []string
	for _, env := range envs {
		isDeployed, err := o.isEnvironmentDeployed(env)
		if err != nil {
			return nil, err
		}
		if isDeployed {
			deployedEnvs = append(deployedEnvs, env)
		}
	}
	return deployedEnvs, nil
}

func (o *localRunOpts) isEnvironmentDeployed(envName string) (bool, error) {
	var checker versionCompatibilityChecker

	envDescriber, err := o.envFeaturesDescriber(envName)
	if err != nil {
		return false, err
	}
	checker = envDescriber

	currVersion, err := checker.Version()
	if err != nil {
		return false, fmt.Errorf("get environment %q version: %w", envName, err)
	}
	if currVersion == deploy.EnvTemplateVersionBootstrap {
		return false, nil
	}
	return true, nil
}
func (o *localRunOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return fmt.Errorf("get environment %s: %w", o.envName, err)
	}
	isDeployed, err := o.isEnvironmentDeployed(o.envName)
	if err != nil {
		return err
	}
	if !isDeployed {
		return fmt.Errorf(`cannot use an environment which is not deployed Please run "copilot env deploy, --name %s" to deploy the environment first`, o.envName)
	}
	return nil
}

func (o *localRunOpts) validateOrAskWorkloadName() error {
	if o.wkldName != "" {
		return o.validateWkldName()
	}
	services, err := o.deployStore.ListDeployedServices(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("Get services: %w", err)
	}

	jobs, err := o.deployStore.ListDeployedJobs(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("Get jobs: %w", err)
	}

	workloads := append(services, jobs...)
	if len(workloads) == 0 {
		return fmt.Errorf("no workloads found in this environment %s", o.envName)
	}
	if len(workloads) == 1 {
		log.Infof("Only one workload found in this environment, defaulting to: %s\n", color.HighlightUserInput(workloads[0]))
		o.wkldName = workloads[0]
		return nil
	}

	selectedWorloadName, err := o.prompt.SelectOne("Select a workload that you want to run locally", "", workloads, prompt.WithFinalMessage("workload name"))
	if err != nil {
		return fmt.Errorf("select Workload: %w", err)
	}
	o.wkldName = selectedWorloadName
	return nil
}

func (o *localRunOpts) validateWkldName() error {
	names, err := o.ws.ListWorkloads()
	if err != nil {
		return fmt.Errorf("list workloads in the workspace %s : %w", o.wkldName, err)
	}
	if !contains(o.wkldName, names) {
		return fmt.Errorf("service %q does not exist in the workspace", o.wkldName)
	}

	if _, err := o.store.GetWorkload(o.appName, o.wkldName); err != nil {
		return fmt.Errorf("Workload %q does not exist in smm", o.wkldName)
	}
	return nil
}

// BuildLocalRunCmd builds the command for running a workload locally
func BuildLocalRunCmd() *cobra.Command {
	vars := localRunVars{}
	cmd := &cobra.Command{
		Use:    "local run",
		Short:  "Run the workload locally",
		Long:   "Run the workload locally for debugging in a simulated AWS environment",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newLocalRunOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
