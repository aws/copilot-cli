// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/spf13/afero"
)


// InitPipelineOpts holds the configuration needed to create a new pipeilne
type InitPipelineOpts struct {
	// Fields with matching flags.
	GitHubRepo  string
	GitHubAccessToken string
	AppName string
	Deploy bool
	EnableCD bool
	Environments []string

	// Interfaces to interact with dependencies.  // TODO
	fs             afero.Fs
	appStore       archer.ApplicationStore
	manifestWriter archer.ManifestIO
	prompt         prompter

	// Outputs stored on successful actions.
	manifestPath string
}

// Ask prompts for fields that are required but not passed in.
func (opts *InitPipelineOpts) Ask() error {
	// TODO do we need?
	// if opts.AppName == "" {
	// 	if err := opts.askAppName(); err != nil {
	// 		return err
	// 	}
	// }
	// if opts.GitHubRepo == "" {
	// 	if err := opts.askGitHubRepo(); err != nil {
	// 		return err
	// 	}
	// }
	// if opts.GitHubAccessToken == "" {
	// 	if err := opts.askGitHubAccessToken(); err != nil {
	// 		return err
	// 	}
	// }

	// if len(opts.Environments) == 0 {
	// 	if err := opts.askEnvironments(); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *InitPipelineOpts) Validate() error {
	// TODO
	return nil
}

// Execute writes the pipline manifest file.
func (opts *InitPipelineOpts) Execute() error {
	// TODO
	return nil
}

func (opts *InitPipelineOpts) askAppName() error {
	projectName := viper.GetString(projectFlag)
	apps, err := opts.appStore.ListApplications(projectName)
	if err != nil {
		return err
	}

	selections := make([]string, len(apps))
	for _, app := range apps {
		selections = append(selections, app.Name)
	}

	sel, err := opts.prompt.SelectOne(
		fmt.Sprintf("Which application would you like to create a pipeline for?"),
		`The application to create a pipeline for.`,
		selections,
	)
	if err != nil {
		return fmt.Errorf("failed to select application: %w", err)
	}

	opts.AppName = sel

	return nil
}

// BuildPipelineInitCmd build the command for creating a new pipeline.
func BuildPipelineInitCmd() *cobra.Command {
	opts := &InitPipelineOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for an application.",
		Long: `Creates a pipeline for an application in a project, using the environments associated with the application."`,
		Example: `
  Create a pipeline for your "frontend".
  /code $ archer pipeline init \
    --app frontend \
    --github-repo "gitHubUserName/myFrontendApp" \
    --github-access-token file://myGitHubToken \
    --environments stage prod \
    --deploy`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.fs = &afero.Afero{Fs: afero.NewOsFs()}
			opts.prompt = prompt.New()

			store, err := ssm.NewStore()
			if err != nil {
				return fmt.Errorf("couldn't connect to project datastore: %w", err)
			}
			opts.appStore = store

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
	}
	cmd.Flags().StringVarP(&opts.GitHubRepo, "github-repo", "r", "", "GitHub repository for your application.")
	cmd.Flags().StringVarP(&opts.GitHubAccessToken, "github-access-token", "t", "", "GitHub personal access token for your GitHub repository.")
	cmd.Flags().StringVarP(&opts.AppName, "app", "a", "", "Name of the application.")
	cmd.Flags().BoolVarP(&opts.Deploy, "deploy", "d", false, "Deploy pipline automatically.")
	cmd.Flags().BoolVarP(&opts.EnableCD, "enable-cd", "", false, "Enables automatic deployment to production environment.")
	cmd.Flags().StringSliceVarP(&opts.Environments, "environments", "e", []string{"build"}, "Environments to add to the pipeline.")

	return cmd
}
