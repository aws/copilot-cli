// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi/resourcegroupstaggingapiiface"
	"github.com/spf13/cobra"
)

const (
	envDeleteNamePrompt     = "What is your environment's name?"
	envDeleteNameHelpPrompt = "The unique identifier for an existing environment."

	fmtEnvDeleteProfilePrompt  = "Which named profile should we use to delete %s?"
	envDeleteProfileHelpPrompt = "This is usually the same AWS CLI named profile used to create the environment."

	fmtDeleteEnvPrompt = "Are you sure you want to delete environment %s from project %s?"
)

const (
	fmtDeleteEnvStart    = "Deleting environment %s from project %s."
	fmtDeleteEnvFailed   = "Failed to delete environment %s from project %s: %v."
	fmtDeleteEnvComplete = "Deleted environment %s from project %s."
)

// DeleteEnvOpts holds the fields needed to delete an environment.
type DeleteEnvOpts struct {
	// Required flags.
	EnvName          string
	EnvProfile       string
	SkipConfirmation bool

	// Interfaces for dependencies.
	storeClient  archer.EnvironmentStore
	rgClient     resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI
	deployClient environmentDeployer
	prog         progress

	// Cached objects to avoid multiple requests.
	env *archer.Environment

	*GlobalOpts
}

// Ask prompts for fields that are required but not passed in.
func (opts *DeleteEnvOpts) Ask() error {
	if opts.EnvName == "" {
		envName, err := opts.prompt.Get(envDeleteNamePrompt, envDeleteNameHelpPrompt, validateEnvironmentName)
		if err != nil {
			return fmt.Errorf("prompt to get environment name: %w", err)
		}
		opts.EnvName = envName
	}
	if opts.EnvProfile == "" {
		profile, err := opts.prompt.Get(
			fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput(opts.EnvName)),
			envDeleteProfileHelpPrompt,
			nil, // no validation needed
			prompt.WithDefaultInput("default"))
		if err != nil {
			return fmt.Errorf("prompt to get the profile name: %w", err)
		}
		opts.EnvProfile = profile
	}
	return nil
}

// Validate returns an error if the environment name does not exist in the project, or if there are live applications under the environment.
func (opts *DeleteEnvOpts) Validate() error {
	// Validate that the environment is stored in SSM.
	env, err := opts.storeClient.GetEnvironment(opts.ProjectName(), opts.EnvName)
	if err != nil {
		return fmt.Errorf("get environment %s metadata in project %s: %w", opts.EnvName, opts.ProjectName(), err)
	}
	opts.env = env

	// Look up applications by searching for cloudformation stacks in the environment's region.
	// TODO We should move this to a package like "describe" similar to the existing "store" pkg.
	stacks, err := opts.rgClient.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
		ResourceTypeFilters: []*string{aws.String("cloudformation")},
		TagFilters: []*resourcegroupstaggingapi.TagFilter{
			{
				Key:    aws.String(stack.AppTagKey),
				Values: []*string{}, // Matches any application stack.
			},
			{
				Key:    aws.String(stack.EnvTagKey),
				Values: []*string{aws.String(env.Name)},
			},
			{
				Key:    aws.String(stack.ProjectTagKey),
				Values: []*string{aws.String(env.Project)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("find application cloudformation stacks: %w", err)
	}
	if len(stacks.ResourceTagMappingList) > 0 {
		var appNames []string
		for _, cfnStack := range stacks.ResourceTagMappingList {
			for _, t := range cfnStack.Tags {
				if *t.Key != stack.AppTagKey {
					continue
				}
				appNames = append(appNames, *t.Value)
			}
		}
		return fmt.Errorf("applications: '%s' still exist within the environment %s", strings.Join(appNames, ", "), env.Name)
	}
	return nil
}

// Execute deletes the environment from the project by first deleting the stack and then removing the entry from the store.
// If an operation fails, it moves on to the next one instead of halting the execution.
// The environment is removed from the store only if other delete operations succeed.
// Execute assumes that Validate is invoked first.
func (opts *DeleteEnvOpts) Execute() error {
	shouldDelete, err := opts.shouldDelete(opts.env.Project, opts.env.Name)
	if err != nil {
		return err
	}
	if !shouldDelete {
		return nil
	}

	isStackDeleted := opts.deleteStack()
	if isStackDeleted { // TODO Add a --force flag that attempts to remove from SSM regardless.
		// Only remove from SSM if the stack and roles were deleted. Otherwise, the command will error when re-run.
		opts.deleteFromStore()
	}
	return nil
}

// RecommendedActions is a no-op for this command.
func (opts *DeleteEnvOpts) RecommendedActions() []string {
	return nil
}

func (opts *DeleteEnvOpts) shouldDelete(projName, envName string) (bool, error) {
	if opts.SkipConfirmation {
		return true, nil
	}

	shouldDelete, err := opts.prompt.Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, envName, projName), "")
	if err != nil {
		return false, fmt.Errorf("prompt for environment deletion: %w", err)
	}
	return shouldDelete, nil
}

// deleteStack returns true if the stack was deleted successfully. Otherwise, returns false.
func (opts *DeleteEnvOpts) deleteStack() bool {
	opts.prog.Start(fmt.Sprintf(fmtDeleteEnvStart, opts.env.Name, opts.env.Project))
	if err := opts.deployClient.DeleteEnvironment(opts.env.Project, opts.env.Name); err != nil {
		opts.prog.Stop(fmt.Sprintf(fmtDeleteEnvFailed, opts.env.Name, opts.env.Project, err))
		return false
	}
	opts.prog.Stop(fmt.Sprintf(fmtDeleteEnvComplete, opts.env.Name, opts.env.Project))
	return true
}

func (opts *DeleteEnvOpts) deleteFromStore() {
	if err := opts.storeClient.DeleteEnvironment(opts.env.Project, opts.env.Name); err != nil {
		log.Infof("Failed to remove environment %s from project %s store: %w\n", opts.env.Name, opts.env.Project, err)
	}
}

// BuildEnvDeleteCmd builds the command to delete environment(s).
func BuildEnvDeleteCmd() *cobra.Command {
	opts := &DeleteEnvOpts{
		prog:       termprogress.NewSpinner(),
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes an environment from your project.",
		Example: `
  Delete the "test" environment.
  /code $ ecs-preview env delete --name test --profile default

  Delete the "test" environment without prompting.
  /code $ ecs-preview env delete --name test --profile default --yes`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to ecs-cli metadata store: %w", err)
			}
			opts.storeClient = store
			profileSess, err := session.FromProfile(opts.EnvProfile)
			if err != nil {
				return fmt.Errorf("cannot create session from profile %s: %w", opts.EnvProfile, err)
			}
			opts.rgClient = resourcegroupstaggingapi.New(profileSess)
			opts.deployClient = cloudformation.New(profileSess)
			return opts.Validate()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&opts.EnvName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&opts.EnvProfile, profileFlag, "", profileFlagDescription)
	cmd.Flags().BoolVar(&opts.SkipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
