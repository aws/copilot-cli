// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	svcListAppNamePrompt     = "Which application's services would you like to list?"
	svcListAppNameHelpPrompt = "An application groups all of your services together."

	// Display settings.
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

type listSvcVars struct {
	*GlobalOpts
	ShouldOutputJSON        bool
	ShouldShowLocalServices bool
}

type listSvcOpts struct {
	listSvcVars

	// Interfaces to dependencies.
	store store
	ws    wsSvcReader
	w     io.Writer
	sel   appSelector
}

func newListSvcOpts(vars listSvcVars) (*listSvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	return &listSvcOpts{
		listSvcVars: vars,

		store: store,
		ws:    ws,
		w:     os.Stdout,
		sel:   selector.NewSelect(vars.prompt, store),
	}, nil
}

// Ask asks for fields that are required but not passed in.
func (o *listSvcOpts) Ask() error {
	if o.AppName() != "" {
		return nil
	}

	name, err := o.sel.Application(svcListAppNamePrompt, svcListAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

// Execute lists the services through the prompt.
func (o *listSvcOpts) Execute() error {
	// Ensure the application actually exists before we try to list its services.
	if _, err := o.store.GetApplication(o.AppName()); err != nil {
		return fmt.Errorf("get application: %w", err)
	}

	svcs, err := o.store.ListServices(o.AppName())
	if err != nil {
		return err
	}

	if o.ShouldShowLocalServices {
		localNames, err := o.ws.ServiceNames()
		if err != nil {
			return fmt.Errorf("get local services names: %w", err)
		}
		svcs = filterSvcsByName(svcs, localNames)
	}

	var out string
	if o.ShouldOutputJSON {
		data, err := o.jsonOutput(svcs)
		if err != nil {
			return err
		}
		out = data
		fmt.Fprintf(o.w, out)
	} else {
		o.humanOutput(svcs)
	}

	return nil
}

func (o *listSvcOpts) humanOutput(svcs []*config.Service) {
	writer := tabwriter.NewWriter(o.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "%s\t%s\n", "Name", "Type")
	nameLengthMax := len("Name")
	typeLengthMax := len("Type")
	for _, svc := range svcs {
		nameLengthMax = int(math.Max(float64(nameLengthMax), float64(len(svc.Name))))
		typeLengthMax = int(math.Max(float64(typeLengthMax), float64(len(svc.Type))))
	}
	fmt.Fprintf(writer, "%s\t%s\n", strings.Repeat("-", nameLengthMax), strings.Repeat("-", typeLengthMax))
	for _, svc := range svcs {
		fmt.Fprintf(writer, "%s\t%s\n", svc.Name, svc.Type)
	}
	writer.Flush()
}

func (o *listSvcOpts) jsonOutput(svcs []*config.Service) (string, error) {
	type out struct {
		Services []*config.Service `json:"services"`
	}
	b, err := json.Marshal(out{Services: svcs})
	if err != nil {
		return "", fmt.Errorf("marshal services: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func filterSvcsByName(svcs []*config.Service, wantedNames []string) []*config.Service {
	isWanted := make(map[string]bool)
	for _, name := range wantedNames {
		isWanted[name] = true
	}
	var filtered []*config.Service
	for _, svc := range svcs {
		if _, ok := isWanted[svc.Name]; !ok {
			continue
		}
		filtered = append(filtered, svc)
	}
	return filtered
}

// BuildSvcListCmd builds the command for listing services in an appication.
func BuildSvcListCmd() *cobra.Command {
	vars := listSvcVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the services in an application.",
		Example: `
  Lists all the services for the "myapp" application.
  /code $ copilot svc ls --app myapp`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newListSvcOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().BoolVar(&vars.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.ShouldShowLocalServices, localFlag, false, localSvcFlagDescription)
	return cmd
}
