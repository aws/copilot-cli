// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"

	"github.com/spf13/cobra"
)

const (
	pipelineAddEnvPrompt          = "Would you like to add an environment to your pipeline?"
	pipelineSelectEnvPrompt       = "Which environment would you like to add to your pipeline?"
	pipelineEnterGitHubRepoPrompt = "What is your application's GitHub repository?"

	pipelineNoEnvError = "There were no more environments found that can be added to your pipeline. Please run `archer env init` to create a new environment."
)

// InitPipelineOpts holds the configuration needed to create a new pipeilne
type InitPipelineOpts struct {
	// Fields with matching flags.
	Environments      []string
	GitHubRepo        string
	GitHubAccessToken string
	EnableCD          bool
	Deploy            bool

	// Interfaces to interact with dependencies.
	envStore archer.EnvironmentStore
	prompt   prompter

	// Outputs stored on successful actions.
	manifestPath string

	globalOpts
}

func NewInitPipelineOpts() *InitPipelineOpts {
	return &InitPipelineOpts{
		globalOpts: newGlobalOpts(),
		prompt:     prompt.New(),
	}
}

// Ask prompts for fields that are required but not passed in.
func (opts *InitPipelineOpts) Ask() error {
	if len(opts.Environments) == 0 {
		if err := opts.selectEnvironments(true); err != nil {
			return err
		}
	}

	if opts.GitHubRepo == "" {
		if err := opts.selectGitHubRepo(); err != nil {
			return err
		}
	}

	if opts.GitHubAccessToken == "" {
		if err := opts.getGitHubAccessToken(); err != nil {
			return err
		}
	}

	// if err := opts.askEnableCD(); err != nil {
	// 	return err
	// }

	// TODO ask this after pipeline.yml is written
	// if err := opts.askDeploy(); err != nil {
	// 	return err
	// }

	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *InitPipelineOpts) Validate() error {
	// TODO
	if opts.projectName == "" {
		return errNoProjectInWorkspace
	}
	return nil
}

// Execute writes the pipline manifest file.
func (opts *InitPipelineOpts) Execute() error {
	opts.manifestPath = "pipeline.yml" // TODO: placeholder

	log.Infoln()
	log.Successf("Wrote the pipeline for %s app at '%s'\n", color.HighlightUserInput(opts.GitHubRepo), color.HighlightResource(opts.manifestPath))
	log.Infoln("Your pipeline manifest contains configurations for your CodePipeline resources, such as your pipeline stages and build steps.")
	log.Infoln()
	return nil
}

func (opts *InitPipelineOpts) selectEnvironments(addMore bool) error {
	if addMore == false {
		return nil
	}

	addEnv, err := opts.prompt.Confirm(
		pipelineAddEnvPrompt,
		"Adds an environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially.",
	)

	if err != nil {
		return fmt.Errorf("failed to confirm adding an environment: %w", err)
	}

	var selectMoreEnvs bool
	if addEnv {
		selectMore, err := opts.selectEnvironment()
		if err != nil {
			return err
		}
		selectMoreEnvs = selectMore
	}

	return opts.selectEnvironments(selectMoreEnvs)
}

func (opts *InitPipelineOpts) getEnvironments() ([]*archer.Environment, error) {
	envs, err := opts.envStore.ListEnvironments(opts.projectName)
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", opts.projectName, err)
	}

	if len(envs) == 0 {
		return nil, fmt.Errorf(pipelineNoEnvError)
	}

	return envs, nil
}

func (opts *InitPipelineOpts) listAvailableEnvironments() ([]string, error) {
	envs, err := opts.getEnvironments()
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, env := range envs {
		// Check if environment has already been added to pipeline
		if opts.envCanBeAdded(env.Name) {
			names = append(names, env.Name)
		}
	}

	return names, nil
}

func (opts *InitPipelineOpts) envCanBeAdded(selectedEnv string) bool {
	for _, env := range opts.Environments {
		if selectedEnv == env {
			return false
		}
	}
	return true
}

func (opts *InitPipelineOpts) selectEnvironment() (bool, error) {
	selectMoreEnvs := false

	envs, err := opts.listAvailableEnvironments()
	if err != nil {
		return selectMoreEnvs, fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 && len(opts.Environments) != 0 {
		log.Infoln("There are no more environments to add.")
		return selectMoreEnvs, nil
	}

	env, err := opts.prompt.SelectOne(
		pipelineSelectEnvPrompt,
		"Environment to be added as the next stage in your pipeline.",
		envs,
	)

	if err != nil {
		return selectMoreEnvs, fmt.Errorf("failed to add environment: %w", err)
	}

	opts.Environments = append(opts.Environments, env)
	selectMoreEnvs = true

	return selectMoreEnvs, nil
}

// TODO: Nice-to-have: have an opts.listRemoteRepos() method that execs out to `git remote -v` and parse repo name to offer select menu
func (opts *InitPipelineOpts) selectGitHubRepo() error {
	repo, err := opts.prompt.Get(
		pipelineEnterGitHubRepoPrompt,
		fmt.Sprintf(`The GitHub repository linked to your workspace. Pushing to this repository will trigger your pipeline build stage.`),
		nil)

	if err != nil {
		return fmt.Errorf("failed to get GitHub repository: %w", err)
	}

	opts.GitHubRepo = repo
	// TODO validate github repo?

	return nil
}

func (opts *InitPipelineOpts) getGitHubAccessToken() error {
	token, err := opts.prompt.GetSecret(
		fmt.Sprintf("Please enter your GitHub Personal Access Token for your repository: %s", opts.GitHubRepo),
		fmt.Sprintf(`The personal access token for the GitHub repository linked to your workspace. For more information on how to create a personal access token, please refer to: https://help.github.com/en/enterprise/2.17/user/authenticating-to-github/creating-a-personal-access-token-for-the-command-line.`),
	)

	if err != nil {
		return fmt.Errorf("failed to get GitHub access token: %w", err)
	}

	opts.GitHubAccessToken = token

	return nil
}

func (opts *InitPipelineOpts) askEnableCD() error {
	enable, err := opts.prompt.Confirm(
		"Would you like to automatically enable deploying to production?",
		"Enables the transition to your production environment automatically through your pipeline.",
	)

	if err != nil {
		return fmt.Errorf("failed to confirm enabling CD: %w", err)
	}

	opts.EnableCD = enable

	return nil
}

func (opts *InitPipelineOpts) askDeploy() error {
	deploy, err := opts.prompt.Confirm(
		"Would you like to deploy your pipeline?",
		"Deploys your pipeline through CloudFormation.",
	)

	if err != nil {
		return fmt.Errorf("failed to confirm deploying pipeline: %w", err)
	}

	opts.Deploy = deploy

	return nil
}

// BuildPipelineInitCmd build the command for creating a new pipeline.
func BuildPipelineInitCmd() *cobra.Command {
	opts := NewInitPipelineOpts()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for applications in your workspace.",
		Long:  `Creates a pipeline for the applications in your workspace, using the environments associated with the applications."`,
		Example: `
  Create a pipeline for the applications in your workspace:
  /code $ archer pipeline init \
    --github-repo "gitHubUserName/myFrontendApp" \
    --github-access-token file://myGitHubToken \
    --environments "stage,prod" \
    --deploy`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			store, err := ssm.NewStore()
			if err != nil {
				return fmt.Errorf("couldn't connect to environment datastore: %w", err)
			}
			opts.envStore = store

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
	cmd.Flags().StringSliceVarP(&opts.Environments, "environments", "e", []string{}, "Environments to add to the pipeline.")

	return cmd
}
