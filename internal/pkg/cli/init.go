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
	wkldType       string
	svcName        string
	dockerfilePath string
	image          string
	imageTag       string
	port           uint16

	schedule string
	retries  int
	timeout  string
}

type initOpts struct {
	ShouldDeploy          bool // true means we should create a test environment and deploy the service to it. Defaults to false.
	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.

	// Sub-commands to execute.
	initAppCmd   actionCommand
	initSvcCmd   actionCommand
	initJobCmd   actionCommand
	initEnvCmd   actionCommand
	deploySvcCmd actionCommand
	deployJobCmd actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	appName           *string
	svcType           *string
	jobType           *string
	svcName           *string
	jobName           *string
	svcDockerfilePath *string
	svcImage          *string
	svcPort           *uint16

	wkldVars initWkldVars

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
	wlInitializer := &initialize.WorkloadInitializer{Store: ssm, Ws: ws, Prog: spin, Deployer: deployer}
	wkldVars := initWkldVars{
		appName:        vars.appName,
		wkldType:       vars.wkldType,
		name:           vars.svcName,
		dockerfilePath: vars.dockerfilePath,
		image:          vars.image,
		port:           vars.port,
		schedule:       vars.schedule,
		retries:        vars.retries,
		timeout:        vars.timeout,
	}
	initSvcCmd := &initSvcOpts{
		initWkldVars: wkldVars,

		fs:     &afero.Afero{Fs: afero.NewOsFs()},
		init:   wlInitializer,
		sel:    sel,
		prompt: prompt,
		setupParser: func(o *initSvcOpts) {
			o.df = dockerfile.New(o.fs, o.dockerfilePath)
		},
	}
	initJobCmd := &initJobOpts{
		initWkldVars: wkldVars,

		fs:     &afero.Afero{Fs: afero.NewOsFs()},
		init:   wlInitializer,
		sel:    sel,
		prompt: prompt,
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
	deployJobCmd := &deployJobOpts{
		deployJobVars: deployJobVars{
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
		initJobCmd:   initJobCmd,
		initEnvCmd:   initEnvCmd,
		deploySvcCmd: deploySvcCmd,
		deployJobCmd: deployJobCmd,

		appName:           &initAppCmd.name,
		svcType:           &initSvcCmd.wkldType,
		jobType:           &initJobCmd.wkldType,
		svcName:           &initSvcCmd.name,
		jobName:           &initJobCmd.name,
		svcPort:           &initSvcCmd.port,
		svcDockerfilePath: &initSvcCmd.dockerfilePath,
		svcImage:          &initSvcCmd.image,

		wkldVars: wkldVars,

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

	wlInitializeCmd, err := o.loadWkld()
	if err != nil {
		return err
	}
	var name string
	if o.svcName == nil && o.jobName != nil {
		name = *o.jobName
	} else if o.svcName != nil && o.jobName == nil {
		name = *o.svcName
	} else {
		return fmt.Errorf("can")
	}
	log.Infof("Ok great, we'll set up a %s named %s in application %s listening on port %s.\n",
		color.HighlightUserInput(o.wkldVars.wkldType), color.HighlightUserInput(name), color.HighlightUserInput(*o.appName), color.HighlightUserInput(fmt.Sprintf("%d", *o.svcPort)))
	log.Infoln()
	if err := o.initAppCmd.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}
	if err := wlInitializeCmd.Execute(); err != nil {
		return fmt.Errorf("execute job or svc init: %w", err)
	}

	if err := o.deployEnv(); err != nil {
		return err
	}
	if o.wkldVars.wkldType == manifest.ScheduledJobType {
		return o.deployJob()
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

func (o *initOpts) loadWkld() (actionCommand, error) {
	cmd, err := o.loadWkldCmd()
	if err != nil {
		return nil, err
	}
	if err := cmd.Ask(); err != nil {
		return nil, fmt.Errorf("ask job or svc init: %w", err)
	}
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("validate job or svc init: %w", err)
	}

	return cmd, nil
}

func (o *initOpts) loadWkldCmd() (actionCommand, error) {
	wkldType, err := o.askWorkload()
	if err != nil {
		return nil, err
	}
	o.wkldVars.wkldType = wkldType

	switch wkldType {
	case manifest.ScheduledJobType:
		cmd, ok := o.initJobCmd.(*initJobOpts)
		if !ok {
			return nil, fmt.Errorf("build job init command")
		}
		cmd.wkldType = wkldType
		cmd.appName = *o.appName
		o.jobName = &cmd.name
		return cmd, nil
	case manifest.LoadBalancedWebServiceType:
		fallthrough
	case manifest.BackendServiceType:
		cmd, ok := o.initSvcCmd.(*initSvcOpts)
		if !ok {
			return nil, fmt.Errorf("build svc init command")
		}
		cmd.wkldType = wkldType
		cmd.appName = *o.appName
		o.svcName = &cmd.name
		return cmd, nil
	default:
		return nil, fmt.Errorf("invalid job or service type %s", wkldType)
	}
}

func (o *initOpts) askWorkload() (string, error) {
	if o.wkldVars.wkldType != "" {
		return o.wkldVars.wkldType, nil
	}
	wkldInitTypePrompt := "Which " + color.Emphasize("workload type") + " best represents your architecture?"
	// Build the workload help prompt from existing helps text.
	wkldHelp := fmt.Sprintf(fmtSvcInitSvcTypeHelpPrompt,
		manifest.LoadBalancedWebServiceType,
		manifest.BackendServiceType,
	) + `

` + fmt.Sprintf(fmtJobInitTypeHelp, manifest.ScheduledJobType)

	workloadTypes := []string{manifest.ScheduledJobType}
	for _, t := range manifest.ServiceTypes {
		workloadTypes = append(workloadTypes, t)
	}
	t, err := o.prompt.SelectOne(wkldInitTypePrompt, wkldHelp, workloadTypes, prompt.WithFinalMessage("Workload type:"))
	if err != nil {
		return "", fmt.Errorf("select service type: %w", err)
	}
	o.wkldVars.wkldType = t
	return t, nil
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

func (o *initOpts) deployJob() error {
	if !o.ShouldDeploy {
		return nil
	}
	if deployOpts, ok := o.deployJobCmd.(*deployJobOpts); ok {
		// Set the service's name and app name to the deploy sub-command.
		deployOpts.name = *o.jobName
		deployOpts.appName = *o.appName
	}

	if err := o.deployJobCmd.Ask(); err != nil {
		return err
	}
	return o.deployJobCmd.Execute()
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
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, svcFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.wkldType, svcTypeFlag, svcTypeFlagShort, "", svcTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldDeploy, deployFlag, false, deployTestFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().Uint16Var(&vars.port, svcPortFlag, 0, svcPortFlagDescription)
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
