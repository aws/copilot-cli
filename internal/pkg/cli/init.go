// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the copilot commands.
package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	defaultEnvironmentName    = "test"
	defaultEnvironmentProfile = "default"
)

const (
	initShouldDeployPrompt     = "Would you like to deploy a test environment?"
	initShouldDeployHelpPrompt = "An environment with your service deployed to it. This will allow you to test your service before placing it in production."
)

type initVars struct {
	// Flags unique to "init" that's not provided by other sub-commands.
	shouldDeploy   bool
	appName        string
	svcType        string
	svcName        string
	dockerfilePath string
	imageTag       string
	port           uint16
}

type initOpts struct {
	ShouldDeploy          bool // true means we should create a test environment and deploy the service to it. Defaults to false.
	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.

	// Sub-commands to execute.
	initAppCmd   actionCommand
	initSvcCmd   actionCommand
	initEnvCmd   actionCommand
	deploySvcCmd actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	appName        *string
	svcType        *string
	svcName        *string
	svcPort        *uint16
	dockerfilePath *string

	prompt prompter
}

func newInitOpts(vars initVars) (*initOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	ssm, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	sessProvider := sessions.NewProvider()
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	prompt := prompt.New()
	sel := selector.NewWorkspaceSelect(prompt, ssm, ws)
	spin := termprogress.NewSpinner()
	id := identity.New(defaultSess)
	deployer := cloudformation.New(defaultSess)
	if err != nil {
		return nil, err
	}

	initAppCmd := &initAppOpts{
		initAppVars: initAppVars{
			name: vars.appName,
		},
		store:    ssm,
		ws:       ws,
		prompt:   prompt,
		identity: id,
		cfn:      deployer,
		prog:     spin,
	}
	wkldInitter := initialize.NewWorkloadInitializer(
		ssm,
		ws,
		spin,
		deployer,
	)
	initSvcCmd := &initSvcOpts{
		initWkldVars: initWkldVars{
			wkldType:       vars.svcType,
			name:           vars.svcName,
			dockerfilePath: vars.dockerfilePath,
			port:           vars.port,
			appName:        vars.appName,
		},
		fs: &afero.Afero{Fs: afero.NewOsFs()},

		init:   wkldInitter,
		sel:    sel,
		prompt: prompt,
		setupParser: func(o *initSvcOpts) {
			o.df = dockerfile.New(o.fs, o.dockerfilePath)
		},
	}
	initEnvCmd := &initEnvOpts{
		initEnvVars: initEnvVars{
			appName:      vars.appName,
			name:         defaultEnvironmentName,
			isProduction: false,
		},
		store:       ssm,
		appDeployer: deployer,
		prog:        spin,
		prompt:      prompt,
		identity:    id,

		sess: defaultSess,
	}

	deploySvcCmd := &deploySvcOpts{
		deploySvcVars: deploySvcVars{
			envName:  defaultEnvironmentName,
			imageTag: vars.imageTag,
			appName:  vars.appName,
		},

		store:        ssm,
		prompt:       prompt,
		ws:           ws,
		unmarshal:    manifest.UnmarshalWorkload,
		sel:          sel,
		spinner:      spin,
		cmd:          command.New(),
		sessProvider: sessProvider,
	}

	return &initOpts{
		ShouldDeploy: vars.shouldDeploy,

		initAppCmd:   initAppCmd,
		initSvcCmd:   initSvcCmd,
		initEnvCmd:   initEnvCmd,
		deploySvcCmd: deploySvcCmd,

		appName:        &initAppCmd.name,
		svcType:        &initSvcCmd.wkldType,
		svcName:        &initSvcCmd.name,
		svcPort:        &initSvcCmd.port,
		dockerfilePath: &initSvcCmd.dockerfilePath,

		prompt: prompt,
	}, nil
}

