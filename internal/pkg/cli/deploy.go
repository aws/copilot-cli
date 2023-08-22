// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	svcWkldType = "svc"
	jobWkldType = "job"
)

type deployVars struct {
	deployWkldVars
	deployEnvVars

	yesInitWkld *bool
	deployEnv   *bool
	yesInitEnv  *bool

	region    string
	tempCreds tempCredsVars
	profile   string
}

type deployOpts struct {
	deployVars

	deployWkld       actionCommand
	newWorkloadAdder func() wkldInitializerWithoutManifest
	setupDeployCmd   func(*deployOpts, string)

	newInitEnvCmd   func(o *deployOpts) (cmd, error)
	newDeployEnvCmd func(o *deployOpts) (cmd, error)

	sel    wsSelector
	store  store
	ws     wsWlDirReader
	prompt prompter

	// values for logging
	wlType string

	// values for initialization logic
	envExistsInApp bool
	envExistsInWs  bool
}

func newDeployOpts(vars deployVars) (*deployOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("deploy"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}
	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	prompter := prompt.New()
	return &deployOpts{
		deployVars: vars,
		store:      store,
		sel:        selector.NewLocalWorkloadSelector(prompter, store, ws),
		ws:         ws,
		prompt:     prompter,

		newWorkloadAdder: func() wkldInitializerWithoutManifest {
			return &initialize.WorkloadInitializer{
				Store:    store,
				Deployer: cloudformation.New(defaultSess),
				Ws:       ws,
				Prog:     termprogress.NewSpinner(log.DiagnosticWriter),
			}
		},
		newDeployEnvCmd: func(o *deployOpts) (cmd, error) {
			// This command passes flags down from
			cmd, err := newEnvDeployOpts(deployEnvVars{
				appName:           o.deployWkldVars.appName,
				name:              o.envName,
				forceNewUpdate:    o.deployWkldVars.forceNewUpdate,
				disableRollback:   o.deployWkldVars.disableRollback,
				showDiff:          o.deployWkldVars.showDiff,
				skipDiffPrompt:    o.deployWkldVars.skipDiffPrompt,
				allowEnvDowngrade: o.deployWkldVars.allowWkldDowngrade,
				detach:            o.deployWkldVars.detach,
			})
			if err != nil {
				return nil, err
			}
			return cmd, nil
		},

		newInitEnvCmd: func(o *deployOpts) (cmd, error) {
			// This vars struct sets "default config" so that no vpc questions are asked during env init and the manifest
			// is not written. It passes in credential flags and allow-downgrade from the parent command.
			cmd, err := newInitEnvOpts(initEnvVars{
				appName:           o.deployWkldVars.appName,
				name:              o.envName,
				profile:           o.profile,
				defaultConfig:     true,
				allowAppDowngrade: o.allowWkldDowngrade,
				tempCreds:         o.tempCreds,
				region:            o.region,
			})
			if err != nil {
				return nil, err
			}
			return cmd, err
		},

		setupDeployCmd: func(o *deployOpts, workloadType string) {
			switch {
			case contains(workloadType, manifestinfo.JobTypes()):
				opts := &deployJobOpts{
					deployWkldVars: o.deployWkldVars,

					store:           o.store,
					ws:              o.ws,
					newInterpolator: newManifestInterpolator,
					unmarshal:       manifest.UnmarshalWorkload,
					sel:             selector.NewLocalWorkloadSelector(o.prompt, o.store, ws),
					cmd:             exec.NewCmd(),
					templateVersion: version.LatestTemplateVersion(),
					sessProvider:    sessProvider,
				}
				opts.newJobDeployer = func() (workloadDeployer, error) {
					return newJobDeployer(opts)
				}
				o.deployWkld = opts
			case contains(workloadType, manifestinfo.ServiceTypes()):
				opts := &deploySvcOpts{
					deployWkldVars: o.deployWkldVars,

					store:           o.store,
					ws:              o.ws,
					newInterpolator: newManifestInterpolator,
					unmarshal:       manifest.UnmarshalWorkload,
					spinner:         termprogress.NewSpinner(log.DiagnosticWriter),
					sel:             selector.NewLocalWorkloadSelector(o.prompt, o.store, ws),
					prompt:          o.prompt,
					cmd:             exec.NewCmd(),
					sessProvider:    sessProvider,
					templateVersion: version.LatestTemplateVersion(),
				}
				opts.newSvcDeployer = func() (workloadDeployer, error) {
					return newSvcDeployer(opts)
				}
				o.deployWkld = opts
			}
		},
	}, nil
}

