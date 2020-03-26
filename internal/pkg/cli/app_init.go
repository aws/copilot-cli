// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/docker"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	appInitAppTypePrompt     = "Which type of " + color.Emphasize("infrastructure pattern") + " best represents your application?"
	appInitAppTypeHelpPrompt = `Your application's architecture. Most applications need additional AWS resources to run.
To help setup the infrastructure resources, select what "kind" or "type" of application you want to build.`

	fmtAppInitAppNamePrompt     = "What do you want to " + color.Emphasize("name") + " this %s?"
	fmtAppInitAppNameHelpPrompt = `The name will uniquely identify this application within your %s project.
Deployed resources (such as your service, logs) will contain this app's name and be tagged with it.`

	fmtAppInitDockerfilePrompt  = "Which Dockerfile would you like to use for %s?"
	appInitDockerfileHelpPrompt = "Dockerfile to use for building your application's container image."

	appInitAppPortPrompt     = "What port do you want requests from your load balancer forwarded to?"
	appInitAppPortHelpPrompt = `The app port will be used by the load balancer to route incoming traffic to this application.
You should set this to the port which your Dockerfile uses to communicate with the internet.`
)

const (
	fmtAddAppToProjectStart    = "Creating ECR repositories for application %s."
	fmtAddAppToProjectFailed   = "Failed to create ECR repositories for application %s."
	fmtAddAppToProjectComplete = "Created ECR repositories for application %s."
)

const (
	fmtParsePortFromDockerfileStart        = "Parsing dockerfile at path %s for application %s..."
	parsePortFromDockerfileFailedTooMany   = "It looks like your Dockerfile exposes more than one port."
	fmtParsePortFromDockerfileFailedNoPort = "Couldn't find an exposed port in dockerfile for application %s."
	fmtParsePortFromDockerfileComplete     = "It looks like your Dockerfile exposes port %d. We'll use that to route traffic to your container from your load balancer."
)

const (
	defaultAppPortString = "80"
)

type initAppVars struct {
	*GlobalOpts
	AppType        string
	AppName        string
	DockerfilePath string
	AppPort        uint16
}

type initAppOpts struct {
	initAppVars

	// Interfaces to interact with dependencies.
	fs           afero.Fs
	ws           wsAppManifestWriter
	appStore     archer.ApplicationStore
	projGetter   archer.ProjectGetter
	projDeployer projectDeployer
	prog         progress
	df           docker.Dockerfile

	// Caches variables
	proj *archer.Project

	// Outputs stored on successful actions.
	manifestPath string

	// sets up Dockerfile parser using fs and input path
	setupParser func(*initAppOpts)
}

func initDockerfileFsFromOpts(o *initAppOpts) {
	o.df = docker.NewDockerfileConfig(o.fs, o.DockerfilePath)
}

func newInitAppOpts(vars initAppVars) (*initAppOpts, error) {
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to project datastore: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	p := session.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, err
	}

	return &initAppOpts{
		initAppVars: vars,

		fs:           &afero.Afero{Fs: afero.NewOsFs()},
		appStore:     store,
		projGetter:   store,
		ws:           ws,
		projDeployer: cloudformation.New(sess),
		prog:         termprogress.NewSpinner(),

		setupParser: initDockerfileFsFromOpts,
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initAppOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if o.AppType != "" {
		if err := validateApplicationType(o.AppType); err != nil {
			return err
		}
	}
	if o.AppName != "" {
		if err := validateApplicationName(o.AppName); err != nil {
			return err
		}
	}
	if o.DockerfilePath != "" {
		if _, err := o.fs.Stat(o.DockerfilePath); err != nil {
			return err
		}
	}
	if o.AppPort != 0 {
		if err := validateApplicationPort(o.AppPort); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initAppOpts) Ask() error {
	if err := o.askAppType(); err != nil {
		return err
	}
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askDockerfile(); err != nil {
		return err
	}
	if err := o.askAppPort(); err != nil {
		return err
	}

	return nil
}

// Execute writes the application's manifest file and stores the application in SSM.
func (o *initAppOpts) Execute() error {
	proj, err := o.projGetter.GetProject(o.ProjectName())
	if err != nil {
		return fmt.Errorf("get project %s: %w", o.ProjectName(), err)
	}
	o.proj = proj

	manifestPath, err := o.createManifest()
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath

	o.prog.Start(fmt.Sprintf(fmtAddAppToProjectStart, o.AppName))
	if err := o.projDeployer.AddAppToProject(o.proj, o.AppName); err != nil {
		o.prog.Stop(log.Serrorf(fmtAddAppToProjectFailed, o.AppName))
		return fmt.Errorf("add app %s to project %s: %w", o.AppName, o.ProjectName(), err)
	}
	o.prog.Stop(log.Ssuccessf(fmtAddAppToProjectComplete, o.AppName))

	if err := o.appStore.CreateApplication(&archer.Application{
		Project: o.ProjectName(),
		Name:    o.AppName,
		Type:    o.AppType,
	}); err != nil {
		return fmt.Errorf("saving application %s: %w", o.AppName, err)
	}
	return nil
}

func (o *initAppOpts) createManifest() (string, error) {
	manifest, err := o.createLoadBalancedAppManifest()
	if err != nil {
		return "", err
	}
	var manifestExists bool
	manifestPath, err := o.ws.WriteAppManifest(manifest, o.AppName)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", err
		}
		manifestExists = true
		manifestPath = e.FileName
	}
	wkdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	relPath, err := filepath.Rel(wkdir, manifestPath)
	if err != nil {
		return "", fmt.Errorf("get relative path of manifest file: %w", err)
	}

	log.Infoln()
	manifestMsgFmt := "Wrote the manifest for %s app at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s app already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.AppName), color.HighlightResource(relPath))
	log.Infoln("Your manifest contains configurations like your container size and ports.")
	log.Infoln()

	return relPath, nil
}

