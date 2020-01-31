// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteProjectOptsValidate(t *testing.T) {
	tests := map[string]struct {
		projectName string

		want error
	}{
		"should return error if not in a workspace": {
			projectName: "",
			want:        errNoProjectInWorkspace,
		},
		"should return nil if project name is set": {
			projectName: "mockProjectName",
			want:        nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						projectName: test.projectName,
					},
				},
			}

			got := opts.Validate()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteProjectOptsAsk(t *testing.T) {
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError)")

	var mockPrompter *mocks.Mockprompter

	tests := map[string]struct {
		skipConfirmation bool

		setupMocks func(ctrl *gomock.Controller)

		want error
	}{
		"return nil if skipConfirmation is enabled": {
			skipConfirmation: true,
			setupMocks:       func(ctrl *gomock.Controller) {},
			want:             nil,
		},
		"echo error returned from prompting": {
			skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
						gomock.Any()).
					Return(false, mockError)
			},
			want: mockError,
		},
		"return error if user cancels operation": {skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
						gomock.Any()).
					Return(false, nil)
			},
			want: errOperationCancelled,
		},
		"return nil if user confirms": {skipConfirmation: false,
			setupMocks: func(ctrl *gomock.Controller) {
				mockPrompter = mocks.NewMockprompter(ctrl)
				mockPrompter.EXPECT().
					Confirm(fmt.Sprintf(fmtConfirmProjectDeletePrompt, mockProjectName),
						confirmProjectDeleteHelp,
						gomock.Any()).
					Return(true, nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
						prompt:      mockPrompter,
					},
					skipConfirmation: test.skipConfirmation,
				},
			}

			got := opts.Ask()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteProjectOptsDeleteApps(t *testing.T) {
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError")

	var mockStore *mocks.MockprojectService

	tests := map[string]struct {
		setupMocks func(ctrl *gomock.Controller)
		want       error
	}{
		"return error is listing applications fails": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockprojectService(ctrl)

				mockStore.EXPECT().
					ListApplications(mockProjectName).
					Return(nil, mockError)
			},
			want: mockError,
		},
		"return nil if no apps returned from listing applications": {
			setupMocks: func(ctrl *gomock.Controller) {
				mockStore = mocks.NewMockprojectService(ctrl)

				mockStore.EXPECT().
					ListApplications(mockProjectName).
					Return(nil, nil)
			},
			want: nil,
		},
		// TODO: add more tests when app deletion workflow is inline mockable (provider pattern?)
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := deleteProjOpts{
				deleteProjVars: deleteProjVars{
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
					},
				},
				store: mockStore,
			}

			got := opts.deleteApps()

			require.Equal(t, test.want, got)
		})
	}
}
