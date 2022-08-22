// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showEnvMocks struct {
	storeSvc  *mocks.Mockstore
	describer *mocks.MockenvDescriber
	sel       *mocks.MockconfigSelector
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
		"ensure resources are registered in SSM if users passes flags": {
			inputApp: "my-app",
			inputEnv: "my-env",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, nil)
				m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(nil, nil)
			},

			wantedApp: "my-app",
			wantedEnv: "my-env",
		},
		"should wrap error if validation of app name fails": {
			inputApp: "my-app",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, mockErr)
			},
			wantedError: errors.New(`validate application name "my-app": some error`),
		},
		"should wrap error if validation of env name fails": {
			inputApp: "my-app",
			inputEnv: "my-env",

			setupMocks: func(m showEnvMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, nil)
				m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(nil, mockErr)
			},
			wantedError: errors.New(`validate environment name "my-env" in application "my-app": some error`),
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
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, nil)
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
			mockStore := mocks.NewMockstore(ctrl)

			mocks := showEnvMocks{
				sel:      mockSelector,
				storeSvc: mockStore,
			}

			tc.setupMocks(mocks)

			showEnvs := &showEnvOpts{
				showEnvVars: showEnvVars{
					name:    tc.inputEnv,
					appName: tc.inputApp,
				},
				sel:   mockSelector,
				store: mockStore,
			}
			// WHEN
			err := showEnvs.Ask()
			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, showEnvs.appName, "expected app name to match")
				require.Equal(t, tc.wantedEnv, showEnvs.name, "expected environment name to match")
			}
		})
	}
}

func TestEnvShow_Execute(t *testing.T) {
	mockError := errors.New("some error")
	mockEnvDescription := describe.EnvDescription{
		Environment: &config.Environment{
			App:              "testApp",
			Name:             "testEnv",
			Region:           "us-west-2",
			AccountID:        "123456789012",
			RegistryURL:      "",
			ExecutionRoleARN: "",
			ManagerRoleARN:   "",
		},
		Services: []*config.Workload{
			{
				App:  "testApp",
				Name: "testSvc1",
				Type: "load-balanced",
			}, {
				App:  "testApp",
				Name: "testSvc2",
				Type: "load-balanced",
			}, {
				App:  "testApp",
				Name: "testSvc3",
				Type: "load-balanced",
			}},
		Jobs: []*config.Workload{{
			App:  "testApp",
			Name: "testJob1",
			Type: "Scheduled Job",
		}, {
			App:  "testApp",
			Name: "testJob2",
			Type: "Scheduled Job",
		}},
		Tags: map[string]string{"copilot-application": "testApp", "copilot-environment": "testEnv", "key1": "value1", "key2": "value2"},
		Resources: []*stack.Resource{
			{
				Type:       "AWS::IAM::Role",
				PhysicalID: "testApp-testEnv-CFNExecutionRole",
			},
			{
				Type:       "testApp-testEnv-Cluster",
				PhysicalID: "AWS::ECS::Cluster-jI63pYBWU6BZ",
			},
		},
	}

	testCases := map[string]struct {
		inputEnv             string
		shouldOutputJSON     bool
		shouldOutputManifest bool

		setupMocks func(mocks showEnvMocks)

		wantedContent string
		wantedError   error
	}{
		"return error if fail to describe the env": {
			inputEnv: "testEnv",
			setupMocks: func(m showEnvMocks) {
				m.describer.EXPECT().Describe().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("describe environment testEnv: some error"),
		},
		"return error if fail to generate JSON output": {
			inputEnv:         "testEnv",
			shouldOutputJSON: true,
			setupMocks: func(m showEnvMocks) {
				m.describer.EXPECT().Describe().Return(&mockEnvDescription, mockError)
			},

			wantedError: fmt.Errorf("describe environment testEnv: some error"),
		},
		"should print human format": {
			inputEnv: "testEnv",
			setupMocks: func(m showEnvMocks) {
				m.describer.EXPECT().Describe().Return(&mockEnvDescription, nil)
			},

			wantedContent: `About

  Name        testEnv
  Region      us-west-2
  Account ID  123456789012

Workloads

  Name      Type
  ----      ----
  testSvc1  load-balanced
  testSvc2  load-balanced
  testSvc3  load-balanced
  testJob1  Scheduled Job
  testJob2  Scheduled Job

Tags

  Key                  Value
  ---                  -----
  copilot-application  testApp
  copilot-environment  testEnv
  key1                 value1
  key2                 value2

Resources

  AWS::IAM::Role           testApp-testEnv-CFNExecutionRole
  testApp-testEnv-Cluster  AWS::ECS::Cluster-jI63pYBWU6BZ
`,
		},
		"should print JSON format": {
			inputEnv:         "testEnv",
			shouldOutputJSON: true,
			setupMocks: func(m showEnvMocks) {
				m.describer.EXPECT().Describe().Return(&mockEnvDescription, nil)
			},

			wantedContent: "{\"environment\":{\"app\":\"testApp\",\"name\":\"testEnv\",\"region\":\"us-west-2\",\"accountID\":\"123456789012\",\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"services\":[{\"app\":\"testApp\",\"name\":\"testSvc1\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc2\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc3\",\"type\":\"load-balanced\"}],\"jobs\":[{\"app\":\"testApp\",\"name\":\"testJob1\",\"type\":\"Scheduled Job\"},{\"app\":\"testApp\",\"name\":\"testJob2\",\"type\":\"Scheduled Job\"}],\"tags\":{\"copilot-application\":\"testApp\",\"copilot-environment\":\"testEnv\",\"key1\":\"value1\",\"key2\":\"value2\"},\"resources\":[{\"type\":\"AWS::IAM::Role\",\"physicalID\":\"testApp-testEnv-CFNExecutionRole\"},{\"type\":\"testApp-testEnv-Cluster\",\"physicalID\":\"AWS::ECS::Cluster-jI63pYBWU6BZ\"}],\"environmentVPC\":{\"id\":\"\",\"publicSubnetIDs\":null,\"privateSubnetIDs\":null}}\n",
		},
		"should print manifest file": {
			inputEnv:             "testEnv",
			shouldOutputManifest: true,
			setupMocks: func(m showEnvMocks) {
				m.describer.EXPECT().Manifest().Return([]byte("hello"), nil)
			},

			wantedContent: "hello\n",
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
					name:                 tc.inputEnv,
					shouldOutputJSON:     tc.shouldOutputJSON,
					shouldOutputManifest: tc.shouldOutputManifest,
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
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
