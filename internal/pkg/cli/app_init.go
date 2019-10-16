// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
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
	fs     afero.Fs
	prompt prompter
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
	return nil
}

func (opts *AppInitOpts) askAppType() error {
	t, err := opts.prompt.SelectOne(
		"What type of application do you want to make?",
		"List of infrastructure patterns.",
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
		"Collection of AWS services to achieve a business capability. Must be unique within a project.",
		validateApplicationName)
	if err != nil {
		return fmt.Errorf("failed to get application name: %w", err)
	}
	opts.AppName = name
	return nil
}

func (opts *AppInitOpts) askDockerfile() error {
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

// BuildAppInitCmd build the command for creating a new application.
func BuildAppInitCmd() *cobra.Command {
	opts := &AppInitOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new application in a project.",
		Example: `
  Create a "frontend" web application.
  $ archer app init -n frontend -t "Load Balanced Web App" -d ./frontend/Dockerfile`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.projectName = viper.GetString(projectFlag) // inject from parent command
			opts.fs = &afero.Afero{Fs: afero.NewOsFs()}
			opts.prompt = prompt.New()

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
	}
	cmd.Flags().StringVarP(&opts.AppType, "app-type", "t", "" /* default */, "Type of application to create.")
	cmd.Flags().StringVarP(&opts.AppName, "name", "n", "" /* default */, "Name of the application.")
	cmd.Flags().StringVarP(&opts.DockerfilePath, "dockerfile", "d", "" /* default */, "Path to the Dockerfile.")
	return cmd
}
