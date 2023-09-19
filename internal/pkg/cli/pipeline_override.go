// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type overridePipelineOpts struct {
	*overrideOpts

	// Interfaces to interact with dependencies.
	ws       wsPipelineReader
	wsPrompt wsPipelineSelector
}

func newOverridePipelineOpts(vars overrideVars) (*overridePipelineOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline override"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	prompt := prompt.New()

	cmd := &overridePipelineOpts{
		overrideOpts: &overrideOpts{
			overrideVars: vars,
			fs:           fs,
			cfgStore:     config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region)),
			prompt:       prompt,
			cfnPrompt:    selector.NewCFNSelector(prompt),
			spinner:      termprogress.NewSpinner(log.DiagnosticWriter),
		},
		ws:       ws,
		wsPrompt: selector.NewWsPipelineSelector(prompt, ws),
	}
	cmd.overrideOpts.packageCmd = cmd.newPipelinePackageCmd
	return cmd, nil
}

// Validate returns an error for any invalid optional flags.
func (o *overridePipelineOpts) Validate() error {
	if err := o.overrideOpts.Validate(); err != nil {
		return err
	}
	return o.validatePipelineName()
}

// Ask prompts for and validates any required flags.
func (o *overridePipelineOpts) Ask() error {
	if o.name == "" {
		if err := o.askPipelineName(); err != nil {
			return err
		}
	}
	return o.overrideOpts.Ask()
}

// Execute writes IaC override files to the local workspace.
func (o *overridePipelineOpts) Execute() error {
	o.overrideOpts.dir = func() string {
		return o.ws.PipelineOverridesPath(o.name)
	}
	return o.overrideOpts.Execute()

}

func (o *overridePipelineOpts) validatePipelineName() error {
	if o.name == "" {
		return nil
	}
	pipelines, err := o.ws.ListPipelines()
	if err != nil {
		return fmt.Errorf("list pipelines in the workspace: %v", err)
	}
	var names []string
	for _, pipeline := range pipelines {
		names = append(names, pipeline.Name)
	}
	if !slices.Contains(names, o.name) {
		return fmt.Errorf("pipeline %q does not exist in the workspace", o.name)
	}
	return nil
}

func (o *overridePipelineOpts) askPipelineName() error {
	pipeline, err := o.wsPrompt.WsPipeline("Which pipeline's resources would you like to override?", "")
	if err != nil {
		return fmt.Errorf("select pipeline name from workspace: %v", err)
	}
	o.name = pipeline.Name
	return nil
}

func (o *overridePipelineOpts) newPipelinePackageCmd(tplBuf stringWriteCloser) (executor, error) {
	cmd, err := newPackagePipelineOpts(packagePipelineVars{
		name:    o.name,
		appName: o.appName,
	})
	if err != nil {
		return nil, err
	}
	cmd.tmplWriter = tplBuf
	return cmd, nil
}

func buildPipelineOverrideCmd() *cobra.Command {
	vars := overrideVars{}
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Override the AWS CloudFormation template of a Pipeline.",
		Long: `Scaffold Infrastructure as Code patch files. 
Customize the patch files to change resource properties, delete 
or add new resources to the Pipeline's AWS CloudFormation template.`,
		Example: `
  Create a new Cloud Development Kit application to override the "myrepo-main" pipeline template.
  /code $ copilot pipeline override -n myrepo-main --toolkit cdk`,

		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverridePipelineOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	cmd.Flags().BoolVar(&vars.skipResources, skipResourcesFlag, false, skipResourcesFlagDescription)
	return cmd
}
