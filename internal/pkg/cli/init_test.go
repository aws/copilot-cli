// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"

	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	climocks "github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitOpts_Run(t *testing.T) {
	mockSchedule := "@hourly"
	var mockPort uint16 = 80
	var mockAppName = "demo"
	testCases := map[string]struct {
		inShouldDeploy *bool

		inEnvName string
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
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Times(1).Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(errors.New("my error"))
			},
			wantedError: "ask Backend Service: my error",
		},
		"returns validation error for service": {
			inWlType: "Load Balanced Web Service",
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
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
		"fail to deploy an environment": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.sel.(*climocks.MockconfigSelector).EXPECT().Environment(initExistingEnvSelectPrompt, initExistingEnvSelectHelp, mockAppName, prompt.Option{Value: envPromptCreateNew}).Return(envPromptCreateNew, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).Return("test2", nil)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: mockAppName,
					EnvironmentName: "test2",
				})
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(errors.New("some error"))
			},
			wantedError: "some error",
		},
		"fail to get env name": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.sel.(*climocks.MockconfigSelector).EXPECT().Environment(initExistingEnvSelectPrompt, initExistingEnvSelectHelp, mockAppName, prompt.Option{Value: envPromptCreateNew}).Return(envPromptCreateNew, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: "get environment name: some error",
		},
		"deploys environment": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.sel.(*climocks.MockconfigSelector).EXPECT().Environment(initExistingEnvSelectPrompt, initExistingEnvSelectHelp, mockAppName, prompt.Option{Value: envPromptCreateNew}).Return(envPromptCreateNew, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).Return("test2", nil)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: mockAppName,
					EnvironmentName: "test2",
				})
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"env name flag skips prompt": {
			inEnvName: "test2",
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(&config.Environment{}, nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"should not error out if environment change set is empty": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(initShouldDeployPrompt, initShouldDeployHelpPrompt, gomock.Any()).
					Return(true, nil)
				opts.sel.(*climocks.MockconfigSelector).EXPECT().Environment(initExistingEnvSelectPrompt, initExistingEnvSelectHelp, mockAppName, prompt.Option{Value: envPromptCreateNew}).Return(envPromptCreateNew, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(envInitNamePrompt, envInitNameHelpPrompt, gomock.Any(), gomock.Any()).Return("test2", nil)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(&config.Environment{}, nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(fmt.Errorf("wrap: %w", &awscfn.ErrChangeSetEmpty{}))
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"deploy workload happy path": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), []prompt.Option{
					{
						Value: manifestinfo.RequestDrivenWebServiceType,
						Hint:  rdwsTypeHint,
					},
					{
						Value: manifestinfo.LoadBalancedWebServiceType,
						Hint:  lbwsTypeHint,
					},
					{
						Value: manifestinfo.BackendServiceType,
						Hint:  besTypeHint,
					},
					{
						Value: manifestinfo.WorkerServiceType,
						Hint:  wsTypeHint,
					},
					{
						Value: manifestinfo.StaticSiteType,
						Hint:  ssTypeHint,
					},
					{
						Value: manifestinfo.ScheduledJobType,
						Hint:  jobTypeHint,
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
				opts.sel.(*climocks.MockconfigSelector).EXPECT().Environment(initExistingEnvSelectPrompt, initExistingEnvSelectHelp, mockAppName, prompt.Option{Value: envPromptCreateNew}).Return("test2", nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"should not deploy the svc if shouldDeploy is false": {
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
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
		"should not deploy the svc if --deploy=false is specified": {
			inShouldDeploy: aws.Bool(false),
			expect: func(opts *initOpts) {
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(manifestinfo.LoadBalancedWebServiceType, nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
			},
		},
		"should skip prompting if all flags and --deploy explicitly specified": {
			inShouldDeploy: aws.Bool(true),
			inEnvName:      "test2",
			inWlType:       manifestinfo.LoadBalancedWebServiceType,
			inAppName:      mockAppName,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(nil, &config.ErrNoSuchEnvironment{ApplicationName: mockAppName, EnvironmentName: "test2"})
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"env already exists; should skip initializing": {
			inShouldDeploy: aws.Bool(true),
			inEnvName:      "test2",
			inWlType:       manifestinfo.LoadBalancedWebServiceType,
			inAppName:      mockAppName,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(nil, nil)
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Return(nil)
			},
		},
		"error retrieving environment": {
			inShouldDeploy: aws.Bool(true),
			inEnvName:      "test2",
			inWlType:       manifestinfo.LoadBalancedWebServiceType,
			inAppName:      mockAppName,
			expect: func(opts *initOpts) {
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Ask().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Validate().Return(nil)
				opts.initAppCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initWlCmd.(*climocks.MockactionCommand).EXPECT().Execute().Return(nil)
				opts.initEnvCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.store.(*climocks.Mockstore).EXPECT().GetEnvironment(mockAppName, "test2").Return(nil, fmt.Errorf("some error"))
				opts.deployEnvCmd.(*climocks.Mockcmd).EXPECT().Execute().Times(0)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Ask().Times(0)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().Execute().Times(0)
				opts.deploySvcCmd.(*climocks.MockactionCommand).EXPECT().RecommendActions().Times(0)
			},
			wantedError: "some error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &initOpts{
				initVars: initVars{
					appName:      tc.inAppName,
					wkldType:     tc.inWlType,
					envName:      tc.inEnvName,
					shouldDeploy: tc.inShouldDeploy,
				},

				initAppCmd:   climocks.NewMockactionCommand(ctrl),
				initWlCmd:    climocks.NewMockactionCommand(ctrl),
				initEnvCmd:   climocks.NewMockactionCommand(ctrl),
				deployEnvCmd: climocks.NewMockcmd(ctrl),
				deploySvcCmd: climocks.NewMockactionCommand(ctrl),

				sel:    climocks.NewMockconfigSelector(ctrl),
				prompt: climocks.NewMockprompter(ctrl),
				store:  climocks.NewMockstore(ctrl),

				// These fields are used for logging, the values are not important for tests.
				appName:           &mockAppName,
				initWkldVars:      &initWkldVars{},
				schedule:          &mockSchedule,
				port:              &mockPort,
				setupWorkloadInit: func(*initOpts, string) error { return nil },
				useExistingWorkspaceForCMDs: func(opts *initOpts) error {
					return nil
				},
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
