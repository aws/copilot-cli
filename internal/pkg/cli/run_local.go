// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	sdksecretsmanager "github.com/aws/aws-sdk-go/service/secretsmanager"
	sdkssm "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/orchestrator"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	termcolor "github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/term/syncbuffer"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/fatih/color"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	workloadAskPrompt = "Which workload would you like to run locally?"

	pauseContainerURI  = "public.ecr.aws/amazonlinux/amazonlinux:2023"
	pauseContainerName = "pause"
)

type containerOrchestrator interface {
	Start() chan error
	RunTask(task orchestrator.Task)
	Stop()
}

type runLocalVars struct {
	wkldName      string
	wkldType      string
	appName       string
	envName       string
	envOverrides  map[string]string
	portOverrides portOverrides
}

type runLocalOpts struct {
	runLocalVars

	sel             deploySelector
	ecsLocalClient  ecsLocalClient
	ssm             secretGetter
	secretsManager  secretGetter
	sessProvider    sessionProvider
	sess            *session.Session
	envSess         *session.Session
	targetEnv       *config.Environment
	targetApp       *config.Application
	store           store
	ws              wsWlDirReader
	cmd             execRunner
	dockerEngine    dockerEngineRunner
	repository      repositoryService
	containerSuffix string
	newColor        func() *color.Color
	prog            progress
	newOrchestrator func() containerOrchestrator

	buildContainerImages func(mft manifest.DynamicWorkload) (map[string]string, error)
	configureClients     func(o *runLocalOpts) error
	labeledTermPrinter   func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) clideploy.LabeledTermPrinter
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
}

func newRunLocalOpts(vars runLocalVars) (*runLocalOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("run local"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSess), sdkssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, store)
	if err != nil {
		return nil, err
	}

	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	labeledTermPrinter := func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) clideploy.LabeledTermPrinter {
		return syncbuffer.NewLabeledTermPrinter(fw, bufs, opts...)
	}
	opts := &runLocalOpts{
		runLocalVars:       vars,
		sel:                selector.NewDeploySelect(prompt.New(), store, deployStore),
		store:              store,
		ws:                 ws,
		newInterpolator:    newManifestInterpolator,
		sessProvider:       sessProvider,
		unmarshal:          manifest.UnmarshalWorkload,
		sess:               defaultSess,
		cmd:                exec.NewCmd(),
		dockerEngine:       dockerengine.New(exec.NewCmd()),
		labeledTermPrinter: labeledTermPrinter,
		newColor:           termcolor.ColorGenerator(),
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
	}
	opts.configureClients = func(o *runLocalOpts) error {
		defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create default session with region %s: %w", o.targetEnv.Region, err)
		}
		o.envSess, err = o.sessProvider.FromRole(o.targetEnv.ManagerRoleARN, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create env session %s: %w", o.targetEnv.Region, err)
		}

		// EnvManagerRole has permissions to get task def and get SSM values.
		// However, it doesn't have permissions to get secrets from secrets manager,
		// so use the default sess and *hope* they have permissions.
		o.ecsLocalClient = ecs.New(o.envSess)
		o.ssm = ssm.New(o.envSess)
		o.secretsManager = secretsmanager.New(defaultSessEnvRegion)

		resources, err := cloudformation.New(o.sess, cloudformation.WithProgressTracker(os.Stderr)).GetAppResourcesByRegion(o.targetApp, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("get application %s resources from region %s: %w", o.appName, o.envName, err)
		}
		repoName := clideploy.RepoName(o.appName, o.wkldName)
		o.repository = repository.NewWithURI(ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[o.wkldName])
		return nil
	}
	opts.buildContainerImages = func(mft manifest.DynamicWorkload) (map[string]string, error) {
		gitShortCommit := imageTagFromGit(opts.cmd)
		image := clideploy.ContainerImageIdentifier{
			GitShortCommitTag: gitShortCommit,
		}
		out := &clideploy.UploadArtifactsOutput{}
		if err := clideploy.BuildContainerImages(&clideploy.ImageActionInput{
			Name:               opts.wkldName,
			WorkspacePath:      opts.ws.Path(),
			Image:              image,
			Mft:                mft.Manifest(),
			GitShortCommitTag:  gitShortCommit,
			Builder:            opts.repository,
			Login:              opts.repository.Login,
			CheckDockerEngine:  opts.dockerEngine.CheckDockerEngineRunning,
			LabeledTermPrinter: opts.labeledTermPrinter,
		}, out); err != nil {
			return nil, err
		}

		containerURIs := make(map[string]string, len(out.ImageDigests))
		for name, info := range out.ImageDigests {
			if len(info.RepoTags) == 0 {
				// this shouldn't happen, but just to avoid a panic in case
				return nil, fmt.Errorf("no repo tags for image %q", name)
			}
			containerURIs[name] = info.RepoTags[0]
		}
		return containerURIs, nil
	}
	opts.newOrchestrator = func() containerOrchestrator {
		idPrefix := fmt.Sprintf("%s-%s-%s-", opts.appName, opts.envName, opts.wkldName)
		colorGen := termcolor.ColorGenerator()

		return orchestrator.New(opts.dockerEngine, idPrefix, func(name string, ctr orchestrator.ContainerDefinition) dockerengine.RunLogOptions {
			return dockerengine.RunLogOptions{
				Color:      colorGen(),
				Output:     os.Stderr,
				LinePrefix: fmt.Sprintf("[%s] ", name),
			}
		})
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *runLocalOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	// Ensure that the application name provided exists in the workspace
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}
	o.targetApp = app
	return nil
}