func (o *initAppOpts) createLoadBalancedAppManifest() (*manifest.LBFargateManifest, error) {
	props := &manifest.LBFargateManifestProps{
		AppManifestProps: &manifest.AppManifestProps{
			AppName:    o.AppName,
			Dockerfile: o.DockerfilePath,
		},
		Port: o.AppPort,
		Path: "/",
	}
	existingApps, err := o.appStore.ListApplications(o.ProjectName())
	if err != nil {
		return nil, err
	}
	// We default to "/" for the first app, but if there's another
	// load balanced web app, we use the app name as the default, instead.
	for _, existingApp := range existingApps {
		if existingApp.Type == manifest.LoadBalancedWebApplication && existingApp.Name != o.AppName {
			props.Path = o.AppName
			break
		}
	}
	return manifest.NewLoadBalancedFargateManifest(props), nil
}

func (o *initAppOpts) askAppType() error {
	if o.AppType != "" {
		return nil
	}

	t, err := o.prompt.SelectOne(appInitAppTypePrompt, appInitAppTypeHelpPrompt, manifest.AppTypes)
	if err != nil {
		return fmt.Errorf("failed to get type selection: %w", err)
	}
	o.AppType = t
	return nil
}

func (o *initAppOpts) askAppName() error {
	if o.AppName != "" {
		return nil
	}

	name, err := o.prompt.Get(
		fmt.Sprintf(fmtAppInitAppNamePrompt, color.HighlightUserInput(o.AppType)),
		fmt.Sprintf(fmtAppInitAppNameHelpPrompt, o.ProjectName()),
		validateApplicationName)
	if err != nil {
		return fmt.Errorf("failed to get application name: %w", err)
	}
	o.AppName = name
	return nil
}

// askDockerfile prompts for the Dockerfile by looking at sub-directories with a Dockerfile.
// If the user chooses to enter a custom path, then we prompt them for the path.
func (o *initAppOpts) askDockerfile() error {
	if o.DockerfilePath != "" {
		return nil
	}

	// TODO https://github.com/aws/amazon-ecs-cli-v2/issues/206
	dockerfiles, err := listDockerfiles(o.fs, ".")
	if err != nil {
		return err
	}

	sel, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtAppInitDockerfilePrompt, color.HighlightUserInput(o.AppName)),
		appInitDockerfileHelpPrompt,
		dockerfiles,
	)
	if err != nil {
		return fmt.Errorf("failed to select Dockerfile: %w", err)
	}

	o.DockerfilePath = sel

	return nil
}

func (o *initAppOpts) askAppPort() error {
	// Use flag before anything else
	if o.AppPort != 0 {
		return nil
	}

	o.prog.Start(fmt.Sprintf(fmtParsePortFromDockerfileStart, o.DockerfilePath, o.AppName))

	o.setupParser(o)
	ports := o.df.GetExposedPorts()

	var defaultPort = defaultAppPortString
	switch len(ports) {
	case 0:
		o.prog.Stop(fmt.Sprintf(fmtParsePortFromDockerfileFailedNoPort, o.AppName))
	case 1:
		o.AppPort = ports[0]
		o.prog.Stop(fmt.Sprintf(fmtParsePortFromDockerfileComplete, o.AppPort))
	default:
		defaultPort = strconv.Itoa(int(ports[0]))
		o.prog.Stop(parsePortFromDockerfileFailedTooMany)
	}

	if o.AppPort != 0 {
		return nil
	}
	port, err := o.prompt.Get(
		fmt.Sprintf(appInitAppPortPrompt),
		fmt.Sprintf(appInitAppPortHelpPrompt),
		validateApplicationPort,
		prompt.WithDefaultInput(defaultPort),
	)
	if err != nil {
		return fmt.Errorf("get port: %w", err)
	}

	portUint, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return fmt.Errorf("parse port string: %w", err)
	}

	o.AppPort = uint16(portUint)

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initAppOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.manifestPath)),
		fmt.Sprintf("Run %s to deploy your application to a %s environment.",
			color.HighlightCode(fmt.Sprintf("ecs-preview app deploy --name %s --env %s", o.AppName, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
}

// BuildAppInitCmd build the command for creating a new application.
func BuildAppInitCmd() *cobra.Command {
	vars := initAppVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new application in a project.",
		Long: `Creates a new application in a project.
This command is also run as part of "ecs-preview init".`,
		Example: `
  Create a "frontend" web application.
	/code $ ecs-preview app init --name frontend --app-type "Load Balanced Web App" --dockerfile ./frontend/Dockerfile`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitAppOpts(vars)
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
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.AppName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.AppType, appTypeFlag, appTypeFlagShort, "", appTypeFlagDescription)
	cmd.Flags().StringVarP(&vars.DockerfilePath, dockerFileFlag, dockerFileFlagShort, "", dockerFileFlagDescription)
	cmd.Flags().Uint16Var(&vars.AppPort, appPortFlag, 0, appPortFlagDescription)
	return cmd
}
