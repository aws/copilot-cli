// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showPipelineMocks struct {
	store *mocks.MockstoreReader
	ws    *mocks.MockwsPipelineReader
}

func TestPipelineShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject string
		setupMocks   func(mocks showPipelineMocks)

		wantedError error
	}{
		"with valid project name": {
			inputProject: "dinder",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetProject("dinder").Return(&archer.Project{
						Name: "dinder",
					}, nil),
				)
			},
			wantedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// NOTE: we don't want to actually make network (in the
			// case of the store, which calls SSM) or system calls
			// (in the case of the workspace, which reads a file on
			// disc) so we create these mocks which we use when
			// creating the &showPipelineOpts. The
			// deletePipelineMocks struct is there to make the
			// tests cases cleaner when we assert expectations on
			// what they're supposed to do within the method we are
			// testing.
			mockStoreReader := mocks.NewMockstoreReader(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)

			mocks := showPipelineMocks{
				store: mockStoreReader,
				ws:    mockWorkspace,
			}

			tc.setupMocks(mocks) // NOTE: this function call is where we make the actual assertions on what we expect the mock objects to do

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				ws:    mockWorkspace,
				store: mockStoreReader,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
