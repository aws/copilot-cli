// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// App contains serialized parameters for an application.
type App struct {
	Name      string                   `json:"name"`
	URI       string                   `json:"uri"`
	Envs      []*config.Environment    `json:"environments"`
	Services  []*config.Workload       `json:"services"`
	Pipelines []*codepipeline.Pipeline `json:"pipelines"`
}

// JSONString returns the stringified App struct with json format.
func (a *App) JSONString() (string, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("marshal application description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified App struct with human readable format.
func (a *App) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", a.Name)
	fmt.Fprintf(writer, "  %s\t%s\n", "URI", a.URI)
	fmt.Fprint(writer, color.Bold.Sprint("\nEnvironments\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Name", "AccountID", "Region")
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", strings.Repeat("-", len("Name")), strings.Repeat("-", len("AccountID")), strings.Repeat("-", len("Region")))
	for _, env := range a.Envs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", env.Name, env.AccountID, env.Region)
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nServices\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	fmt.Fprintf(writer, "  %s\t%s\n", strings.Repeat("-", len("Name")), strings.Repeat("-", len("Type")))
	for _, svc := range a.Services {
		fmt.Fprintf(writer, "  %s\t%s\n", svc.Name, svc.Type)
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nPipelines\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\n", "Name")
	fmt.Fprintf(writer, "  %s\n", strings.Repeat("-", len("Name")))
	for _, pipeline := range a.Pipelines {
		fmt.Fprintf(writer, "  %s\n", pipeline.Name)
	}
	writer.Flush()
	return b.String()
}