// Ask prompts the user for any unprovided required fields and validates them.
func (o *runLocalOpts) Ask() error {
	return o.validateAndAskWkldEnvName()
}

func (o *runLocalOpts) validateAndAskWkldEnvName() error {
	if o.envName != "" {
		env, err := o.store.GetEnvironment(o.appName, o.envName)
		if err != nil {
			return err
		}
		o.targetEnv = env
	}
	if o.wkldName != "" {
		if _, err := o.store.GetWorkload(o.appName, o.wkldName); err != nil {
			return err
		}
	}

	deployedWorkload, err := o.sel.DeployedWorkload(workloadAskPrompt, "", o.appName, selector.WithEnv(o.envName), selector.WithName(o.wkldName))
	if err != nil {
		return fmt.Errorf("select a deployed workload from application %s: %w", o.appName, err)
	}
	if o.envName == "" {
		env, err := o.store.GetEnvironment(o.appName, deployedWorkload.Env)
		if err != nil {
			return fmt.Errorf("get environment %q configuration: %w", o.envName, err)
		}
		o.targetEnv = env
	}

	o.wkldName = deployedWorkload.Name
	o.envName = deployedWorkload.Env
	o.wkldType = deployedWorkload.Type
	return nil
}

// Execute builds and runs the workload images locally.
func (o *runLocalOpts) Execute() error {
	if err := o.configureClients(o); err != nil {
		return err
	}

	ctx := context.Background()

	task, err := o.getTask(ctx)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	mft, _, err := workloadManifest(&workloadManifestInput{
		name:         o.wkldName,
		appName:      o.appName,
		envName:      o.envName,
		ws:           o.ws,
		interpolator: o.newInterpolator(o.appName, o.envName),
		unmarshal:    o.unmarshal,
		sess:         o.envSess,
	})
	if err != nil {
		return err
	}

	containerURIs, err := o.buildContainerImages(mft)
	if err != nil {
		return fmt.Errorf("build images: %w", err)
	}

	// replace built images with the local built URI
	for name, uri := range containerURIs {
		ctr, ok := task.Containers[name]
		if !ok {
			return fmt.Errorf("built an image for %q, which doesn't exist in the task", name)
		}

		ctr.ImageURI = uri
		task.Containers[name] = ctr
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	orch := o.newOrchestrator()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := orch.Start()
	orch.RunTask(task)

	for {
		select {
		case err, ok := <-errCh:
			// we loop until errCh closes, since Start()
			// closes errCh when the orchestrator is completely done.
			if !ok {
				return nil
			}

			fmt.Printf("error: %s\n", err)
			orch.Stop()
		case <-sigCh:
			signal.Stop(sigCh)
			orch.Stop()
		}
	}
}

func (o *runLocalOpts) getTask(ctx context.Context) (orchestrator.Task, error) {
	td, err := o.ecsLocalClient.TaskDefinition(o.appName, o.envName, o.wkldName)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("get task definition: %w", err)
	}

	envVars, err := o.getEnvVars(ctx, td)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("get env vars: %w", err)
	}

	task := orchestrator.Task{
		Containers: make(map[string]orchestrator.ContainerDefinition),
	}

	for _, ctr := range td.ContainerDefinitions {
		name := aws.StringValue(ctr.Name)
		def := orchestrator.ContainerDefinition{
			ImageURI: aws.StringValue(ctr.Image),
			EnvVars:  envVars[name].EnvVars(),
			Secrets:  envVars[name].Secrets(),
			Ports:    make(map[string]string, len(ctr.PortMappings)),
		}

		for _, port := range ctr.PortMappings {
			hostPort := strconv.FormatInt(aws.Int64Value(port.HostPort), 10)
			ctrPort := hostPort
			if port.ContainerPort != nil {
				ctrPort = strconv.FormatInt(aws.Int64Value(port.ContainerPort), 10)
			}

			for _, override := range o.portOverrides {
				if override.container == ctrPort {
					hostPort = override.host
					break
				}
			}

			def.Ports[hostPort] = ctrPort
		}

		task.Containers[name] = def
	}

	return task, nil
}

