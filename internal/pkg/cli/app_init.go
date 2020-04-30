// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package cli

import (
	"encoding"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker/dockerfile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	appInitAppTypePrompt        = "Which type of " + color.Emphasize("infrastructure pattern") + " best represents your application?"
	fmtAppInitAppTypeHelpPrompt = `A %s is a public, internet-facing, HTTP server that's behind a load balancer. 
To learn more see: https://git.io/JfIpv

A %s is a private, non internet-facing service.
To learn more see: https://git.io/JfIpT`

	fmtAppInitAppNamePrompt     = "What do you want to " + color.Emphasize("name") + " this %s?"
	fmtAppInitAppNameHelpPrompt = `The name will uniquely identify this application within your %s project.
Deployed resources (such as your service, logs) will contain this app's name and be tagged with it.`

	fmtAppInitDockerfilePrompt  = "Which Dockerfile would you like to use for %s?"
	appInitDockerfileHelpPrompt = "Dockerfile to use for building your application's container image."

	appInitAppPortPrompt     = "Which port do you want customer traffic sent to?"
	appInitAppPortHelpPrompt = `The app port will be used by the load balancer to route incoming traffic to this application.
You should set this to the port which your Dockerfile uses to communicate with the internet.`
)

const (
	fmtAddAppToProjectStart    = "Creating ECR repositories for application %s."
	fmtAddAppToProjectFailed   = "Failed to create ECR repositories for application %s."
	fmtAddAppToProjectComplete = "Created ECR repositories for application %s."
)

const (
	fmtParsePortFromDockerfileStart    = "Parsing dockerfile at path %s for application %s...\n"
	parseFromDockerfileTooManyPorts    = "It looks like your Dockerfile exposes more than one port.\n"
	fmtParseFromDockerfileNoPort       = "Couldn't find an exposed port in dockerfile for application %s.\n"
	fmtParsePortFromDockerfileComplete = "It looks like your Dockerfile exposes port %s. We'll use that to route traffic to your container from your load balancer.\n"
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
	df           dockerfileParser

	// Caches variables
	proj *archer.Project

	// Outputs stored on successful actions.
	manifestPath string

	// sets up Dockerfile parser using fs and input path
	setupParser func(*initAppOpts)
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

		setupParser: func(o *initAppOpts) {
			o.df = dockerfile.New(o.fs, o.DockerfilePath)
		},
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
	proj, err := o.projGetter.GetApplication(o.ProjectName())
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

	if err := o.appStore.CreateService(&archer.Application{
		Project: o.ProjectName(),
		Name:    o.AppName,
		Type:    o.AppType,
	}); err != nil {
		return fmt.Errorf("saving application %s: %w", o.AppName, err)
	}
	return nil
}

func (o *initAppOpts) createManifest() (string, error) {
	manifest, err := o.newManifest()
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
	manifestPath, err = relPath(manifestPath)
	if err != nil {
		return "", err
	}

	log.Infoln()
	manifestMsgFmt := "Wrote the manifest for %s app at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for %s app already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.AppName), color.HighlightResource(manifestPath))
	log.Infoln("Your manifest contains configurations like your container size and ports.")
	log.Infoln()

	return manifestPath, nil
}

func (o *initAppOpts) newManifest() (encoding.BinaryMarshaler, error) {
	switch o.AppType {
	case manifest.LoadBalancedWebApplication:
		return o.newLoadBalancedWebAppManifest()
	case manifest.BackendApplication:
		return o.newBackendAppManifest()
	default:
		return nil, fmt.Errorf("application type %s doesn't have a manifest", o.AppType)
	}
}

func (o *initAppOpts) newLoadBalancedWebAppManifest() (*manifest.LoadBalancedWebApp, error) {
	props := &manifest.LoadBalancedWebAppProps{
		AppProps: &manifest.AppProps{
			AppName:    o.AppName,
			Dockerfile: o.DockerfilePath,
		},
		Port: o.AppPort,
		Path: "/",
	}
	existingApps, err := o.appStore.ListServices(o.ProjectName())
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
	return manifest.NewLoadBalancedWebApp(props), nil
}

func (o *initAppOpts) newBackendAppManifest() (*manifest.BackendApp, error) {
	return manifest.NewBackendApp(manifest.BackendAppProps{
		AppProps: manifest.AppProps{
			AppName:    o.AppName,
			Dockerfile: o.DockerfilePath,
		},
		Port: o.AppPort,
	}), nil
}

func (o *initAppOpts) askAppType() error {
	if o.AppType != "" {
		return nil
	}

	help := fmt.Sprintf(fmtAppInitAppTypeHelpPrompt,
		manifest.LoadBalancedWebApplication,
		manifest.BackendApplication,
	)
	t, err := o.prompt.SelectOne(appInitAppTypePrompt, help, manifest.AppTypes)
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

	log.Infof(fmtParsePortFromDockerfileStart,
		color.HighlightUserInput(o.DockerfilePath),
		color.HighlightUserInput(o.AppName),
	)

	o.setupParser(o)
	ports, err := o.df.GetExposedPorts()
	// Ignore any errors in dockerfile parsing--we'll use the default instead.
	if err != nil {
		log.Debugln(err.Error())
	}
	var defaultPort = defaultAppPortString
	switch len(ports) {
	case 0:
		log.Infof(fmtParseFromDockerfileNoPort,
			color.HighlightUserInput(o.AppName),
		)
	case 1:
		o.AppPort = ports[0]
		log.Successf(fmtParsePortFromDockerfileComplete,
			color.HighlightUserInput(strconv.Itoa(int(o.AppPort))),
		)
		return nil
	default:
		defaultPort = strconv.Itoa(int(ports[0]))
		log.Infoln(parseFromDockerfileTooManyPorts)
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
  Create a "frontend" load balanced web application.
  /code $ ecs-preview app init --name frontend --app-type "Load Balanced Web App" --dockerfile ./frontend/Dockerfile

  Create an "subscribers" backend application.
  /code $ ecs-preview app init --name subscribers --app-type "Backend App"`,
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

	// Bucket flags by application type.
	requiredFlags := pflag.NewFlagSet("Required Flags", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(appTypeFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(dockerFileFlag))

	lbWebAppFlags := pflag.NewFlagSet(manifest.LoadBalancedWebApplication, pflag.ContinueOnError)
	lbWebAppFlags.AddFlag(cmd.Flags().Lookup(appPortFlag))

	backendAppFlags := pflag.NewFlagSet(manifest.BackendApplication, pflag.ContinueOnError)
	backendAppFlags.AddFlag(cmd.Flags().Lookup(appPortFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":                          fmt.Sprintf(`Required,%s`, strings.Join(manifest.AppTypes, ",")),
		"Required":                          requiredFlags.FlagUsages(),
		manifest.LoadBalancedWebApplication: lbWebAppFlags.FlagUsages(),
		manifest.BackendApplication:         lbWebAppFlags.FlagUsages(),
	}
	cmd.SetUsageTemplate(`{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{$annotations := .Annotations}}{{$sections := split .Annotations.sections ","}}{{if gt (len $sections) 0}}

{{range $i, $sectionName := $sections}}{{h1 (print $sectionName " Flags")}}
{{(index $annotations $sectionName) | trimTrailingWhitespaces}}{{if ne (inc $i) (len $sections)}}

{{end}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`)
	return cmd
}
