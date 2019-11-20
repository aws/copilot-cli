// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi/resourcegroupstaggingapiiface"
	"github.com/spf13/cobra"
)

const (
	fmtDeleteEnvPrompt = "Delete environment %s from project %s?"
)

const (
	fmtDeleteEnvStart    = "Deleting environment %s from project %s."
	fmtDeleteEnvFailed   = "Failed to delete environment %s from project %s: %v."
	fmtDeleteEnvComplete = "Deleted environment %s from project %s."
)

// DeleteEnvOpts holds the fields needed to delete an environment.
type DeleteEnvOpts struct {
	// Arguments and flags.
	EnvName          string
	SkipConfirmation bool

	// Interfaces for dependencies.
	storeClient  archer.EnvironmentStore
	rgClient     resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI
	iamClient    iamiface.IAMAPI
	deployClient environmentDeployer
	prog         progress

	// Cached objects to avoid multiple requests.
	env *archer.Environment

	*GlobalOpts
}

// Ask is a no-op for this command.
func (opts *DeleteEnvOpts) Ask() error {
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

	if opts.rgClient == nil {
		// Tests mock the client.
		if err := opts.initClientsFromRole(opts.env.ManagerRoleARN, opts.env.Region); err != nil {
			return err
		}
	}

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
	areRolesDeleted := opts.deleteRoles()
	if isStackDeleted && areRolesDeleted { // TODO Add a --force flag that attempts to remove from SSM regardless.
		// Only remove from SSM if the stack and roles were deleted. Otherwise, the command will error when re-run.
		opts.deleteFromStore()
	}
	return nil
}

// RecommendedActions is a no-op for this command.
func (opts *DeleteEnvOpts) RecommendedActions() []string {
	return nil
}

func (opts *DeleteEnvOpts) initClientsFromRole(roleARN, region string) error {
	sess, err := session.FromRole(roleARN, region)
	if err != nil {
		return fmt.Errorf("get session from role %s and region %s: %w", roleARN, region, err)
	}
	opts.rgClient = resourcegroupstaggingapi.New(sess)
	opts.iamClient = iam.New(sess)
	opts.deployClient = cloudformation.New(sess)
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

// deleteRoles returns true if the the execution role and manager role were deleted successfully. Otherwise, returns false.
func (opts *DeleteEnvOpts) deleteRoles() bool {
	// We must delete the EnvManagerRole last since as it's the assumed role for deletions.
	var roleNames []string
	roleARNs := []string{opts.env.ExecutionRoleARN, opts.env.ManagerRoleARN}
	for _, roleARN := range roleARNs {
		parsedARN, err := arn.Parse(roleARN)
		if err != nil {
			log.Infof("Failed to parse the role arn %s: %v\n", roleARN, err)
			return false
		}
		roleNames = append(roleNames, strings.TrimPrefix(parsedARN.Resource, "role/"))
	}

	for _, roleName := range roleNames {
		// We need to delete the policies attached to the role first.
		policies, err := opts.iamClient.ListRolePolicies(&iam.ListRolePoliciesInput{
			RoleName: aws.String(roleName),
		})
		if err != nil {
			log.Infof("Failed to list policies for role %s: %v\n", roleName, err)
			return false
		}
		for _, policy := range policies.PolicyNames {
			if _, err := opts.iamClient.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
				PolicyName: policy,
				RoleName:   aws.String(roleName),
			}); err != nil {
				log.Infof("Failed to delete policy %s from role %s: %v\n", *policy, roleName, err)
				return false
			}
		}

		if _, err := opts.iamClient.DeleteRole(&iam.DeleteRoleInput{
			RoleName: aws.String(roleName),
		}); err != nil {
			log.Infof("Failed to delete role %s: %v\n", roleName, err)
			return false
		}
	}
	return true
}

func (opts *DeleteEnvOpts) deleteFromStore() {
	if err := opts.storeClient.DeleteEnvironment(opts.env.Project, opts.env.Name); err != nil {
		log.Infof("Failed to remove environment %s from project %s store: %w\n", opts.env.Name, opts.env.Project, err)
	}
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
			opts.storeClient = store
			opts.prog = termprogress.NewSpinner()
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
