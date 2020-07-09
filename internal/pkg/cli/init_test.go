// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package cli

import (
	"errors"
	"testing"

	climocks "github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitOpts_Run(t *testing.T) {
	testCases := map[string]struct {
		inShouldDeploy          bool
		inPromptForShouldDeploy bool

		expect      func(opts *initOpts)
		wantedError string
	}{
		"returns prompt error for application": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "ask app init: my error",
		},
		"returns validation error for application": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "my error",
		},
		"returns prompt error for service": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "ask svc init: my error",
		},
		"returns validation error for service": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "my error",
		},
		"returns execute error for application": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
			},
			wantedError: "execute app init: my error",
		},
		"returns execute error for service": {
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
			},
			wantedError: "execute svc init: my error",
		},
		"deploys environment": {
			inPromptForShouldDeploy: true,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"svc deploy happy path": {
			inPromptForShouldDeploy: true,
			inShouldDeploy:          true,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"should not deploy the svc if shouldDeploy is false": {
			inPromptForShouldDeploy: true,
			inShouldDeploy:          false,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initSvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(false, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mockAppName, mockSvcName, mockSvcType string
			var mockAppPort uint16
			opts := &initOpts{
				ShouldDeploy:          tc.inShouldDeploy,
				promptForShouldDeploy: tc.inPromptForShouldDeploy,

				initAppCmd:   climocks.NewMockactionCommand(ctrl),
				initSvcCmd:   climocks.NewMockactionCommand(ctrl),
				initEnvCmd:   climocks.NewMockactionCommand(ctrl),
				deploySvcCmd: climocks.NewMockactionCommand(ctrl),

				prompt: climocks.NewMockprompter(ctrl),

				// These fields are used for logging, the values are not important for tests.
				appName: &mockAppName,
				svcName: &mockSvcName,
				svcType: &mockSvcType,
				svcPort: &mockAppPort,
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
