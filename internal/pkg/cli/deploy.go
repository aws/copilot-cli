// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"slices"
	"sort"
	"strconv"
	"strings"

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
	"github.com/aws/copilot-cli/internal/pkg/term/color"
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

	workloadNames      []string
	deployAllWorkloads bool

	yesInitWkld *bool
	deployEnv   *bool
	yesInitEnv  *bool

	region    string
	tempCreds tempCredsVars
	profile   string
}

type deployOpts struct {
	deployVars

	newWorkloadAdder func() wkldInitializerWithoutManifest
	setupDeployCmd   func(*deployOpts, string, string) (actionCommand, error)

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

	// Cached variables
	wsEnvironments         []string
	wsWorkloads            []string
	storeWorkloads         []*config.Workload
	initializedWsWorkloads []string
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
			return newEnvDeployOpts(deployEnvVars{
				appName:           o.appName,
				name:              o.envName,
				forceNewUpdate:    o.forceNewUpdate,
				disableRollback:   o.disableRollback,
				showDiff:          o.showDiff,
				skipDiffPrompt:    o.skipDiffPrompt,
				allowEnvDowngrade: o.allowWkldDowngrade,
				detach:            o.detach,
			})
		},

		newInitEnvCmd: func(o *deployOpts) (cmd, error) {
			// This vars struct sets "default config" so that no vpc questions are asked during env init and the manifest
			// is not written. It passes in credential flags and allow-downgrade from the parent command.
			return newInitEnvOpts(initEnvVars{
				appName:           o.appName,
				name:              o.envName,
				profile:           o.profile,
				defaultConfig:     true,
				allowAppDowngrade: o.allowWkldDowngrade,
				tempCreds:         o.tempCreds,
				region:            o.region,
			})
		},

		setupDeployCmd: func(o *deployOpts, workloadName, workloadType string) (actionCommand, error) {
			switch {
			case slices.Contains(manifestinfo.JobTypes(), workloadType):
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
				opts.name = workloadName
				return opts, nil
			case slices.Contains(manifestinfo.ServiceTypes(), workloadType):
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
				opts.name = workloadName
				return opts, nil
			}
			return nil, fmt.Errorf("unrecognized workload type %s", workloadType)
		},
	}, nil
}

func (o *deployOpts) maybeInitWkld(name string) error {
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
	if slices.Contains(wlNames, name) {
		return nil
	}

	// Get workload type and confirm readable manifest.
	mf, err := o.ws.ReadWorkloadManifest(name)
	if err != nil {
		return fmt.Errorf("read manifest for workload %s: %w", name, err)
	}
	workloadType, err := mf.WorkloadType()
	if err != nil {
		return fmt.Errorf("get workload type from manifest for workload %s: %w", name, err)
	}

	if !slices.Contains(manifestinfo.WorkloadTypes(), workloadType) {
		return fmt.Errorf("unrecognized workload type %q in manifest for workload %s", workloadType, name)
	}

	if o.yesInitWkld == nil {
		confirmInitWkld, err := o.prompt.Confirm(fmt.Sprintf("Found manifest for uninitialized %s %q. Initialize it?", workloadType, name), "This workload will be initialized, then deployed.", prompt.WithFinalMessage(fmt.Sprintf("Initialize %s:", workloadType)))
		if err != nil {
			return fmt.Errorf("confirm initialize workload: %w", err)
		}
		o.yesInitWkld = aws.Bool(confirmInitWkld)
	}

	if !aws.BoolValue(o.yesInitWkld) {
		return fmt.Errorf("workload %s is uninitialized but --%s=false was specified", name, yesInitWorkloadFlag)
	}

	wkldAdder := o.newWorkloadAdder()
	if err = wkldAdder.AddWorkloadToApp(o.appName, name, workloadType); err != nil {
		return fmt.Errorf("add workload to app: %w", err)
	}
	return nil
}

func parseDeploymentOrderTags(namesWithOptionalOrder []string) (map[string]int, error) {
	prioritiesMap := make(map[string]int)
	// First pass through flags to identify priority groups
	for _, wkldName := range namesWithOptionalOrder {
		parts := strings.Split(wkldName, "/")
		// If there's no priority tag, deploy this service after everything else. Signify this with -1 priority.
		// If there's a valid tag, add it to the map of priorities.
		if len(parts) == 1 {
			prioritiesMap[parts[0]] = -1
		} else if len(parts) == 2 {
			order, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("parse deployment order for workloads: %w", err)
			}
			prioritiesMap[parts[0]] = order
		} else {
			return nil, fmt.Errorf("invalid deployment order for workload %s", wkldName)
		}
	}
	return prioritiesMap, nil
}