func (o *deployOpts) maybeInitWkld() error {
	// Confirm that the workload needs to be initialized after asking for the name.
	initializedWorkloads, err := o.store.ListWorkloads(o.deployWkldVars.appName)
	if err != nil {
		return fmt.Errorf("retrieve workloads: %w", err)
	}
	wlNames := make([]string, len(initializedWorkloads))
	for i := range initializedWorkloads {
		wlNames[i] = initializedWorkloads[i].Name
	}

	// Workload is already initialized. Return early.
	if contains(o.deployWkldVars.name, wlNames) {
		return nil
	}

	// Get workload type and confirm readable manifest.
	mf, err := o.ws.ReadWorkloadManifest(o.deployWkldVars.name)
	if err != nil {
		return fmt.Errorf("read manifest for workload %s: %w", o.deployWkldVars.name, err)
	}
	workloadType, err := mf.WorkloadType()
	if err != nil {
		return fmt.Errorf("get workload type from manifest for workload %s: %w", o.deployWkldVars.name, err)
	}

	if !contains(workloadType, manifestinfo.WorkloadTypes()) {
		return fmt.Errorf("unrecognized workload type %q in manifest for workload %s", workloadType, o.deployWkldVars.name)
	}

	if o.yesInitWkld == nil {
		confirmInitWkld, err := o.prompt.Confirm(fmt.Sprintf("Found manifest for uninitialized %s %q. Initialize it?", workloadType, o.deployWkldVars.name), "This workload will be initialized, then deployed.", prompt.WithConfirmFinalMessage())
		if err != nil {
			return fmt.Errorf("confirm initialize workload: %w", err)
		}
		o.yesInitWkld = aws.Bool(confirmInitWkld)
	}

	if !aws.BoolValue(o.yesInitWkld) {
		return fmt.Errorf("workload %s is uninitialized but --%s=false was specified", o.name, yesInitWorkloadFlag)
	}

	wkldAdder := o.newWorkloadAdder()
	if err = wkldAdder.AddWorkloadToApp(o.deployWkldVars.appName, o.deployWkldVars.name, workloadType); err != nil {
		return fmt.Errorf("add workload to app: %w", err)
	}
	return nil
}

func (o *deployOpts) Run() error {
	if err := o.askName(); err != nil {
		return err
	}

	if err := o.askEnv(); err != nil {
		return err
	}

	if err := o.checkEnvExists(); err != nil {
		return err
	}

	if err := o.maybeInitEnv(); err != nil {
		return err
	}

	if err := o.maybeInitWkld(); err != nil {
		return err
	}

	if err := o.maybeDeployEnv(); err != nil {
		return err
	}

	if err := o.loadWkld(); err != nil {
		return err
	}
	if err := o.deployWkld.Execute(); err != nil {
		return fmt.Errorf("execute %s deploy: %w", o.wlType, err)
	}
	if err := o.deployWkld.RecommendActions(); err != nil {
		return err
	}
	return nil
}

func (o *deployOpts) askName() error {
	if o.deployWkldVars.name != "" {
		return nil
	}
	name, err := o.sel.Workload("Select a service or job in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service or job: %w", err)
	}
	o.deployWkldVars.name = name
	return nil
}

