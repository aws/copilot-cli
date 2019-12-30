// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
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
)

const (
	fmtAddAppToProjectStart    = "Creating ECR repositories for application %s."
	fmtAddAppToProjectFailed   = "Failed to create ECR repositories for application %s."
	fmtAddAppToProjectComplete = "Created ECR repositories for application %s."
)

// InitAppOpts holds the configuration needed to create a new application.
type InitAppOpts struct {
	// Fields with matching flags.
	AppType        string
	AppName        string
	DockerfilePath string

	// Interfaces to interact with dependencies.
	fs             afero.Fs
	manifestWriter archer.ManifestIO
	appStore       archer.ApplicationStore
	projGetter     archer.ProjectGetter
	projDeployer   projectDeployer
	prog           progress

	// Outputs stored on successful actions.
	manifestPath string

	*GlobalOpts
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *InitAppOpts) Validate() error {
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}
	if opts.AppType != "" {
		if err := validateApplicationType(opts.AppType); err != nil {
			return err
		}
	}
	if opts.AppName != "" {
		if err := validateApplicationName(opts.AppName); err != nil {
			return err
		}
	}
	if opts.DockerfilePath != "" {
		if _, err := opts.fs.Stat(opts.DockerfilePath); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (opts *InitAppOpts) Ask() error {
	if err := opts.askAppType(); err != nil {
		return err
	}
	if err := opts.askAppName(); err != nil {
		return err
	}
	if err := opts.askDockerfile(); err != nil {
		return err
	}
	return nil
}

// Execute writes the application's manifest file and stores the application in SSM.
func (opts *InitAppOpts) Execute() error {
	if err := opts.ensureNoExistingApp(opts.ProjectName(), opts.AppName); err != nil {
		return err
	}

	manifestPath, err := opts.createManifest()
	if err != nil {
		return err
	}
	opts.manifestPath = manifestPath

	log.Infoln()
	log.Successf("Wrote the manifest for %s app at %s\n", color.HighlightUserInput(opts.AppName), color.HighlightResource(opts.manifestPath))
	log.Infoln("Your manifest contains configurations like your container size and ports.")
	log.Infoln()

	proj, err := opts.projGetter.GetProject(opts.ProjectName())
	if err != nil {
		return fmt.Errorf("get project %s: %w", opts.ProjectName(), err)
	}
	opts.prog.Start(fmt.Sprintf(fmtAddAppToProjectStart, opts.AppName))
	if err := opts.projDeployer.AddAppToProject(proj, opts.AppName); err != nil {
		opts.prog.Stop(log.Serrorf(fmtAddAppToProjectFailed, opts.AppName))
		return fmt.Errorf("add app %s to project %s: %w", opts.AppName, opts.ProjectName(), err)
	}
	opts.prog.Stop(log.Ssuccessf(fmtAddAppToProjectComplete, opts.AppName))

	return opts.createAppInProject(opts.ProjectName())
}

func (opts *InitAppOpts) createManifest() (string, error) {
	manifest, err := manifest.CreateApp(opts.AppName, opts.AppType, opts.DockerfilePath)
	if err != nil {
		return "", fmt.Errorf("generate a manifest: %w", err)
	}
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}
	filename := opts.manifestWriter.AppManifestFileName(opts.AppName)
	manifestPath, err := opts.manifestWriter.WriteFile(manifestBytes, filename)
	if err != nil {
		return "", fmt.Errorf("write manifest for app %s: %w", opts.AppName, err)
	}
	wkdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	relPath, err := filepath.Rel(wkdir, manifestPath)
	if err != nil {
		return "", fmt.Errorf("relative path of manifest file: %w", err)
	}
	return relPath, nil
}

func (opts *InitAppOpts) createAppInProject(projectName string) error {
	if err := opts.appStore.CreateApplication(&archer.Application{
		Project: projectName,
		Name:    opts.AppName,
		Type:    opts.AppType,
	}); err != nil {
		return fmt.Errorf("saving application %s: %w", opts.AppName, err)
	}
	return nil
}

