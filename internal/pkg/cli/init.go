// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the copilot commands.
package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	cmdtemplate "github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
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

	// Service specific flags
	port uint16

	// Scheduled Job specific flags
	schedule string
	retries  int
	timeout  string
}

type initOpts struct {
	initVars

	ShouldDeploy          bool // true means we should create a test environment and deploy the service to it. Defaults to false.
	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.

	// Sub-commands to execute.
	initAppCmd   actionCommand
	initWlCmd    actionCommand
	initEnvCmd   actionCommand
	deploySvcCmd actionCommand
	deployJobCmd actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	appName      *string
	port         *uint16
	schedule     *string
	initWkldVars *initWkldVars

	prompt prompter

	setupWorkloadInit func(*initOpts, string) error
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
	spin := termprogress.NewSpinner(log.DiagnosticWriter)
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
		appCFN:      cloudformation.New(defaultSess),
		uploader:    template.New(),
		newS3: func(region string) (zipAndUploader, error) {
			sess, err := sessProvider.DefaultWithRegion(region)
			if err != nil {
				return nil, err
			}
			return s3.New(sess), nil
		},

		sess: defaultSess,
	}

	deploySvcCmd := &deploySvcOpts{
		deployWkldVars: deployWkldVars{
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
		cmd:          exec.NewCmd(),
		sessProvider: sessProvider,
	}
	deployJobCmd := &deployJobOpts{
		deployWkldVars: deployWkldVars{
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
		cmd:          exec.NewCmd(),
		sessProvider: sessProvider,
	}
	fs := &afero.Afero{Fs: afero.NewOsFs()}
	return &initOpts{
		initVars:     vars,
		ShouldDeploy: vars.shouldDeploy,

		initAppCmd:   initAppCmd,
		initEnvCmd:   initEnvCmd,
		deploySvcCmd: deploySvcCmd,
		deployJobCmd: deployJobCmd,

		appName: &initAppCmd.name,

		prompt: prompt,

		setupWorkloadInit: func(o *initOpts, wkldType string) error {
			wlInitializer := &initialize.WorkloadInitializer{Store: ssm, Ws: ws, Prog: spin, Deployer: deployer}
			wkldVars := initWkldVars{
				appName:        *o.appName,
				wkldType:       wkldType,
				name:           vars.svcName,
				dockerfilePath: vars.dockerfilePath,
				image:          vars.image,
			}
			switch t := wkldType; {
			case t == manifest.ScheduledJobType:
				jobVars := initJobVars{
					initWkldVars: wkldVars,
					schedule:     vars.schedule,
					retries:      vars.retries,
					timeout:      vars.timeout,
				}

				opts := initJobOpts{
					initJobVars: jobVars,

					fs:                    fs,
					init:                  wlInitializer,
					sel:                   sel,
					prompt:                prompt,
					dockerEngineValidator: exec.NewDockerCommand(),
					initParser: func(s string) dockerfileParser {
						return exec.NewDockerfile(fs, s)
					},
				}
				o.initWlCmd = &opts
				o.schedule = &opts.schedule // Surfaced via pointer for logging
				o.initWkldVars = &opts.initWkldVars
			case manifest.IsTypeAService(t):
				svcVars := initSvcVars{
					initWkldVars: wkldVars,
					port:         vars.port,
				}
				opts := initSvcOpts{
					initSvcVars: svcVars,

					fs:                    fs,
					init:                  wlInitializer,
					sel:                   sel,
					prompt:                prompt,
					dockerEngineValidator: exec.NewDockerCommand(),
				}
				opts.dockerfile = func(path string) dockerfileParser {
					if opts.df != nil {
						return opts.df
					}
					opts.df = exec.NewDockerfile(opts.fs, opts.dockerfilePath)
					return opts.df
				}
				o.initWlCmd = &opts
				o.port = &opts.port // Surfaced via pointer for logging.
				o.initWkldVars = &opts.initWkldVars
			default:
				return fmt.Errorf("unrecognized workload type")
			}
			return nil
		},
	}, nil
}

// Run executes "app init", "env init", "svc init" and "svc deploy".
func (o *initOpts) Run() error {
	if !workspace.IsInGitRepository(afero.NewOsFs()) {
		log.Warningln("It's best to run this command in the root of your Git repository.")
	}
	log.Infoln(color.Help(`Welcome to the Copilot CLI! We're going to walk you through some questions
to help you get set up with a containerized application on AWS. An application is a collection of
containerized services that operate together.`))
	log.Infoln()

	if err := o.loadApp(); err != nil {
		return err
	}

	if err := o.loadWkld(); err != nil {
		return err
	}

	o.logWorkloadTypeAck()

	log.Infoln()
	if err := o.initAppCmd.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}
	if err := o.initWlCmd.Execute(); err != nil {
		return fmt.Errorf("execute %s init: %w", o.wkldType, err)
	}

	if err := o.deployEnv(); err != nil {
		return err
	}
	return o.deploy()
}

