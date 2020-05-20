// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe


// PipelineStatus retrieves status of a pipeline.
type PipelineStatus struct {
}

// PipelineStatusDesc contains the status for a pipeline.
type PipelineStatusDesc struct {
}

// NewPipelineStatus instantiates a new PipelineStatus struct.
func NewPipelineStatus() (*PipelineStatus, error) {
	return &PipelineStatus{}, nil
}

// Describe returns status of a pipeline.
func (w *PipelineStatus) Describe() (*PipelineStatusDesc, error) {
	return &PipelineStatusDesc{}, nil
}
