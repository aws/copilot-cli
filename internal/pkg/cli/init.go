// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

// Package cli contains the ecs-preview subcommands.
package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/profile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/selector"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker/dockerfile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
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
	initShouldDeployHelpPrompt = "An environment with your application deployed to it. This will allow you to test your application before placing it in production."
)

type initVars struct {
	// Flags unique to "init" that's not provided by other sub-commands.
	shouldDeploy   bool
	projectName    string
	appType        string
	appName        string
	dockerfilePath string
	profile        string
	imageTag       string
	port           uint16
}

type initOpts struct {
	ShouldDeploy          bool // true means we should create a test environment and deploy the application in it. Defaults to false.
	promptForShouldDeploy bool // true means that the user set the ShouldDeploy flag explicitly.

	// Sub-commands to execute.
	initProject actionCommand
	initApp     actionCommand
	initEnv     actionCommand
	appDeploy   actionCommand

	// Pointers to flag values part of sub-commands.
	// Since the sub-commands implement the actionCommand interface, without pointers to their internal fields
	// we have to resort to type-casting the interface. These pointers simplify data access.
	projectName    *string
	appType        *string
	appName        *string
	appPort        *uint16
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

	initProject := &initAppOpts{
		initAppVars: initAppVars{
			AppName: vars.projectName,
		},
		store:    ssm,
		ws:       ws,
		prompt:   prompt,
		identity: id,
		cfn:      deployer,
		prog:     spin,
	}
	initApp := &initSvcOpts{
		initSvcVars: initSvcVars{
			ServiceType:    vars.appType,
			Name:           vars.appName,
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
	initEnv := &initEnvOpts{
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

	appDeploy := &deploySvcOpts{
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

		initProject: initProject,
		initApp:     initApp,
		initEnv:     initEnv,
		appDeploy:   appDeploy,

		projectName:    &initProject.AppName,
		appType:        &initApp.ServiceType,
		appName:        &initApp.Name,
		appPort:        &initApp.Port,
		dockerfilePath: &initApp.DockerfilePath,

		prompt: prompt,
	}, nil
}

// Run executes "project init", "env init", "app init" and "app deploy".
func (o *initOpts) Run() error {
	log.Warningln("It's best to run this command in the root of your Git repository.")
	log.Infoln(`Welcome to the ECS CLI! We're going to walk you through some questions
to help you get set up with a project on ECS. A project is a collection of
containerized applications (or micro-services) that operate together.`)
	log.Infoln()

	if err := o.loadProject(); err != nil {
		return err
	}
	if err := o.loadApp(); err != nil {
		return err
	}

	log.Infof("Ok great, we'll set up a %s named %s in project %s listening on port %s.\n",
		color.HighlightUserInput(*o.appType), color.HighlightUserInput(*o.appName), color.HighlightUserInput(*o.projectName), color.HighlightUserInput(fmt.Sprintf("%d", *o.appPort)))

	if err := o.initProject.Execute(); err != nil {
		return fmt.Errorf("execute project init: %w", err)
	}
	if err := o.initApp.Execute(); err != nil {
		return fmt.Errorf("execute app init: %w", err)
	}

	if err := o.deployEnv(); err != nil {
		return err
	}

	return o.deployApp()
}

func (o *initOpts) loadProject() error {
	if err := o.initProject.Ask(); err != nil {
		return fmt.Errorf("prompt for project init: %w", err)
	}
	if err := o.initProject.Validate(); err != nil {
		return err
	}
	// Write the project name to viper so that sub-commands can retrieve its value.
	viper.Set(appFlag, o.projectName)
	return nil
}

func (o *initOpts) loadApp() error {
	if err := o.initApp.Ask(); err != nil {
		return fmt.Errorf("prompt for app init: %w", err)
	}
	return o.initApp.Validate()
}

// deployEnv prompts the user to deploy a test environment if the project doesn't already have one.
func (o *initOpts) deployEnv() error {
	if o.promptForShouldDeploy {
		log.Infoln("All right, you're all set for local development.")
		if err := o.askShouldDeploy(); err != nil {
			return err
		}
	}
	if !o.ShouldDeploy {
		// User chose not to deploy the application, exit.
		return nil
	}

	return o.initEnv.Execute()
}

func (o *initOpts) deployApp() error {
	if !o.ShouldDeploy {
		return nil
	}
	if deployOpts, ok := o.appDeploy.(*deploySvcOpts); ok {
		// Set the application's name to the deploy sub-command.
		deployOpts.Name = *o.appName
	}

	if err := o.appDeploy.Ask(); err != nil {
		return err
	}
	return o.appDeploy.Execute()
}

func (o *initOpts) askShouldDeploy() error {
	v, err := o.prompt.Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt)
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
				log.Info("\nNo problem, you can deploy your application later:\n")
				log.Infof("- Run %s to create your staging environment.\n",
					color.HighlightCode(fmt.Sprintf("ecs-preview env init --name %s --profile default --project %s", defaultEnvironmentName, *opts.projectName)))
				for _, followup := range opts.initApp.RecommendedActions() {
					log.Infof("- %s\n", followup)
				}
			}
			return nil
		}),
	}
	cmd.Flags().StringVar(&vars.profile, profileFlag, defaultEnvironmentProfile, profileFlagDescription)
	cmd.Flags().StringVarP(&vars.projectName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, svcFlag, svcFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.appType, svcTypeFlag, svcTypeFlagShort, "", svcTypeFlagDescription)
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
