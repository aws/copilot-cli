// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"

	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
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
	wkldInitImagePromptHelp = `The name of an existing Docker image. Images in the Docker Hub registry are available by default.
Other repositories are specified with either repository-url/image:tag or repository-url/image@digest`
	wkldInitAppRunnerImagePromptHelp = `The name of an existing Docker image. App Runner supports images hosted in ECR or ECR Public registries.`
)

const (
	defaultSvcPortString = "80"
	service              = "service"
	job                  = "job"
)

var (
	fmtSvcInitSvcTypePrompt  = "Which %s best represents your service's architecture?"
	svcInitSvcTypeHelpPrompt = fmt.Sprintf(`A %s is an internet-facing HTTP server managed by AWS App Runner that scales based on incoming requests.
To learn more see: https://git.io/JEEfb

A %s is an internet-facing HTTP server managed by Amazon ECS on AWS Fargate behind a load balancer.
To learn more see: https://git.io/JEEJe

A %s is a private, non internet-facing service accessible from other services in your VPC.
To learn more see: https://git.io/JEEJt

A %s is a private service that can consume messages published to topics in your application.
To learn more see: https://git.io/JEEJY`,
		manifest.RequestDrivenWebServiceType,
		manifest.LoadBalancedWebServiceType,
		manifest.BackendServiceType,
		manifest.WorkerServiceType,
	)

	fmtWkldInitNamePrompt     = "What do you want to %s this %s?"
	fmtWkldInitNameHelpPrompt = `The name will uniquely identify this %s within your app %s.
Deployed resources (such as your ECR repository, logs) will contain this %[1]s's name and be tagged with it.`

	fmtWkldInitDockerfilePrompt      = "Which " + color.Emphasize("Dockerfile") + " would you like to use for %s?"
	wkldInitDockerfileHelpPrompt     = "Dockerfile to use for building your container image."
	fmtWkldInitDockerfilePathPrompt  = "What is the path to the " + color.Emphasize("Dockerfile") + " for %s?"
	wkldInitDockerfilePathHelpPrompt = "Path to Dockerfile to use for building your container image."

	svcInitSvcPortPrompt     = "Which %s do you want customer traffic sent to?"
	svcInitSvcPortHelpPrompt = `The port will be used by the load balancer to route incoming traffic to this service.
You should set this to the port which your Dockerfile uses to communicate with the internet.`

	svcInitPublisherPrompt     = "Which topics do you want to subscribe to?"
	svcInitPublisherHelpPrompt = `A publisher is an existing SNS Topic to which a service publishes messages. 
These messages can be consumed by the Worker Service.`

	wkldInitImagePrompt = fmt.Sprintf("What's the %s ([registry/]repository[:tag|@digest]) of the image to use?", color.Emphasize("location"))
)

var serviceTypeHints = map[string]string{
	manifest.RequestDrivenWebServiceType: "App Runner",
	manifest.LoadBalancedWebServiceType:  "Internet to ECS on Fargate",
	manifest.BackendServiceType:          "ECS on Fargate",
	manifest.WorkerServiceType:           "Events to SQS to ECS on Fargate",
}

type initWkldVars struct {
	appName        string
	wkldType       string
	name           string
	dockerfilePath string
	image          string
	subscriptions  []string
	noSubscribe    bool
}

type initSvcVars struct {
	initWkldVars

	port uint16
}

type initSvcOpts struct {
	initSvcVars

	// Interfaces to interact with dependencies.
	fs           afero.Fs
	init         svcInitializer
	prompt       prompter
	store        store
	dockerEngine dockerEngine
	sel          dockerfileSelector
	topicSel     topicSelector
	mftReader    manifestReader

	// Outputs stored on successful actions.
	manifestPath string
	platform     *manifest.PlatformString
	topics       []manifest.TopicSubscription

	// For workspace validation.
	wsAppName         string
	wsPendingCreation bool

	// Cache variables
	df             dockerfileParser
	manifestExists bool

	// Init a Dockerfile parser using fs and input path
	dockerfile func(string) dockerfileParser
}