// getDeploymentOrder parses names and tags from the --name flag, respecting "all".
// It returns an ordered list of which workloads should be deployed and when.
// For example, when a customer specifies "fe/2,be/1,worker,job" and the --all flag
// in a workspace where there also exists a `db` service, this function will return
//
//	[][]string{ {"be"}, {"fe"}, {"worker", "job", "db"} }.
//
// TODO: when there's a dependsOn field in the manifest, we should modify this function to respect it.
func (o *deployOpts) getDeploymentOrder() ([][]string, error) {
	// Get a map from workload name to deployment priority
	prioritiesMap, err := parseDeploymentOrderTags(o.workloadNames)
	if err != nil {
		return nil, err
	}

	// Iterate over priority map to invert it and get groups of workloads with the same priority.
	groupsMap := make(map[int][]string)
	for k, v := range prioritiesMap {
		groupsMap[v] = append(groupsMap[v], k)
	}

	// If --all is specified, we need to add the remainder of workloads with -1 priority according to whether or not --init-wkld is specified.
	if o.deployAllWorkloads {
		specifiedWorkloadList := make([]string, 0, len(prioritiesMap))
		for k := range prioritiesMap {
			specifiedWorkloadList = append(specifiedWorkloadList, k)
		}

		if o.yesInitWkld != nil && !aws.BoolValue(o.yesInitWkld) {
			// --all and --init-wkld=false: get only get initialized local workloads.
			initializedWorkloads, err := o.listInitializedLocalWorkloads()
			if err != nil {
				return nil, err
			}
			workloadsToAppend := selector.FilterOutItems(initializedWorkloads, specifiedWorkloadList, func(s string) string { return s })
			if len(workloadsToAppend) != 0 {
				groupsMap[-1] = append(groupsMap[-1], workloadsToAppend...)
			}
		} else {
			// otherwise, add all unspecified local workloads.
			localWorkloads, err := o.listLocalWorkloads()
			if err != nil {
				return nil, err
			}
			workloadsToAppend := selector.FilterOutItems(localWorkloads, specifiedWorkloadList, func(s string) string { return s })
			if len(workloadsToAppend) != 0 {
				groupsMap[-1] = append(groupsMap[-1], workloadsToAppend...)
			}
		}
	}

	type workloadPriority struct {
		priority  int
		workloads []string
	}
	deploymentGroups := make([]workloadPriority, len(groupsMap))
	i := 0
	for k, v := range groupsMap {
		deploymentGroups[i] = workloadPriority{
			priority:  k,
			workloads: v,
		}
		i++
	}
	// sort by priority
	sort.Slice(deploymentGroups, func(i, j int) bool { return deploymentGroups[i].priority < deploymentGroups[j].priority })

	res := make([][]string, len(deploymentGroups))
	for i, g := range deploymentGroups {
		res[i] = g.workloads
	}
	if deploymentGroups[0].priority != -1 {
		return res, nil
	}

	// rotate the array to the left one element so that the -1 priority appears at the end.
	first := res[0]
	for i := 1; i < len(res); i++ {
		res[i-1] = res[i]
	}
	res[len(res)-1] = first

	return res, nil
}

func (o *deployOpts) listStoreWorkloads() ([]*config.Workload, error) {
	if o.storeWorkloads == nil {
		wls, err := o.store.ListWorkloads(o.appName)
		if err != nil {
			return nil, fmt.Errorf("retrieve store workloads: %w", err)
		}
		o.storeWorkloads = wls
	}
	return o.storeWorkloads, nil
}

func (o *deployOpts) listLocalWorkloads() ([]string, error) {
	if o.wsWorkloads == nil {
		localWorkloads, err := o.ws.ListWorkloads()
		if err != nil {
			return nil, fmt.Errorf("retrieve workspace workloads: %w", err)
		}
		o.wsWorkloads = localWorkloads
	}

	return o.wsWorkloads, nil
}

func (o *deployOpts) listInitializedLocalWorkloads() ([]string, error) {
	if o.initializedWsWorkloads == nil {
		storeWls, err := o.listStoreWorkloads()
		if err != nil {
			return nil, err
		}
		localWorkloads, err := o.listLocalWorkloads()
		if err != nil {
			return nil, err
		}

		o.initializedWsWorkloads = selector.FilterItemsByStrings(localWorkloads, storeWls, func(workload *config.Workload) string { return workload.Name })
	}
	return o.initializedWsWorkloads, nil
}

type workloadCommand struct {
	name string
	cmd  actionCommand
}

