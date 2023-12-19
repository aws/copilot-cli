// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/rds"
	sdksecretsmanager "github.com/aws/aws-sdk-go/service/secretsmanager"
	sdkssm "github.com/aws/aws-sdk-go/service/ssm"
	cmdtemplate "github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/ssm"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/file"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/orchestrator"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/template"
	termcolor "github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/term/syncbuffer"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	workloadAskPrompt = "Which workload would you like to run locally?"
)

const (
	// Command to retrieve container credentials with ecs exec. See more at https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html.
	// Example output: {"AccessKeyId":"ACCESS_KEY_ID","Expiration":"EXPIRATION_DATE","RoleArn":"TASK_ROLE_ARN","SecretAccessKey":"SECRET_ACCESS_KEY","Token":"SECURITY_TOKEN_STRING"}
	curlContainerCredentialsCmd = "curl 169.254.170.2$AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"
)

type containerOrchestrator interface {
	Start() <-chan error
	RunTask(orchestrator.Task, ...orchestrator.RunTaskOption)
	Stop()
}

type hostFinder interface {
	Hosts(context.Context) ([]orchestrator.Host, error)
}

type taggedResourceGetter interface {
	GetResourcesByTags(string, map[string]string) ([]*resourcegroups.Resource, error)
}

type rdsDescriber interface {
	DescribeDBInstancesPagesWithContext(context.Context, *rds.DescribeDBInstancesInput, func(*rds.DescribeDBInstancesOutput, bool) bool, ...request.Option) error
	DescribeDBClustersPagesWithContext(context.Context, *rds.DescribeDBClustersInput, func(*rds.DescribeDBClustersOutput, bool) bool, ...request.Option) error
}