func newInitSvcOpts(vars initSvcVars) (*initSvcOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc init"))
	sess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(sess), ssm.New(sess), aws.StringValue(sess.Config.Region))
	prompter := prompt.New()
	deployStore, err := deploy.NewStore(sessProvider, store)
	if err != nil {
		return nil, err
	}
	snsSel := selector.NewDeploySelect(prompter, store, deployStore)

	initSvc := &initialize.WorkloadInitializer{
		Store:    store,
		Ws:       ws,
		Prog:     termprogress.NewSpinner(log.DiagnosticWriter),
		Deployer: cloudformation.New(sess, cloudformation.WithProgressTracker(os.Stderr)),
	}
	opts := &initSvcOpts{
		initSvcVars:  vars,
		store:        store,
		fs:           &afero.Afero{Fs: afero.NewOsFs()},
		init:         initSvc,
		prompt:       prompter,
		sel:          selector.NewWorkspaceSelector(prompter, ws),
		topicSel:     snsSel,
		mftReader:    ws,
		dockerEngine: dockerengine.New(exec.NewCmd()),
		wsAppName:    tryReadingAppName(),
	}
	opts.dockerfile = func(path string) dockerfileParser {
		if opts.df != nil {
			return opts.df
		}
		opts.df = dockerfile.New(opts.fs, opts.dockerfilePath)
		return opts.df
	}
	return opts, nil
}

// Validate returns an error for any invalid optional flags.
func (o *initSvcOpts) Validate() error {
	// If this app is pending creation, we'll skip validation.
	if !o.wsPendingCreation {
		if err := validateWorkspaceApp(o.wsAppName, o.appName, o.store); err != nil {
			return err
		}
		o.appName = o.wsAppName
	}
	if o.dockerfilePath != "" && o.image != "" {
		return fmt.Errorf("--%s and --%s cannot be specified together", dockerFileFlag, imageFlag)
	}
	if o.dockerfilePath != "" {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}
	if o.port != 0 {
		if err := validateSvcPort(o.port); err != nil {
			return err
		}
	}
	if o.image != "" && o.wkldType == manifest.RequestDrivenWebServiceType {
		if err := validateAppRunnerImage(o.image); err != nil {
			return err
		}
	}
	if err := validateSubscribe(o.noSubscribe, o.subscriptions); err != nil {
		return err
	}
	return nil
}

// Ask prompts for and validates any required flags.
func (o *initSvcOpts) Ask() error {
	// NOTE: we optimize the case where `name` is given as a flag while `wkldType` is not.
	// In this case, we can try reading the manifest, and set `wkldType` to the value found in the manifest
	// without having to validate it. We can then short circuit the rest of the prompts for an optimal UX.
	if o.name != "" && o.wkldType == "" {
		// Best effort to validate the service name without type.
		if err := o.validateSvc(); err != nil {
			return err
		}
		shouldSkipAsking, err := o.shouldSkipAsking()
		if err != nil {
			return err
		}
		if shouldSkipAsking {
			return nil
		}
	}
	if o.wkldType != "" {
		if err := validateSvcType(o.wkldType); err != nil {
			return err
		}
	} else {
		if err := o.askSvcType(); err != nil {
			return err
		}
	}
	if o.name == "" {
		if err := o.askSvcName(); err != nil {
			return err
		}
	}
	if err := o.validateSvc(); err != nil {
		return err
	}
	shouldSkipAsking, err := o.shouldSkipAsking()
	if err != nil {
		return err
	}
	if shouldSkipAsking {
		return nil
	}
	err = o.askDockerfile()
	if err != nil {
		return err
	}
	if o.dockerfilePath == "" {
		if err := o.askImage(); err != nil {
			return err
		}
	}
	if err := o.askSvcPort(); err != nil {
		return err
	}
	if err := o.askSvcPublishers(); err != nil {
		return err
	}
	return nil
}

// Execute writes the service's manifest file and stores the service in SSM.
func (o *initSvcOpts) Execute() error {
	// Check for a valid healthcheck and add it to the opts.
	var hc manifest.ContainerHealthCheck
	var err error
	if o.dockerfilePath != "" {
		hc, err = parseHealthCheck(o.dockerfile(o.dockerfilePath))
		if err != nil {
			log.Warningf("Cannot parse the HEALTHCHECK instruction from the Dockerfile: %v\n", err)
		}
	}
	// If the user passes in an image, their docker engine isn't necessarily running, and we can't do anything with the platform because we're not building the Docker image.
	if o.image == "" && !o.manifestExists {
		platform, err := legitimizePlatform(o.dockerEngine, o.wkldType)
		if err != nil {
			return err
		}
		if platform != "" {
			o.platform = &platform
		}
	}
	manifestPath, err := o.init.Service(&initialize.ServiceProps{
		WorkloadProps: initialize.WorkloadProps{
			App:            o.appName,
			Name:           o.name,
			Type:           o.wkldType,
			DockerfilePath: o.dockerfilePath,
			Image:          o.image,
			Platform: manifest.PlatformArgsOrString{
				PlatformString: o.platform,
			},
			Topics: o.topics,
		},
		Port:        o.port,
		HealthCheck: hc,
	})
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *initSvcOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your service to a %s environment.",
			color.HighlightCode(fmt.Sprintf("copilot svc deploy --name %s --env %s", o.name, defaultEnvironmentName)),
			defaultEnvironmentName),
	})
	return nil
}