func (o *deployOpts) Run() error {
	if err := o.askNames(); err != nil {
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

	if err := o.maybeDeployEnv(); err != nil {
		return err
	}

	deploymentOrderGroups, err := o.getDeploymentOrder()
	if err != nil {
		return err
	}

	cmds := make([][]workloadCommand, len(deploymentOrderGroups))
	// Do all our asking before executing deploy commands.
	log.Infoln("Checking for all required information. We may ask you some questions.")
	for i, deploymentGroup := range deploymentOrderGroups {
		for _, workload := range deploymentGroup {
			if err := o.maybeInitWkld(workload); err != nil {
				return err
			}
			deployCmd, err := o.loadWkldCmd(workload)
			if err != nil {
				return err
			}
			cmds[i] = append(cmds[i], workloadCommand{
				name: workload,
				cmd:  deployCmd,
			})
			if err := deployCmd.Ask(); err != nil {
				return fmt.Errorf("ask %s deploy: %w", o.wlType, err)
			}
			if err := deployCmd.Validate(); err != nil {
				return fmt.Errorf("validate %s deploy: %w", o.wlType, err)
			}
		}
	}

	// Count number of deployments.
	count := 0
	for i := 0; i < len(cmds); i++ {
		count += len(cmds[i])
	}
	log.Infof("Will deploy %d %s in the following order.\n", count, english.PluralWord(count, "workload", ""))
	for i := 0; i < len(cmds); i++ {
		names := ""
		for _, cmd := range cmds[i] {
			names += cmd.name + " "
		}
		log.Infof("%d. %s\n", i+1, names)
	}

	totalNumDeployed := 1
	for _, deploymentGroup := range cmds {
		// TODO parallelize this. Steps involve modifying the cmd to provide an option to
		// disable the progress tracker, and to create several syncbuffers to hold spinners.
		for g, cmd := range deploymentGroup {
			if err := cmd.cmd.Execute(); err != nil {
				return fmt.Errorf("execute deployment %d of %d in group %d: %w", totalNumDeployed, len(deploymentGroup), g+1, err)
			}
			if err := cmd.cmd.RecommendActions(); err != nil {
				return err
			}
			totalNumDeployed++
		}
	}

	return nil
}

func (o *deployOpts) askNames() error {
	if o.workloadNames != nil || len(o.workloadNames) != 0 {
		return nil
	}

	if !o.deployAllWorkloads {
		names, err := o.sel.Workloads("Select a service or job in your workspace", "")
		if err != nil {
			return fmt.Errorf("select service or job: %w", err)
		}
		o.workloadNames = names
		return nil
	}

	// --all and --init-wkld=false means we should only use the initialized local workloads.
	if o.yesInitWkld != nil && !aws.BoolValue(o.yesInitWkld) {
		o.workloadNames = o.initializedWsWorkloads
		return nil
	}

	// --all and --init-wkld=true, or --init-wkld unspecified, means we should use ALL local workloads as our list of names.
	o.workloadNames = o.wsWorkloads
	return nil
}

func (o *deployOpts) listWsEnvironments() ([]string, error) {
	if o.wsEnvironments == nil {
		envs, err := o.ws.ListEnvironments()
		if err != nil {
			return nil, err
		}
		if len(envs) == 0 {
			envs = []string{}
		}
		o.wsEnvironments = envs
		return o.wsEnvironments, nil
	}
	return o.wsEnvironments, nil
}

func (o *deployOpts) askEnv() error {
	if o.envName != "" {
		return nil
	}
	localEnvs, err := o.listWsEnvironments()
	if err != nil {
		return fmt.Errorf("get workspace environments: %w", err)
	}
	initializedEnvs, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return fmt.Errorf("get initialized environments: %w", err)
	}

	// Get uninitialized local environments and append them to the env selector call.
	var extraOptions []prompt.Option
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
		extraOptions = append(extraOptions, prompt.Option{Value: localEnv, Hint: "uninitialized"})
	}

	o.envName, err = o.sel.Environment("Select an environment to deploy to", "", o.appName, extraOptions...)
	if err != nil {
		return fmt.Errorf("get environment name: %w", err)
	}
	return nil
}

