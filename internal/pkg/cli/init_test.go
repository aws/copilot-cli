// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockAppDeployer struct{}

func (mad mockAppDeployer) init() error {
	return nil
}

func (mad mockAppDeployer) Ask() error {
	return nil
}

func (mad mockAppDeployer) Execute() error {
	return nil
}

func TestInitOpts_Run(t *testing.T) {
	testCases := map[string]struct {
		inShouldDeploy          bool
		inPromptForShouldDeploy bool

		expect      func(opts *initOpts)
		wantedError string
	}{
		"returns prompt error for project": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "prompt for project init: my error",
		},
		"returns validation error for project": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "my error",
		},
		"returns prompt error for app": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "prompt for app init: my error",
		},
		"returns validation error for app": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "my error",
		},
		"returns execute error for project": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
			},
			wantedError: "execute project init: my error",
		},
		"returns execute error for app": {
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
			},
			wantedError: "execute app init: my error",
		},
		"deploys environment": {
			inPromptForShouldDeploy: true,
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt).
					Return(true, nil)
				opts.initEnv.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.appDeploy.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.appDeploy.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"app deploy happy path": {
			inPromptForShouldDeploy: true,
			inShouldDeploy:          true,
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt).
					Return(true, nil)
				opts.initEnv.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.appDeploy.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.appDeploy.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"should not deploy the app if shouldDeploy is false": {
			inPromptForShouldDeploy: true,
			inShouldDeploy:          false,
			expect: func(opts *initOpts) {
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initProject.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initApp.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt).
					Return(false, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mockProjectName, mockAppName, mockAppType string

			opts := &initOpts{
				ShouldDeploy:          tc.inShouldDeploy,
				promptForShouldDeploy: tc.inPromptForShouldDeploy,

				initProject: climocks.NewMockactionCommand(ctrl),
				initApp:     climocks.NewMockactionCommand(ctrl),
				initEnv:     climocks.NewMockactionCommand(ctrl),
				appDeploy:   climocks.NewMockactionCommand(ctrl),

				prompt: climocks.NewMockprompter(ctrl),

				// These fields are used for logging, the values are not important for tests.
				projectName: &mockProjectName,
				appName:     &mockAppName,
				appType:     &mockAppType,
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
