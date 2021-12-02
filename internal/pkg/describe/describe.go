// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"io"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"

	"github.com/dustin/go-humanize"
)

const (
	// Display settings.
	minCellWidth           = 10    // minimum number of characters in a table's cell.
	tabWidth               = 4     // number of characters in between columns.
	cellPaddingWidth       = 2     // number of padding characters added by default to a cell.
	paddingChar            = ' '   // character in between columns.
	dittoSymbol            = `  "` // Symbol used while displaying values in human format.
	noAdditionalFormatting = 0

	statusMinCellWidth     = 12 // minimum number of characters in a table's cell.
	statusCellPaddingWidth = 2
)

// humanizeTime is overridden in tests so that its output is constant as time passes.
var humanizeTime = humanize.Time

// HumanJSONStringer contains methods that stringify app info for output.
type HumanJSONStringer interface {
	HumanString() string
	JSONString() (string, error)
}

type stackDescriber interface {
	Describe() (stack.StackDescription, error)
	Resources() ([]*stack.Resource, error)
	StackMetadata() (string, error)
	StackSetMetadata() (string, error)
}

type deployedSvcResources map[string][]*stack.Resource

func (c deployedSvcResources) humanStringByEnv(w io.Writer, envs []string) {
	for _, env := range envs {
		resources := c[env]
		fmt.Fprintf(w, "\n  %s\n", env)
		for _, resource := range resources {
			fmt.Fprintf(w, "    %s\t%s\n", resource.Type, resource.PhysicalID)
		}
	}
}

func flattenContainerEnvVars(envName string, envVars []*ecs.ContainerEnvVar) []*containerEnvVar {
	var out []*containerEnvVar
	for _, v := range envVars {
		out = append(out, &containerEnvVar{
			envVar: &envVar{
				Name:        v.Name,
				Environment: envName,
				Value:       v.Value,
			},
			Container: v.Container,
		})
	}
	return out
}

func flattenSecrets(envName string, secrets []*ecs.ContainerSecret) []*secret {
	var out []*secret
	for _, s := range secrets {
		out = append(out, &secret{
			Name:        s.Name,
			Container:   s.Container,
			Environment: envName,
			ValueFrom:   s.ValueFrom,
		})
	}
	return out
}

func printTable(w io.Writer, headers []string, rows [][]string) {
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	if len(rows) > 0 {
		fmt.Fprintf(w, "  %s\n", strings.Join(rows[0], "\t"))
	}
	for prev, cur := 0, 1; cur < len(rows); prev, cur = prev+1, cur+1 {
		cells := make([]string, len(rows[cur]))
		copy(cells, rows[cur])
		for i, v := range cells {
			if v == rows[prev][i] {
				cells[i] = dittoSymbol
			}
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cells, "\t"))
	}
}