// Run executes "app init", "env init", "svc init" and "svc deploy".
func (o *initOpts) Run() error {
	if !workspace.IsInGitRepository(afero.NewOsFs()) {
		log.Warningln("It's best to run this command in the root of your Git repository.")
	}
	log.Infoln(color.Help(`Welcome to the Copilot CLI! We're going to walk you through some questions
to help you get set up with an application on ECS. An application is a collection of
containerized services that operate together.`))
	log.Infoln()

	if err := o.loadApp(); err != nil {
		return err
	}
	if err := o.loadSvc(); err != nil {
		return err
	}

	log.Infof("Ok great, we'll set up a %s named %s in application %s listening on port %s.\n",
		color.HighlightUserInput(*o.svcType), color.HighlightUserInput(*o.svcName), color.HighlightUserInput(*o.appName), color.HighlightUserInput(fmt.Sprintf("%d", *o.svcPort)))
	log.Infoln()
	if err := o.initAppCmd.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}
	if err := o.initSvcCmd.Execute(); err != nil {
		return fmt.Errorf("execute svc init: %w", err)
	}

	if err := o.deployEnv(); err != nil {
		return err
	}

	return o.deploySvc()
}

func (o *initOpts) loadApp() error {
	if err := o.initAppCmd.Ask(); err != nil {
		return fmt.Errorf("ask app init: %w", err)
	}
	if err := o.initAppCmd.Validate(); err != nil {
		return err
	}
	return nil
}

func (o *initOpts) loadSvc() error {
	if initSvcOpts, ok := o.initSvcCmd.(*initSvcOpts); ok {
		// Set the application name from app init to the service init command.
		initSvcOpts.appName = *o.appName
	}

	if err := o.initSvcCmd.Ask(); err != nil {
		return fmt.Errorf("ask svc init: %w", err)
	}
	return o.initSvcCmd.Validate()
}

// deployEnv prompts the user to deploy a test environment if the application doesn't already have one.
func (o *initOpts) deployEnv() error {
	if o.promptForShouldDeploy {
		log.Infoln("All right, you're all set for local development.")
		if err := o.askShouldDeploy(); err != nil {
			return err
		}
	}
	if !o.ShouldDeploy {
		// User chose not to deploy the service, exit.
		return nil
	}
	if initEnvCmd, ok := o.initEnvCmd.(*initEnvOpts); ok {
		// Set the application name from app init to the env init command.
		initEnvCmd.appName = *o.appName
	}

	log.Infoln()
	return o.initEnvCmd.Execute()
}

func (o *initOpts) deploySvc() error {
	if !o.ShouldDeploy {
		return nil
	}
	if deployOpts, ok := o.deploySvcCmd.(*deploySvcOpts); ok {
		// Set the service's name and app name to the deploy sub-command.
		deployOpts.name = *o.svcName
		deployOpts.appName = *o.appName
	}

	if err := o.deploySvcCmd.Ask(); err != nil {
		return err
	}
	return o.deploySvcCmd.Execute()
}

func (o *initOpts) askShouldDeploy() error {
	v, err := o.prompt.Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, prompt.WithFinalMessage("Deploy:"))
	if err != nil {
		return fmt.Errorf("failed to confirm deployment: %w", err)
	}
	o.ShouldDeploy = v
	return nil
}

// BuildInitCmd builds the command for bootstrapping an application.
func BuildInitCmd() *cobra.Command {
	vars := initVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application.",
		Long:  "Create a new ECS application.",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitOpts(vars)
			if err != nil {
				return err
			}
			opts.promptForShouldDeploy = !cmd.Flags().Changed(deployFlag)
			if err := opts.Run(); err != nil {
				return err
			}
			if !opts.ShouldDeploy {
				log.Info("\nNo problem, you can deploy your service later:\n")
				log.Infof("- Run %s to create your staging environment.\n",
					color.HighlightCode(fmt.Sprintf("copilot env init --name %s --profile %s --app %s", defaultEnvironmentName, defaultEnvironmentProfile, *opts.appName)))
				for _, followup := range opts.initSvcCmd.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.svcName, svcFlag, svcFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.svcType, svcTypeFlag, svcTypeFlagShort, "", svcTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldDeploy, deployFlag, false, deployTestFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().Uint16Var(&vars.port, svcPortFlag, 0, svcPortFlagDescription)
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
