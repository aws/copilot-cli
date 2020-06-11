// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker/dockerfile"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type runTaskVars struct {
	*GlobalOpts
	Num    uint8
	CPU    uint16
	Memory uint16

	Image          string
	DockerfilePath string

	TaskRole string

	SubnetID         string
	SecurityGroupIDs []string
	App              string
	Env              string

	EnvVars  map[string]string
	Commands string
}

type runTaskOpts struct {
	runTaskVars

	// Interfaces to interact with dependencies.
	fs    afero.Fs
	store store
	df    dockerfileParser

	// sets up Dockerfile parser using fs and input path
	setupParser func(opts *runTaskOpts)
}

func newTaskRunOpts(vars runTaskVars) (*runTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	return &runTaskOpts{
		runTaskVars: vars,

		fs:    &afero.Afero{Fs: afero.NewOsFs()},
		store: store,

		setupParser: func(o *runTaskOpts) {
			o.df = dockerfile.New(o.fs, o.DockerfilePath)
		},
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *runTaskOpts) Validate() error {
	if o.Num == 0 {
		return errors.New("number of tasks must be positive")
	}

	if o.CPU == 0 {
		return errors.New("CPU units must be positive")
	}

	if o.Memory == 0 {
		return errors.New("memory must be positive")
	}

	if o.Image != "" && o.DockerfilePath != "" {
		return errors.New("cannot specify both image and dockerfile path")
	}

	if o.Image != "" {
		if err := o.validateImageName(); err != nil {
			return err
		}
	}

	if o.DockerfilePath != "" {
		if _, err := o.fs.Stat(o.DockerfilePath); err != nil {
			return err
		}
	}

	if o.Env != "" && (o.SubnetID != "" || o.SecurityGroupIDs != nil) {
		return errors.New("can only specify one of a)env and b)subnet id and (or) security groups")
	}

	if o.SubnetID != "" {
		if err := o.validateSubnetID(); err != nil {
			return err
		}
	}

	if o.SecurityGroupIDs != nil {
		if err := o.validateSecurityGroupIDs(); err != nil {
			return err
		}
	}

	if o.App != "" {
		if err := o.validateAppName(); err != nil {
			return err
		}
	}

	if o.Env != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}

	return nil
}

func (o *runTaskOpts) validateImageName() error {
	valid, err := regexp.MatchString(`^\d+\.dkr\.ecr\.[a-z0-9\-]+.amazonaws.com/[a-z][a-z0-9\-]*$`, o.Image)
	if err != nil {
		return fmt.Errorf("validate image name: %w", err)
	}

	if !valid {
		return errors.New("image name is malformed")
	}

	return nil
}

func (o *runTaskOpts) validateSubnetID() error {
	valid, err := regexp.MatchString(`^subnet-[a-z0-9]+`, o.SubnetID)
	if err != nil {
		return fmt.Errorf("validate subnet id: %w", err)
	}

	if !valid {
		return errors.New("subnet id is malformed")
	}

	return nil
}

func (o *runTaskOpts) validateSecurityGroupIDs() error {
	for _, id := range o.SecurityGroupIDs {
		valid, err := regexp.MatchString(`^sg-[a-z0-9]+`, id)
		if err != nil {
			return fmt.Errorf("validate security group ids: %w", err)
		}

		if !valid {
			return errors.New("one or more malformed security group id(s)")
		}
	}
	return nil
}

func (o *runTaskOpts) validateAppName() error {
	if o.AppName() == "" {
		if _, err := o.store.GetApplication(o.App); err != nil {
			return fmt.Errorf("get application: %w", err)
		}
	}
	return nil
}

func (o *runTaskOpts) validateEnvName() error {
	if o.App != "" {
		if _, err := o.store.GetEnvironment(o.App, o.Env); err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
	} else {
		if o.AppName() == "" {
			return errNoAppInWorkspace
		}

		if _, err := o.store.GetEnvironment(o.AppName(), o.Env); err != nil {
			return fmt.Errorf("get environment %s: %w", o.Env, err)
		}
	}

	return nil
}

// BuildTaskRunCmd build the command for running a new task
func BuildTaskRunCmd() *cobra.Command {
	vars := runTaskVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a task",
		Long:  `Run a task.`,
		Example: `
Run a task with default setting
/code $ copilot task run
Run a task to the "test" environment under the current workspace.
/code $ copilot task run --env test
Starts 4 tasks with 2GB memory, Runs a particular image.
/code $ copilot task run --num 4 --memory 2048 --task-role frontend-exec-role
`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskRunOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil { // validate flags
				return err
			}
			return nil
		}),
	}

	cmd.Flags().Uint8Var(&vars.Num, numFlag, 1, numFlagDescription)
	cmd.Flags().Uint16Var(&vars.CPU, cpuFlag, 256, cpuFlagDescription)
	cmd.Flags().Uint16Var(&vars.Memory, memoryFlag, 512, memoryFlagDescription)

	cmd.Flags().StringVar(&vars.Image, imageFlag, "", imageFlagDescription)
	cmd.Flags().StringVar(&vars.DockerfilePath, dockerFileFlag, "", dockerFileFlagDescription)

	cmd.Flags().StringVar(&vars.TaskRole, taskRoleFlag, "", taskRoleFlagDescription)

	cmd.Flags().StringVar(&vars.App, envFlag, "", appFlagDescription)
	cmd.Flags().StringVar(&vars.Env, envFlag, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.SubnetID, subnetIDFlag, "", subnetIDFlagDescription)
	cmd.Flags().StringSliceVar(&vars.SecurityGroupIDs, securityGroupIDsFlag, nil, securityGroupIDsFlagDescription)

	cmd.Flags().StringToStringVar(&vars.EnvVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.Commands, commandFlag, "", commandFlagDescription)

	return cmd
}
