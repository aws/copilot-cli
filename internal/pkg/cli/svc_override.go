// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/override"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type overrideSvcVars struct {
	overrideVars
	envName string // Optional.
}

type overrideSvcOpts struct {
	overrideSvcVars

	// Interfaces to interact with dependencies.
	ws         wsWlDirReader
	fs         afero.Fs
	cfgStore   store
	prompt     prompter
	wsPrompt   wsSelector
	cfnPrompt  cfnSelector
	packageCmd func(w stringWriteCloser) (executor, error)
}

func newOverrideSvcOpts(vars overrideSvcVars) (*overrideSvcOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc override"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}
	cfgStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	prompt := prompt.New()
	cmd := &overrideSvcOpts{
		overrideSvcVars: vars,
		ws:              ws,
		fs:              fs,
		cfgStore:        cfgStore,
		prompt:          prompt,
		wsPrompt:        selector.NewLocalWorkloadSelector(prompt, cfgStore, ws),
		cfnPrompt:       selector.NewCFNSelector(prompt),
	}
	cmd.packageCmd = cmd.newSvcPackageCmd
	return cmd, nil
}

// Validate returns an error for any invalid optional flags.
func (o *overrideSvcOpts) Validate() error {
	if err := o.validateAppName(); err != nil {
		return err
	}
	if err := o.validateEnvName(); err != nil {
		return err
	}
	return o.validateCDKLang()
}

// Ask prompts for and validates any required flags.
func (o *overrideSvcOpts) Ask() error {
	if err := o.validateOrAskServiceName(); err != nil {
		return err
	}
	if err := o.validateOrAskIaCTool(); err != nil {
		return err
	}
	return o.askResourcesToOverride()
}

// Execute writes IaC override files to the local workspace.
// This method assumes that the IaC tool chosen by the user is valid.
func (o *overrideSvcOpts) Execute() error {
	dir := o.ws.WorkloadOverridesPath(o.name)
	switch o.iacTool {
	case cdkIaCTool:
		if err := override.ScaffoldWithCDK(o.fs, dir, o.resources); err != nil {
			return fmt.Errorf("scaffold CDK application under %q: %v", dir, err)
		}
		log.Successf("Created a new CDK application at %q to override resources\n", displayPath(dir))
	}
	return nil
}

// RecommendActions prints optional follow-up actions.
func (o *overrideSvcOpts) RecommendActions() error {
	readmePath := filepath.Join(o.ws.WorkloadOverridesPath(o.name), "README.md")
	logRecommendedActions([]string{
		fmt.Sprintf("Please follow the instructions in %q", displayPath(readmePath)),
	})
	return nil
}

func (o *overrideSvcOpts) validateAppName() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	_, err := o.cfgStore.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %q configuration: %v", o.appName, err)
	}
	return nil
}

func (o *overrideSvcOpts) validateEnvName() error {
	if o.envName == "" {
		return nil
	}
	_, err := o.cfgStore.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %q configuration: %v", o.envName, err)
	}
	return nil
}

func (o *overrideSvcOpts) validateCDKLang() error {
	for _, valid := range validCDKLangs {
		if o.cdkLang == valid {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid CDK language: must be one of: %s",
		o.cdkLang,
		strings.Join(applyAll(validCDKLangs, strconv.Quote), ", "))
}

func (o *overrideSvcOpts) validateOrAskServiceName() error {
	if o.name == "" {
		return o.askServiceName()
	}
	return o.validateServiceName()
}

func (o *overrideSvcOpts) validateServiceName() error {
	names, err := o.ws.ListServices()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %v", err)
	}
	if !contains(o.name, names) {
		return fmt.Errorf("service %q does not exist in the workspace", o.name)
	}
	return nil
}

func (o *overrideSvcOpts) askServiceName() error {
	name, err := o.wsPrompt.Service("Which service's resources would you like to override?", "")
	if err != nil {
		return fmt.Errorf("select service name from workspace: %v", err)
	}
	o.name = name
	return nil
}

func (o *overrideSvcOpts) validateOrAskIaCTool() error {
	if o.iacTool == "" {
		return o.askIaCTool()
	}
	return o.validateIaCTool()
}

func (o *overrideSvcOpts) validateIaCTool() error {
	for _, valid := range validIaCTools {
		if o.iacTool == valid {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid IaC tool: must be one of: %s",
		o.iacTool,
		strings.Join(applyAll(validIaCTools, strconv.Quote), ", "))
}

func (o *overrideSvcOpts) askResourcesToOverride() error {
	if o.skipResources {
		return nil
	}

	buf := &closableStringBuilder{
		Builder: new(strings.Builder),
	}
	pkgCmd, err := o.packageCmd(buf)
	if err != nil {
		return err
	}
	if err := pkgCmd.Execute(); err != nil {
		return fmt.Errorf("generate CloudFormation template for service %q: %v", o.name, err)
	}
	msg := fmt.Sprintf("Which resources in %q would you like to override?", o.name)
	resources, err := o.cfnPrompt.Resources(msg, "Resources:", "", buf.String())
	if err != nil {
		return err
	}
	o.resources = resources
	return nil
}

func (o *overrideSvcOpts) newSvcPackageCmd(tplBuf stringWriteCloser) (executor, error) {
	envName, err := o.targetEnvName()
	if err != nil {
		return nil, err
	}
	cmd, err := newPackageSvcOpts(packageSvcVars{
		name:    o.name,
		envName: envName,
		appName: o.appName,
	})
	if err != nil {
		return nil, err
	}
	cmd.templateWriter = tplBuf
	return cmd, nil
}

// targetEnvName returns the name of the environment to use when running "svc package".
// If the user does not explicitly provide an environment, default to a random environment.
func (o *overrideSvcOpts) targetEnvName() (string, error) {
	if o.envName != "" {
		return o.envName, nil
	}
	envs, err := o.cfgStore.ListEnvironments(o.appName)
	if err != nil {
		return "", fmt.Errorf("list environments in application %q: %v", o.appName, err)
	}
	if len(envs) == 0 {
		return "", fmt.Errorf("no environments found in application %q", o.appName)
	}
	return envs[0].Name, nil
}

func (o *overrideSvcOpts) askIaCTool() error {
	msg := fmt.Sprintf("Which Infrastructure as Code tool would you like to use to override %q?", o.name)
	help := `The AWS Cloud Development Kit (CDK) lets you override templates using
the expressive power of programming languages.
This option is recommended for users that need to override several resources.
To learn more about the CDK: https://docs.aws.amazon.com/cdk/v2/guide/home.html`
	tool, err := o.prompt.SelectOne(msg, help, validIaCTools, prompt.WithFinalMessage("IaC tool:"))
	if err != nil {
		return fmt.Errorf("select IaC tool: %v", err)
	}
	o.iacTool = tool
	return nil
}

func buildSvcOverrideCmd() *cobra.Command {
	vars := overrideSvcVars{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "override",
		Short:  "Override the AWS CloudFormation template of a service.",
		Long: `Scaffold Infrastructure as Code (IaC) extension files. 
The generated files allow you to extend and override Copilot generated AWS CloudFormation template.
You can edit the files to change existing resource properties, delete 
or add new resources to the service's template.`,
		Example: `
  Create a new Cloud Development Kit application to override the "frontend" service template.
  /code $ copilot svc override -n frontend --tool cdk`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverrideSvcOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", overrideEnvFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	cmd.Flags().BoolVar(&vars.skipResources, skipResourcesFlag, false, skipResourcesFlagDescription)
	return cmd
}
