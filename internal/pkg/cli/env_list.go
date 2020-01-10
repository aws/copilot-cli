// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	environmentListProjectNamePrompt = "Which project's environments would you like to list?"
	environmentListProjectNameHelper = "A project groups all of your environments together."
)

type listEnvOpts struct {
	ShouldOutputJSON bool

	manager       archer.EnvironmentLister
	projectGetter archer.ProjectGetter
	projectLister archer.ProjectLister

	w io.Writer

	*GlobalOpts
}

func (o *listEnvOpts) selectProject() (string, error) {
	projs, err := o.projectLister.ListProjects()
	if err != nil {
		return "", err
	}
	var projStrs []string
	for _, projStr := range projs {
		projStrs = append(projStrs, projStr.Name)
	}
	if len(projStrs) == 0 {
		log.Infoln("There are no projects to select.")
		return "", nil
	}
	proj, err := o.prompt.SelectOne(
		environmentListProjectNamePrompt,
		environmentListProjectNameHelper,
		projStrs,
	)
	return proj, err
}

// Ask asks for fields that are required but not passed in.
func (o *listEnvOpts) Ask() error {
	if o.ProjectName() != "" {
		return nil
	}
	projectName, err := o.selectProject()
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}
	o.projectName = projectName

	return nil
}

// Execute lists the environments through the prompt.
func (o *listEnvOpts) Execute() error {
	// Ensure the project actually exists before we try to list its environments.
	if _, err := o.projectGetter.GetProject(o.ProjectName()); err != nil {
		return err
	}

	envs, err := o.manager.ListEnvironments(o.ProjectName())
	if err != nil {
		return err
	}

	var out string
	if o.ShouldOutputJSON {
		data, err := o.jsonOutput(envs)
		if err != nil {
			return err
		}
		out = data
	} else {
		out = o.humanOutput(envs)
	}
	fmt.Fprintf(o.w, out)

	return nil
}

func (o *listEnvOpts) humanOutput(envs []*archer.Environment) string {
	b := &strings.Builder{}
	prodColor := color.New(color.FgYellow, color.Bold).SprintFunc()
	for _, env := range envs {
		if env.Prod {
			fmt.Fprintf(b, "%s (prod)\n", prodColor(env.Name))
		} else {
			fmt.Fprintln(b, env.Name)
		}
	}
	return b.String()
}

func (o *listEnvOpts) jsonOutput(envs []*archer.Environment) (string, error) {
	type serializedEnvs struct {
		Environments []*archer.Environment `json:"environments"`
	}
	b, err := json.Marshal(serializedEnvs{Environments: envs})
	if err != nil {
		return "", fmt.Errorf("marshal environments: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// BuildEnvListCmd builds the command for listing environments in a project.
func BuildEnvListCmd() *cobra.Command {
	opts := listEnvOpts{
		w:          os.Stdout,
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the environments in a project",
		Example: `
  Lists all the environments for the test project
  /code $ ecs-preview env ls --project test`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.projectLister = ssmStore
			opts.manager = ssmStore
			opts.projectGetter = ssmStore
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().BoolVar(&opts.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	return cmd
}
