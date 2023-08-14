// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
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

type localRunVars struct {
	wkldName string
	wkldType string
	appName  string
	envName  string
}

type localRunOpts struct {
	localRunVars

	sel               deploySelector
	ecsLocalClient    ecsLocalClient
	sessProvider      sessionProvider
	sess              *session.Session
	targetEnv         *config.Environment
	targetApp         *config.Application
	store             store
	ws                wsWlDirReader
	cmd               execRunner
	dockerEngine      dockerEngineRunner
	repository        repositoryService
	appliedDynamicMft manifest.DynamicWorkload
	out               clideploy.UploadArtifactsOutput
	imageInfoList     []clideploy.ImagePerContainer
	containerSuffix   string
	newColor          func() *color.Color

	buildContainerImages func(o *localRunOpts) error
	configureClients     func(o *localRunOpts) error
	labeledTermPrinter   func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) clideploy.LabeledTermPrinter
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
}

func newLocalRunOpts(vars localRunVars) (*localRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("local run"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
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
	opts := &localRunOpts{
		localRunVars:       vars,
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
		newColor:           colorGenerator(),
	}
	opts.configureClients = func(o *localRunOpts) error {
		defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create default session with region %s: %w", o.targetEnv.Region, err)
		}
		envSess, err := o.sessProvider.FromRole(o.targetEnv.ManagerRoleARN, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create env session %s: %w", o.targetEnv.Region, err)
		}

		o.ecsLocalClient = ecs.New(envSess)

		resources, err := cloudformation.New(o.sess, cloudformation.WithProgressTracker(os.Stderr)).GetAppResourcesByRegion(o.targetApp, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("get application %s resources from region %s: %w", o.appName, o.envName, err)
		}
		repoName := clideploy.RepoName(o.appName, o.wkldName)
		o.repository = repository.NewWithURI(ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[o.wkldName])
		return nil
	}
	opts.buildContainerImages = func(o *localRunOpts) error {
		gitShortCommit := imageTagFromGit(o.cmd)
		image := clideploy.ContainerImageIdentifier{
			GitShortCommitTag: gitShortCommit,
		}
		return clideploy.BuildContainerImages(&clideploy.ImageActionInput{
			Name:               o.wkldName,
			WorkspacePath:      o.ws.Path(),
			Image:              image,
			Mft:                o.appliedDynamicMft.Manifest(),
			GitShortCommitTag:  gitShortCommit,
			Builder:            o.repository,
			Login:              o.repository.Login,
			CheckDockerEngine:  o.dockerEngine.CheckDockerEngineRunning,
			LabeledTermPrinter: o.labeledTermPrinter}, &o.out)
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *localRunOpts) Validate() error {
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
func (o *localRunOpts) Ask() error {
	return o.validateAndAskWkldEnvName()
}

func (o *localRunOpts) validateAndAskWkldEnvName() error {
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
	o.containerSuffix = o.getContainerSuffix()
	return nil
}

// Execute builds and runs the workload images locally.
func (o *localRunOpts) Execute() error {
	if err := o.configureClients(o); err != nil {
		return err
	}

	taskDef, err := o.ecsLocalClient.TaskDefinition(o.appName, o.envName, o.wkldName)
	if err != nil {
		return fmt.Errorf("get task definition: %w", err)
	}

	secrets := taskDef.Secrets()
	decryptedSecrets, err := o.ecsLocalClient.DecryptedSecrets(secrets)
	if err != nil {
		return fmt.Errorf("get secret values: %w", err)
	}

	secretsList := make(map[string]string, len(decryptedSecrets))
	for _, s := range decryptedSecrets {
		secretsList[s.Name] = s.Value
	}

	envVars := make(map[string]string, len(taskDef.EnvironmentVariables()))
	for _, e := range taskDef.EnvironmentVariables() {
		envVars[e.Name] = e.Value
	}

	containerPorts := make(map[string]string, len(taskDef.ContainerDefinitions))
	for _, container := range taskDef.ContainerDefinitions {
		for _, portMapping := range container.PortMappings {
			hostPort := strconv.FormatInt(aws.Int64Value(portMapping.HostPort), 10)

			containerPort := hostPort
			if portMapping.ContainerPort == nil {
				containerPort = strconv.FormatInt(aws.Int64Value(portMapping.ContainerPort), 10)
			}
			containerPorts[hostPort] = containerPort
		}
	}

	envSess, err := o.sessProvider.FromRole(o.targetEnv.ManagerRoleARN, o.targetEnv.Region)
	if err != nil {
		return fmt.Errorf("get env session: %w", err)
	}
	mft, err := workloadManifest(&workloadManifestInput{
		name:         o.wkldName,
		appName:      o.appName,
		envName:      o.envName,
		interpolator: o.newInterpolator(o.appName, o.envName),
		ws:           o.ws,
		unmarshal:    o.unmarshal,
		sess:         envSess,
	})
	if err != nil {
		return err
	}
	o.appliedDynamicMft = mft

	if err := o.buildContainerImages(o); err != nil {
		return err
	}

	for name, imageInfo := range o.out.ImageDigests {
		o.imageInfoList = append(o.imageInfoList, clideploy.ImagePerContainer{
			ContainerName: name,
			ImageURI:      imageInfo.ImageName,
		})
	}

	var sidecarImageLocations []clideploy.ImagePerContainer //sidecarImageLocations has the image locations which are already built
	manifestContent := o.appliedDynamicMft.Manifest()
	switch t := manifestContent.(type) {
	case *manifest.ScheduledJob:
		sidecarImageLocations = getBuiltSidecarImageLocations(t.Sidecars)
	case *manifest.LoadBalancedWebService:
		sidecarImageLocations = getBuiltSidecarImageLocations(t.Sidecars)
	case *manifest.WorkerService:
		sidecarImageLocations = getBuiltSidecarImageLocations(t.Sidecars)
	case *manifest.BackendService:
		sidecarImageLocations = getBuiltSidecarImageLocations(t.Sidecars)
	}
	o.imageInfoList = append(o.imageInfoList, sidecarImageLocations...)

	err = o.runPauseContainer(context.Background(), containerPorts)
	if err != nil {
		return err
	}

	err = o.runContainers(context.Background(), o.imageInfoList, secretsList, envVars)
	if err != nil {
		return err
	}

	return nil
}

func (o *localRunOpts) getContainerSuffix() string {
	return fmt.Sprintf("%s-%s-%s", o.appName, o.envName, o.wkldName)
}

func getBuiltSidecarImageLocations(sidecars map[string]*manifest.SidecarConfig) []clideploy.ImagePerContainer {
	//get the image location for the sidecars which are already built and are in a registry
	var sideCarBuiltImageLocations []clideploy.ImagePerContainer
	for sideCarName, sidecar := range sidecars {
		if uri, hasLocation := sidecar.ImageURI(); hasLocation {
			sideCarBuiltImageLocations = append(sideCarBuiltImageLocations, clideploy.ImagePerContainer{
				ContainerName: sideCarName,
				ImageURI:      uri,
			})
		}
	}
	return sideCarBuiltImageLocations
}

func (o *localRunOpts) runPauseContainer(ctx context.Context, containerPorts map[string]string) error {
	containerNameWithSuffix := fmt.Sprintf("%s-%s", pauseContainerName, o.containerSuffix)
	runOptions := &dockerengine.RunOptions{
		ImageURI:       pauseContainerURI,
		ContainerName:  containerNameWithSuffix,
		ContainerPorts: containerPorts,
		Command:        []string{"sleep", "infinity"},
		LogOptions: dockerengine.RunLogOptions{
			Color:      o.newColor(),
			LinePrefix: "[pause] ",
		},
	}

	//channel to receive any error from the goroutine
	errCh := make(chan error, 1)

	go func() {
		errCh <- o.dockerEngine.Run(ctx, runOptions)
	}()

	// go routine to check if pause container is running
	go func() {
		for {
			isRunning, err := o.dockerEngine.IsContainerRunning(containerNameWithSuffix)
			if err != nil {
				errCh <- fmt.Errorf("check if pause container is running: %w", err)
				return
			}
			if isRunning {
				errCh <- nil
				return
			}
			// If the container isn't running yet, sleep for a short duration before checking again.
			time.Sleep(time.Second)
		}
	}()
	err := <-errCh
	if err != nil {
		return fmt.Errorf("run pause container: %w", err)
	}

	return nil

}

func (o *localRunOpts) runContainers(ctx context.Context, imageInfoList []clideploy.ImagePerContainer, secrets map[string]string, envVars map[string]string) error {
	g, ctx := errgroup.WithContext(ctx)

	// Iterate over the image info list and perform parallel container runs
	for _, imageInfo := range imageInfoList {
		imageInfo := imageInfo

		containerNameWithSuffix := fmt.Sprintf("%s-%s", imageInfo.ContainerName, o.containerSuffix)
		containerNetwork := fmt.Sprintf("%s-%s", pauseContainerName, o.containerSuffix)
		// Execute each container run in a separate goroutine
		g.Go(func() error {
			runOptions := &dockerengine.RunOptions{
				ImageURI:         imageInfo.ImageURI,
				ContainerName:    containerNameWithSuffix,
				Secrets:          secrets,
				EnvVars:          envVars,
				ContainerNetwork: containerNetwork,
				LogOptions: dockerengine.RunLogOptions{
					Color:      o.newColor(),
					LinePrefix: fmt.Sprintf("[%s] ", imageInfo.ContainerName),
				},
			}
			if err := o.dockerEngine.Run(ctx, runOptions); err != nil {
				return fmt.Errorf("run container: %w", err)
			}
			return nil
		})
	}

	// Wait for all the container runs to complete
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func colorGenerator() func() *color.Color {
	// don't use red since it looks like an error
	colors := []*color.Color{
		color.New(color.FgHiGreen),
		color.New(color.FgHiYellow),
		color.New(color.FgHiBlue),
		color.New(color.FgHiMagenta),
		color.New(color.FgHiCyan),
		color.New(color.FgGreen),
		color.New(color.FgYellow),
		color.New(color.FgBlue),
	}
	i := 0
	return func() *color.Color {
		defer func() { i++ }()
		return colors[i%len(colors)]
	}
}

// BuildLocalRunCmd builds the command for running a workload locally
func BuildLocalRunCmd() *cobra.Command {
	vars := localRunVars{}
	cmd := &cobra.Command{
		Use:    "local run",
		Short:  "Run the workload locally",
		Long:   "Run the workload locally",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newLocalRunOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}
