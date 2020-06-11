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
	num    int8
	cpu    int16
	memory int16

	image          string
	dockerfilePath string

	taskRole string

	subnet         string
	securityGroups []string
	env            string

	envVars  map[string]string
	commands string
}

type runTaskOpts struct {
	runTaskVars

	// Interfaces to interact with dependencies.
	fs     afero.Fs
	store  store
	parser dockerfileParser

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
			o.parser = dockerfile.New(o.fs, o.dockerfilePath)
		},
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *runTaskOpts) Validate() error {
	if o.num <= 0 {
		return errors.New("number of tasks must be positive")
	}

	if o.cpu <= 0 {
		return errors.New("cpu units must be positive")
	}

	if o.memory <= 0 {
		return errors.New("memory must be positive")
	}

	if o.image != "" && o.dockerfilePath != "" {
		return errors.New("cannot specify both image and Dockerfile path")
	}

	if o.image != "" {
		if err := o.validateImageName(); err != nil {
			return err
		}
	} else if o.dockerfilePath != "" {
		if _, err := o.fs.Stat(o.dockerfilePath); err != nil {
			return err
		}
	}

	if o.env != "" && (o.subnet != "" || o.securityGroups != nil) {
		return errors.New("neither subnet nor Security Groups should be specified if environment is specified")
	}

	if o.appName != "" {
		if err := o.validateAppName(); err != nil {
			return err
		}
	}

	if o.env != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}

	return nil
}

func (o *runTaskOpts) validateImageName() error {
	valid, err := regexp.MatchString(`^\d+\.dkr\.ecr\.[a-z0-9\-]+.amazonaws.com/[a-z][a-z0-9\-]*$`, o.image)
	if err != nil {
		return fmt.Errorf("validate image name: %w", err)
	}

	if !valid {
		return errors.New("image name is malformed")
	}

	return nil
}

func (o *runTaskOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	return nil
}

func (o *runTaskOpts) validateEnvName() error {
	if o.AppName() != "" {
		if _, err := o.store.GetEnvironment(o.AppName(), o.env); err != nil {
			return fmt.Errorf("get environment: %w", err)
		}
	} else {
		return errNoAppInWorkspace
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
		Short: "Run a one-off task",
		Long:  `Run a one-off task with configurations such as cpu-units, memory, image, etc.`,
		Example: `
Run a task with default setting.
/code $ copilot task run
Run a task in the "test" environment under the current workspace.
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

	cmd.Flags().Int8Var(&vars.num, numFlag, 1, numFlagDescription)
	cmd.Flags().Int16Var(&vars.cpu, cpuFlag, 256, cpuFlagDescription)
	cmd.Flags().Int16Var(&vars.memory, memoryFlag, 512, memoryFlagDescription)

	cmd.Flags().StringVar(&vars.image, imageFlag, "", imageFlagDescription)
	cmd.Flags().StringVar(&vars.dockerfilePath, dockerFileFlag, "", dockerFileFlagDescription)

	cmd.Flags().StringVar(&vars.taskRole, taskRoleFlag, "", taskRoleFlagDescription)

	cmd.Flags().StringVar(&vars.appName, appFlag, "", appFlagDescription)
	cmd.Flags().StringVar(&vars.env, envFlag, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.subnet, subnetFlag, "", subnetFlagDescription)
	cmd.Flags().StringSliceVar(&vars.securityGroups, securityGroupsFlag, nil, securityGroupsFlagDescription)

	cmd.Flags().StringToStringVar(&vars.envVars, envVarsFlag, nil, envVarsFlagDescription)
	cmd.Flags().StringVar(&vars.commands, commandFlag, "", commandFlagDescription)

	return cmd
}
