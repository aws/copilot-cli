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
	"github.com/spf13/cobra"
)

const (
	envDeleteNamePrompt = "Which environment would you like to delete?"

	fmtEnvDeleteProfilePrompt  = "Which named profile should we use to delete %s?"
	envDeleteProfileHelpPrompt = "This is usually the same AWS CLI named profile used to create the environment."

	fmtDeleteEnvPrompt = "Are you sure you want to delete environment %s from project %s?"
)

const (
	fmtDeleteEnvStart    = "Deleting environment %s from project %s."
	fmtDeleteEnvFailed   = "Failed to delete environment %s from project %s: %v."
	fmtDeleteEnvComplete = "Deleted environment %s from project %s."
)

type resourceGetter interface {
	GetResources(*resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

// DeleteEnvOpts holds the fields needed to delete an environment.
type DeleteEnvOpts struct {
	// Required flags.
	EnvName          string
	EnvProfile       string
	SkipConfirmation bool

	// Interfaces for dependencies.
	storeClient  archer.EnvironmentStore
	rgClient     resourceGetter
	deployClient environmentDeployer
	prog         progress

	// Cached objects to avoid multiple requests.
	env *archer.Environment

	*GlobalOpts
}

// Ask prompts for fields that are required but not passed in.
func (o *DeleteEnvOpts) Ask() error {
	if err := o.askEnvName(); err != nil {
		return err
	}
	return o.askProfile()
}

// Validate returns an error if the environment name does not exist in the project, or if there are live applications under the environment.
func (o *DeleteEnvOpts) Validate() error {
	// Validate that the environment is stored in SSM.
	env, err := o.storeClient.GetEnvironment(o.ProjectName(), o.EnvName)
	if err != nil {
		return fmt.Errorf("get environment %s metadata in project %s: %w", o.EnvName, o.ProjectName(), err)
	}
	o.env = env

	// Look up applications by searching for cloudformation stacks in the environment's region.
	// TODO We should move this to a package like "describe" similar to the existing "store" pkg.
	stacks, err := o.rgClient.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
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
func (o *DeleteEnvOpts) Execute() error {
	shouldDelete, err := o.shouldDelete(o.env.Project, o.env.Name)
	if err != nil {
		return err
	}
	if !shouldDelete {
		return nil
	}

	isStackDeleted := o.deleteStack()
	if isStackDeleted { // TODO Add a --force flag that attempts to remove from SSM regardless.
		// Only remove from SSM if the stack and roles were deleted. Otherwise, the command will error when re-run.
		o.deleteFromStore()
	}
	return nil
}

// RecommendedActions is a no-op for this command.
func (o *DeleteEnvOpts) RecommendedActions() []string {
	return nil
}

func (o *DeleteEnvOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	envs, err := o.storeClient.ListEnvironments(o.ProjectName())
	if err != nil {
		return fmt.Errorf("list environments under project %s: %w", o.ProjectName(), err)
	}
	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}
	name, err := o.prompt.SelectOne(envDeleteNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("prompt for environment name: %w", err)
	}
	o.EnvName = name
	return nil
}

func (o *DeleteEnvOpts) askProfile() error {
	if o.EnvProfile != "" {
		return nil
	}

	profile, err := o.prompt.Get(
		fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput(o.EnvName)),
		envDeleteProfileHelpPrompt,
		nil, // no validation needed
		prompt.WithDefaultInput("default"))
	if err != nil {
		return fmt.Errorf("prompt to get the profile name: %w", err)
	}
	o.EnvProfile = profile
	return nil
}

func (o *DeleteEnvOpts) shouldDelete(projName, envName string) (bool, error) {
	if o.SkipConfirmation {
		return true, nil
	}

	shouldDelete, err := o.prompt.Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, envName, projName), "")
	if err != nil {
		return false, fmt.Errorf("prompt for environment deletion: %w", err)
	}
	return shouldDelete, nil
}

// deleteStack returns true if the stack was deleted successfully. Otherwise, returns false.
func (o *DeleteEnvOpts) deleteStack() bool {
	o.prog.Start(fmt.Sprintf(fmtDeleteEnvStart, o.env.Name, o.env.Project))
	if err := o.deployClient.DeleteEnvironment(o.env.Project, o.env.Name); err != nil {
		o.prog.Stop(fmt.Sprintf(fmtDeleteEnvFailed, o.env.Name, o.env.Project, err))
		return false
	}
	o.prog.Stop(fmt.Sprintf(fmtDeleteEnvComplete, o.env.Name, o.env.Project))
	return true
}

func (o *DeleteEnvOpts) deleteFromStore() {
	if err := o.storeClient.DeleteEnvironment(o.env.Project, o.env.Name); err != nil {
		log.Infof("Failed to remove environment %s from project %s store: %w\n", o.env.Name, o.env.Project, err)
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
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to ecs-cli metadata store: %w", err)
			}
			opts.storeClient = store

			if err := opts.Ask(); err != nil {
				return err
			}

			p := session.NewProvider()
			profileSess, err := p.FromProfile(opts.EnvProfile)
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