func (o *deployOpts) askEnv() error {
	if o.deployWkldVars.envName != "" {
		return nil
	}
	localEnvs, err := o.ws.ListEnvironments()
	if err != nil {
		return fmt.Errorf("get workspace environments: %w", err)
	}
	initializedEnvs, err := o.store.ListEnvironments(o.deployWkldVars.appName)
	if err != nil {
		return fmt.Errorf("get initialized environments: %w", err)
	}

	// Get uninitialized local environments and append them to the env selector call.
	var extraOptions []string
	for _, localEnv := range localEnvs {
		var envIsInitted bool
		for _, inittedEnv := range initializedEnvs {
			if inittedEnv.Name == localEnv {
				envIsInitted = true
				break
			}
		}
		if envIsInitted {
			continue
		}
		extraOptions = append(extraOptions, fmt.Sprintf("%s (uninitialized)", localEnv))
	}

	o.deployWkldVars.envName, err = o.sel.Environment("Select an environment to deploy to", "", o.deployWkldVars.appName, extraOptions...)
	if err != nil {
		return fmt.Errorf("get environment name: %w", err)
	}
	return nil
}

// checkEnvExists checks whether the environment is initialized and has a local manifest.
func (o *deployOpts) checkEnvExists() error {
	o.envExistsInApp = true
	_, err := o.store.GetEnvironment(o.deployWkldVars.appName, o.envName)
	if err != nil {
		var errNotFound *config.ErrNoSuchEnvironment
		if !errors.As(err, &errNotFound) {
			return fmt.Errorf("get environment from config store: %w", err)
		}
		o.envExistsInApp = false
	}
	envs, err := o.ws.ListEnvironments()
	if err != nil {
		return fmt.Errorf("list environments in workspace: %w", err)
	}
	o.envExistsInWs = contains(o.envName, envs)

	// the desired environment doesn't actually exist.
	if !o.envExistsInApp && !o.envExistsInWs {
		log.Errorf("Environment %q does not exist in the current application or workspace. Please initialize it by running %s.\n", o.envName, color.HighlightCode("copilot env init"))
		return fmt.Errorf("environment %q does not exist in the workspace", o.envName)
	}
	if o.envExistsInApp && !o.envExistsInWs {
		log.Infof("Manifest for environment %q does not exist in the current workspace. To deploy this environment, generate a manifest with %s", o.deployWkldVars.envName, color.HighlightCode("copilot env show --manifest"))
	}

	return nil
}

func (o *deployOpts) maybeInitEnv() error {
	// Env has no manifest and doesn't exist in app; we can't do anything more.
	if !o.envExistsInWs && !o.envExistsInApp {
		return fmt.Errorf("environment %q does not exist in the workspace", o.envName)
	}

	if o.envExistsInApp {
		return nil
	}
	// If no initialization flags were specified and the env wasn't initialized, ask to confirm.
	if !o.envExistsInApp && o.yesInitEnv == nil {
		v, err := o.prompt.Confirm(fmt.Sprintf("Environment %q does not exist in app %q. Initialize it?", o.envName, o.deployEnvVars.appName), "")
		if err != nil {
			return fmt.Errorf("confirm env init: %w", err)
		}
		o.yesInitEnv = aws.Bool(v)
	}

	if aws.BoolValue(o.yesInitEnv) {
		cmd, err := o.newInitEnvCmd(o)
		if err != nil {
			return fmt.Errorf("load env init command : %w", err)
		}
		if err = cmd.Validate(); err != nil {
			return err
		}
		if err = cmd.Ask(); err != nil {
			return err
		}
		return cmd.Execute()
	}
	log.Errorf("Environment %q does not exist in application %q and was not initialized after prompting.\n", o.deployWkldVars.envName, o.deployWkldVars.appName)
	return fmt.Errorf("env %s does not exist in app %s", o.deployWkldVars.envName, o.deployWkldVars.appName)
}

