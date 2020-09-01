// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	fmtJobInitNameHelpPrompt = `The name will uniquely identify this job within your app %s.
Deployed resources (such as your job, logs) will contain this job's name and be tagged with it.`

	jobInitDockerfileHelpPrompt = "Dockerfile to use for building your job's container image."
)

// const (
// 	fmtAddJobToAppStart    = "Creating ECR repositories for job %s."
// 	fmtAddJobToAppFailed   = "Failed to create ECR repositories for job %s.\n"
// 	fmtAddJobToAppComplete = "Created ECR repositories for job %s.\n"
// )

type initJobVars struct {
	*GlobalOpts
	Name           string
	DockerfilePath string
	Timeout        string
	Retries        uint16
	Schedule       string
}

type initJobOpts struct {
	initJobVars

	// Interfaces to interact with dependencies.
	fs          afero.Fs
	ws          svcManifestWriter
	store       store
	appDeployer appDeployer
	prog        progress

	// Outputs stored on successful actions.
	manifestPath string
}

func newInitJobOpts(vars initJobVars) (*initJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, err
	}

	return &initJobOpts{
		initJobVars: vars,

		fs:          &afero.Afero{Fs: afero.NewOsFs()},
		store:       store,
		ws:          ws,
		appDeployer: cloudformation.New(sess),
		prog:        termprogress.NewSpinner(),
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initJobOpts) Validate() error {
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initJobOpts) Ask() error {
	return nil
}

// Execute writes the job's manifest file and stores the name in SSM.
func (o *initJobOpts) Execute() error {
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initJobOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your job to a %s environment.",
			color.HighlightCode(fmt.Sprintf("copilot job deploy --name %s --env %s", o.Name, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
}

// BuildJobInitCmd builds the command for creating a new job.
func BuildJobInitCmd() *cobra.Command {
	vars := initJobVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new scheduled job in an application.",
		Example: `
  Create a "reaper" scheduled task to run once per day.
  /code $ copilot job init --name reaper --dockerfile ./frontend/Dockerfile --rate "1 day"

  Create a "report-generator" scheduled task with retries.
  /code $ copilot job init --name report-generator --rate "@monthly" --retries 3 --timeout 900s`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitJobOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.DockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVar(&vars.Timeout, timeoutFlag, "", timeoutFlagDescription)
	cmd.Flags().Uint16Var(&vars.Retries, retriesFlag, 0, retriesFlagDescription)
	cmd.Flags().StringVarP(&vars.Schedule, scheduleFlag, scheduleFlagShort, "", scheduleFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required Flags", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(dockerFileFlag))

	return cmd
}
