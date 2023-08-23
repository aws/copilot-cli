// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/fatih/color"
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
		inAppName   string
		setupMocks  func(m *localRunAskMocks)
		wantAppName string
		wantError   error
	}{
		"no app in workspace": {
			wantError: errNoAppInWorkspace,
		},
		"fail to read the application from SSM store": {
			inAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(nil, testError)
			},
			wantError: fmt.Errorf("get application testApp: %w", testError),
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
					appName: tc.inAppName,
				},
				store: m.store,
			}
			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
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
	store          *mocks.Mockstore
	sessCreds      credentials.Provider
	interpolator   *mocks.Mockinterpolator
	ws             *mocks.MockwsWlDirReader
	mockMft        *mockWorkloadMft
	mockRunner     *mocks.MockexecRunner
	dockerEngine   *mocks.MockdockerEngineRunner
	repository     *mocks.MockrepositoryService
	ssm            *mocks.MocksecretGetter
	secretsManager *mocks.MocksecretGetter
}

type mockProvider struct {
	FnRetrieve func() (credentials.Value, error)
}

func (m *mockProvider) Retrieve() (credentials.Value, error) {
	return m.FnRetrieve()
}

func (m *mockProvider) IsExpired() bool {
	return false
}

func TestLocalRunOpts_Execute(t *testing.T) {
	const (
		testAppName       = "testApp"
		testEnvName       = "testEnv"
		testWkldName      = "testWkld"
		testWkldType      = "testWkldType"
		testRegion        = "us-test"
		testContainerName = "testConatiner"
	)

	mockApp := config.Application{
		Name: "testApp",
	}
	mockEnv := config.Environment{
		App:            "testApp",
		Name:           "testEnv",
		Region:         "us-test",
		AccountID:      "123456789",
		ManagerRoleARN: "arn::env-manager",
	}

	mockContainerSuffix := fmt.Sprintf("%s-%s-%s", testAppName, testEnvName, testWkldName)
	mockPauseContainerName := pauseContainerName + "-" + mockContainerSuffix

	mockImageInfoList := []clideploy.ImagePerContainer{
		{
			ContainerName: "foo",
			ImageURI:      "image1",
		},
		{
			ContainerName: "bar",
			ImageURI:      "image2",
		},
	}

	taskDef := &ecs.TaskDefinition{
		ContainerDefinitions: []*sdkecs.ContainerDefinition{
			{
				Name: aws.String("foo"),
				Environment: []*sdkecs.KeyValuePair{
					{
						Name:  aws.String("FOO_VAR"),
						Value: aws.String("foo-value"),
					},
				},
				Secrets: []*sdkecs.Secret{
					{
						Name:      aws.String("SHARED_SECRET"),
						ValueFrom: aws.String("mysecret"),
					},
				},
				PortMappings: []*sdkecs.PortMapping{
					{
						HostPort:      aws.Int64(80),
						ContainerPort: aws.Int64(8080),
					},
					{
						HostPort: aws.Int64(9999),
					},
				},
			},
			{
				Name: aws.String("bar"),
				Environment: []*sdkecs.KeyValuePair{
					{
						Name:  aws.String("BAR_VAR"),
						Value: aws.String("bar-value"),
					},
				},
				Secrets: []*sdkecs.Secret{
					{
						Name:      aws.String("SHARED_SECRET"),
						ValueFrom: aws.String("mysecret"),
					},
				},
				PortMappings: []*sdkecs.PortMapping{
					{
						HostPort: aws.Int64(10000),
					},
					{
						HostPort:      aws.Int64(77),
						ContainerPort: aws.Int64(7777),
					},
				},
			},
		},
	}
	expectedRunPauseArgs := &dockerengine.RunOptions{
		ImageURI:      pauseContainerURI,
		ContainerName: mockPauseContainerName,
		ContainerPorts: map[string]string{
			"80":    "8080",
			"999":   "9999",
			"10000": "10000",
			"777":   "7777",
		},
		Command: []string{"sleep", "infinity"},
		LogOptions: dockerengine.RunLogOptions{
			LinePrefix: "[pause] ",
		},
	}
	expectedRunFooArgs := &dockerengine.RunOptions{
		ContainerName: "foo" + "-" + mockContainerSuffix,
		ImageURI:      "image1",
		EnvVars: map[string]string{
			"FOO_VAR":               "foo-value",
			"AWS_ACCESS_KEY_ID":     "myID",
			"AWS_SECRET_ACCESS_KEY": "mySecret",
			"AWS_SESSION_TOKEN":     "myToken",
		},
		Secrets: map[string]string{
			"SHARED_SECRET": "secretvalue",
		},
		ContainerNetwork: mockPauseContainerName,
		LogOptions: dockerengine.RunLogOptions{
			LinePrefix: "[foo] ",
		},
	}
	expectedRunBarArgs := &dockerengine.RunOptions{
		ContainerName: "bar" + "-" + mockContainerSuffix,
		ImageURI:      "image2",
		EnvVars: map[string]string{
			"BAR_VAR":               "bar-value",
			"AWS_ACCESS_KEY_ID":     "myID",
			"AWS_SECRET_ACCESS_KEY": "mySecret",
			"AWS_SESSION_TOKEN":     "myToken",
		},
		Secrets: map[string]string{
			"SHARED_SECRET": "secretvalue",
		},
		ContainerNetwork: mockPauseContainerName,
		LogOptions: dockerengine.RunLogOptions{
			LinePrefix: "[bar] ",
		},
	}
	testCases := map[string]struct {
		inputAppName       string
		inputEnvName       string
		inputWkldName      string
		inputEnvOverrides  map[string]string
		inputPortOverrides []string
		buildImagesError   error

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
		"error getting env vars due to bad override": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputEnvOverrides: map[string]string{
				"bad:OVERRIDE": "i fail",
			},
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
			},
			wantedError: errors.New(`get env vars: parse env overrides: "bad:OVERRIDE" targets invalid container`),
		},
		"error reading workload manifest": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New(`read manifest file for testWkld: some error`),
		},
		"error interpolating workload manifest": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", errors.New("some error"))
			},
			wantedError: errors.New(`interpolate environment variables for testWkld manifest: some error`),
		},
		"error building container images": {
			inputAppName:     testAppName,
			inputWkldName:    testWkldName,
			inputEnvName:     testEnvName,
			buildImagesError: errors.New("some error"),
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
			},
			wantedError: errors.New(`build images: some error`),
		},
		"error if fail to run pause container": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).Return(errors.New("some error"))
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).Return(false, nil).AnyTimes()

				m.dockerEngine.EXPECT().Stop(gomock.Any()).Return(nil).AnyTimes()
				m.dockerEngine.EXPECT().Rm(gomock.Any()).Return(nil).AnyTimes()
			},
			wantedError: errors.New(`run pause container: some error`),
		},
		"error if fail to check if pause container running": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				runCalled := make(chan struct{})
				isRunningCalled := make(chan struct{})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					close(runCalled)
					<-isRunningCalled
					return nil
				})
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).DoAndReturn(func(name string) (bool, error) {
					<-runCalled
					defer close(isRunningCalled)
					return false, errors.New("some error")
				})

				m.dockerEngine.EXPECT().Stop(gomock.Any()).Return(nil).Times(3)
				m.dockerEngine.EXPECT().Rm(gomock.Any()).Return(nil).Times(3)
			},
			wantedError: errors.New(`run pause container: check if container is running: some error`),
		},
		"error if fail to run service container": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				runCalled := make(chan struct{})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					close(runCalled)
					return nil
				})
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).DoAndReturn(func(name string) (bool, error) {
					<-runCalled
					return true, nil
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunFooArgs).Return(errors.New("some error"))
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunBarArgs).Return(nil)

				m.dockerEngine.EXPECT().Stop(gomock.Any()).Return(nil).Times(3)
				m.dockerEngine.EXPECT().Rm(gomock.Any()).Return(nil).Times(3)
			},
			wantedError: errors.New(`run container "foo": some error`),
		},
		"success": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				runCalled := make(chan struct{})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					close(runCalled)
					return nil
				})
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).DoAndReturn(func(name string) (bool, error) {
					<-runCalled
					return true, nil
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunFooArgs).Return(nil)
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunBarArgs).Return(nil)

				m.dockerEngine.EXPECT().Stop(gomock.Any()).Return(nil).Times(3)
				m.dockerEngine.EXPECT().Rm(gomock.Any()).Return(nil).Times(3)
			},
		},
		"ctrl-c, errors stopping and removing containers": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				runCalled := make(chan struct{})
				stopCalled := make(chan struct{})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					close(runCalled)
					return nil
				})
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).DoAndReturn(func(name string) (bool, error) {
					<-runCalled
					return true, nil
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunFooArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					syscall.Kill(syscall.Getpid(), syscall.SIGINT)
					<-stopCalled
					return errors.New("hi")
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunBarArgs).Return(nil)

				m.dockerEngine.EXPECT().Stop(expectedRunFooArgs.ContainerName).DoAndReturn(func(id string) error {
					close(stopCalled)
					return errors.New("stop foo")
				})
				m.dockerEngine.EXPECT().Stop(expectedRunBarArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Rm(expectedRunBarArgs.ContainerName).Return(errors.New("rm bar"))
				m.dockerEngine.EXPECT().Stop(expectedRunPauseArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Rm(expectedRunPauseArgs.ContainerName).Return(errors.New("rm stop"))
			},
			wantedError: fmt.Errorf("clean up %q: stop: stop foo\nclean up %q: rm: rm bar\nclean up %q: rm: rm stop", expectedRunFooArgs.ContainerName, expectedRunBarArgs.ContainerName, expectedRunPauseArgs.ContainerName),
		},
		"handles ctrl-c successfully, errors from running ignored": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *localRunExecuteMocks) {
				m.ecsLocalClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				runCalled := make(chan struct{})
				stopCalled := make(chan struct{})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunPauseArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					close(runCalled)
					return nil
				})
				m.dockerEngine.EXPECT().IsContainerRunning(mockPauseContainerName).DoAndReturn(func(name string) (bool, error) {
					<-runCalled
					return true, nil
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunFooArgs).DoAndReturn(func(ctx context.Context, opts *dockerengine.RunOptions) error {
					syscall.Kill(syscall.Getpid(), syscall.SIGINT)
					<-stopCalled
					return errors.New("hi")
				})
				m.dockerEngine.EXPECT().Run(gomock.Any(), expectedRunBarArgs).Return(nil)

				m.dockerEngine.EXPECT().Stop(expectedRunFooArgs.ContainerName).DoAndReturn(func(id string) error {
					close(stopCalled)
					return nil
				})
				m.dockerEngine.EXPECT().Rm(expectedRunFooArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Stop(expectedRunBarArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Rm(expectedRunBarArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Stop(expectedRunPauseArgs.ContainerName).Return(nil)
				m.dockerEngine.EXPECT().Rm(expectedRunPauseArgs.ContainerName).Return(nil)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunExecuteMocks{
				ecsLocalClient: mocks.NewMockecsLocalClient(ctrl),
				ssm:            mocks.NewMocksecretGetter(ctrl),
				secretsManager: mocks.NewMocksecretGetter(ctrl),
				store:          mocks.NewMockstore(ctrl),
				interpolator:   mocks.NewMockinterpolator(ctrl),
				ws:             mocks.NewMockwsWlDirReader(ctrl),
				mockRunner:     mocks.NewMockexecRunner(ctrl),
				dockerEngine:   mocks.NewMockdockerEngineRunner(ctrl),
				repository:     mocks.NewMockrepositoryService(ctrl),
			}
			tc.setupMocks(m)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName:      tc.inputAppName,
					wkldName:     tc.inputWkldName,
					envName:      tc.inputEnvName,
					envOverrides: tc.inputEnvOverrides,
					portOverrides: portOverrides{
						{
							host:      "777",
							container: "7777",
						},
						{
							host:      "999",
							container: "9999",
						},
					},
				},
				newInterpolator: func(app, env string) interpolator {
					return m.interpolator
				},
				unmarshal: func(b []byte) (manifest.DynamicWorkload, error) {
					return m.mockMft, nil
				},
				configureClients: func(o *localRunOpts) error {
					return nil
				},
				buildContainerImages: func(o *localRunOpts) error {
					return tc.buildImagesError
				},
				imageInfoList:  mockImageInfoList,
				ws:             m.ws,
				ecsLocalClient: m.ecsLocalClient,
				ssm:            m.ssm,
				secretsManager: m.secretsManager,
				store:          m.store,
				sess: &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewStaticCredentials("myID", "mySecret", "myToken"),
					},
				},
				cmd:             m.mockRunner,
				dockerEngine:    m.dockerEngine,
				repository:      m.repository,
				targetEnv:       &mockEnv,
				targetApp:       &mockApp,
				containerSuffix: mockContainerSuffix,
				newColor: func() *color.Color {
					return nil
				},
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

func TestLocalRunOpts_getEnvVars(t *testing.T) {
	newVar := func(v string, overridden, secret bool) envVarValue {
		return envVarValue{
			Value:    v,
			Override: overridden,
			Secret:   secret,
		}
	}

	tests := map[string]struct {
		taskDef      *awsecs.TaskDefinition
		envOverrides map[string]string
		setupMocks   func(m *localRunExecuteMocks)
		credsError   error

		want      map[string]containerEnv
		wantError string
	}{
		"error getting creds": {
			credsError: errors.New("some error"),
			wantError:  `get IAM credentials: some error`,
		},
		"invalid container in env override": {
			taskDef: &awsecs.TaskDefinition{},
			envOverrides: map[string]string{
				"bad:OVERRIDE": "bad",
			},
			wantError: `parse env overrides: "bad:OVERRIDE" targets invalid container`,
		},
		"overrides parsed and applied correctly": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
					},
					{
						Name: aws.String("bar"),
					},
				},
			},
			envOverrides: map[string]string{
				"OVERRIDE_ALL": "all",
				"foo:OVERRIDE": "foo",
				"bar:OVERRIDE": "bar",
			},
			want: map[string]containerEnv{
				"foo": {
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("foo", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
				"bar": {
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("bar", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
			},
		},
		"overrides merged with existing env vars correctly": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Environment: []*sdkecs.KeyValuePair{
							{
								Name:  aws.String("RANDOM_FOO"),
								Value: aws.String("foo"),
							},
							{
								Name:  aws.String("OVERRIDE_ALL"),
								Value: aws.String("bye"),
							},
							{
								Name:  aws.String("OVERRIDE"),
								Value: aws.String("bye"),
							},
						},
					},
					{
						Name: aws.String("bar"),
						Environment: []*sdkecs.KeyValuePair{
							{
								Name:  aws.String("RANDOM_BAR"),
								Value: aws.String("bar"),
							},
							{
								Name:  aws.String("OVERRIDE_ALL"),
								Value: aws.String("bye"),
							},
							{
								Name:  aws.String("OVERRIDE"),
								Value: aws.String("bye"),
							},
						},
					},
				},
			},
			envOverrides: map[string]string{
				"OVERRIDE_ALL": "all",
				"foo:OVERRIDE": "foo",
				"bar:OVERRIDE": "bar",
			},
			want: map[string]containerEnv{
				"foo": {
					"RANDOM_FOO":            newVar("foo", false, false),
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("foo", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
				"bar": {
					"RANDOM_BAR":            newVar("bar", false, false),
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("bar", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
			},
		},
		"error getting secret": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("SECRET"),
								ValueFrom: aws.String("defaultSSM"),
							},
						},
					},
				},
			},
			setupMocks: func(m *localRunExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "defaultSSM").Return("", errors.New("some error"))
			},
			wantError: `get secrets: get secret "defaultSSM": some error`,
		},
		"error getting secret if invalid arn": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("SECRET"),
								ValueFrom: aws.String("arn:aws:ecs:us-west-2:123456789:service/mycluster/myservice"),
							},
						},
					},
				},
			},
			wantError: `get secrets: get secret "arn:aws:ecs:us-west-2:123456789:service/mycluster/myservice": invalid ARN; not a SSM or Secrets Manager ARN`,
		},
		"error if secret redefines a var": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Environment: []*sdkecs.KeyValuePair{
							{
								Name:  aws.String("SHOULD_BE_A_VAR"),
								Value: aws.String("foo"),
							},
						},
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("SHOULD_BE_A_VAR"),
								ValueFrom: aws.String("bad"),
							},
						},
					},
				},
			},
			wantError: `get secrets: secret names must be unique, but an environment variable "SHOULD_BE_A_VAR" already exists`,
		},
		"correct service used based on arn": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("SSM"),
								ValueFrom: aws.String("arn:aws:ssm:us-east-2:123456789:parameter/myparam"),
							},
							{
								Name:      aws.String("SECRETS_MANAGER"),
								ValueFrom: aws.String("arn:aws:secretsmanager:us-west-2:123456789:secret:mysecret"),
							},
							{
								Name:      aws.String("DEFAULT"),
								ValueFrom: aws.String("myparam"),
							},
						},
					},
				},
			},
			setupMocks: func(m *localRunExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "arn:aws:ssm:us-east-2:123456789:parameter/myparam").Return("ssm", nil)
				m.secretsManager.EXPECT().GetSecretValue(gomock.Any(), "arn:aws:secretsmanager:us-west-2:123456789:secret:mysecret").Return("secretsmanager", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "myparam").Return("default", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"SSM":                   newVar("ssm", false, true),
					"SECRETS_MANAGER":       newVar("secretsmanager", false, true),
					"DEFAULT":               newVar("default", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
			},
		},
		"only unique secrets pulled": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("ONE"),
								ValueFrom: aws.String("shared"),
							},
							{
								Name:      aws.String("TWO"),
								ValueFrom: aws.String("foo"),
							},
						},
					},
					{
						Name: aws.String("bar"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("THREE"),
								ValueFrom: aws.String("shared"),
							},
							{
								Name:      aws.String("FOUR"),
								ValueFrom: aws.String("bar"),
							},
						},
					},
				},
			},
			setupMocks: func(m *localRunExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "shared").Return("shared-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "foo").Return("foo-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "bar").Return("bar-value", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"ONE":                   newVar("shared-value", false, true),
					"TWO":                   newVar("foo-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
				"bar": {
					"THREE":                 newVar("shared-value", false, true),
					"FOUR":                  newVar("bar-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
			},
		},
		"secrets set via overrides not pulled": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name: aws.String("foo"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("ONE"),
								ValueFrom: aws.String("shared"),
							},
							{
								Name:      aws.String("TWO"),
								ValueFrom: aws.String("foo"),
							},
						},
					},
					{
						Name: aws.String("bar"),
						Secrets: []*sdkecs.Secret{
							{
								Name:      aws.String("THREE"),
								ValueFrom: aws.String("shared"),
							},
							{
								Name:      aws.String("FOUR"),
								ValueFrom: aws.String("bar"),
							},
						},
					},
				},
			},
			envOverrides: map[string]string{
				"ONE":      "one-overridden",
				"bar:FOUR": "four-overridden",
			},
			setupMocks: func(m *localRunExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "shared").Return("shared-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "foo").Return("foo-value", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"ONE":                   newVar("one-overridden", true, false),
					"TWO":                   newVar("foo-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
				"bar": {
					"ONE":                   newVar("one-overridden", true, false),
					"THREE":                 newVar("shared-value", false, true),
					"FOUR":                  newVar("four-overridden", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, false),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, false),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, false),
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunExecuteMocks{
				ssm:            mocks.NewMocksecretGetter(ctrl),
				secretsManager: mocks.NewMocksecretGetter(ctrl),
				sessCreds: &mockProvider{
					FnRetrieve: func() (credentials.Value, error) {
						return credentials.Value{
							AccessKeyID:     "myID",
							SecretAccessKey: "mySecret",
							SessionToken:    "myToken",
						}, tc.credsError
					},
				},
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			o := &localRunOpts{
				localRunVars: localRunVars{
					envOverrides: tc.envOverrides,
				},
				sess: &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewCredentials(m.sessCreds),
					},
				},
				ssm:            m.ssm,
				secretsManager: m.secretsManager,
			}

			got, err := o.getEnvVars(context.Background(), tc.taskDef)
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