func (o *initSvcOpts) askSvcType() error {
	if o.wkldType != "" {
		return nil
	}

	msg := fmt.Sprintf(fmtSvcInitSvcTypePrompt, color.Emphasize("service type"))
	t, err := o.prompt.SelectOption(msg, svcInitSvcTypeHelpPrompt, svcTypePromptOpts(), prompt.WithFinalMessage("Service type:"))
	if err != nil {
		return fmt.Errorf("select service type: %w", err)
	}
	o.wkldType = t
	return nil
}

func (o *initSvcOpts) validateSvc() error {
	if err := validateSvcName(o.name, o.wkldType); err != nil {
		return err
	}
	return o.validateDuplicateSvc()
}

func (o *initSvcOpts) validateDuplicateSvc() error {
	_, err := o.store.GetService(o.appName, o.name)
	if err == nil {
		log.Errorf(`It seems like you are trying to init a service that already exists.
To recreate the service, please run:
1. %s. Note: The manifest file will not be deleted and will be used in Step 2.
If you'd prefer a new default manifest, please manually delete the existing one.
2. And then %s
`,
			color.HighlightCode(fmt.Sprintf("copilot svc delete --name %s", o.name)),
			color.HighlightCode(fmt.Sprintf("copilot svc init --name %s", o.name)))
		return fmt.Errorf("service %s already exists", color.HighlightUserInput(o.name))
	}

	var errNoSuchSvc *config.ErrNoSuchService
	if !errors.As(err, &errNoSuchSvc) {
		return fmt.Errorf("validate if service exists: %w", err)
	}
	return nil
}

func (o *initSvcOpts) askSvcName() error {
	name, err := o.prompt.Get(
		fmt.Sprintf(fmtWkldInitNamePrompt, color.Emphasize("name"), "service"),
		fmt.Sprintf(fmtWkldInitNameHelpPrompt, service, o.appName),
		func(val interface{}) error {
			return validateSvcName(val, o.wkldType)
		},
		prompt.WithFinalMessage("Service name:"))
	if err != nil {
		return fmt.Errorf("get service name: %w", err)
	}
	o.name = name
	return nil
}

func (o *initSvcOpts) askImage() error {
	if o.image != "" {
		return nil
	}

	validator := prompt.RequireNonEmpty
	promptHelp := wkldInitImagePromptHelp
	if o.wkldType == manifest.RequestDrivenWebServiceType {
		promptHelp = wkldInitAppRunnerImagePromptHelp
		validator = validateAppRunnerImage
	}

	image, err := o.prompt.Get(
		wkldInitImagePrompt,
		promptHelp,
		validator,
		prompt.WithFinalMessage("Image:"),
	)
	if err != nil {
		return fmt.Errorf("get image location: %w", err)
	}
	o.image = image
	return nil
}

func (o *initSvcOpts) shouldSkipAsking() (bool, error) {
	localMft, err := o.mftReader.ReadWorkloadManifest(o.name)
	if err != nil {
		var (
			errNotFound          *workspace.ErrFileNotExists
			errWorkspaceNotFound *workspace.ErrWorkspaceNotFound
		)
		if !errors.As(err, &errNotFound) && !errors.As(err, &errWorkspaceNotFound) {
			return false, fmt.Errorf("read manifest file for service %s: %w", o.name, err)
		}
		return false, nil
	}
	o.manifestExists = true

	svcType, err := localMft.WorkloadType()
	if err != nil {
		return false, fmt.Errorf(`read "type" field for service %s from local manifest: %w`, o.name, err)
	}
	if o.wkldType != "" {
		if o.wkldType != svcType {
			return false, fmt.Errorf("manifest file for service %s exists with a different type %s", o.name, svcType)
		}
	} else {
		o.wkldType = svcType
	}
	log.Infof("Manifest file for service %s already exists. Skipping configuration.\n", o.name)
	return true, nil
}

