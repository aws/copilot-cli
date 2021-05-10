// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	climocks "github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitOpts_Run(t *testing.T) {
	mockSchedule := "@hourly"
	var mockPort uint16 = 80
	testCases := map[string]struct {
		inShouldDeploy          bool
		inPromptForShouldDeploy bool

		inAppName string
		inWlType  string

		expect      func(opts *initOpts)
		wantedError string
	}{
		"returns prompt error for application": {
			inWlType: "Load Balanced Web Service",
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
			inWlType: "Backend Service",
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Times(0)
			},
			wantedError: "ask Backend Service: my error",
		},
		"returns validation error for service": {
			inWlType: "Load Balanced Web Service",
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(errors.New("my error"))
			},
			wantedError: "validate Load Balanced Web Service: my error",
		},
		"returns execute error for application": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
			},
			wantedError: "execute app init: my error",
		},
		"returns execute error for service": {
			inWlType: "Load Balanced Web Service",
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(errors.New("my error"))
			},
			wantedError: "execute Load Balanced Web Service init: my error",
		},
		"deploys environment": {
			inPromptForShouldDeploy: true,
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifest.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"deploy workload happy path": {
			inPromptForShouldDeploy: true,
			inShouldDeploy:          true,
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), []prompt.Option{
					{
						Value: manifest.RequestDrivenWebServiceType,
						Hint:  "App Runner",
					},
					{
						Value: manifest.LoadBalancedWebServiceType,
						Hint:  "Internet to ECS on Fargate",
					},
					{
						Value: manifest.BackendServiceType,
						Hint:  "ECS on Fargate",
					},
					{
						Value: manifest.ScheduledJobType,
						Hint:  "Scheduled event to State Machine to Fargate",
					},
				}, gomock.Any())
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

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
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifest.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)

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

			opts := &initOpts{
				initVars: initVars{
					appName:  tc.inAppName,
					wkldType: tc.inWlType,
				},
				ShouldDeploy:          tc.inShouldDeploy,
				promptForShouldDeploy: tc.inPromptForShouldDeploy,

				initAppCmd:   climocks.NewMockactionCommand(ctrl),
				initWlCmd:    climocks.NewMockactionCommand(ctrl),
				initEnvCmd:   climocks.NewMockactionCommand(ctrl),
				deploySvcCmd: climocks.NewMockactionCommand(ctrl),

				prompt: climocks.NewMockprompter(ctrl),

				// These fields are used for logging, the values are not important for tests.
				appName:           &mockAppName,
				initWkldVars:      &initWkldVars{},
				schedule:          &mockSchedule,
				port:              &mockPort,
				setupWorkloadInit: func(*initOpts, string) error { return nil },
			}
			tc.expect(opts)

			// WHEN
			err := opts.Run()

			// THEN
			if tc.wantedError != "" {
				require.EqualError(t, err, tc.wantedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
