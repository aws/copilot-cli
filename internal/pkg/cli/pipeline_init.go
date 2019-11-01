// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// InitPipelineOpts holds the configuration needed to create a new pipeilne
type InitPipelineOpts struct {
	// Fields with matching flags.
	GitHubRepo        string
	GitHubAccessToken string
	Deploy            bool
	EnableCD          bool
	Environments      []string

	// Interfaces to interact with dependencies.  // TODO
	appStore archer.ApplicationStore
	ws       archer.Workspace
	prompt   prompter

	// Outputs stored on successful actions.
	manifestPath string
}

// Ask prompts for fields that are required but not passed in.
func (opts *InitPipelineOpts) Ask() error {
	if opts.GitHubRepo == "" {
		if err := opts.selectGitHubRepo(); err != nil {
			return err
		}
	}
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

// TODO: Nice-to-have: have an opts.listRemoteRepos() method that execs out to `git remote -v` and parse repo name to offer select menu
func (opts *InitPipelineOpts) selectGitHubRepo() error {
	repo, err := opts.prompt.Get(
		fmt.Sprintf("What is your application's GitHub repository?"),
		fmt.Sprintf(`The GitHub repository linked to your workspace. Pushing to this repository will trigger your pipeline build stage.`),
		nil)
	if err != nil {
		return fmt.Errorf("failed to get GitHub repository: %w", err)
	}

	opts.GitHubRepo = repo
	// TODO validate github repo?

	return nil
}

// BuildPipelineInitCmd build the command for creating a new pipeline.
func BuildPipelineInitCmd() *cobra.Command {
	opts := &InitPipelineOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for applications in your workspace.",
		Long:  `Creates a pipeline for the applications in your workspace, using the environments associated with the applications."`,
		Example: `
  Create a pipeline for the applications in your workspace
  /code $ archer pipeline init \
    --github-repo "gitHubUserName/myFrontendApp" \
    --github-access-token file://myGitHubToken \
    --environments "stage,prod" \
    --deploy`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			opts.ws = ws

			return opts.Validate()
		},

		RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().BoolVarP(&opts.Deploy, "deploy", "d", false, "Deploy pipline automatically.")
	cmd.Flags().BoolVarP(&opts.EnableCD, "enable-cd", "", false, "Enables automatic deployment to production environment.")
	cmd.Flags().StringSliceVarP(&opts.Environments, "environments", "e", []string{"build"}, "Environments to add to the pipeline.")

	return cmd
}