// checkEnvExists checks whether the environment is initialized and has a local manifest.
func (o *deployOpts) checkEnvExists() error {
	o.envExistsInApp = true
	_, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		var errNotFound *config.ErrNoSuchEnvironment
		if !errors.As(err, &errNotFound) {
			return fmt.Errorf("get environment from config store: %w", err)
		}
		o.envExistsInApp = false
	}
	envs, err := o.listWsEnvironments()
	if err != nil {
		return fmt.Errorf("list environments in workspace: %w", err)
	}
	o.envExistsInWs = slices.Contains(envs, o.envName)

	// the desired environment doesn't actually exist.
	if !o.envExistsInApp && !o.envExistsInWs {
		log.Errorf("Environment %q does not exist in the current application or workspace. Please initialize it by running %s.\n", o.envName, color.HighlightCode("copilot env init"))
		return fmt.Errorf("environment %q does not exist in the workspace", o.envName)
	}
	if o.envExistsInApp && !o.envExistsInWs {
		log.Infof("Manifest for environment %q does not exist in the current workspace. To deploy this environment, generate a manifest with %s", o.envName, color.HighlightCode("copilot env show --manifest"))
	}

	return nil
}

func (o *deployOpts) maybeInitEnv() error {
	if o.envExistsInApp {
		return nil
	}

	// If no initialization flags were specified and the env wasn't initialized, ask to confirm.
	if !o.envExistsInApp && o.yesInitEnv == nil {
		v, err := o.prompt.Confirm(fmt.Sprintf("Environment %q does not exist in app %q. Initialize it?", o.envName, o.appName), "")
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
		if err = cmd.Execute(); err != nil {
			return err
		}
		if o.deployEnv == nil {
			log.Infof("Environment %q was just initialized. We'll deploy it now.\n", o.envName)
			o.deployEnv = aws.Bool(true)
		} else if !aws.BoolValue(o.deployEnv) {
			log.Errorf("Environment is not deployed but --%s=false was specified. Deploy the environment with %s in order to deploy a workload to it.\n", deployEnvFlag, color.HighlightCode("copilot env deploy"))
			return fmt.Errorf("environment %s was initialized but has not been deployed", o.envName)
		}
		return nil
	}
	log.Errorf("Environment %q does not exist in application %q and was not initialized after prompting.\n", o.envName, o.appName)
	return fmt.Errorf("env %s does not exist in app %s", o.envName, o.appName)
}

func (o *deployOpts) maybeDeployEnv() error {
	if !o.envExistsInWs {
		return nil
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

func (o *deployOpts) loadWkldCmd(name string) (actionCommand, error) {
	wl, err := o.store.GetWorkload(o.appName, name)
	if err != nil {
		return nil, fmt.Errorf("retrieve %s from application %s: %w", o.appName, name, err)
	}
	cmd, err := o.setupDeployCmd(o, name, wl.Type)
	if err != nil {
		return nil, err
	}
	if slices.Contains(manifestinfo.JobTypes(), wl.Type) {
		o.wlType = jobWkldType
		return cmd, nil
	}
	o.wlType = svcWkldType
	return cmd, nil
}

// BuildDeployCmd is the deploy command.
func BuildDeployCmd() *cobra.Command {
	vars := deployVars{}
	var initWorkload bool
	var initEnvironment bool
	var deployEnvironment bool
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy one or more Copilot jobs or services.",
		Long:  "Deploy one or more Copilot jobs or services.",
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot deploy --name frontend --env test --deploy-env=false
  Deploys a job named "mailer" with additional resource tags to a "prod" environment.
  /code $ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual --deploy-env=false
  Initializes and deploys an environment named "test" in us-west-2 under the "default" profile with local manifest, 
    then deploys a service named "api"
  /code $ copilot deploy --init-env --deploy-env --env test --name api --profile default --region us-west-2
  Initializes and deploys a service named "backend" to a "prod" environment.
  /code $ copilot deploy --init-wkld --deploy-env=false --env prod --name backend
  Deploys all local, initialized workloads in no particular order.
  /code $ copilot deploy --all --env prod --name backend --init-wkld=false
  Deploys multiple workloads in a prescribed order (fe and worker, then be).
  /code $ copilot deploy -n fe/1 -n be/2 -n worker/1`,

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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringSliceVarP(&vars.workloadNames, nameFlag, nameFlagShort, nil, workloadsFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.allowWkldDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.detach, detachFlag, false, detachFlagDescription)

	cmd.Flags().BoolVar(&deployEnvironment, deployEnvFlag, false, deployEnvFlagDescription)
	cmd.Flags().BoolVar(&initEnvironment, yesInitEnvFlag, false, yesInitEnvFlagDescription)
	cmd.Flags().BoolVar(&initWorkload, yesInitWorkloadFlag, false, yesInitWorkloadFlagDescription)
	cmd.Flags().BoolVar(&vars.deployAllWorkloads, allFlag, false, allWorkloadsFlagDescription)

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
