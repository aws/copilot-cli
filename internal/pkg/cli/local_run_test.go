// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testError = errors.New("some error")

type localRunAskMocks struct {
	store *mocks.Mockstore
	sel   *mocks.MockdeploySelector
}

func TestLocalRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputAppName  string
		setupMocks    func(m *localRunAskMocks)
		wantedAppName string
		wantedError   error
	}{
		"no app in workspace": {
			wantedError: errNoAppInWorkspace,
		},
		"fail to read the application from SSM store": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(nil, testError)
			},
			wantedError: fmt.Errorf("get application testApp: %w", testError),
		},
		"successful validation": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(&config.Application{Name: "testApp"}, nil)
			},
			wantedAppName: "testApp",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName: tc.inputAppName,
				},
				store: m.store,
			}
			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLocalRunOpts_Ask(t *testing.T) {
	const (
		testAppName  = "testApp"
		testEnvName  = "testEnv"
		testWkldName = "testWkld"
		testWkldType = "testWkldType"
	)
	testCases := map[string]struct {
		inputAppName  string
		inputEnvName  string
		inputWkldName string

		setupMocks     func(m *localRunAskMocks)
		wantedWkldName string
		wantedEnvName  string
		wantedWkldType string
		wantedError    error
	}{
		"error if provided environment is not present in the workspace": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment(testAppName, testEnvName).Return(nil, testError)
			},
			wantedError: testError,
		},
		"error if provided workload is not present in the workspace": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(nil, testError)
			},
			wantedError: testError,
		},
		"successfully validate env and svc with flags passed in": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment(testAppName, testEnvName).Return(&config.Environment{Name: "testEnv"}, nil)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(&config.Workload{Name: "testWkld"}, nil)
				m.sel.EXPECT().DeployedWorkload(workloadAskPrompt, "", testAppName, gomock.Any()).Return(&selector.DeployedWorkload{
					Env:  "testEnv",
					Name: "testWkld",
					Type: "testWkldType",
				}, nil)
			},
			wantedEnvName:  testEnvName,
			wantedWkldName: testWkldName,
			wantedWkldType: testWkldType,
		},
		"prompt for workload and environment": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetWorkload(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedWorkload(workloadAskPrompt, "", testAppName, gomock.Any()).Return(&selector.DeployedWorkload{
					Env:  "testEnv",
					Name: "testWkld",
					Type: "testWkldType",
				}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(1)
			},
			wantedEnvName:  testEnvName,
			wantedWkldName: testWkldName,
			wantedWkldType: testWkldType,
		},
		"return error while failed to select workload": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.sel.EXPECT().DeployedWorkload(workloadAskPrompt, "", testAppName, gomock.Any()).
					Return(nil, testError)
			},
			wantedError: fmt.Errorf("select a deployed workload from application %s: %w", testAppName, testError),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockdeploySelector(ctrl),
			}
			tc.setupMocks(m)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName:  tc.inputAppName,
					wkldName: tc.inputWkldName,
					envName:  tc.inputEnvName,
				},
				store: m.store,
				sel:   m.sel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWkldName, opts.wkldName)
				require.Equal(t, tc.wantedEnvName, opts.envName)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

type localRunExecuteMocks struct {
	ecsLocalClient *mocks.MockecsLocalClient
}

func TestLocalRunOpts_Execute(t *testing.T) {
	const (
		testAppName  = "testApp"
		testEnvName  = "testEnv"
		testWkldName = "testWkld"
		testWkldType = "testWkldType"
	)
	var taskDefinition = &awsecs.TaskDefinition{
		ContainerDefinitions: []*ecsapi.ContainerDefinition{
			{
				Name: aws.String("container"),
				Environment: []*ecsapi.KeyValuePair{
					{
						Name:  aws.String("COPILOT_SERVICE_NAME"),
						Value: aws.String("testWkld"),
					},
					{
						Name:  aws.String("COPILOT_ENVIRONMENT_NAME"),
						Value: aws.String("testEnv"),
					},
				},
			},
		},
	}
	testCases := map[string]struct {
		inputAppName  string
		inputEnvName  string
		inputWkldName string

		setupMocks     func(m *localRunExecuteMocks)
		wantedWkldName string
		wantedEnvName  string
		wantedWkldType string
		wantedError    error
	}{
		"error getting the task Definition": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("get task definition: %w", testError),
		},
		"error decryting secrets from task definition": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDefinition, nil)
				m.ecsLocalClient.EXPECT().DecryptedSecrets(gomock.Any()).Return(nil, testError)
			},
			wantedError: fmt.Errorf("get secret values: %w", testError),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunExecuteMocks{
				ecsLocalClient: mocks.NewMockecsLocalClient(ctrl),
			}
			tc.setupMocks(m)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName:  tc.inputAppName,
					wkldName: tc.inputWkldName,
					envName:  tc.inputEnvName,
				},
				ecsLocalClient: m.ecsLocalClient,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}