type containerEnv map[string]envVarValue

type envVarValue struct {
	Value    string
	Secret   bool
	Override bool
}

func (c containerEnv) EnvVars() map[string]string {
	if c == nil {
		return nil
	}

	out := make(map[string]string)
	for k, v := range c {
		if !v.Secret {
			out[k] = v.Value
		}
	}
	return out
}

func (c containerEnv) Secrets() map[string]string {
	if c == nil {
		return nil
	}

	out := make(map[string]string)
	for k, v := range c {
		if v.Secret {
			out[k] = v.Value
		}
	}
	return out
}

// getEnvVars uses env overrides passed by flags and environment variables/secrets
// specified in the Task Definition to return a set of environment varibles for each
// continer defined in the TaskDefinition. The returned map is a map of container names,
// each of which contains a mapping of key->envVarValue, which defines if the variable is a secret or not.
func (o *runLocalOpts) getEnvVars(ctx context.Context, taskDef *awsecs.TaskDefinition) (map[string]containerEnv, error) {
	creds, err := o.sess.Config.Credentials.GetWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get IAM credentials: %w", err)
	}

	envVars := make(map[string]containerEnv)
	for _, ctr := range taskDef.ContainerDefinitions {
		name := aws.StringValue(ctr.Name)
		envVars[name] = map[string]envVarValue{
			"AWS_ACCESS_KEY_ID": {
				Value: creds.AccessKeyID,
			},
			"AWS_SECRET_ACCESS_KEY": {
				Value: creds.SecretAccessKey,
			},
			"AWS_SESSION_TOKEN": {
				Value: creds.SessionToken,
			},
		}
		if o.sess.Config.Region != nil {
			val := envVarValue{
				Value: aws.StringValue(o.sess.Config.Region),
			}
			envVars[name]["AWS_DEFAULT_REGION"] = val
			envVars[name]["AWS_REGION"] = val
		}
	}

	for _, e := range taskDef.EnvironmentVariables() {
		envVars[e.Container][e.Name] = envVarValue{
			Value: e.Value,
		}
	}

	if err := o.fillEnvOverrides(envVars); err != nil {
		return nil, fmt.Errorf("parse env overrides: %w", err)
	}

	if err := o.fillSecrets(ctx, envVars, taskDef); err != nil {
		return nil, fmt.Errorf("get secrets: %w", err)
	}
	return envVars, nil
}

