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

	buildContainerImages func(o *localRunOpts) error
	configureClients     func(o *localRunOpts) (repositoryService, error)
	labeledTermPrinter   func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) clideploy.LabeledTermPrinter
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
}

type imageInfo struct {
	containerName string
	imageURI      string
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
		ecsLocalClient:     ecs.New(defaultSess),
		newInterpolator:    newManifestInterpolator,
		sessProvider:       sessProvider,
		unmarshal:          manifest.UnmarshalWorkload,
		sess:               defaultSess,
		cmd:                exec.NewCmd(),
		dockerEngine:       dockerengine.New(exec.NewCmd()),
		labeledTermPrinter: labeledTermPrinter,
		out:                clideploy.UploadArtifactsOutput{},
	}
	opts.configureClients = func(o *localRunOpts) (repositoryService, error) {
		defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnv.Region)
		if err != nil {
			return nil, fmt.Errorf("create default session with region %s: %w", o.targetEnv.Region, err)
		}
		resources, err := cloudformation.New(o.sess, cloudformation.WithProgressTracker(os.Stderr)).GetAppResourcesByRegion(o.targetApp, o.targetEnv.Region)
		if err != nil {
			return nil, fmt.Errorf("get application %s resources from region %s: %w", o.appName, o.envName, err)
		}
		repoName := clideploy.RepoName(o.appName, o.wkldName)
		repository := repository.NewWithURI(
			ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[o.wkldName])
		return repository, nil
	}
	opts.buildContainerImages = func(o *localRunOpts) error {
		gitShortCommit := imageTagFromGit(o.cmd)
		image := clideploy.ContainerImageIdentifier{
			GitShortCommitTag: gitShortCommit,
		}
		in := &clideploy.BuildImageArgs{
			Name:               o.wkldName,
			WorkspacePath:      o.ws.Path(),
			Image:              image,
			Mft:                o.appliedDynamicMft.Manifest(),
			Out:                &o.out,
			GitShortCommitTag:  gitShortCommit,
			BuildFunc:          o.repository.Build,
			Login:              o.repository.Login,
			CheckDockerEngine:  o.dockerEngine.CheckDockerEngineRunning,
			LabeledTermPrinter: o.labeledTermPrinter,
		}
		return clideploy.BuildContainerImages(in)
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *localRunOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	// Ensure that the application name provided exists in the workspace
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}
	return nil
}

// Ask prompts the user for any unprovided required fields and validates them.
func (o *localRunOpts) Ask() error {
	return o.validateAndAskWkldEnvName()
}

func (o *localRunOpts) validateAndAskWkldEnvName() error {
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
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
	o.wkldName = deployedWorkload.Name
	o.envName = deployedWorkload.Env
	o.wkldType = deployedWorkload.Type
	return nil
}

