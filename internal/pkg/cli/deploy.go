// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"container/heap"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/dustin/go-humanize/english"
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
				// Multi-deployments can have flags specified which are not compatible with all service types.
				// Currently only forceNewUpdate is incompatible with Static Site types.
				if workloadType == manifestinfo.StaticSiteType {
					opts.forceNewUpdate = false
				}
				return opts, nil
			}
			return nil, fmt.Errorf("unrecognized workload type %s", workloadType)
		},
	}, nil
}

// maybeInitWkld decides whether a workload needs to be initialized before deployment.
// We do not prompt for workload initialization; specifying the workload by name suffices
// to convey the customer intention. When the customer specifies --all and --init-wkld,
// we will add all un-initialized local workloads to the list to be deployed.
// When the customer does not specify --init-wkld with --all, we will only deploy initialized workloads.
func (o *deployOpts) maybeInitWkld(name string) error {
	// Confirm that the workload needs to be initialized after asking for the name.
	initializedWorkloads, err := o.listInitializedLocalWorkloads()
	if err != nil {
		return err
	}

	// Workload is already initialized. Return early.
	if slices.Contains(initializedWorkloads, name) {
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

	wkldAdder := o.newWorkloadAdder()
	if err = wkldAdder.AddWorkloadToApp(o.appName, name, workloadType); err != nil {
		return fmt.Errorf("add workload to app: %w", err)
	}
	return nil
}

type workloadPriority struct {
	priority  int
	workloads []string
}
type pq []workloadPriority

// These methods satisfy heap.Interface.

// Len returns the length of the data structure.
func (p pq) Len() int { return len(p) }

// Less returns the lesser of two elements in the array, compared by priority.
func (p pq) Less(i, j int) bool { return p[i].priority < p[j].priority }

// Swap swaps the positions of two elements in the priority queue.
func (p pq) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// Push appends a new element to the back of the underlying array.
func (p *pq) Push(x any) {
	*p = append(*p, x.(workloadPriority))
}

// Pop removes the last element from the array.
func (p *pq) Pop() any {
	old := *p
	n := len(old)
	res := old[n-1]
	*p = old[:n-1]
	return res
}

var _ heap.Interface = (*pq)(nil)

// parseDeploymentOrderTags takes a list of workload names, optionally tagged with a priority. Lower priorities will be
// deployed first.
//
//	[]string{"fe/1", "be/1", "worker/2"}
//
// It returns a map from the workload name to its priority, and errors out if the order tag is incorrectly formatted.
//
//	map[string]int{"fe": 1, "be": 1, "worker": 2}
//
// It is possible to have multiple workloads of the same priority. We don't guarantee that workloads within priority
// groups will have identical deployment orders each time.
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

	deploymentGroups := make(pq, 0, len(groupsMap))
	heap.Init(&deploymentGroups)
	for k, v := range groupsMap {
		heap.Push(&deploymentGroups, workloadPriority{
			priority:  k,
			workloads: v,
		})

	}

	res := make([][]string, 0, len(deploymentGroups))
	sortedDeploymentGroups := make([]workloadPriority, 0, len(deploymentGroups))

	for deploymentGroups.Len() > 0 {
		v := heap.Pop(&deploymentGroups).(workloadPriority)
		sortedDeploymentGroups = append(sortedDeploymentGroups, v)
		res = append(res, v.workloads)
	}

	if len(sortedDeploymentGroups) == 1 || sortedDeploymentGroups[0].priority != -1 {
		return res, nil
	}

	// Rotate the array to the left one element so that the -1 priority appears at the end.
	first := res[0]
	res = append(res[1:], first)

	return res, nil
}

func (o *deployOpts) listStoreWorkloads() ([]*config.Workload, error) {
	if o.storeWorkloads != nil {
		return o.storeWorkloads, nil
	}
	wls, err := o.store.ListWorkloads(o.appName)
	if err != nil {
		return nil, fmt.Errorf("retrieve store workloads: %w", err)
	}
	o.storeWorkloads = wls
	return o.storeWorkloads, nil
}

