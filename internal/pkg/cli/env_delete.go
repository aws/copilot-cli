// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/copilot-cli/internal/pkg/aws/profile"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	envDeleteNamePrompt = "Which environment would you like to delete?"

	fmtEnvDeleteProfilePrompt  = "Which named profile should we use to delete %s?"
	envDeleteProfileHelpPrompt = "This is usually the same AWS CLI named profile used to create the environment."

	fmtDeleteEnvPrompt = "Are you sure you want to delete environment %s from application %s?"
)

const (
	fmtDeleteEnvStart    = "Deleting environment %s from application %s."
	fmtDeleteEnvFailed   = "Failed to delete environment %s from application %s: %v."
	fmtDeleteEnvComplete = "Deleted environment %s from application %s."
)

var (
	errEnvDeleteCancelled = errors.New("env delete cancelled - no changes made")
)

type resourceGetter interface {
	GetResources(*resourcegroupstaggingapi.GetResourcesInput) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

type deleteEnvVars struct {
	*GlobalOpts
	EnvName          string
	EnvProfile       string
	SkipConfirmation bool
}

type deleteEnvOpts struct {
	deleteEnvVars
	// Interfaces for dependencies.
	store         environmentStore
	rgClient      resourceGetter
	deployClient  environmentDeployer
	profileConfig profileNames
	prog          progress
	sel           configSelector

	// initProfileClients is overriden in tests.
	initProfileClients func(*deleteEnvOpts) error
}

func newDeleteEnvOpts(vars deleteEnvVars) (*deleteEnvOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to copilot config store: %w", err)
	}
	cfg, err := profile.NewConfig()
	if err != nil {
		return nil, err
	}

	return &deleteEnvOpts{
		deleteEnvVars: vars,
		store:         store,
		profileConfig: cfg,
		prog:          termprogress.NewSpinner(),
		sel:           selector.NewConfigSelect(vars.prompt, store),
		initProfileClients: func(o *deleteEnvOpts) error {
			profileSess, err := session.NewProvider().FromProfile(o.EnvProfile)
			if err != nil {
				return fmt.Errorf("cannot create session from profile %s: %w", o.EnvProfile, err)
			}
			o.rgClient = resourcegroupstaggingapi.New(profileSess)
			o.deployClient = cloudformation.New(profileSess)
			return nil
		},
	}, nil
}

// Validate returns an error if the individual user inputs are invalid.
func (o *deleteEnvOpts) Validate() error {
	if o.EnvName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *deleteEnvOpts) Ask() error {
	if err := o.askEnvName(); err != nil {
		return err
	}
	if err := o.askProfile(); err != nil {
		return err
	}

	if o.SkipConfirmation {
		return nil
	}
	deleteConfirmed, err := o.prompt.Confirm(fmt.Sprintf(fmtDeleteEnvPrompt, o.EnvName, o.AppName()), "")
	if err != nil {
		return fmt.Errorf("confirm to delete environment %s: %w", o.EnvName, err)
	}
	if !deleteConfirmed {
		return errEnvDeleteCancelled
	}

	return nil
}

// Execute deletes the environment from the application by first deleting the stack and then removing the entry from the store.
// If an operation fails, it moves on to the next one instead of halting the execution.
// The environment is removed from the store only if other delete operations succeed.
// Execute assumes that Validate is invoked first.
func (o *deleteEnvOpts) Execute() error {
	if err := o.initProfileClients(o); err != nil {
		return err
	}
	if err := o.validateNoRunningServices(); err != nil {
		return err
	}

	isStackDeleted := o.deleteStack()
	if isStackDeleted { // TODO Add a --force flag that attempts to remove from SSM regardless.
		// Only remove from SSM if the stack and roles were deleted. Otherwise, the command will error when re-run.
		o.deleteFromStore()
	}
	return nil
}

// RecommendedActions is a no-op for this command.
func (o *deleteEnvOpts) RecommendedActions() []string {
	return nil
}

func (o *deleteEnvOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.AppName(), o.EnvName); err != nil {
		return err
	}
	return nil
}

func (o *deleteEnvOpts) validateNoRunningServices() error {
	stacks, err := o.rgClient.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
		ResourceTypeFilters: []*string{aws.String("cloudformation")},
		TagFilters: []*resourcegroupstaggingapi.TagFilter{
			{
				Key:    aws.String(stack.ServiceTagKey),
				Values: []*string{}, // Matches any service stack.
			},
			{
				Key:    aws.String(stack.EnvTagKey),
				Values: []*string{aws.String(o.EnvName)},
			},
			{
				Key:    aws.String(stack.AppTagKey),
				Values: []*string{aws.String(o.AppName())},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("find service cloudformation stacks: %w", err)
	}
	if len(stacks.ResourceTagMappingList) > 0 {
		var svcNames []string
		for _, cfnStack := range stacks.ResourceTagMappingList {
			for _, t := range cfnStack.Tags {
				if *t.Key != stack.ServiceTagKey {
					continue
				}
				svcNames = append(svcNames, *t.Value)
			}
		}
		return fmt.Errorf("service '%s' still exist within the environment %s", strings.Join(svcNames, ", "), o.EnvName)
	}
	return nil
}

func (o *deleteEnvOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}
	env, err := o.sel.Environment(envDeleteNamePrompt, "", o.AppName())
	if err != nil {
		return fmt.Errorf("select environment to delete: %w", err)
	}
	o.EnvName = env
	return nil
}

func (o *deleteEnvOpts) askProfile() error {
	if o.EnvProfile != "" {
		return nil
	}

	names := o.profileConfig.Names()
	if len(names) == 0 {
		return errNamedProfilesNotFound
	}
	if len(names) == 1 {
		o.EnvProfile = names[0]
		log.Infof("Only found one profile, defaulting to: %s\n", color.HighlightUserInput(o.EnvProfile))
		return nil
	}

	profile, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtEnvDeleteProfilePrompt, color.HighlightUserInput(o.EnvName)),
		envDeleteProfileHelpPrompt,
		names)
	if err != nil {
		return fmt.Errorf("get the profile name: %w", err)
	}
	o.EnvProfile = profile
	return nil
}

// deleteStack returns true if the stack was deleted successfully. Otherwise, returns false.
func (o *deleteEnvOpts) deleteStack() bool {
	o.prog.Start(fmt.Sprintf(fmtDeleteEnvStart, o.EnvName, o.AppName()))
	if err := o.deployClient.DeleteEnvironment(o.AppName(), o.EnvName); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeleteEnvFailed, o.EnvName, o.AppName(), err))
		return false
	}
	o.prog.Stop(log.Ssuccessf(fmtDeleteEnvComplete, o.EnvName, o.AppName()))
	return true
}

func (o *deleteEnvOpts) deleteFromStore() {
	if err := o.store.DeleteEnvironment(o.AppName(), o.EnvName); err != nil {
		log.Infof("Failed to remove environment %s from application %s store: %v\n", o.EnvName, o.AppName(), err)
	}
}

// BuildEnvDeleteCmd builds the command to delete environment(s).
func BuildEnvDeleteCmd() *cobra.Command {
	vars := deleteEnvVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes an environment from your application.",
		Example: `
  Delete the "test" environment.
  /code $ copilot env delete --name test --profile default

  Delete the "test" environment without prompting.
  /code $ copilot env delete --name test --profile default --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteEnvOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.EnvName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.EnvProfile, profileFlag, "", profileFlagDescription)
	cmd.Flags().BoolVar(&vars.SkipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}
