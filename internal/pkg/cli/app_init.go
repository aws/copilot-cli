// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AppInitOpts holds the configuration needed to create a new application.
type AppInitOpts struct {
	// Fields with matching flags.
	AppType        string
	AppName        string
	DockerfilePath string

	// Injected fields by parent commands.
	projectName string

	// Interfaces to interact with dependencies.
	fs             afero.Fs
	manifestWriter archer.ManifestIO
	prompt         prompter

	// Outputs stored on successful actions.
	manifestPath string
}

// Ask prompts for fields that are required but not passed in.
func (opts *AppInitOpts) Ask() error {
	if opts.AppType == "" {
		if err := opts.askAppType(); err != nil {
			return err
		}
	}
	if opts.AppName == "" {
		if err := opts.askAppName(); err != nil {
			return err
		}
	}
	if opts.DockerfilePath == "" {
		if err := opts.askDockerfile(); err != nil {
			return err
		}
	}
	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *AppInitOpts) Validate() error {
	if opts.AppType != "" {
		if err := validateApplicationType(opts.AppType); err != nil {
			return err
		}
	}
	if opts.AppName != "" {
		if err := validateApplicationName(opts.AppName); err != nil {
			return fmt.Errorf("invalid app name %s: %w", opts.AppName, err)
		}
	}
	if opts.DockerfilePath != "" {
		if _, err := opts.fs.Stat(opts.DockerfilePath); err != nil {
			return err
		}
	}
	if opts.projectName == "" {
		return errors.New("no project found, run `project init` first")
	}
	return nil
}

// Execute writes the application's manifest file and stores the application in SSM.
func (opts *AppInitOpts) Execute() error {
	manifest, err := manifest.CreateApp(opts.AppName, opts.AppType, opts.DockerfilePath)
	if err != nil {
		return fmt.Errorf("generate a manifest: %w", err)
	}
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestPath, err := opts.manifestWriter.WriteManifest(manifestBytes, opts.AppName)
	if err != nil {
		return fmt.Errorf("write manifest for app %s: %w", opts.AppName, err)
	}
	wkdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	relPath, err := filepath.Rel(wkdir, manifestPath)
	if err != nil {
		return fmt.Errorf("relative path of manifest file: %w", err)
	}
	opts.manifestPath = relPath
	log.Infoln()
	log.Successf("Wrote the manifest for %s app at '%s'\n", color.HighlightUserInput(opts.AppName), color.HighlightResource(opts.manifestPath))
	log.Infoln("Your manifest contains configurations like your container size and ports.")
	log.Infoln()
	return nil
}

func (opts *AppInitOpts) askAppType() error {
	t, err := opts.prompt.SelectOne(
		"Which type of infrastructure pattern best represents your application?",
		`Your application's architecture. Most applications need additional AWS resources to run.
To help setup the infrastructure resources, select what "kind" or "type" of application you want to build.`,
		manifest.AppTypes)

	if err != nil {
		return fmt.Errorf("failed to get type selection: %w", err)
	}
	opts.AppType = t
	return nil
}

func (opts *AppInitOpts) askAppName() error {
	name, err := opts.prompt.Get(
		fmt.Sprintf("What do you want to call this %s?", opts.AppType),
		fmt.Sprintf(`The name will uniquely identify this application within your %s project.
Deployed resources (such as your service, logs) will contain this app's name and be tagged with it.`, opts.projectName),
		validateApplicationName)
	if err != nil {
		return fmt.Errorf("failed to get application name: %w", err)
	}
	opts.AppName = name
	return nil
}

// askDockerfile prompts for the Dockerfile by looking at sub-directories with a Dockerfile.
// If the user chooses to enter a custom path, then we prompt them for the path.
func (opts *AppInitOpts) askDockerfile() error {
	// TODO https://github.com/aws/amazon-ecs-cli-v2/issues/206
	dockerfiles, err := opts.listDockerfiles()
	if err != nil {
		return err
	}
	const customPathOpt = "Enter a custom path"
	selections := make([]string, len(dockerfiles))
	copy(selections, dockerfiles)
	selections = append(selections, customPathOpt)

	sel, err := opts.prompt.SelectOne(
		fmt.Sprintf("Which Dockerfile would you like to use for %s app?", opts.AppName),
		"Dockerfile to use for building your application's container image.",
		selections,
	)
	if err != nil {
		return fmt.Errorf("failed to select Dockerfile: %w", err)
	}

	if sel == customPathOpt {
		sel, err = opts.prompt.Get("OK, what's the path to your Dockerfile?", "", nil)
	}
	if err != nil {
		return fmt.Errorf("failed to get Dockerfile: %w", err)
	}
	opts.DockerfilePath = sel
	return nil
}

// listDockerfiles returns the list of Dockerfiles within a sub-directory below current working directory.
// If an error occurs while reading directories, returns the error.
func (opts *AppInitOpts) listDockerfiles() ([]string, error) {
	wdFiles, err := afero.ReadDir(opts.fs, ".")
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var dockerfiles []string
	for _, wdFile := range wdFiles {
		if !wdFile.IsDir() {
			continue
		}
		subFiles, err := afero.ReadDir(opts.fs, filepath.Join(".", wdFile.Name()))
		if err != nil {
			return nil, fmt.Errorf("read directory: %w", err)
		}
		for _, f := range subFiles {
			if f.Name() == "Dockerfile" {
				dockerfiles = append(dockerfiles, filepath.Join(".", wdFile.Name(), "Dockerfile"))
			}
		}
	}
	sort.Strings(dockerfiles)
	return dockerfiles, nil
}

// LogRecommendedActions logs follow-up actions the user can take after successfully executing the command.
func (opts *AppInitOpts) LogRecommendedActions() {
	log.Infoln("Recommended follow-up actions:")
	log.Infof("- Update your manifest %s to change the defaults.\n",
		color.HighlightResource(opts.manifestPath))
	log.Infof("- Run %s to create your staging environment.\n",
		color.HighlightCode(fmt.Sprintf("archer env init --name %s --project %s", defaultEnvironmentName, opts.projectName)))
	log.Infof("- Run %s to deploy your application to the environment.\n",
		color.HighlightCode(fmt.Sprintf("archer app deploy --name %s --env %s --project %s", opts.AppName, defaultEnvironmentName, opts.projectName)))
}

// BuildAppInitCmd build the command for creating a new application.
func BuildAppInitCmd() *cobra.Command {
	opts := &AppInitOpts{}
	cmd := &cobra.Command{
		Use: "init",
		Long: `Create a new application in a project.
This command is also run as part of "archer init".`,
		Example: `
  Create a "frontend" web application.
  $ archer app init --name frontend --app-type "Load Balanced Web App" --dockerfile ./frontend/Dockerfile`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.projectName = viper.GetString(projectFlag) // inject from parent command
			opts.fs = &afero.Afero{Fs: afero.NewOsFs()}
			opts.prompt = prompt.New()

			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("workspace cannot be created: %w", err)
			}
			opts.manifestWriter = ws

			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Warningln("It's best to run this command in the root of your workspace.")
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			return opts.Execute()
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			opts.LogRecommendedActions()
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.AppType, "app-type", "t", "" /* default */, "Type of application to create.")
	cmd.Flags().StringVarP(&opts.AppName, "name", "n", "" /* default */, "Name of the application.")
	cmd.Flags().StringVarP(&opts.DockerfilePath, "dockerfile", "d", "" /* default */, "Path to the Dockerfile.")
	return cmd
}
