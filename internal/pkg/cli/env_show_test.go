// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showEnvMocks struct {
	storeSvc  *mocks.Mockstore
	prompt    *mocks.Mockprompter
	describer *mocks.MockenvDescriber
	sel       *mocks.MockconfigSelector
}

func TestEnvShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputApp         string
		inputEnvironment string
		setupMocks       func(mocks showEnvMocks)

		wantedError error
	}{
		"valid app name and environment name": {
			inputApp:         "my-app",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{
						Name: "my-env",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"invalid app name": {
			inputApp:         "my-app",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputApp:         "my-app",
			inputEnvironment: "my-env",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := showEnvMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				store: mockStoreReader,
			}

			// WHEN
			err := showEnvs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestEnvShow_Ask(t *testing.T) {
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inputApp string
		inputEnv string

		setupMocks func(mocks showEnvMocks)

		wantedApp   string
		wantedEnv   string
		wantedError error
	}{
		"with all flags": {
			inputApp: "my-app",
			inputEnv: "my-env",

			setupMocks: func(mocks showEnvMocks) {},

			wantedApp: "my-app",
			wantedEnv: "my-env",
		},
		"returns error when fail to select app": {
			inputApp: "",
			inputEnv: "",

			setupMocks: func(m showEnvMocks) {
				m.sel.EXPECT().Application(envShowAppNamePrompt, envShowAppNameHelpPrompt).Return("", mockErr)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"returns error when fail to select environment": {
			inputApp: "my-app",
			inputEnv: "",

			setupMocks: func(m showEnvMocks) {
				m.sel.EXPECT().Environment(fmt.Sprintf(envShowNamePrompt, color.HighlightUserInput("my-app")), envShowHelpPrompt, "my-app").Return("", mockErr)
			},

			wantedError: fmt.Errorf("select environment for application my-app: some error"),
		},
		"success with no flag set": {
			inputApp: "",
			inputEnv: "",

			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(envShowAppNamePrompt, envShowAppNameHelpPrompt).Return("my-app", nil),
					m.sel.EXPECT().Environment(fmt.Sprintf(envShowNamePrompt, color.HighlightUserInput("my-app")), envShowHelpPrompt, "my-app").Return("my-env", nil),
				)
			},

			wantedApp: "my-app",
			wantedEnv: "my-env",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockconfigSelector(ctrl)

			mocks := showEnvMocks{
				sel: mockSelector,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					envName: tc.inputEnv,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				sel: mockSelector,
			}
			// WHEN
			err := showEnvs.Ask()
			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedApp, showEnvs.AppName(), "expected app name to match")
				require.Equal(t, tc.wantedEnv, showEnvs.envName, "expected environment name to match")
			}
		})
	}
}

func TestEnvShow_Execute(t *testing.T) {
	mockError := errors.New("some error")
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Service{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Service{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Service{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	var wantedResources = []*describe.CfnResource{
		{
			Type:       "AWS::IAM::Role",
			PhysicalID: "testApp-testEnv-CFNExecutionRole",
		},
		{
			Type:       "testApp-testEnv-Cluster",
			PhysicalID: "AWS::ECS::Cluster-jI63pYBWU6BZ",
		},
	}
	mockTags := map[string]string{"copilot-application": "testApp", "copilot-environment": "testEnv", "key1": "value1", "key2": "value2"}
	mockEnvDescription := describe.EnvDescription{
		Environment: testEnv,
		Services:    []*config.Service{testSvc1, testSvc2, testSvc3},
		Tags:        mockTags,
		Resources:   wantedResources,
	}

	testCases := map[string]struct {
		inputEnv         string
		shouldOutputJSON bool

		setupMocks func(mocks showEnvMocks)

		wantedContent string
		wantedError   error
	}{
		"return error if fail to describe the env": {
			inputEnv: "testEnv",
			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("describe environment testEnv: some error"),
		},
		"return error if fail to generate JSON output": {
			inputEnv:         "testEnv",
			shouldOutputJSON: true,
			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&mockEnvDescription, mockError),
				)
			},

			wantedError: fmt.Errorf("describe environment testEnv: some error"),
		},
		"success in human format": {
			inputEnv: "testEnv",
			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&mockEnvDescription, nil),
				)
			},

			wantedContent: "About\n\n  Name              testEnv\n  Production        false\n  Region            us-west-2\n  Account ID        123456789012\n\nServices\n\n  Name              Type\n  ----              ----\n  testSvc1          load-balanced\n  testSvc2          load-balanced\n  testSvc3          load-balanced\n\nTags\n\n  Key                  Value\n  ---                  -----\n  copilot-application  testApp\n  copilot-environment  testEnv\n  key1              value1\n  key2              value2\n\nResources\n\n  AWS::IAM::Role           testApp-testEnv-CFNExecutionRole\n  testApp-testEnv-Cluster  AWS::ECS::Cluster-jI63pYBWU6BZ\n",
		},
		"success in JSON format": {
			inputEnv:         "testEnv",
			shouldOutputJSON: true,
			setupMocks: func(m showEnvMocks) {
				gomock.InOrder(
					m.describer.EXPECT().Describe().Return(&mockEnvDescription, nil),
				)
			},

			wantedContent: "{\"environment\":{\"app\":\"testApp\",\"name\":\"testEnv\",\"region\":\"us-west-2\",\"accountID\":\"123456789012\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"services\":[{\"app\":\"testApp\",\"name\":\"testSvc1\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc2\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc3\",\"type\":\"load-balanced\"}],\"tags\":{\"copilot-application\":\"testApp\",\"copilot-environment\":\"testEnv\",\"key1\":\"value1\",\"key2\":\"value2\"},\"resources\":[{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"testApp-testEnv-CFNExecutionRole\"},{\"type\":\"testApp-testEnv-Cluster\",\"physicalID\":\"AWS::ECS::Cluster-jI63pYBWU6BZ\"}]}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := mocks.NewMockstore(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)

			mocks := showEnvMocks{
				describer: mockEnvDescriber,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					envName:          tc.inputEnv,
					shouldOutputJSON: tc.shouldOutputJSON,
				},
				store:            mockStoreReader,
				describer:        mockEnvDescriber,
				initEnvDescriber: func() error { return nil },
				w:                b,
			}

			// WHEN
			err := showEnvs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
