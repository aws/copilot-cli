// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// App contains serialized parameters for an application.
type App struct {
	Name     string                `json:"name"`
	URI      string                `json:"uri"`
	Envs     []*config.Environment `json:"environments"`
	Services []*config.Service     `json:"services"`
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
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", a.Name)
	fmt.Fprintf(writer, "  %s\t%s\n", "URI", a.URI)
	fmt.Fprintf(writer, color.Bold.Sprint("\nEnvironments\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Name", "AccountID", "Region")
	for _, env := range a.Envs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", env.Name, env.AccountID, env.Region)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nServices\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	for _, svc := range a.Services {
		fmt.Fprintf(writer, "  %s\t%s\n", svc.Name, svc.Type)
	}
	writer.Flush()
	return b.String()
}
