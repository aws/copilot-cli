// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
)

type pipelineStateGetter interface {
	GetPipelineState(pipelineName string) (*codepipeline.PipelineState, error)
}

// PipelineStatus retrieves status of a pipeline.
type PipelineStatus struct {
	pipelineName string

	pipelineSvc pipelineStateGetter
}

// PipelineStatusDesc contains the status for a pipeline.
type PipelineStatusDesc struct {
	codepipeline.PipelineState
}

// NewPipelineStatus instantiates a new PipelineStatus struct.
func NewPipelineStatus(pipelineName string) (*PipelineStatus, error) {
	sess, err := session.NewProvider().Default()
	if err != nil {
		return nil, err
	}
	pipelineSvc := codepipeline.New(sess)
	return &PipelineStatus{
		pipelineName: pipelineName,
		pipelineSvc:  pipelineSvc,
	}, nil
}

// Describe returns status of a pipeline.
func (w *PipelineStatus) Describe() (*PipelineStatusDesc, error) {
	return &PipelineStatusDesc{}, nil
}
