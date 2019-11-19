// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi/resourcegroupstaggingapiiface"
	"github.com/spf13/cobra"
)

// DeleteEnvOpts holds the fields needed to delete an environment.
type DeleteEnvOpts struct {
	// Arguments and flags.
	EnvName          string
	SkipConfirmation bool

	// Interfaces for dependencies.
	store          archer.EnvironmentStore
	resourceGroups resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI

	// Cached objects to avoid multiple requests.
	env      *archer.Environment
	stackARN string

	*GlobalOpts
}

// Ask is a no-op for this command.
func (opts *DeleteEnvOpts) Ask() error {
	return nil
}

// Validate returns an error if the environment name does not exist in the project, or if there are live applications under the environment.
func (opts *DeleteEnvOpts) Validate() error {
	// Validate that the environment is stored in SSM.
	env, err := opts.store.GetEnvironment(opts.ProjectName(), opts.EnvName)
	if err != nil {
		return fmt.Errorf("get environment %s metadata in project %s: %w", opts.EnvName, opts.ProjectName(), err)
	}
	opts.env = env

	if err := opts.initResourceGroupsClient(opts.env.ManagerRoleARN, opts.env.Region); err != nil {
		return err
	}
	// Look up applications by searching for cloudformation stacks in the environment's region.
	stacks, err := opts.resourceGroups.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
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
// Execute assumes that Validate is invoked first.
func (opts *DeleteEnvOpts) Execute() error {
	return nil
}

// RecommendedActions is a no-op for this command.
func (opts *DeleteEnvOpts) RecommendedActions() []string {
	return nil
}

func (opts *DeleteEnvOpts) initResourceGroupsClient(roleARN, region string) error {
	if opts.resourceGroups != nil {
		// Tests initialize these clients.
		return nil
	}

	sess, err := session.FromRole(roleARN, region)
	if err != nil {
		return fmt.Errorf("get session from role %s and region %s: %w", roleARN, region, err)
	}
	opts.resourceGroups = resourcegroupstaggingapi.New(sess)
	return nil
}

// BuildEnvDeleteCmd builds the command to delete environment(s).
func BuildEnvDeleteCmd() *cobra.Command {
	opts := &DeleteEnvOpts{}
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment from your project.",
		Example: `
  Delete the "test" environment.
  /code $ archer env delete test

  Delete the "test" environment without prompting.
  /code $ archer env delete test prod --yes`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires a single environment name as argument")
			}
			return nil
		},
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts.EnvName = args[0]
			opts.GlobalOpts = NewGlobalOpts()
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to ecs-cli metadata store: %w", err)
			}
			opts.store = store
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().BoolVar(&opts.SkipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