func (o *deployOpts) listLocalWorkloads() ([]string, error) {
	if o.wsWorkloads != nil {
		return o.wsWorkloads, nil
	}
	localWorkloads, err := o.ws.ListWorkloads()
	if err != nil {
		return nil, fmt.Errorf("retrieve workspace workloads: %w", err)
	}
	o.wsWorkloads = localWorkloads

	return o.wsWorkloads, nil
}

func (o *deployOpts) listInitializedLocalWorkloads() ([]string, error) {
	if o.initializedWsWorkloads != nil {
		return o.initializedWsWorkloads, nil
	}
	storeWls, err := o.listStoreWorkloads()
	if err != nil {
		return nil, err
	}
	localWorkloads, err := o.listLocalWorkloads()
	if err != nil {
		return nil, err
	}

	o.initializedWsWorkloads = selector.FilterItemsByStrings(localWorkloads, storeWls, func(workload *config.Workload) string { return workload.Name })
	return o.initializedWsWorkloads, nil
}

type workloadCommand struct {
	actionCommand
	name string
}

func getTotalNumberOfWorkloads(deploymentGroups [][]workloadCommand) int {
	count := 0
	for i := 0; i < len(deploymentGroups); i++ {
		count += len(deploymentGroups[i])
	}
	return count
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
	// Also initialize workloads that need initialization (if they're specified by name and un-initialized,
	// it means the customer probably wants to init them).
	log.Infoln("Checking for all required information. We may ask you some questions.")
	for order, deploymentGroup := range deploymentOrderGroups {
		for _, workload := range deploymentGroup {
			// 1. Decide whether the current workload needs initialization.
			if err := o.maybeInitWkld(workload); err != nil {
				return err
			}
			// 2. Set up workload command.
			deployCmd, err := o.loadWkldCmd(workload)
			if err != nil {
				return err
			}

			cmds[order] = append(cmds[order], workloadCommand{
				name:          workload,
				actionCommand: deployCmd,
			})
			// 3. Ask() and Validate() for required info.
			if err := deployCmd.Ask(); err != nil {
				return fmt.Errorf("ask %s deploy: %w", o.wlType, err)
			}
			if err := deployCmd.Validate(); err != nil {
				return fmt.Errorf("validate %s deploy: %w", o.wlType, err)
			}
		}
	}

	if getTotalNumberOfWorkloads(cmds) > 1 {
		logDeploymentOrderInfo(cmds)
	}

	for g, deploymentGroup := range cmds {
		// TODO parallelize this. Steps involve:
		// 1. Modify the cmd to optionally disable progress tracker
		// 2. Modify labeledSyncBuffer so it can display a spinner.
		// 3. Wrap Execute() in a goroutine with ErrorGroup and context
		for i, cmd := range deploymentGroup {
			if err := cmd.Execute(); err != nil {
				var errNoInfraChanges *errNoInfrastructureChanges
				if !errors.As(err, &errNoInfraChanges) {
					return fmt.Errorf("execute deployment %d of %d in group %d: %w", i+1, len(deploymentGroup), g+1, err)
				}
			}
			if err := cmd.RecommendActions(); err != nil {
				return err
			}
		}
	}

	return nil
}

func logDeploymentOrderInfo(cmds [][]workloadCommand) {
	// Count number of deployments.
	count := 0
	for i := 0; i < len(cmds); i++ {
		count += len(cmds[i])
	}
	log.Infof("Will deploy %d %s in the following order.\n", count, english.PluralWord(count, "workload", ""))
	for i := 0; i < len(cmds); i++ {
		names := ""
		for _, cmd := range cmds[i] {
			names += cmd.name + ", "
		}
		log.Infof("%d. %s\n", i+1, names)
	}
}

func (o *deployOpts) askNames() error {
	if o.workloadNames != nil || len(o.workloadNames) != 0 {
		return nil
	}

	if o.deployAllWorkloads {
		// --all and --init-wkld=false means we should only use the initialized local workloads.
		if o.yesInitWkld != nil && !aws.BoolValue(o.yesInitWkld) {
			o.workloadNames = o.initializedWsWorkloads
			return nil
		}

		// --all and --init-wkld=true, or --init-wkld unspecified, means we should use ALL local workloads as our list of names.
		o.workloadNames = o.wsWorkloads
		return nil
	}

	names, err := o.sel.Workloads("Select a service or job in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service or job: %w", err)
	}
	o.workloadNames = names
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