// isDfSelected indicates if any Dockerfile is in use.
func (o *initSvcOpts) askDockerfile() error {
	if o.dockerfilePath != "" || o.image != "" {
		return nil
	}
	if err := o.dockerEngine.CheckDockerEngineRunning(); err != nil {
		var errDaemon *dockerengine.ErrDockerDaemonNotResponsive
		switch {
		case errors.Is(err, dockerengine.ErrDockerCommandNotFound):
			log.Info("Docker command is not found; Copilot won't build from a Dockerfile.\n")
			return nil
		case errors.As(err, &errDaemon):
			log.Info("Docker daemon is not responsive; Copilot won't build from a Dockerfile.\n")
			return nil
		default:
			return fmt.Errorf("check if docker engine is running: %w", err)
		}
	}
	df, err := o.sel.Dockerfile(
		fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.HighlightUserInput(o.name)),
		fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, color.HighlightUserInput(o.name)),
		wkldInitDockerfileHelpPrompt,
		wkldInitDockerfilePathHelpPrompt,
		func(v interface{}) error {
			return validatePath(afero.NewOsFs(), v)
		},
	)
	if err != nil {
		return fmt.Errorf("select Dockerfile: %w", err)
	}
	if df == selector.DockerfilePromptUseImage {
		return nil
	}
	o.dockerfilePath = df
	return nil
}

func (o *initSvcOpts) askSvcPort() (err error) {
	// If the port flag was set, use that and don't ask.
	if o.port != 0 {
		return nil
	}

	var ports []dockerfile.Port
	if o.dockerfilePath != "" && o.image == "" {
		// Check for exposed ports.
		ports, err = o.dockerfile(o.dockerfilePath).GetExposedPorts()
		// Ignore any errors in dockerfile parsing--we'll use the default port instead.
		if err != nil {
			log.Debugln(err.Error())
		}
	}

	defaultPort := defaultSvcPortString
	if o.dockerfilePath != "" {
		switch len(ports) {
		case 0:
			// There were no ports detected, keep the default port prompt.
		case 1:
			o.port = ports[0].Port
			return nil
		default:
			defaultPort = strconv.Itoa(int(ports[0].Port))
		}
	}
	// Skip asking if it is a backend or worker service.
	if o.wkldType == manifest.BackendServiceType || o.wkldType == manifest.WorkerServiceType {
		return nil
	}

	port, err := o.prompt.Get(
		fmt.Sprintf(svcInitSvcPortPrompt, color.Emphasize("port")),
		svcInitSvcPortHelpPrompt,
		validateSvcPort,
		prompt.WithDefaultInput(defaultPort),
		prompt.WithFinalMessage("Port:"),
	)
	if err != nil {
		return fmt.Errorf("get port: %w", err)
	}

	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return fmt.Errorf("parse port string: %w", err)
	}

	o.port = uint16(portUint)

	return nil
}

func legitimizePlatform(engine dockerEngine, wkldType string) (manifest.PlatformString, error) {
	if err := engine.CheckDockerEngineRunning(); err != nil {
		// This is a best-effort attempt to detect the platform for users.
		// If docker is not available, we skip this information.
		var errDaemon *dockerengine.ErrDockerDaemonNotResponsive
		switch {
		case errors.Is(err, dockerengine.ErrDockerCommandNotFound):
			log.Info("Docker command is not found; Copilot won't detect and populate the \"platform\" field in the manifest.\n")
			return "", nil
		case errors.As(err, &errDaemon):
			log.Info("Docker daemon is not responsive; Copilot won't detect and populate the \"platform\" field in the manifest.\n")
			return "", nil
		default:
			return "", fmt.Errorf("check if docker engine is running: %w", err)
		}
	}
	detectedOs, detectedArch, err := engine.GetPlatform()
	if err != nil {
		return "", fmt.Errorf("get docker engine platform: %w", err)
	}
	redirectedPlatform, err := manifest.RedirectPlatform(detectedOs, detectedArch, wkldType)
	if err != nil {
		return "", fmt.Errorf("redirect docker engine platform: %w", err)
	}
	if redirectedPlatform == "" {
		return "", nil
	}
	// Return an error if a platform cannot be redirected.
	if wkldType == manifest.RequestDrivenWebServiceType && detectedOs == manifest.OSWindows {
		return "", manifest.ErrAppRunnerInvalidPlatformWindows
	}
	// Messages are logged only if the platform was redirected.
	msg := fmt.Sprintf("Architecture type %s has been detected. We will set platform '%s' instead. If you'd rather build and run as architecture type %s, please change the 'platform' field in your workload manifest to '%s'.\n", detectedArch, redirectedPlatform, manifest.ArchARM64, dockerengine.PlatformString(detectedOs, manifest.ArchARM64))
	if manifest.IsArmArch(detectedArch) && wkldType == manifest.RequestDrivenWebServiceType {
		msg = fmt.Sprintf("Architecture type %s has been detected. At this time, %s architectures are not supported for App Runner workloads. We will set platform '%s' instead.\n", detectedArch, detectedArch, redirectedPlatform)
	}
	log.Warningf(msg)
	return manifest.PlatformString(redirectedPlatform), nil
}