type recursiveWatcher interface {
	Add(path string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type runLocalVars struct {
	wkldName      string
	wkldType      string
	appName       string
	envName       string
	envOverrides  map[string]string
	watch         bool
	useTaskRole   bool
	portOverrides portOverrides
	proxy         bool
	proxyNetwork  net.IPNet
}

type runLocalOpts struct {
	runLocalVars

	sel                 deploySelector
	ecsClient           ecsClient
	ecsExecutor         ecsCommandExecutor
	ssm                 secretGetter
	secretsManager      secretGetter
	sessProvider        sessionProvider
	sess                *session.Session
	envManagerSess      *session.Session
	targetEnv           *config.Environment
	targetApp           *config.Application
	store               store
	ws                  wsWlDirReader
	cmd                 execRunner
	dockerEngine        dockerEngineRunner
	repository          repositoryService
	prog                progress
	orchestrator        containerOrchestrator
	hostFinder          hostFinder
	envChecker          versionCompatibilityChecker
	debounceTime        time.Duration
	newRecursiveWatcher func() (recursiveWatcher, error)

	buildContainerImages func(mft manifest.DynamicWorkload) (map[string]string, error)
	configureClients     func() error
	labeledTermPrinter   func(fw syncbuffer.FileWriter, bufs []*syncbuffer.LabeledSyncBuffer, opts ...syncbuffer.LabeledTermPrinterOption) clideploy.LabeledTermPrinter
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator

	captureStdout func() (io.Reader, error)
	releaseStdout func()
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
	o := &runLocalOpts{
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
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
	}
	o.configureClients = func() error {
		defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create default session with region %s: %w", o.targetEnv.Region, err)
		}
		o.envManagerSess, err = o.sessProvider.FromRole(o.targetEnv.ManagerRoleARN, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("create env manager session %s: %w", o.targetEnv.Region, err)
		}

		// EnvManagerRole has permissions to get task def and get SSM values.
		// However, it doesn't have permissions to get secrets from secrets manager,
		// so use the default sess and *hope* they have permissions.
		o.ecsClient = ecs.New(o.envManagerSess)
		o.ssm = ssm.New(o.envManagerSess)
		o.ecsExecutor = awsecs.New(o.envManagerSess)
		o.secretsManager = secretsmanager.New(defaultSessEnvRegion)

		resources, err := cloudformation.New(o.sess, cloudformation.WithProgressTracker(os.Stderr)).GetAppResourcesByRegion(o.targetApp, o.targetEnv.Region)
		if err != nil {
			return fmt.Errorf("get application %s resources from region %s: %w", o.appName, o.envName, err)
		}
		repoName := clideploy.RepoName(o.appName, o.wkldName)
		o.repository = repository.NewWithURI(ecr.New(defaultSessEnvRegion), repoName, resources.RepositoryURLs[o.wkldName])

		idPrefix := fmt.Sprintf("%s-%s-%s-", o.appName, o.envName, o.wkldName)
		colorGen := termcolor.ColorGenerator()
		o.orchestrator = orchestrator.New(o.dockerEngine, idPrefix, func(name string, ctr orchestrator.ContainerDefinition) dockerengine.RunLogOptions {
			return dockerengine.RunLogOptions{
				Color:      colorGen(),
				Output:     os.Stderr,
				LinePrefix: fmt.Sprintf("[%s] ", name),
			}
		})

		o.hostFinder = &hostDiscoverer{
			app:  o.appName,
			env:  o.envName,
			wkld: o.wkldName,
			ecs:  ecs.New(o.envManagerSess),
			rg:   resourcegroups.New(o.envManagerSess),
			rds:  rds.New(o.envManagerSess),
		}
		envDesc, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         o.appName,
			Env:         o.envName,
			ConfigStore: store,
		})
		if err != nil {
			return fmt.Errorf("create env describer: %w", err)
		}
		o.envChecker = envDesc
		return nil
	}
	o.buildContainerImages = func(mft manifest.DynamicWorkload) (map[string]string, error) {
		gitShortCommit := imageTagFromGit(o.cmd)
		image := clideploy.ContainerImageIdentifier{
			GitShortCommitTag: gitShortCommit,
		}
		out := &clideploy.UploadArtifactsOutput{}
		if err := clideploy.BuildContainerImages(&clideploy.ImageActionInput{
			Name:               o.wkldName,
			WorkspacePath:      o.ws.Path(),
			Image:              image,
			Mft:                mft.Manifest(),
			GitShortCommitTag:  gitShortCommit,
			Builder:            o.repository,
			Login:              o.repository.Login,
			CheckDockerEngine:  o.dockerEngine.CheckDockerEngineRunning,
			LabeledTermPrinter: o.labeledTermPrinter,
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
	o.debounceTime = 5 * time.Second
	o.newRecursiveWatcher = func() (recursiveWatcher, error) {
		return file.NewRecursiveWatcher(0)
	}

	// Capture stdout by replacing it with a piped writer and returning an attached io.Reader.
	// Functions are concurrency safe and idempotent.
	var mu sync.Mutex
	var savedWriter, savedStdout *os.File
	savedStdout = os.Stdout
	o.captureStdout = func() (io.Reader, error) {
		if savedWriter != nil {
			savedWriter.Close()
		}
		pipeReader, pipeWriter, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		mu.Lock()
		defer mu.Unlock()
		savedWriter = pipeWriter
		os.Stdout = savedWriter
		return (io.Reader)(pipeReader), nil
	}
	o.releaseStdout = func() {
		mu.Lock()
		defer mu.Unlock()
		os.Stdout = savedStdout
		savedWriter.Close()
	}
	return o, nil
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
	if err := o.configureClients(); err != nil {
		return err
	}

	ctx := context.Background()

	task, err := o.prepareTask(ctx)
	if err != nil {
		return err
	}

	var hosts []orchestrator.Host
	var ssmTarget string
	if o.proxy {
		if err := validateMinEnvVersion(o.ws, o.envChecker, o.appName, o.envName, template.RunLocalProxyMinEnvVersion, "run local --proxy"); err != nil {
			return err
		}

		hosts, err = o.hostFinder.Hosts(ctx)
		if err != nil {
			return fmt.Errorf("find hosts to connect to: %w", err)
		}

		ssmTarget, err = o.getSSMTarget(ctx)
		if err != nil {
			return fmt.Errorf("get proxy target container: %w", err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := o.orchestrator.Start()
	var runTaskOpts []orchestrator.RunTaskOption
	if o.proxy {
		runTaskOpts = append(runTaskOpts, orchestrator.RunTaskWithProxy(ssmTarget, o.proxyNetwork, hosts...))
	}
	o.orchestrator.RunTask(task, runTaskOpts...)

	var watchCh <-chan interface{}
	var watchErrCh <-chan error
	stopCh := make(chan struct{})
	if o.watch {
		watchCh, watchErrCh, err = o.watchLocalFiles(stopCh)
		if err != nil {
			return fmt.Errorf("setup watch: %s", err)
		}
	}

	for {
		select {
		case err, ok := <-errCh:
			// we loop until errCh closes, since Start()
			// closes errCh when the orchestrator is completely done.
			if !ok {
				close(stopCh)
				return nil
			}

			log.Errorf("error: %s\n", err)
			o.orchestrator.Stop()
		case <-sigCh:
			signal.Stop(sigCh)
			o.orchestrator.Stop()
		case <-watchErrCh:
			log.Errorf("watch: %s\n", err)
			o.orchestrator.Stop()
		case <-watchCh:
			task, err = o.prepareTask(ctx)
			if err != nil {
				log.Errorf("rerun task: %s\n", err)
				o.orchestrator.Stop()
				break
			}
			o.orchestrator.RunTask(task)
		}
	}
}

// getSSMTarget returns a AWS SSM target for a running container
// that supports ECS Service Exec.
func (o *runLocalOpts) getSSMTarget(ctx context.Context) (string, error) {
	svc, err := o.ecsClient.DescribeService(o.appName, o.envName, o.wkldName)
	if err != nil {
		return "", fmt.Errorf("describe service: %w", err)
	}

	for _, task := range svc.Tasks {
		// TaskArn should have the format: arn:aws:ecs:us-west-2:123456789:task/clusterName/taskName
		taskARN, err := arn.Parse(aws.StringValue(task.TaskArn))
		if err != nil {
			return "", fmt.Errorf("parse task arn: %w", err)
		}

		split := strings.Split(taskARN.Resource, "/")
		if len(split) != 3 {
			return "", fmt.Errorf("task ARN in unexpected format: %q", taskARN)
		}
		taskName := split[2]

		for _, ctr := range task.Containers {
			id := aws.StringValue(ctr.RuntimeId)
			hasECSExec := slices.ContainsFunc(ctr.ManagedAgents, func(a *sdkecs.ManagedAgent) bool {
				return aws.StringValue(a.Name) == "ExecuteCommandAgent" && aws.StringValue(a.LastStatus) == "RUNNING"
			})
			if id != "" && hasECSExec && aws.StringValue(ctr.LastStatus) == "RUNNING" {
				return fmt.Sprintf("ecs:%s_%s_%s", svc.ClusterName, taskName, aws.StringValue(ctr.RuntimeId)), nil
			}
		}
	}

	return "", errors.New("no running tasks have running containers with ecs exec enabled")
}

func (o *runLocalOpts) getTask(ctx context.Context) (orchestrator.Task, error) {
	td, err := o.ecsClient.TaskDefinition(o.appName, o.envName, o.wkldName)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("get task definition: %w", err)
	}

	envVars, err := o.getEnvVars(ctx, td)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("get env vars: %w", err)
	}

	if o.useTaskRole {
		taskRoleCredsVars, err := o.taskRoleCredentials(ctx)
		if err != nil {
			return orchestrator.Task{}, fmt.Errorf("retrieve task role credentials: %w", err)
		}

		// overwrite environment variables
		for ctr := range envVars {
			for k, v := range taskRoleCredsVars {
				envVars[ctr][k] = envVarValue{
					Value:  v,
					Secret: true,
				}
			}
		}
	}

	containerDeps := o.getContainerDependencies(td)

	task := orchestrator.Task{
		Containers: make(map[string]orchestrator.ContainerDefinition, len(td.ContainerDefinitions)),
	}

	if o.proxy {
		pauseSecrets, err := sessionEnvVars(ctx, o.envManagerSess)
		if err != nil {
			return orchestrator.Task{}, fmt.Errorf("get pause container secrets: %w", err)
		}
		task.PauseSecrets = pauseSecrets
	}

	for _, ctr := range td.ContainerDefinitions {
		name := aws.StringValue(ctr.Name)
		def := orchestrator.ContainerDefinition{
			ImageURI:    aws.StringValue(ctr.Image),
			EnvVars:     envVars[name].EnvVars(),
			Secrets:     envVars[name].Secrets(),
			Ports:       make(map[string]string, len(ctr.PortMappings)),
			IsEssential: containerDeps[name].isEssential,
			DependsOn:   containerDeps[name].dependsOn,
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

func (o *runLocalOpts) prepareTask(ctx context.Context) (orchestrator.Task, error) {
	task, err := o.getTask(ctx)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("get task: %w", err)
	}

	mft, _, err := workloadManifest(&workloadManifestInput{
		name:         o.wkldName,
		appName:      o.appName,
		envName:      o.envName,
		ws:           o.ws,
		interpolator: o.newInterpolator(o.appName, o.envName),
		unmarshal:    o.unmarshal,
		sess:         o.envManagerSess,
	})
	if err != nil {
		return orchestrator.Task{}, err
	}

	containerURIs, err := o.buildContainerImages(mft)
	if err != nil {
		return orchestrator.Task{}, fmt.Errorf("build images: %w", err)
	}

	// replace built images with the local built URI
	for name, uri := range containerURIs {
		ctr, ok := task.Containers[name]
		if !ok {
			return orchestrator.Task{}, fmt.Errorf("built an image for %q, which doesn't exist in the task", name)
		}

		ctr.ImageURI = uri
		task.Containers[name] = ctr
	}

	containerDeps := manifest.ContainerDependencies(mft.Manifest())
	for name, dep := range containerDeps {
		ctr, ok := task.Containers[name]
		if !ok {
			return orchestrator.Task{}, fmt.Errorf("missing container: %q is listed as a dependency, which doesn't exist in the task", name)
		}
		ctr.IsEssential = dep.IsEssential
		ctr.DependsOn = dep.DependsOn
		task.Containers[name] = ctr
	}

	return task, nil
}

func (o *runLocalOpts) watchLocalFiles(stopCh <-chan struct{}) (<-chan interface{}, <-chan error, error) {
	workspacePath := o.ws.Path()

	watchCh := make(chan interface{})
	watchErrCh := make(chan error)

	watcher, err := o.newRecursiveWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("file: %w", err)
	}

	if err = watcher.Add(workspacePath); err != nil {
		return nil, nil, err
	}

	watcherEvents := watcher.Events()
	watcherErrors := watcher.Errors()

	debounceTimer := time.NewTimer(o.debounceTime)
	if !debounceTimer.Stop() {
		// flush the timer in case stop is called after the timer finishes
		<-debounceTimer.C
	}

	go func() {
		for {
			select {
			case <-stopCh:
				watcher.Close()
				return
			case err, ok := <-watcherErrors:
				watchErrCh <- err
				if !ok {
					watcher.Close()
					return
				}
			case event, ok := <-watcherEvents:
				if !ok {
					watcher.Close()
					return
				}

				// skip chmod events
				if event.Has(fsnotify.Chmod) {
					break
				}

				// check if any subdirectories within copilot directory are hidden
				isHidden := false
				parent := workspacePath
				suffix, _ := strings.CutPrefix(event.Name, parent+"/")
				// fsnotify events are always of form /a/b/c, don't use filepath.Split as that's OS dependent
				for _, child := range strings.Split(suffix, "/") {
					parent = filepath.Join(parent, child)
					subdirHidden, err := file.IsHiddenFile(child)
					if err != nil {
						break
					}
					if subdirHidden {
						isHidden = true
					}
				}

				// TODO(Aiden): implement dockerignore blacklist for update
				if !isHidden {
					debounceTimer.Reset(o.debounceTime)
				}
			case <-debounceTimer.C:
				watchCh <- nil
			}
		}
	}()

	return watchCh, watchErrCh, nil
}

func sessionEnvVars(ctx context.Context, sess *session.Session) (map[string]string, error) {
	creds, err := sess.Config.Credentials.GetWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get IAM credentials: %w", err)
	}

	env := map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
		"AWS_SESSION_TOKEN":     creds.SessionToken,
	}
	if sess.Config.Region != nil {
		env["AWS_DEFAULT_REGION"] = aws.StringValue(sess.Config.Region)
		env["AWS_REGION"] = aws.StringValue(sess.Config.Region)
	}
	return env, nil
}

func (o *runLocalOpts) taskRoleCredentials(ctx context.Context) (map[string]string, error) {
	// assumeRoleMethod tries to directly call sts:AssumeRole for TaskRole using default session
	// calls sts:AssumeRole through aws-sdk-go here https://github.com/aws/aws-sdk-go/blob/ac58203a9054cc9d901429bdd94edfc0a7a1de46/aws/credentials/stscreds/assume_role_provider.go#L352
	assumeRoleMethod := func() (map[string]string, error) {
		taskDef, err := o.ecsClient.TaskDefinition(o.appName, o.envName, o.wkldName)
		if err != nil {
			return nil, err
		}

		taskRoleSess, err := o.sessProvider.FromRole(aws.StringValue(taskDef.TaskRoleArn), o.targetEnv.Region)
		if err != nil {
			return nil, err
		}

		return sessionEnvVars(ctx, taskRoleSess)
	}

	// ecsExecMethod tries to use ECS Exec to retrive credentials from running container
	ecsExecMethod := func() (map[string]string, error) {
		svcDesc, err := o.ecsClient.DescribeService(o.appName, o.envName, o.wkldName)
		if err != nil {
			return nil, fmt.Errorf("describe ECS service for %s in environment %s: %w", o.wkldName, o.envName, err)
		}

		stdoutReader, err := o.captureStdout()
		if err != nil {
			return nil, err
		}
		defer o.releaseStdout()

		// try exec on each container within the service
		var wg sync.WaitGroup
		containerErr := make(chan error)
		for _, task := range svcDesc.Tasks {
			taskID, err := awsecs.TaskID(aws.StringValue(task.TaskArn))
			if err != nil {
				return nil, err
			}

			for _, container := range task.Containers {
				wg.Add(1)
				containerName := aws.StringValue(container.Name)
				go func() {
					defer wg.Done()
					err := o.ecsExecutor.ExecuteCommand(awsecs.ExecuteCommandInput{
						Cluster:   svcDesc.ClusterName,
						Command:   fmt.Sprintf("/bin/sh -c %q\n", curlContainerCredentialsCmd),
						Task:      taskID,
						Container: containerName,
					})
					if err != nil {
						containerErr <- fmt.Errorf("container %s in task %s: %w", containerName, taskID, err)
					}
				}()
			}
		}

		// wait for containers to finish and reset stdout
		containersFinished := make(chan struct{})
		go func() {
			wg.Wait()
			o.releaseStdout()
			close(containersFinished)
		}()

		type containerCredentialsOutput struct {
			AccessKeyId     string
			SecretAccessKey string
			Token           string
		}

		// parse stdout to try and find credentials
		credsResult := make(chan map[string]string)
		parseErr := make(chan error)
		go func() {
			select {
			case <-containersFinished:
				buf, err := io.ReadAll(stdoutReader)
				if err != nil {
					parseErr <- err
					return
				}
				lines := bytes.Split(buf, []byte("\n"))
				var creds containerCredentialsOutput
				for _, line := range lines {
					err := json.Unmarshal(line, &creds)
					if err != nil {
						continue
					}
					credsResult <- map[string]string{
						"AWS_ACCESS_KEY_ID":     creds.AccessKeyId,
						"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
						"AWS_SESSION_TOKEN":     creds.Token,
					}
					return
				}
				parseErr <- errors.New("all containers failed to retrieve credentials")
			case <-ctx.Done():
				return
			}
		}()

		var containerErrs []error
		for {
			select {
			case creds := <-credsResult:
				return creds, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			case err := <-parseErr:
				return nil, errors.Join(append([]error{err}, containerErrs...)...)
			case err := <-containerErr:
				containerErrs = append(containerErrs, err)
			}
		}
	}

	credentialsChain := []func() (map[string]string, error){
		assumeRoleMethod,
		ecsExecMethod,
	}

	credentialsChainWrappedErrs := []string{
		"assume role",
		"ecs exec",
	}

	// return TaskRole credentials from first successful method
	var errs []error
	for errIndex, method := range credentialsChain {
		vars, err := method()
		if err == nil {
			return vars, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", credentialsChainWrappedErrs[errIndex], err))
	}

	return nil, &errTaskRoleRetrievalFailed{errs}
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
// container defined in the TaskDefinition. The returned map is a map of container names,
// each of which contains a mapping of key->envVarValue, which defines if the variable is a secret or not.
func (o *runLocalOpts) getEnvVars(ctx context.Context, taskDef *awsecs.TaskDefinition) (map[string]containerEnv, error) {
	envVars := make(map[string]containerEnv, len(taskDef.ContainerDefinitions))
	for _, ctr := range taskDef.ContainerDefinitions {
		envVars[aws.StringValue(ctr.Name)] = make(map[string]envVarValue)
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

	// inject session variables if they haven't been already set
	sessionVars, err := sessionEnvVars(ctx, o.sess)
	if err != nil {
		return nil, err
	}

	for ctr := range envVars {
		for k, v := range sessionVars {
			if _, ok := envVars[ctr][k]; !ok {
				envVars[ctr][k] = envVarValue{
					Value:  v,
					Secret: true,
				}
			}
		}
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

type containerDependency struct {
	isEssential bool
	dependsOn   map[string]string
}

func (o *runLocalOpts) getContainerDependencies(taskDef *awsecs.TaskDefinition) map[string]containerDependency {
	dependencies := make(map[string]containerDependency, len(taskDef.ContainerDefinitions))
	for _, ctr := range taskDef.ContainerDefinitions {
		dep := containerDependency{
			isEssential: aws.BoolValue(ctr.Essential),
			dependsOn:   make(map[string]string),
		}
		for _, containerDep := range ctr.DependsOn {
			dep.dependsOn[aws.StringValue(containerDep.ContainerName)] = strings.ToLower(aws.StringValue(containerDep.Condition))
		}
		dependencies[aws.StringValue(ctr.Name)] = dep
	}
	return dependencies
}

type hostDiscoverer struct {
	ecs  ecsClient
	app  string
	env  string
	wkld string

	rg  taggedResourceGetter
	rds rdsDescriber
}

func (h *hostDiscoverer) Hosts(ctx context.Context) ([]orchestrator.Host, error) {
	svcs, err := h.ecs.ServiceConnectServices(h.app, h.env, h.wkld)
	if err != nil {
		return nil, fmt.Errorf("get service connect services: %w", err)
	}

	var hosts []orchestrator.Host
	for _, svc := range svcs {
		// find the primary deployment with service connect enabled
		idx := slices.IndexFunc(svc.Deployments, func(dep *sdkecs.Deployment) bool {
			return aws.StringValue(dep.Status) == "PRIMARY" && aws.BoolValue(dep.ServiceConnectConfiguration.Enabled)
		})
		if idx == -1 {
			continue
		}

		for _, sc := range svc.Deployments[idx].ServiceConnectConfiguration.Services {
			for _, alias := range sc.ClientAliases {
				hosts = append(hosts, orchestrator.Host{
					Name: aws.StringValue(alias.DnsName),
					Port: uint16(aws.Int64Value(alias.Port)),
				})
			}
		}
	}

	rdsHosts, err := h.rdsHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("get rds hosts: %w", err)
	}

	return append(hosts, rdsHosts...), nil
}

// rdsHosts gets rds endpoints for workloads tagged for this workload
// or for the environment using direct AWS SDK calls.
func (h *hostDiscoverer) rdsHosts(ctx context.Context) ([]orchestrator.Host, error) {
	var hosts []orchestrator.Host

	resources, err := h.rg.GetResourcesByTags(resourcegroups.ResourceTypeRDS, map[string]string{
		deploy.AppTagKey: h.app,
		deploy.EnvTagKey: h.env,
	})
	switch {
	case err != nil:
		return nil, fmt.Errorf("get tagged resources: %w", err)
	case len(resources) == 0:
		return nil, nil
	}

	dbFilter := &rds.Filter{
		Name: aws.String("db-instance-id"),
	}
	clusterFilter := &rds.Filter{
		Name: aws.String("db-cluster-id"),
	}
	for i := range resources {
		// we don't want resources that belong to other services
		// but we do want env level services
		if wkld, ok := resources[i].Tags[deploy.ServiceTagKey]; ok && wkld != h.wkld {
			continue
		}

		arn, err := arn.Parse(resources[i].ARN)
		if err != nil {
			return nil, fmt.Errorf("invalid arn %q: %w", resources[i].ARN, err)
		}

		switch {
		case strings.HasPrefix(arn.Resource, "db:"):
			dbFilter.Values = append(dbFilter.Values, aws.String(resources[i].ARN))
		case strings.HasPrefix(arn.Resource, "cluster:"):
			clusterFilter.Values = append(clusterFilter.Values, aws.String(resources[i].ARN))
		}
	}

	if len(dbFilter.Values) > 0 {
		err = h.rds.DescribeDBInstancesPagesWithContext(ctx, &rds.DescribeDBInstancesInput{
			Filters: []*rds.Filter{dbFilter},
		}, func(out *rds.DescribeDBInstancesOutput, lastPage bool) bool {
			for _, db := range out.DBInstances {
				if db.Endpoint != nil {
					hosts = append(hosts, orchestrator.Host{
						Name: aws.StringValue(db.Endpoint.Address),
						Port: uint16(aws.Int64Value(db.Endpoint.Port)),
					})
				}
			}
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}
	}

	if len(clusterFilter.Values) > 0 {
		err = h.rds.DescribeDBClustersPagesWithContext(ctx, &rds.DescribeDBClustersInput{
			Filters: []*rds.Filter{clusterFilter},
		}, func(out *rds.DescribeDBClustersOutput, lastPage bool) bool {
			for _, db := range out.DBClusters {
				add := func(s *string) {
					if s != nil {
						hosts = append(hosts, orchestrator.Host{
							Name: aws.StringValue(s),
							Port: uint16(aws.Int64Value(db.Port)),
						})
					}
				}

				add(db.Endpoint)
				add(db.ReaderEndpoint)
				for i := range db.CustomEndpoints {
					add(db.CustomEndpoints[i])
				}
			}
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("describe clusters: %w", err)
		}
	}

	return hosts, nil
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
	cmd.SetUsageTemplate(cmdtemplate.Usage)

	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.watch, watchFlag, false, watchFlagDescription)
	cmd.Flags().BoolVar(&vars.useTaskRole, useTaskRoleFlag, true, useTaskRoleFlagDescription)
	cmd.Flags().Var(&vars.portOverrides, portOverrideFlag, portOverridesFlagDescription)
	cmd.Flags().StringToStringVar(&vars.envOverrides, envVarOverrideFlag, nil, envVarOverrideFlagDescription)
	cmd.Flags().BoolVar(&vars.proxy, proxyFlag, false, proxyFlagDescription)
	cmd.Flags().IPNetVar(&vars.proxyNetwork, proxyNetworkFlag, net.IPNet{
		// docker uses 172.17.0.0/16 for networking by default
		// so we'll default to different /16 from the 172.16.0.0/12
		// private network defined by RFC 1918.
		IP:   net.IPv4(172, 20, 0, 0),
		Mask: net.CIDRMask(16, 32),
	}, proxyNetworkFlag)
	return cmd
}
