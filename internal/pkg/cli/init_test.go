// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitOpts_Run(t *testing.T) {
	testCases := map[string]struct {
		inProject               string
		inShouldDeploy          bool
		inPromptForShouldDeploy bool

		expect      func(opts *InitOpts)
		wantedError string
	}{
		"returns prompt error for app": {
			inProject: "testproject",
			expect: func(opts *InitOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "prompt for app init: my error",
		},
		"returns validation error for app": {
			inProject: "testproject",
			expect: func(opts *InitOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "my error",
		},
		"returns execute error for app": {
			inProject: "testproject",
			expect: func(opts *InitOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.projStore.(*mocks.MockProjectStore).EXPECT().CreateProject(gomock.Any()).Return(nil)
				opts.ws.(*mocks.MockWorkspace).EXPECT().Create(gomock.Any()).Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
			},
			wantedError: "execute app init: my error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectStore := mocks.NewMockProjectStore(ctrl)
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockEnvDeployer := mocks.NewMockEnvironmentDeployer(ctrl)
			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)
			mockIdentity := climocks.NewMockidentityService(ctrl)
			mockInitApp := climocks.NewMockactionCommand(ctrl)

			var logAppName, logAppType string

			opts := &InitOpts{
				Project:               tc.inProject,
				ShouldDeploy:          tc.inShouldDeploy,
				promptForShouldDeploy: tc.inPromptForShouldDeploy,

				initApp: mockInitApp,

				projStore:   mockProjectStore,
				envStore:    mockEnvStore,
				envDeployer: mockEnvDeployer,
				identity:    mockIdentity,
				ws:          mockWorkspace,
				prog:        mockProgress,
				prompt:      mockPrompt,

				// These fields are used for logging, the values are not important for tests.
				appName: &logAppName,
				appType: &logAppType,
			}
			tc.expect(opts)

			// WHEN
			err := opts.Run()

			// THEN
			if tc.wantedError != "" {
				require.EqualError(t, err, tc.wantedError)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
