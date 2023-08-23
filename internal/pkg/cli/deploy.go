// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	svcWkldType = "svc"
	jobWkldType = "job"
)

type deployVars struct {
	deployWkldVars
	yesInitWkld *bool
}

type deployOpts struct {
	deployVars

	deployWkld       actionCommand
	newWorkloadAdder func() wkldInitializerWithoutManifest
	setupDeployCmd   func(*deployOpts, string)

	sel    wsSelector
	store  store
	ws     wsWlDirReader
	prompt prompter

	// values for logging
	wlType string
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
	initializedWorkloads, err := o.store.ListWorkloads(o.appName)
	if err != nil {
		return fmt.Errorf("retrieve workloads: %w", err)
	}
	wlNames := make([]string, len(initializedWorkloads))
	for i := range initializedWorkloads {
		wlNames[i] = initializedWorkloads[i].Name
	}

	// Workload is already initialized. Return early.
	if contains(o.name, wlNames) {
		return nil
	}

	// Get workload type and confirm readable manifest.
	mf, err := o.ws.ReadWorkloadManifest(o.name)
	if err != nil {
		return fmt.Errorf("read manifest for workload %s: %w", o.name, err)
	}
	workloadType, err := mf.WorkloadType()
	if err != nil {
		return fmt.Errorf("get workload type from manifest for workload %s: %w", o.name, err)
	}

	if !contains(workloadType, manifestinfo.WorkloadTypes()) {
		return fmt.Errorf("unrecognized workload type %q in manifest for workload %s", workloadType, o.name)
	}

	if o.yesInitWkld == nil {
		confirmInitWkld, err := o.prompt.Confirm(fmt.Sprintf("Found manifest for uninitialized %s %q. Initialize it?", workloadType, o.name), "This workload will be initialized, then deployed.", prompt.WithConfirmFinalMessage())
		if err != nil {
			return fmt.Errorf("confirm initialize workload: %w", err)
		}
		o.yesInitWkld = aws.Bool(confirmInitWkld)
	}

	if !aws.BoolValue(o.yesInitWkld) {
		return fmt.Errorf("workload %s is uninitialized but --%s=false was specified", o.name, yesInitWorkloadFlag)
	}

	wkldAdder := o.newWorkloadAdder()
	if err = wkldAdder.AddWorkloadToApp(o.appName, o.name, workloadType); err != nil {
		return fmt.Errorf("add workload to app: %w", err)
	}
	return nil
}

func (o *deployOpts) Run() error {
	if err := o.askName(); err != nil {
		return err
	}

	if err := o.maybeInitWkld(); err != nil {
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
	if o.name != "" {
		return nil
	}
	name, err := o.sel.Workload("Select a service or job in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service or job: %w", err)
	}
	o.name = name
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
	wl, err := o.store.GetWorkload(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("retrieve %s from application %s: %w", o.appName, o.name, err)
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
			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.allowWkldDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.detach, detachFlag, false, detachFlagDescription)
	cmd.Flags().BoolVar(&initWorkload, yesInitWorkloadFlag, false, yesInitWorkloadFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Release,
	}
	return cmd
}
