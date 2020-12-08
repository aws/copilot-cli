// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/exec/mocks"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSSMPlugin_StartSession(t *testing.T) {
	var mockRunner *mocks.Mockrunner
	tests := map[string]struct {
		inSession   *ecs.Session
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"success": {
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = mocks.NewMockrunner(controller)
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPlugin{
				runner: mockRunner,
			}
			err := s.StartSession(tc.inSession)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