func (opts *InitAppOpts) askAppType() error {
	if opts.AppType != "" {
		return nil
	}

	t, err := opts.prompt.SelectOne(appInitAppTypePrompt, appInitAppTypeHelpPrompt, manifest.AppTypes)
	if err != nil {
		return fmt.Errorf("failed to get type selection: %w", err)
	}
	opts.AppType = t
	return nil
}

func (opts *InitAppOpts) askAppName() error {
	if opts.AppName != "" {
		return nil
	}

	name, err := opts.prompt.Get(
		fmt.Sprintf(fmtAppInitAppNamePrompt, color.HighlightUserInput(opts.AppType)),
		fmt.Sprintf(fmtAppInitAppNameHelpPrompt, opts.ProjectName()),
		validateApplicationName)
	if err != nil {
		return fmt.Errorf("failed to get application name: %w", err)
	}
	opts.AppName = name
	return nil
}

// askDockerfile prompts for the Dockerfile by looking at sub-directories with a Dockerfile.
// If the user chooses to enter a custom path, then we prompt them for the path.
func (opts *InitAppOpts) askDockerfile() error {
	if opts.DockerfilePath != "" {
		return nil
	}

	// TODO https://github.com/aws/amazon-ecs-cli-v2/issues/206
	dockerfiles, err := listDockerfiles(opts.fs, ".")
	if err != nil {
		return err
	}

	sel, err := opts.prompt.SelectOne(
		fmt.Sprintf(fmtAppInitDockerfilePrompt, color.HighlightUserInput(opts.AppName)),
		appInitDockerfileHelpPrompt,
		dockerfiles,
	)
	if err != nil {
		return fmt.Errorf("failed to select Dockerfile: %w", err)
	}

	opts.DockerfilePath = sel

	return nil
}

func (opts *InitAppOpts) ensureNoExistingApp(projectName, appName string) error {
	_, err := opts.appStore.GetApplication(projectName, opts.AppName)
	// If the app doesn't exist - that's perfect, return no error.
	var existsErr *store.ErrNoSuchApplication
	if errors.As(err, &existsErr) {
		return nil
	}
	// If there's no error, that means we were able to fetch an existing app
	if err == nil {
		return fmt.Errorf("application %s already exists under project %s", appName, projectName)
	}
	// Otherwise, there was an error calling the store
	return fmt.Errorf("couldn't check if application %s exists in project %s: %w", appName, projectName, err)
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (opts *InitAppOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(opts.manifestPath)),
		fmt.Sprintf("Run %s to deploy your application to a %s environment.",
			color.HighlightCode(fmt.Sprintf("ecs-preview app deploy --name %s --env %s", opts.AppName, defaultEnvironmentName)),
			defaultEnvironmentName),
	}
}

// BuildAppInitCmd build the command for creating a new application.
func BuildAppInitCmd() *cobra.Command {
	opts := &InitAppOpts{
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
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts.fs = &afero.Afero{Fs: afero.NewOsFs()}

			store, err := store.New()
			if err != nil {
				return fmt.Errorf("couldn't connect to project datastore: %w", err)
			}
			opts.appStore = store
			opts.projGetter = store

			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("workspace cannot be created: %w", err)
			}
			opts.manifestWriter = ws

			p := session.NewProvider()
			sess, err := p.Default()
			if err != nil {
				return err
			}
			opts.projDeployer = cloudformation.New(sess)

			opts.prog = termprogress.NewSpinner()
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().StringVarP(&opts.AppType, appTypeFlag, appTypeFlagShort, "" /* default */, appTypeFlagDescription)
	cmd.Flags().StringVarP(&opts.AppName, nameFlag, nameFlagShort, "" /* default */, appFlagDescription)
	cmd.Flags().StringVarP(&opts.DockerfilePath, dockerFileFlag, dockerFileFlagShort, "" /* default */, dockerFileFlagDescription)
	return cmd
}