func (o *initSvcOpts) askSvcPublishers() (err error) {
	if o.wkldType != manifest.WorkerServiceType {
		return nil
	}
	// publishers already specified by flags
	if len(o.subscriptions) > 0 {
		var topicSubscriptions []manifest.TopicSubscription
		for _, sub := range o.subscriptions {
			sub, err := parseSerializedSubscription(sub)
			if err != nil {
				return err
			}
			topicSubscriptions = append(topicSubscriptions, sub)
		}
		o.topics = topicSubscriptions
		return nil
	}

	// if --no-subscriptions flag specified, no need to ask for publishers
	if o.noSubscribe {
		return nil
	}

	topics, err := o.topicSel.Topics(svcInitPublisherPrompt, svcInitPublisherHelpPrompt, o.appName)
	if err != nil {
		return fmt.Errorf("select publisher: %w", err)
	}

	subscriptions := make([]manifest.TopicSubscription, 0, len(topics))
	for _, t := range topics {
		subscriptions = append(subscriptions, manifest.TopicSubscription{
			Name:    aws.String(t.Name()),
			Service: aws.String(t.Workload()),
		})
	}
	o.topics = subscriptions

	return nil
}

func validateWorkspaceApp(wsApp, inputApp string, store store) error {
	if wsApp == "" {
		// NOTE: This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}
	// This command must be run within the app's workspace.
	if inputApp != "" && inputApp != wsApp {
		return fmt.Errorf("cannot specify app %s because the workspace is already registered with app %s", inputApp, wsApp)
	}
	if _, err := store.GetApplication(wsApp); err != nil {
		return fmt.Errorf("get application %s configuration: %w", wsApp, err)
	}
	return nil
}

// parseSerializedSubscription parses the service and topic name out of keys specified in the form "service:topicName"
func parseSerializedSubscription(input string) (manifest.TopicSubscription, error) {
	attrs := regexpMatchSubscription.FindStringSubmatch(input)
	if len(attrs) == 0 {
		return manifest.TopicSubscription{}, fmt.Errorf("parse subscription from key: %s", input)
	}
	return manifest.TopicSubscription{
		Name:    aws.String(attrs[2]),
		Service: aws.String(attrs[1]),
	}, nil
}

func parseHealthCheck(df dockerfileParser) (manifest.ContainerHealthCheck, error) {
	hc, err := df.GetHealthCheck()
	if err != nil {
		return manifest.ContainerHealthCheck{}, fmt.Errorf("get healthcheck: %w", err)
	}
	if hc == nil {
		return manifest.ContainerHealthCheck{}, nil
	}
	return manifest.ContainerHealthCheck{
		Interval:    &hc.Interval,
		Timeout:     &hc.Timeout,
		StartPeriod: &hc.StartPeriod,
		Retries:     &hc.Retries,
		Command:     hc.Cmd,
	}, nil
}

func svcTypePromptOpts() []prompt.Option {
	var options []prompt.Option
	for _, svcType := range manifest.ServiceTypes() {
		options = append(options, prompt.Option{
			Value: svcType,
			Hint:  serviceTypeHints[svcType],
		})
	}
	return options
}

// buildSvcInitCmd build the command for creating a new service.
func buildSvcInitCmd() *cobra.Command {
	vars := initSvcVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new service in an application.",
		Long: `Creates a new service in an application.
This command is also run as part of "copilot init".`,
		Example: `
  Create a "frontend" load balanced web service.
  /code $ copilot svc init --name frontend --svc-type "Load Balanced Web Service" --dockerfile ./frontend/Dockerfile

  Create a "subscribers" backend service.
  /code $ copilot svc init --name subscribers --svc-type "Backend Service"`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitSvcOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			log.Warningln("It's best to run this command in the root of your workspace.")
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			if err := opts.RecommendActions(); err != nil {
				return err
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.wkldType, svcTypeFlag, typeFlagShort, "", svcTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.dockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().StringVarP(&vars.image, imageFlag, imageFlagShort, "", imageFlagDescription)
	cmd.Flags().Uint16Var(&vars.port, svcPortFlag, 0, svcPortFlagDescription)
	cmd.Flags().StringArrayVar(&vars.subscriptions, subscribeTopicsFlag, []string{}, subscribeTopicsFlagDescription)
	cmd.Flags().BoolVar(&vars.noSubscribe, noSubscriptionFlag, false, noSubscriptionFlagDescription)

	return cmd
}
