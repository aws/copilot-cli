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
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

type pipelineStateGetter interface {
	GetPipelineState(pipelineName string) (*codepipeline.PipelineState, error)
}

// PipelineStatusDescriber retrieves status of a deployed pipeline.
type PipelineStatusDescriber struct {
	pipeline    deploy.Pipeline
	pipelineSvc pipelineStateGetter
}

// PipelineStatus contains the status for a pipeline.
type PipelineStatus struct {
	Name string `json:"name"`
	codepipeline.PipelineState
}

// NewPipelineStatusDescriber instantiates a new PipelineStatus struct.
func NewPipelineStatusDescriber(pipeline deploy.Pipeline) (*PipelineStatusDescriber, error) {
	sess, err := sessions.ImmutableProvider().Default()
	if err != nil {
		return nil, err
	}

	pipelineSvc := codepipeline.New(sess)
	return &PipelineStatusDescriber{
		pipeline:    pipeline,
		pipelineSvc: pipelineSvc,
	}, nil
}

// Describe returns status of a pipeline.
func (d *PipelineStatusDescriber) Describe() (HumanJSONStringer, error) {
	ps, err := d.pipelineSvc.GetPipelineState(d.pipeline.ResourceName)
	if err != nil {
		return nil, fmt.Errorf("get pipeline status: %w", err)
	}
	pipelineStatus := &PipelineStatus{
		Name:          d.pipeline.Name,
		PipelineState: *ps,
	}
	return pipelineStatus, nil
}

// JSONString returns stringified PipelineStatus struct with json format.
func (p PipelineStatus) JSONString() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal pipeline status: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns stringified PipelineStatus struct with human readable format.
func (p PipelineStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("Pipeline Status\n\n"))
	writer.Flush()
	headers := []string{"Stage", "Transition", "Status"}
	fmt.Fprintf(writer, "%s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "%s\n", strings.Join(underline(headers), "\t"))
	for _, stage := range p.StageStates {
		fmt.Fprint(writer, stage.HumanString())
	}
	writer.Flush()
	fmt.Fprint(writer, color.Bold.Sprint("\nLast Deployment\n\n"))
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanizeTime(p.UpdatedAt))
	writer.Flush()
	return b.String()
}