// Execute builds and runs the workload images locally.
func (o *localRunOpts) Execute() error {
	taskDef, err := o.ecsLocalClient.TaskDefinition(o.appName, o.envName, o.wkldName)
	if err != nil {
		return fmt.Errorf("get task definition: %w", err)
	}

	secrets := taskDef.Secrets()
	decryptedSecrets, err := o.ecsLocalClient.DecryptedSecrets(secrets)
	if err != nil {
		return fmt.Errorf("get secret values: %w", err)
	}

	envVars := make(map[string]string)
	envVariables := taskDef.EnvironmentVariables()
	for _, envVariable := range envVariables {
		envVars[envVariable.Name] = envVariable.Value
	}

	containerPorts := make(map[string]string)
	containerdef := taskDef.ContainerDefinitions
	for _, container := range containerdef {
		for _, portMapping := range container.PortMappings {
			hostPort := aws.Int64Value(portMapping.HostPort)
			hostPortStr := strconv.FormatInt(hostPort, 10)
			containerport := aws.Int64Value(portMapping.ContainerPort)
			containerportStr := strconv.FormatInt(containerport, 10)
			containerPorts[hostPortStr] = containerportStr
		}
	}

	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %q configuration: %w", o.envName, err)
	}
	o.targetEnv = env

	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %q configuration: %w", o.appName, err)
	}
	o.targetApp = app

	envSess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
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
	o.repository, err = o.configureClients(o)
	if err != nil {
		return err
	}

	err = o.buildContainerImages(o)
	if err != nil {
		return fmt.Errorf("building container image: %w", err)
	}

	containerNames := o.out.ContainerNames
	imageNames := o.out.ImageNames

	secretsList := make(map[string]string)
	for _, s := range decryptedSecrets {
		secretsList[s.Name] = s.Value
	}

	var imageInfoList []imageInfo
	for i, image := range imageNames {
		imageInfo := imageInfo{
			containerName: containerNames[i],
			imageURI:      image,
		}
		imageInfoList = append(imageInfoList, imageInfo)
	}

	var sideCarListInfo []imageInfo
	manifestContent := o.appliedDynamicMft.Manifest()
	switch t := manifestContent.(type) {
	case *manifest.ScheduledJob:
		sideCarListInfo = getBuiltSideCarImages(t.Sidecars)
	case *manifest.LoadBalancedWebService:
		sideCarListInfo = getBuiltSideCarImages(t.Sidecars)
	case *manifest.WorkerService:
		sideCarListInfo = getBuiltSideCarImages(t.Sidecars)
	case *manifest.BackendService:
		sideCarListInfo = getBuiltSideCarImages(t.Sidecars)
	}
	imageInfoList = append(imageInfoList, sideCarListInfo...)

	err = o.runPauseContainer(containerPorts)
	if err != nil {
		return err
	}

	err = o.runContainers(imageInfoList, secretsList, envVars)
	if err != nil {
		return err
	}

	return nil
}

func getBuiltSideCarImages(sidecars map[string]*manifest.SidecarConfig) []imageInfo {
	//get the image URI for the sidecars which are in a registry
	sideCarBuiltImages := make(map[string]string)
	for sideCarName, sidecar := range sidecars {
		if uri, hasLocation := sidecar.ImageURI(); hasLocation {
			sideCarBuiltImages[sideCarName] = uri
		}
	}
	var sideCarBuiltImageInfo []imageInfo
	for sidecarName, image := range sideCarBuiltImages {
		imageInfo := imageInfo{
			containerName: sidecarName,
			imageURI:      image,
		}
		sideCarBuiltImageInfo = append(sideCarBuiltImageInfo, imageInfo)
	}
	return sideCarBuiltImageInfo
}

func (o *localRunOpts) runPauseContainer(containerPorts map[string]string) error {
	runOptions := &dockerengine.RunOptions{
		ImageURI:       pauseContainerURI,
		ContainerName:  pauseContainerName,
		ContainerPorts: containerPorts,
	}

	//channel to receive any error from the goroutine
	errCh := make(chan error, 1)

	go func() {
		errCh <- o.dockerEngine.Run(context.Background(), runOptions)
	}()

	// go routine to check if pause container is running
	go func() {
		for {
			isRunning, err := o.dockerEngine.IsContainerRunning(pauseContainerName)
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

func (o *localRunOpts) runContainers(imageInfoList []imageInfo, secrets map[string]string, envVars map[string]string) error {
	var errGroup errgroup.Group

	// Iterate over the image info list and perform parallel container runs
	for _, imageInfo := range imageInfoList {
		imageInfo := imageInfo

		// Execute each container run in a separate goroutine
		errGroup.Go(func() error {
			runOptions := &dockerengine.RunOptions{
				ImageURI:      imageInfo.imageURI,
				ContainerName: imageInfo.containerName,
				Secrets:       secrets,
				EnvVars:       envVars,
			}
			if err := o.dockerEngine.Run(context.Background(), runOptions); err != nil {
				return fmt.Errorf("run container: %w", err)
			}
			return nil
		})
	}

	// Wait for all the container runs to complete
	if err := errGroup.Wait(); err != nil {
		return err
	}
	return nil
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
