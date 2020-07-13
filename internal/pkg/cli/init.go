// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

// Package cli contains the copilot commands.
package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/profile"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	profile        string
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
	sessProvider := session.NewProvider()
	sess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	prompt := prompt.New()
	spin := termprogress.NewSpinner()
	id := identity.New(sess)
	deployer := cloudformation.New(sess)
	cfg, err := profile.NewConfig()
	if err != nil {
		return nil, err
	}

	initAppCmd := &initAppOpts{
		initAppVars: initAppVars{
			AppName: vars.appName,
		},
		store:    ssm,
		ws:       ws,
		prompt:   prompt,
		identity: id,
		cfn:      deployer,
		prog:     spin,
	}
	initSvcCmd := &initSvcOpts{
		initSvcVars: initSvcVars{
			ServiceType:    vars.svcType,
			Name:           vars.svcName,
			DockerfilePath: vars.dockerfilePath,
			Port:           vars.port,
			GlobalOpts:     NewGlobalOpts(),
		},
		fs:          &afero.Afero{Fs: afero.NewOsFs()},
		ws:          ws,
		store:       ssm,
		appDeployer: deployer,
		prog:        spin,
		setupParser: func(o *initSvcOpts) {
			o.df = dockerfile.New(o.fs, o.DockerfilePath)
		},
	}
	initEnvCmd := &initEnvOpts{
		initEnvVars: initEnvVars{
			GlobalOpts:   NewGlobalOpts(),
			EnvName:      defaultEnvironmentName,
			EnvProfile:   vars.profile,
			IsProduction: false,
		},
		store:         ssm,
		appDeployer:   deployer,
		profileConfig: cfg,
		prog:          spin,
		identity:      id,

		initProfileClients: initEnvProfileClients,
	}

	deploySvcCmd := &deploySvcOpts{
		deploySvcVars: deploySvcVars{
			EnvName:    defaultEnvironmentName,
			ImageTag:   vars.imageTag,
			GlobalOpts: NewGlobalOpts(),
		},

		store:        ssm,
		ws:           ws,
		sel:          selector.NewWorkspaceSelect(prompt, ssm, ws),
		spinner:      spin,
		docker:       docker.New(),
		cmd:          command.New(),
		sessProvider: sessProvider,
	}

	return &initOpts{
		ShouldDeploy: vars.shouldDeploy,

		initAppCmd:   initAppCmd,
		initSvcCmd:   initSvcCmd,
		initEnvCmd:   initEnvCmd,
		deploySvcCmd: deploySvcCmd,

		appName:        &initAppCmd.AppName,
		svcType:        &initSvcCmd.ServiceType,
		svcName:        &initSvcCmd.Name,
		svcPort:        &initSvcCmd.Port,
		dockerfilePath: &initSvcCmd.DockerfilePath,

		prompt: prompt,
	}, nil
}

// Run executes "app init", "env init", "svc init" and "svc deploy".
func (o *initOpts) Run() error {
	log.Warningln("It's best to run this command in the root of your Git repository.")
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
	// Write the application name to viper so that sub-commands can retrieve its value.
	viper.Set(appFlag, o.appName)
	return nil
}

func (o *initOpts) loadSvc() error {
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

	log.Infoln()
	return o.initEnvCmd.Execute()
}

func (o *initOpts) deploySvc() error {
	if !o.ShouldDeploy {
		return nil
	}
	if deployOpts, ok := o.deploySvcCmd.(*deploySvcOpts); ok {
		// Set the service's name to the deploy sub-command.
		deployOpts.Name = *o.svcName
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
	cmd.Flags().StringVar(&vars.profile, profileFlag, defaultEnvironmentProfile, profileFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
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
