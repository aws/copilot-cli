// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPipelineStatus_Execute(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputJSON    bool
		pipelineName        string
		mockStatusDescriber func(m *mocks.MockpipelineStatusDescriber)
		wantedError         error
	}{
		"errors if failed to describe the status of the pipeline": {
			mockStatusDescriber: func(m *mocks.MockpipelineStatusDescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe status of pipeline : some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStatusDescriber := mocks.NewMockpipelineStatusDescriber(ctrl)
			tc.mockStatusDescriber(mockStatusDescriber)

			pipelineStatus := &pipelineStatusOpts{
				pipelineStatusVars: pipelineStatusVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					pipelineName:     tc.pipelineName,
					GlobalOpts:       &GlobalOpts{},
				},
				statusDescriber:     mockStatusDescriber,
				initStatusDescriber: func(*pipelineStatusOpts) error { return nil },
			}

			// WHEN
			err := pipelineStatus.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.NotEmpty(t, b.String(), "expected output content to not be empty")
			}
		})
	}
}