func (o *initOpts) logWorkloadTypeAck() {
	if o.initWkldVars.wkldType == manifest.ScheduledJobType {
		log.Infof("Ok great, we'll set up a %s named %s in application %s running on the schedule %s.\n",
			color.HighlightUserInput(o.initWkldVars.wkldType), color.HighlightUserInput(o.initWkldVars.name), color.HighlightUserInput(o.initWkldVars.appName), color.HighlightUserInput(*o.schedule))
		return
	}
	if aws.Uint16Value(o.port) != 0 {
		log.Infof("Ok great, we'll set up a %s named %s in application %s listening on port %s.\n", color.HighlightUserInput(o.initWkldVars.wkldType), color.HighlightUserInput(o.initWkldVars.name), color.HighlightUserInput(o.initWkldVars.appName), color.HighlightUserInput(fmt.Sprintf("%d", *o.port)))
	} else {
		log.Infof("Ok great, we'll set up a %s named %s in application %s.\n", color.HighlightUserInput(o.initWkldVars.wkldType), color.HighlightUserInput(o.initWkldVars.name), color.HighlightUserInput(o.initWkldVars.appName))
	}
}

func (o *initOpts) deploy() error {
	if o.initWkldVars.wkldType == manifest.ScheduledJobType {
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

func (o *initOpts) loadWkld() error {
	err := o.loadWkldCmd()
	if err != nil {
		return err
	}
	if err := o.initWlCmd.Validate(); err != nil {
		return fmt.Errorf("validate %s: %w", o.wkldType, err)
	}
	if err := o.initWlCmd.Ask(); err != nil {
		return fmt.Errorf("ask %s: %w", o.wkldType, err)
	}

	return nil
}

func (o *initOpts) loadWkldCmd() error {
	wkldType, err := o.askWorkload()
	if err != nil {
		return err
	}
	if err := o.setupWorkloadInit(o, wkldType); err != nil {
		return err
	}

	return nil
}

func (o *initOpts) askWorkload() (string, error) {
	if o.wkldType != "" {
		return o.wkldType, nil
	}
	wkldInitTypePrompt := "Which " + color.Emphasize("workload type") + " best represents your architecture?"
	// Build the workload help prompt from existing helps text.
	wkldHelp := fmt.Sprintf(fmtSvcInitSvcTypeHelpPrompt,
		manifest.RequestDrivenWebServiceType,
		manifest.LoadBalancedWebServiceType,
		manifest.BackendServiceType,
	) + `

` + fmt.Sprintf(fmtJobInitTypeHelp, manifest.ScheduledJobType)

	t, err := o.prompt.SelectOption(wkldInitTypePrompt, wkldHelp, append(svcTypePromptOpts(), jobTypePromptOpts()...), prompt.WithFinalMessage("Workload type:"))
	if err != nil {
		return "", fmt.Errorf("select workload type: %w", err)
	}
	o.wkldType = t
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
		deployOpts.name = o.initWkldVars.name
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
		deployOpts.name = o.initWkldVars.name
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
		Short: "Create a new ECS or App Runner application.",
		Long:  "Create a new ECS or App Runner application.",
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
				for _, followup := range opts.initWlCmd.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.wkldType, typeFlag, typeFlagShort, "", wkldTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldDeploy, deployFlag, false, deployTestFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().Uint16Var(&vars.port, svcPortFlag, 0, svcPortFlagDescription)
	cmd.Flags().StringVar(&vars.schedule, scheduleFlag, "", scheduleFlagDescription)
	cmd.Flags().StringVar(&vars.timeout, timeoutFlag, "", timeoutFlagDescription)
	cmd.Flags().IntVar(&vars.retries, retriesFlag, 0, retriesFlagDescription)
	cmd.SetUsageTemplate(cmdtemplate.Usage)
	cmd.Annotations = map[string]string{
		"group": group.GettingStarted,
	}
	return cmd
}