func (o *deployOpts) maybeDeployEnv() error {
	if o.deployEnv == nil {
		v, err := o.prompt.Confirm("Would you like to deploy the environment %q before deploying your workload?", "")
		if err != nil {
			return fmt.Errorf("confirm env deployment: %w", err)
		}
		o.deployEnv = aws.Bool(v)
	}

	if aws.BoolValue(o.deployEnv) {
		cmd, err := o.newDeployEnvCmd(o)
		if err != nil {
			return fmt.Errorf("set up env deploy command: %w", err)
		}
		if err = cmd.Validate(); err != nil {
			return err
		}
		if err = cmd.Ask(); err != nil {
			return err
		}
		return cmd.Execute()
	}
	return nil
}

func (o *deployOpts) loadWkld() error {
	if err := o.loadWkldCmd(); err != nil {
		return err
	}
	if err := o.deployWkld.Ask(); err != nil {
		return fmt.Errorf("ask %s deploy: %w", o.wlType, err)
	}
	if err := o.deployWkld.Validate(); err != nil {
		return fmt.Errorf("validate %s deploy: %w", o.wlType, err)
	}
	return nil
}

func (o *deployOpts) loadWkldCmd() error {
	wl, err := o.store.GetWorkload(o.deployWkldVars.appName, o.deployWkldVars.name)
	if err != nil {
		return fmt.Errorf("retrieve %s from application %s: %w", o.deployWkldVars.appName, o.deployWkldVars.name, err)
	}
	o.setupDeployCmd(o, wl.Type)
	if strings.Contains(strings.ToLower(wl.Type), jobWkldType) {
		o.wlType = jobWkldType
		return nil
	}
	o.wlType = svcWkldType
	return nil
}

// BuildDeployCmd is the deploy command.
func BuildDeployCmd() *cobra.Command {
	vars := deployVars{}
	var initWorkload bool
	var initEnvironment bool
	var deployEnvironment bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a Copilot job or service.",
		Long:  "Deploy a Copilot job or service.",
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot deploy --name frontend --env test
  Deploys a job named "mailer" with additional resource tags to a "prod" environment.
  /code $ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeployOpts(vars)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed(yesInitWorkloadFlag) {
				opts.yesInitWkld = aws.Bool(false)
				if initWorkload {
					opts.yesInitWkld = aws.Bool(true)
				}
			}

			if cmd.Flags().Changed(yesInitEnvFlag) {
				opts.yesInitEnv = aws.Bool(false)
				if initEnvironment {
					opts.yesInitEnv = aws.Bool(true)
				}
			}

			if cmd.Flags().Changed(deployEnvFlag) {
				opts.deployEnv = aws.Bool(false)
				if deployEnvironment {
					opts.deployEnv = aws.Bool(true)
				}
			}

			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.deployWkldVars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.deployWkldVars.name, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.deployWkldVars.forceNewUpdate, forceFlag, false, forceFlagDescription)
	cmd.Flags().BoolVar(&vars.deployWkldVars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.allowWkldDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.deployWkldVars.detach, detachFlag, false, detachFlagDescription)

	cmd.Flags().BoolVar(&deployEnvironment, deployEnvFlag, false, deployEnvFlagDescription)
	cmd.Flags().BoolVar(&initEnvironment, yesInitEnvFlag, false, yesInitEnvFlagDescription)
	cmd.Flags().BoolVar(&initWorkload, yesInitWorkloadFlag, false, yesInitWorkloadFlagDescription)

	cmd.Flags().StringVar(&vars.profile, profileFlag, "", profileFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.AccessKeyID, accessKeyIDFlag, "", accessKeyIDFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.SecretAccessKey, secretAccessKeyFlag, "", secretAccessKeyFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.SessionToken, sessionTokenFlag, "", sessionTokenFlagDescription)
	cmd.Flags().StringVar(&vars.region, regionFlag, "", envRegionTokenFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Release,
	}
	return cmd
}
