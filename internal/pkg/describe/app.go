// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

// App contains serialized parameters for an application.
type App struct {
	Name string                `json:"name"`
	URI  string                `json:"uri"`
	Envs []*config.Environment `json:"environments"`
	Apps []*config.Service     `json:"applications"`
}

// JSONString returns the stringified App struct with json format.
func (p *App) JSONString() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal application: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified App struct with human readable format.
func (p *App) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", p.Name)
	fmt.Fprintf(writer, "  %s\t%s\n", "URI", p.URI)
	fmt.Fprintf(writer, color.Bold.Sprint("\nEnvironments\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Name", "AccountID", "Region")
	for _, env := range p.Envs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", env.Name, env.AccountID, env.Region)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nApplications\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	for _, app := range p.Apps {
		fmt.Fprintf(writer, "  %s\t%s\n", app.Name, app.Type)
	}
	writer.Flush()
	return b.String()
}
