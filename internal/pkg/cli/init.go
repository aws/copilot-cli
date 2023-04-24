// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the copilot commands.
package cli

import (
	"errors"
	"fmt"
	"os"

	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/iam"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	cmdtemplate "github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
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
	defaultEnvironmentName = "test"
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
	deployEnvCmd cmd
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

	setupWorkloadInit           func(*initOpts, string) error
	useExistingWorkspaceForCMDs func(*initOpts) error
}

func newInitOpts(vars initVars) (*initOpts, error) {
	fs := afero.NewOsFs()
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("init"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	prompt := prompt.New()
	deployStore, err := deploy.NewStore(sessProvider, configStore)
	if err != nil {
		return nil, err
	}
	snsSel := selector.NewDeploySelect(prompt, configStore, deployStore)
	spin := termprogress.NewSpinner(log.DiagnosticWriter)
	id := identity.New(defaultSess)
	deployer := cloudformation.New(defaultSess, cloudformation.WithProgressTracker(os.Stderr))
	iamClient := iam.New(defaultSess)
	initAppCmd := &initAppOpts{
		initAppVars: initAppVars{
			name: vars.appName,
		},
		store:    configStore,
		prompt:   prompt,
		identity: id,
		cfn:      deployer,
		prog:     spin,
		isSessionFromEnvVars: func() (bool, error) {
			return sessions.AreCredsFromEnvVars(defaultSess)
		},
		existingWorkspace: func() (wsAppManager, error) {
			return workspace.Use(fs)
		},
		newWorkspace: func(appName string) (wsAppManager, error) {
			return workspace.Create(appName, fs)
		},
		iam:            iamClient,
		iamRoleManager: iamClient,
	}
	initEnvCmd := &initEnvOpts{
		initEnvVars: initEnvVars{
			appName:      vars.appName,
			name:         defaultEnvironmentName,
			isProduction: false,
		},
		store:       configStore,
		appDeployer: deployer,
		prog:        spin,
		prompt:      prompt,
		identity:    id,
		appCFN:      cloudformation.New(defaultSess, cloudformation.WithProgressTracker(os.Stderr)),

		sess: defaultSess,
	}
	deployEnvCmd := &deployEnvOpts{
		deployEnvVars: deployEnvVars{
			appName: vars.appName,
			name:    defaultEnvironmentName,
		},
		store:           configStore,
		sessionProvider: sessProvider,
		identity:        id,
		fs:              fs,
		newInterpolator: newManifestInterpolator,
	}
	deploySvcCmd := &deploySvcOpts{
		deployWkldVars: deployWkldVars{
			envName:  defaultEnvironmentName,
			imageTag: vars.imageTag,
			appName:  vars.appName,
		},

		store:           configStore,
		prompt:          prompt,
		newInterpolator: newManifestInterpolator,
		unmarshal:       manifest.UnmarshalWorkload,
		spinner:         spin,
		cmd:             exec.NewCmd(),
		sessProvider:    sessProvider,
	}
	deploySvcCmd.newSvcDeployer = func() (workloadDeployer, error) {
		return newSvcDeployer(deploySvcCmd)
	}
	deployJobCmd := &deployJobOpts{
		deployWkldVars: deployWkldVars{
			envName:  defaultEnvironmentName,
			imageTag: vars.imageTag,
			appName:  vars.appName,
		},
		store:           configStore,
		newInterpolator: newManifestInterpolator,
		unmarshal:       manifest.UnmarshalWorkload,
		cmd:             exec.NewCmd(),
		sessProvider:    sessProvider,
	}
	deployJobCmd.newJobDeployer = func() (workloadDeployer, error) {
		return newJobDeployer(deployJobCmd)
	}

	cmd := exec.NewCmd()

	useExistingWorkspaceClient := func(o *initOpts) error {
		ws, err := workspace.Use(fs)
		if err != nil {
			return err
		}
		sel := selector.NewLocalWorkloadSelector(prompt, configStore, ws)
		initEnvCmd.manifestWriter = ws
		deployEnvCmd.ws = ws
		deployEnvCmd.newEnvDeployer = func() (envDeployer, error) {
			return newEnvDeployer(deployEnvCmd, ws)
		}
		deploySvcCmd.ws = ws
		deploySvcCmd.sel = sel
		deployJobCmd.ws = ws
		deployJobCmd.sel = sel
		if initWkCmd, ok := o.initWlCmd.(*initSvcOpts); ok {
			initWkCmd.init = &initialize.WorkloadInitializer{Store: configStore, Ws: ws, Prog: spin, Deployer: deployer}
		}
		if initWkCmd, ok := o.initWlCmd.(*initJobOpts); ok {
			initWkCmd.init = &initialize.WorkloadInitializer{Store: configStore, Ws: ws, Prog: spin, Deployer: deployer}
		}
		return nil
	}
	return &initOpts{
		initVars:     vars,
		ShouldDeploy: vars.shouldDeploy,

		initAppCmd:   initAppCmd,
		initEnvCmd:   initEnvCmd,
		deployEnvCmd: deployEnvCmd,
		deploySvcCmd: deploySvcCmd,
		deployJobCmd: deployJobCmd,

		appName: &initAppCmd.name,

		prompt: prompt,

		setupWorkloadInit: func(o *initOpts, wkldType string) error {
			wkldVars := initWkldVars{
				appName:        *o.appName,
				wkldType:       wkldType,
				name:           vars.svcName,
				dockerfilePath: vars.dockerfilePath,
				image:          vars.image,
			}
			ws, err := workspace.Use(fs)
			if err != nil {
				return err
			}
			sel, err := selector.NewLocalFileSelector(prompt, fs, ws)
			if err != nil {
				return err
			}
			switch t := wkldType; {
			case manifestinfo.IsTypeAJob(t):
				jobVars := initJobVars{
					initWkldVars: wkldVars,
					schedule:     vars.schedule,
					retries:      vars.retries,
					timeout:      vars.timeout,
				}

				opts := initJobOpts{
					initJobVars: jobVars,

					fs:                fs,
					store:             configStore,
					dockerfileSel:     sel,
					scheduleSelector:  selector.NewStaticSelector(prompt),
					prompt:            prompt,
					dockerEngine:      dockerengine.New(cmd),
					wsPendingCreation: true,
					initParser: func(s string) dockerfileParser {
						return dockerfile.New(fs, s)
					},
					initEnvDescriber: func(appName string, envName string) (envDescriber, error) {
						envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
							App:         appName,
							Env:         envName,
							ConfigStore: configStore,
						})
						if err != nil {
							return nil, fmt.Errorf("initiate env describer: %w", err)
						}
						return envDescriber, nil
					},
				}
				o.initWlCmd = &opts
				o.schedule = &opts.schedule // Surfaced via pointer for logging
				o.initWkldVars = &opts.initWkldVars
			case manifestinfo.IsTypeAService(t):
				svcVars := initSvcVars{
					initWkldVars: wkldVars,
					port:         vars.port,
					ingressType:  ingressTypeInternet,
				}
				opts := initSvcOpts{
					initSvcVars: svcVars,

					fs:                fs,
					sel:               sel,
					store:             configStore,
					topicSel:          snsSel,
					prompt:            prompt,
					dockerEngine:      dockerengine.New(cmd),
					wsPendingCreation: true,
				}
				opts.dockerfile = func(path string) dockerfileParser {
					if opts.df != nil {
						return opts.df
					}
					opts.df = dockerfile.New(opts.fs, opts.dockerfilePath)
					return opts.df
				}
				opts.initEnvDescriber = func(appName string, envName string) (envDescriber, error) {
					envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
						App:         appName,
						Env:         envName,
						ConfigStore: opts.store,
					})
					if err != nil {
						return nil, fmt.Errorf("initiate env describer: %w", err)
					}
					return envDescriber, nil
				}
				o.initWlCmd = &opts
				o.port = &opts.port // Surfaced via pointer for logging.
				o.initWkldVars = &opts.initWkldVars
			default:
				return fmt.Errorf("unrecognized workload type")
			}
			return nil
		},
		useExistingWorkspaceForCMDs: useExistingWorkspaceClient,
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
	if err := o.useExistingWorkspaceForCMDs(o); err != nil {
		return fmt.Errorf("set up workspace client for commands: %w", err)
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
	if manifestinfo.IsTypeAJob(o.initWkldVars.wkldType) {
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
	if manifestinfo.IsTypeAJob(o.initWkldVars.wkldType) {
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
	wkldHelp := fmt.Sprintf("%s\n\n%s", svcInitSvcTypeHelpPrompt, jobInitTypeHelp)
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
	if err := o.initEnvCmd.Execute(); err != nil {
		return err
	}
	log.Successf("Provisioned bootstrap resources for environment %s.\n", defaultEnvironmentName)
	if deployEnvCmd, ok := o.deployEnvCmd.(*deployEnvOpts); ok {
		// Set the application name from app init to the env deploy command.
		deployEnvCmd.appName = *o.appName
	}

	if err := o.deployEnvCmd.Execute(); err != nil {
		var errEmptyChangeSet *awscfn.ErrChangeSetEmpty
		if !errors.As(err, &errEmptyChangeSet) {
			return err
		}
	}
	return nil
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
	if err := o.deploySvcCmd.Execute(); err != nil {
		return err
	}
	if err := o.deploySvcCmd.RecommendActions(); err != nil {
		return err
	}
	return nil
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
	if err := o.deployJobCmd.Execute(); err != nil {
		return err
	}
	if err := o.deployJobCmd.RecommendActions(); err != nil {
		return err
	}
	return nil
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
				log.Infof("- Run %s to create your environment.\n", color.HighlightCode("copilot env init"))
				log.Infof("- Run %s to deploy your service.\n", color.HighlightCode("copilot deploy"))
			}
			log.Infoln(`- Be a part of the Copilot ✨community✨!
  Ask or answer a question, submit a feature request...
  Visit 👉 https://aws.github.io/copilot-cli/community/get-involved/ to see how!`)
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
