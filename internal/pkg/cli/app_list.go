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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	applicationListProjectNamePrompt = "Which project's applications would you like to list?"
	applicationListProjectNameHelper = "A project groups all of your applications together."
)

// ListAppOpts contains the fields to collect for listing an application.
type ListAppOpts struct {
	ShouldOutputJSON    bool
	ShouldShowLocalApps bool

	applications  []*archer.Application
	manager       archer.ApplicationLister
	projectGetter archer.ProjectGetter
	projectLister archer.ProjectLister

	ws archer.Workspace
	w  io.Writer

	*GlobalOpts
}

func (opts *ListAppOpts) selectProject() (string, error) {
	projs, err := opts.projectLister.ListProjects()
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
	proj, err := opts.prompt.SelectOne(
		applicationListProjectNamePrompt,
		applicationListProjectNameHelper,
		projStrs,
	)
	return proj, err
}

func (opts *ListAppOpts) localAppsFilter(appNames []string) {
	var localApps []*archer.Application
	for _, appName := range appNames {
		for _, app := range opts.applications {
			if appName == app.Name {
				localApps = append(localApps, app)
			}
		}
	}
	opts.applications = localApps
}

// Ask asks for fields that are required but not passed in.
func (opts *ListAppOpts) Ask() error {
	if opts.ProjectName() != "" {
		return nil
	}
	projectName, err := opts.selectProject()
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}
	opts.projectName = projectName

	return nil
}

// Execute lists the applications through the prompt.
func (opts *ListAppOpts) Execute() error {
	// Ensure the project actually exists before we try to list its applications.
	if _, err := opts.projectGetter.GetProject(opts.ProjectName()); err != nil {
		return err
	}

	apps, err := opts.manager.ListApplications(opts.ProjectName())
	if err != nil {
		return err
	}
	opts.applications = apps

	if opts.ShouldShowLocalApps {
		localAppManifests, err := opts.ws.Apps()
		if err != nil {
			return fmt.Errorf("failed to get local app manifests: %w", err)
		}
		var localAppNames []string
		for _, appManifest := range localAppManifests {
			localAppNames = append(localAppNames, appManifest.AppName())
		}
		opts.localAppsFilter(localAppNames)
	}

	var out string
	if opts.ShouldOutputJSON {
		data, err := opts.jsonOutput()
		if err != nil {
			return err
		}
		out = data
	} else {
		out = opts.humanOutput()
	}
	fmt.Fprintf(opts.w, out)

	return nil
}

func (opts *ListAppOpts) humanOutput() string {
	b := &strings.Builder{}
	for _, app := range opts.applications {
		fmt.Fprintf(b, "%s: %s\n", app.Type, app.Name)
	}
	return b.String()
}

func (opts *ListAppOpts) jsonOutput() (string, error) {
	type serializedApps struct {
		Applications []*archer.Application `json:"applications"`
	}
	b, err := json.Marshal(serializedApps{Applications: opts.applications})
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// BuildAppListCmd builds the command for listing applications in a project.
func BuildAppListCmd() *cobra.Command {
	opts := ListAppOpts{
		w:          os.Stdout,
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the applications in a project",
		Example: `
  Lists all the applications for the test project
  /code $ archer app ls --project test`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.projectLister = ssmStore
			return opts.Ask()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.New()
			if err != nil {
				return err
			}
			opts.ws = ws
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			opts.projectGetter = ssmStore
			return opts.Execute()
		}),
	}
	cmd.Flags().BoolVar(&opts.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&opts.ShouldShowLocalApps, appLocalFlag, false, appLocalFlagDescription)
	return cmd
}