// fillEnvOverrides parses environment variable overrides passed via flag.
// The expected format of the flag values is KEY=VALUE, with an optional container name
// in the format of [containerName]:KEY=VALUE. If the container name is omitted,
// the environment variable override is applied to all containers in the task definition.
func (o *runLocalOpts) fillEnvOverrides(envVars map[string]containerEnv) error {
	for k, v := range o.envOverrides {
		if !strings.Contains(k, ":") {
			// apply override to all containers
			for ctr := range envVars {
				envVars[ctr][k] = envVarValue{
					Value:    v,
					Override: true,
				}
			}
			continue
		}

		// only apply override to the specified container
		split := strings.SplitN(k, ":", 2)
		ctr, key := split[0], split[1] // len(split) will always be 2 since we know there is a ":"
		if _, ok := envVars[ctr]; !ok {
			return fmt.Errorf("%q targets invalid container", k)
		}
		envVars[ctr][key] = envVarValue{
			Value:    v,
			Override: true,
		}
	}

	return nil
}

// fillSecrets collects non-overridden secrets from the task definition and
// makes requests to SSM and Secrets Manager to get their value.
func (o *runLocalOpts) fillSecrets(ctx context.Context, envVars map[string]containerEnv, taskDef *awsecs.TaskDefinition) error {
	// figure out which secrets we need to get, set value to ValueFrom
	unique := make(map[string]string)
	for _, s := range taskDef.Secrets() {
		cur, ok := envVars[s.Container][s.Name]
		if cur.Override {
			// ignore secrets that were overridden
			continue
		}
		if ok {
			return fmt.Errorf("secret names must be unique, but an environment variable %q already exists", s.Name)
		}

		envVars[s.Container][s.Name] = envVarValue{
			Value:  s.ValueFrom,
			Secret: true,
		}
		unique[s.ValueFrom] = ""
	}

	// get value of all needed secrets
	g, ctx := errgroup.WithContext(ctx)
	mu := &sync.Mutex{}
	mu.Lock() // lock until finished ranging over unique
	for valueFrom := range unique {
		valueFrom := valueFrom
		g.Go(func() error {
			val, err := o.getSecret(ctx, valueFrom)
			if err != nil {
				return fmt.Errorf("get secret %q: %w", valueFrom, err)
			}

			mu.Lock()
			defer mu.Unlock()
			unique[valueFrom] = val
			return nil
		})
	}
	mu.Unlock()
	if err := g.Wait(); err != nil {
		return err
	}

	// replace secrets with resolved values
	for ctr, vars := range envVars {
		for key, val := range vars {
			if val.Secret {
				envVars[ctr][key] = envVarValue{
					Value:  unique[val.Value],
					Secret: true,
				}
			}
		}
	}

	return nil
}

func (o *runLocalOpts) getSecret(ctx context.Context, valueFrom string) (string, error) {
	// SSM secrets can be specified as parameter name instead of an ARN.
	// Default to ssm if valueFrom is not an ARN.
	getter := o.ssm
	if parsed, err := arn.Parse(valueFrom); err == nil { // only overwrite if successful
		switch parsed.Service {
		case sdkssm.ServiceName:
			getter = o.ssm
		case sdksecretsmanager.ServiceName:
			getter = o.secretsManager
		default:
			return "", fmt.Errorf("invalid ARN; not a SSM or Secrets Manager ARN")
		}
	}

	return getter.GetSecretValue(ctx, valueFrom)
}

// BuildRunLocalCmd builds the command for running a workload locally
func BuildRunLocalCmd() *cobra.Command {
	vars := runLocalVars{}
	cmd := &cobra.Command{
		Use:   "run local",
		Short: "Run the workload locally.",
		Long:  "Run the workload locally.",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newRunLocalOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
		Annotations: map[string]string{
			"group": group.Develop,
		},
	}
	cmd.SetUsageTemplate(template.Usage)

	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().Var(&vars.portOverrides, portOverrideFlag, portOverridesFlagDescription)
	cmd.Flags().StringToStringVar(&vars.envOverrides, envVarOverrideFlag, nil, envVarOverrideFlagDescription)
	return cmd
}
