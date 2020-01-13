// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	applicationListProjectNamePrompt     = "Which project's applications would you like to list?"
	applicationListProjectNameHelpPrompt = "A project groups all of your applications together."

	// Display settings.
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

// ListAppOpts contains the fields to collect for listing an application.
type ListAppOpts struct {
	ShouldOutputJSON    bool
	ShouldShowLocalApps bool

	applications  []*archer.Application
	appLister     archer.ApplicationLister
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
	var projNames []string
	for _, proj := range projs {
		projNames = append(projNames, proj.Name)
	}
	if len(projNames) == 0 {
		log.Infoln("There are no projects to select.")
		return "", nil
	}
	proj, err := opts.prompt.SelectOne(
		applicationListProjectNamePrompt,
		applicationListProjectNameHelpPrompt,
		projNames,
	)
	return proj, err
}

func (opts *ListAppOpts) localAppsFilter(appNames []string) {
	var filtered []*archer.Application
	isLocal := make(map[string]bool)
	for _, name := range appNames {
		isLocal[name] = true
	}
	for _, app := range opts.applications {
		if _, ok := isLocal[app.Name]; !ok {
			continue
		}
		filtered = append(filtered, app)
	}
	opts.applications = filtered
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

	apps, err := opts.appLister.ListApplications(opts.ProjectName())
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
		fmt.Fprintf(opts.w, out)
	} else {
		opts.humanOutput()
	}

	return nil
}

func (opts *ListAppOpts) humanOutput() {
	writer := tabwriter.NewWriter(opts.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "%s\t%s\n", "Name", "Type")
	nameLengthMax := len("Name")
	typeLengthMax := len("Type")
	for _, app := range opts.applications {
		nameLengthMax = int(math.Max(float64(nameLengthMax), float64(len(app.Name))))
		typeLengthMax = int(math.Max(float64(typeLengthMax), float64(len(app.Type))))
	}
	fmt.Fprintf(writer, "%s\t%s\n", strings.Repeat("-", nameLengthMax), strings.Repeat("-", typeLengthMax))
	for _, app := range opts.applications {
		fmt.Fprintf(writer, "%s\t%s\n", app.Name, app.Type)
	}
	writer.Flush()
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
  /code $ ecs-preview app ls --project test`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.projectLister = ssmStore
			opts.appLister = ssmStore
			opts.projectGetter = ssmStore

			ws, err := workspace.New()
			if err != nil {
				return err
			}
			opts.ws = ws
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&opts.projectName, projectFlag, projectFlagShort, "dw-run", projectFlagDescription)
	cmd.Flags().BoolVar(&opts.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&opts.ShouldShowLocalApps, appLocalFlag, false, appLocalFlagDescription)
	return cmd
}
