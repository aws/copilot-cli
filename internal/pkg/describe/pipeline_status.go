// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"text/tabwriter"
)

type pipelineStateGetter interface {
	GetPipelineState(pipelineName string) (*codepipeline.PipelineState, error)
}

// PipelineStatus retrieves status of a pipeline.
type PipelineStatusDescriber struct {
	pipelineName string
	pipelineSvc  pipelineStateGetter
}

//PipelineStatus contains the status for a pipeline.
type PipelineStatus struct {
	codepipeline.PipelineState
}

// NewPipelineStatus instantiates a new PipelineStatus struct.
func NewPipelineStatusDescriber(pipelineName string) (*PipelineStatusDescriber, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, err
	}

	pipelineSvc := codepipeline.New(sess)
	return &PipelineStatusDescriber{
		pipelineName: pipelineName,
		pipelineSvc:  pipelineSvc,
	}, nil
}

// Describe returns status of a pipeline.
func (d *PipelineStatusDescriber) Describe() (HumanJSONStringer, error) {
	ps, err := d.pipelineSvc.GetPipelineState(d.pipelineName)
	if err != nil {
		return nil, fmt.Errorf("get pipeline status: %w", err)
	}
	pipelineStatus := &PipelineStatus{*ps}
	return pipelineStatus, nil
}

func (p PipelineStatus) JSONString() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal pipeline status: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (p PipelineStatus) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("Pipeline Status\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t\n", "Stage", "Status", "Transition")
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "-----", "------", "----------")
	for _, stage := range p.StageStates {
		fmt.Fprintf(writer, stage.HumanString())
	}
	writer.Flush()
	fmt.Fprintf(writer, color.Bold.Sprint("\nLast Deployment\n\n"))
	fmt.Fprintf(writer, "  %s\t%s\n", "Updated At", humanizeTime(p.UpdatedAt))
	writer.Flush()
	return b.String()
}
