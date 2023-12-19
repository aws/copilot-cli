// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/rds"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/cli/file/filetest"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker/orchestrator"
	"github.com/aws/copilot-cli/internal/pkg/docker/orchestrator/orchestratortest"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testError = errors.New("some error")

type runLocalAskMocks struct {
	store *mocks.Mockstore
	sel   *mocks.MockdeploySelector
}

func TestRunLocalOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName   string
		setupMocks  func(m *runLocalAskMocks)
		wantAppName string
		wantError   error
	}{
		"no app in workspace": {
			wantError: errNoAppInWorkspace,
		},
		"fail to read the application from SSM store": {
			inAppName: "testApp",
			setupMocks: func(m *runLocalAskMocks) {
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
			m := &runLocalAskMocks{
				store: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}
			opts := runLocalOpts{
				runLocalVars: runLocalVars{
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

func TestRunLocalOpts_Ask(t *testing.T) {
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

		setupMocks     func(m *runLocalAskMocks)
		wantedWkldName string
		wantedEnvName  string
		wantedWkldType string
		wantedError    error
	}{
		"error if provided environment is not present in the workspace": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *runLocalAskMocks) {
				m.store.EXPECT().GetEnvironment(testAppName, testEnvName).Return(nil, testError)
			},
			wantedError: testError,
		},
		"error if provided workload is not present in the workspace": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *runLocalAskMocks) {
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(nil, testError)
			},
			wantedError: testError,
		},
		"successfully validate env and svc with flags passed in": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(m *runLocalAskMocks) {
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
			setupMocks: func(m *runLocalAskMocks) {
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
			setupMocks: func(m *runLocalAskMocks) {
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
			m := &runLocalAskMocks{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockdeploySelector(ctrl),
			}
			tc.setupMocks(m)
			opts := runLocalOpts{
				runLocalVars: runLocalVars{
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

type runLocalExecuteMocks struct {
	ecsClient      *mocks.MockecsClient
	ecsExecutor    *mocks.MockecsCommandExecutor
	store          *mocks.Mockstore
	sessCreds      credentials.Provider
	sessProvider   *mocks.MocksessionProvider
	interpolator   *mocks.Mockinterpolator
	ws             *mocks.MockwsWlDirReader
	mockMft        *mockWorkloadMft
	mockRunner     *mocks.MockexecRunner
	dockerEngine   *mocks.MockdockerEngineRunner
	repository     *mocks.MockrepositoryService
	ssm            *mocks.MocksecretGetter
	secretsManager *mocks.MocksecretGetter
	prog           *mocks.Mockprogress
	orchestrator   *orchestratortest.Double
	watcher        *filetest.Double
	hostFinder     *hostFinderDouble
	envChecker     *mocks.MockversionCompatibilityChecker
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

type hostFinderDouble struct {
	HostsFn func(context.Context) ([]orchestrator.Host, error)
}

func (d *hostFinderDouble) Hosts(ctx context.Context) ([]orchestrator.Host, error) {
	if d.HostsFn == nil {
		return nil, nil
	}
	return d.HostsFn(ctx)
}

func TestRunLocalOpts_Execute(t *testing.T) {
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

	mockContainerURIs := map[string]string{
		"foo": "image1",
		"bar": "image2",
	}

	taskDef := &awsecs.TaskDefinition{
		TaskRoleArn: aws.String("mock-arn"),
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
				Essential: aws.Bool(true),
				DependsOn: []*sdkecs.ContainerDependency{
					{
						Condition:     aws.String("START"),
						ContainerName: aws.String("bar"),
					},
				},
			},
			{
				Name:      aws.String("bar"),
				Essential: aws.Bool(true),
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
	alteredTaskDef := &awsecs.TaskDefinition{
		TaskRoleArn: aws.String("mock-arn"),
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
						ContainerPort: aws.Int64(8081),
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
	expectedTask := orchestrator.Task{
		Containers: map[string]orchestrator.ContainerDefinition{
			"foo": {
				ImageURI: "image1",
				EnvVars: map[string]string{
					"FOO_VAR": "foo-value",
				},
				Secrets: map[string]string{
					"SHARED_SECRET":         "secretvalue",
					"AWS_ACCESS_KEY_ID":     "myID",
					"AWS_SECRET_ACCESS_KEY": "mySecret",
					"AWS_SESSION_TOKEN":     "myToken",
				},
				Ports: map[string]string{
					"80":  "8080",
					"999": "9999",
				},
				IsEssential: true,
				DependsOn: map[string]string{
					"bar": "start",
				},
			},
			"bar": {
				ImageURI: "image2",
				EnvVars: map[string]string{
					"BAR_VAR": "bar-value",
				},
				Secrets: map[string]string{
					"SHARED_SECRET":         "secretvalue",
					"AWS_ACCESS_KEY_ID":     "myID",
					"AWS_SECRET_ACCESS_KEY": "mySecret",
					"AWS_SESSION_TOKEN":     "myToken",
				},
				Ports: map[string]string{
					"777":   "7777",
					"10000": "10000",
				},
				IsEssential: true,
				DependsOn:   map[string]string{},
			},
		},
	}
	expectedProxyTask := orchestrator.Task{
		Containers: expectedTask.Containers,
		PauseSecrets: map[string]string{
			"AWS_ACCESS_KEY_ID":     "myEnvID",
			"AWS_SECRET_ACCESS_KEY": "myEnvSecret",
			"AWS_SESSION_TOKEN":     "myEnvToken",
		},
	}
	expectedTaskWithRegion := orchestrator.Task{
		Containers: map[string]orchestrator.ContainerDefinition{
			"foo": {
				ImageURI: "image1",
				EnvVars: map[string]string{
					"FOO_VAR": "foo-value",
				},
				Secrets: map[string]string{
					"SHARED_SECRET":         "secretvalue",
					"AWS_ACCESS_KEY_ID":     "myID",
					"AWS_SECRET_ACCESS_KEY": "mySecret",
					"AWS_SESSION_TOKEN":     "myToken",
					"AWS_DEFAULT_REGION":    testRegion,
					"AWS_REGION":            testRegion,
				},
				Ports: map[string]string{
					"80":  "8080",
					"999": "9999",
				},
				IsEssential: true,
				DependsOn: map[string]string{
					"bar": "start",
				},
			},
			"bar": {
				ImageURI: "image2",
				EnvVars: map[string]string{
					"BAR_VAR": "bar-value",
				},
				Secrets: map[string]string{
					"SHARED_SECRET":         "secretvalue",
					"AWS_ACCESS_KEY_ID":     "myID",
					"AWS_SECRET_ACCESS_KEY": "mySecret",
					"AWS_SESSION_TOKEN":     "myToken",
					"AWS_DEFAULT_REGION":    testRegion,
					"AWS_REGION":            testRegion,
				},
				Ports: map[string]string{
					"777":   "7777",
					"10000": "10000",
				},
				IsEssential: true,
				DependsOn:   map[string]string{},
			},
		},
	}

	testCases := map[string]struct {
		inputAppName       string
		inputEnvName       string
		inputWkldName      string
		inputEnvOverrides  map[string]string
		inputPortOverrides []string
		inputWatch         bool
		inputTaskRole      bool
		inputProxy         bool
		inputReader        io.Reader
		buildImagesError   error

		setupMocks     func(t *testing.T, m *runLocalExecuteMocks)
		wantedWkldName string
		wantedEnvName  string
		wantedWkldType string
		wantedError    error
	}{
		"error getting the task Definition": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("get task: get task definition: %w", testError),
		},
		"error getting env vars due to bad override": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputEnvOverrides: map[string]string{
				"bad:OVERRIDE": "i fail",
			},
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
			},
			wantedError: errors.New(`get task: get env vars: parse env overrides: "bad:OVERRIDE" targets invalid container`),
		},
		"error retrieving task role credentials": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputTaskRole: true,
			inputReader:   strings.NewReader("some error"),
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.sessProvider.EXPECT().FromRole("mock-arn", testRegion).Return(nil, errors.New("some error"))
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/clusterName/taskName"),
							Containers: []*sdkecs.Container{
								{
									RuntimeId:  aws.String("runtime-id"),
									LastStatus: aws.String("RUNNING"),
									ManagedAgents: []*sdkecs.ManagedAgent{
										{
											Name:       aws.String("ExecuteCommandAgent"),
											LastStatus: aws.String("RUNNING"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.ecsExecutor.EXPECT().ExecuteCommand(gomock.Any()).Return(nil)
			},
			wantedError: errors.New(`get task: retrieve task role credentials: assume role: some error
ecs exec: all containers failed to retrieve credentials`),
		},
		"error reading workload manifest": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New(`read manifest file for testWkld: some error`),
		},
		"error interpolating workload manifest": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
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
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
			},
			wantedError: errors.New(`build images: some error`),
		},
		"error getting env version": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("", fmt.Errorf("some error"))
			},
			wantedError: errors.New(`retrieve version of environment stack "testEnv" in application "testApp": some error`),
		},
		"error due to old env version": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.31.0", nil)
			},
			wantedError: errors.New(`environment "testEnv" is on version "v1.31.0" which does not support the "run local --proxy" feature`),
		},
		"error getting hosts to proxy to": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return nil, fmt.Errorf("some error")
				}
			},
			wantedError: errors.New(`find hosts to connect to: some error`),
		},
		"error, proxy, describe service": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return []orchestrator.Host{
						{
							Name: "a-different-service",
							Port: 80,
						},
					}, nil
				}
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get proxy target container: describe service: some error"),
		},
		"error, proxy, parse arn": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return []orchestrator.Host{
						{
							Name: "a-different-service",
							Port: 80,
						},
					}, nil
				}
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("asdf"),
						},
					},
				}, nil)
			},
			wantedError: errors.New(`get proxy target container: parse task arn: arn: invalid prefix`),
		},
		"error, proxy, process task": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return []orchestrator.Host{
						{
							Name: "a-different-service",
							Port: 80,
						},
					}, nil
				}
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/asdf"),
						},
					},
				}, nil)
			},
			wantedError: errors.New(`get proxy target container: task ARN in unexpected format: "arn:aws:ecs:us-west-2:123456789:task/asdf"`),
		},
		"error, proxy, no valid containers": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return []orchestrator.Host{
						{
							Name: "a-different-service",
							Port: 80,
						},
					}, nil
				}
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/clusterName/taskName"),
							Containers: []*sdkecs.Container{
								{
									RuntimeId:  aws.String("runtime-id"),
									LastStatus: aws.String("RUNNING"),
								},
							},
						},
					},
				}, nil)
			},
			wantedError: errors.New(`get proxy target container: no running tasks have running containers with ecs exec enabled`),
		},
		"success, one run task call": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					errCh <- errors.New("some error")
					return errCh
				}
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTask, task)
				}
				m.orchestrator.StopFn = func() {
					require.Len(t, errCh, 0)
					close(errCh)
				}
			},
		},
		"success, one run task call, proxy": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputProxy:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.envChecker.EXPECT().Version().Return("v1.32.0", nil)
				m.hostFinder.HostsFn = func(ctx context.Context) ([]orchestrator.Host, error) {
					return []orchestrator.Host{
						{
							Name: "a-different-service",
							Port: 80,
						},
					}, nil
				}
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/clusterName/taskName"),
							Containers: []*sdkecs.Container{
								{
									RuntimeId:  aws.String("runtime-id"),
									LastStatus: aws.String("RUNNING"),
									ManagedAgents: []*sdkecs.ManagedAgent{
										{
											Name:       aws.String("ExecuteCommandAgent"),
											LastStatus: aws.String("RUNNING"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					errCh <- errors.New("some error")
					return errCh
				}
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedProxyTask, task)
				}
				m.orchestrator.StopFn = func() {
					require.Len(t, errCh, 0)
					close(errCh)
				}
			},
		},
		"success, one run task call, task role assume role method": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputTaskRole: true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				taskRoleSess := &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewStaticCredentials("myID", "mySecret", "myToken"),
						Region:      aws.String(testRegion),
					},
				}
				m.sessProvider.EXPECT().FromRole("mock-arn", testRegion).Return(taskRoleSess, nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					errCh <- errors.New("some error")
					return errCh
				}
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTaskWithRegion, task)
				}
				m.orchestrator.StopFn = func() {
					require.Len(t, errCh, 0)
					close(errCh)
				}
			},
		},
		"success, one run task call, task role ecs exec method": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputTaskRole: true,
			inputReader:   strings.NewReader(`{"AccessKeyId":"myID","SecretAccessKey":"mySecret","Token":"myToken"}`),
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.sessProvider.EXPECT().FromRole("mock-arn", testRegion).Return(nil, errors.New("some error"))
				m.ecsClient.EXPECT().DescribeService(testAppName, testEnvName, testWkldName).Return(&ecs.ServiceDesc{
					Tasks: []*awsecs.Task{
						{
							TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789:task/clusterName/taskName"),
							Containers: []*sdkecs.Container{
								{
									RuntimeId:  aws.String("runtime-id"),
									LastStatus: aws.String("RUNNING"),
									ManagedAgents: []*sdkecs.ManagedAgent{
										{
											Name:       aws.String("ExecuteCommandAgent"),
											LastStatus: aws.String("RUNNING"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.ecsExecutor.EXPECT().ExecuteCommand(gomock.Any()).Return(nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					errCh <- errors.New("some error")
					return errCh
				}
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTask, task)
				}
				m.orchestrator.StopFn = func() {
					require.Len(t, errCh, 0)
					close(errCh)
				}
			},
		},
		"handles ctrl-c, waits to get all errors": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					return errCh
				}
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTask, task)
					syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				}

				count := 1
				m.orchestrator.StopFn = func() {
					switch count {
					case 1:
						errCh <- errors.New("error from stopping after sigint")
					case 2:
						errCh <- errors.New("an error after calling Stop the first time")
					case 3:
						close(errCh)
					default:
						t.Fatalf("Stop() called %v times, only expected 3 times", count)
					}
					count++
				}
			},
		},
		"watch flag receives hidden file update, doesn't restart": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputWatch:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.ws.EXPECT().Path().Return("")

				eventCh := make(chan fsnotify.Event, 1)
				m.watcher.EventsFn = func() <-chan fsnotify.Event {
					eventCh <- fsnotify.Event{
						Name: ".hiddensubdir/mockFilename",
						Op:   fsnotify.Write,
					}
					return eventCh
				}

				watcherErrCh := make(chan error, 1)
				m.watcher.ErrorsFn = func() <-chan error {
					return watcherErrCh
				}

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					return errCh
				}

				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					syscall.Kill(syscall.Getpid(), syscall.SIGINT)
				}

				m.orchestrator.StopFn = func() {
					close(errCh)
				}
			},
		},
		"watch flag restarts, error for pause container definition update": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputWatch:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil).Times(2)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil).Times(2)
				m.interpolator.EXPECT().Interpolate("").Return("", nil).Times(2)
				m.ws.EXPECT().Path().Return("")
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(alteredTaskDef, nil)

				eventCh := make(chan fsnotify.Event, 1)
				m.watcher.EventsFn = func() <-chan fsnotify.Event {
					eventCh <- fsnotify.Event{
						Name: "mockFilename",
						Op:   fsnotify.Write,
					}
					return eventCh
				}

				watcherErrCh := make(chan error, 1)
				m.watcher.ErrorsFn = func() <-chan error {
					return watcherErrCh
				}

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					return errCh
				}

				count := 1
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					switch count {
					case 1:
						require.Equal(t, expectedTask, task)
					case 2:
						require.NotEqual(t, expectedTask, task)
						errCh <- errors.New("new task requires recreating pause container")
					}
					count++
				}

				m.orchestrator.StopFn = func() {
					close(errCh)
				}
			},
		},
		"watcher error succesfully stops all goroutines": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputWatch:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil)
				m.interpolator.EXPECT().Interpolate("").Return("", nil)
				m.ws.EXPECT().Path().Return("")

				eventCh := make(chan fsnotify.Event, 1)
				m.watcher.EventsFn = func() <-chan fsnotify.Event {
					return eventCh
				}

				watcherErrCh := make(chan error, 1)
				m.watcher.ErrorsFn = func() <-chan error {
					watcherErrCh <- errors.New("some error")
					return watcherErrCh
				}

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					return errCh
				}

				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTask, task)
				}

				m.orchestrator.StopFn = func() {
					close(errCh)
				}
			},
		},
		"watch flag restarts and finishes successfully": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			inputEnvName:  testEnvName,
			inputWatch:    true,
			setupMocks: func(t *testing.T, m *runLocalExecuteMocks) {
				m.ecsClient.EXPECT().TaskDefinition(testAppName, testEnvName, testWkldName).Return(taskDef, nil).Times(2)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "mysecret").Return("secretvalue", nil).Times(2)
				m.ws.EXPECT().ReadWorkloadManifest(testWkldName).Return([]byte(""), nil).Times(2)
				m.interpolator.EXPECT().Interpolate("").Return("", nil).Times(2)
				m.ws.EXPECT().Path().Return("")

				eventCh := make(chan fsnotify.Event, 1)
				m.watcher.EventsFn = func() <-chan fsnotify.Event {
					eventCh <- fsnotify.Event{
						Name: "mockFilename",
						Op:   fsnotify.Write,
					}
					return eventCh
				}

				watcherErrCh := make(chan error, 1)
				m.watcher.ErrorsFn = func() <-chan error {
					return watcherErrCh
				}

				errCh := make(chan error, 1)
				m.orchestrator.StartFn = func() <-chan error {
					return errCh
				}
				runCount := 1
				m.orchestrator.RunTaskFn = func(task orchestrator.Task, opts ...orchestrator.RunTaskOption) {
					require.Equal(t, expectedTask, task)
					if runCount > 1 {
						syscall.Kill(syscall.Getpid(), syscall.SIGINT)
					}
					runCount++
				}

				m.orchestrator.StopFn = func() {
					close(errCh)
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &runLocalExecuteMocks{
				ecsClient:      mocks.NewMockecsClient(ctrl),
				ecsExecutor:    mocks.NewMockecsCommandExecutor(ctrl),
				ssm:            mocks.NewMocksecretGetter(ctrl),
				secretsManager: mocks.NewMocksecretGetter(ctrl),
				store:          mocks.NewMockstore(ctrl),
				sessProvider:   mocks.NewMocksessionProvider(ctrl),
				interpolator:   mocks.NewMockinterpolator(ctrl),
				ws:             mocks.NewMockwsWlDirReader(ctrl),
				mockRunner:     mocks.NewMockexecRunner(ctrl),
				dockerEngine:   mocks.NewMockdockerEngineRunner(ctrl),
				repository:     mocks.NewMockrepositoryService(ctrl),
				prog:           mocks.NewMockprogress(ctrl),
				orchestrator:   &orchestratortest.Double{},
				watcher:        &filetest.Double{},
				hostFinder:     &hostFinderDouble{},
				envChecker:     mocks.NewMockversionCompatibilityChecker(ctrl),
			}
			tc.setupMocks(t, m)
			opts := runLocalOpts{
				runLocalVars: runLocalVars{
					appName:      tc.inputAppName,
					wkldName:     tc.inputWkldName,
					envName:      tc.inputEnvName,
					envOverrides: tc.inputEnvOverrides,
					watch:        tc.inputWatch,
					useTaskRole:  tc.inputTaskRole,
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
					proxy: tc.inputProxy,
				},
				newInterpolator: func(app, env string) interpolator {
					return m.interpolator
				},
				unmarshal: func(b []byte) (manifest.DynamicWorkload, error) {
					return m.mockMft, nil
				},
				configureClients: func() error {
					return nil
				},
				buildContainerImages: func(mft manifest.DynamicWorkload) (map[string]string, error) {
					return mockContainerURIs, tc.buildImagesError
				},
				ws:             m.ws,
				ecsClient:      m.ecsClient,
				ecsExecutor:    m.ecsExecutor,
				ssm:            m.ssm,
				secretsManager: m.secretsManager,
				store:          m.store,
				sessProvider:   m.sessProvider,
				sess: &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewStaticCredentials("myID", "mySecret", "myToken"),
					},
				},
				envManagerSess: &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewStaticCredentials("myEnvID", "myEnvSecret", "myEnvToken"),
					},
				},
				cmd:          m.mockRunner,
				dockerEngine: m.dockerEngine,
				repository:   m.repository,
				targetEnv:    &mockEnv,
				targetApp:    &mockApp,
				prog:         m.prog,
				orchestrator: m.orchestrator,
				hostFinder:   m.hostFinder,
				envChecker:   m.envChecker,
				debounceTime: 0, // disable debounce during testing
				newRecursiveWatcher: func() (recursiveWatcher, error) {
					return m.watcher, nil
				},
				captureStdout: func() (io.Reader, error) {
					return tc.inputReader, nil
				},
				releaseStdout: func() {},
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

func TestRunLocalOpts_getEnvVars(t *testing.T) {
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
		setupMocks   func(m *runLocalExecuteMocks)
		credsError   error
		region       *string

		want      map[string]containerEnv
		wantError string
	}{
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
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
				},
				"bar": {
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("bar", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
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
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
				},
				"bar": {
					"RANDOM_BAR":            newVar("bar", false, false),
					"OVERRIDE_ALL":          newVar("all", true, false),
					"OVERRIDE":              newVar("bar", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
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
			setupMocks: func(m *runLocalExecuteMocks) {
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
			setupMocks: func(m *runLocalExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "arn:aws:ssm:us-east-2:123456789:parameter/myparam").Return("ssm", nil)
				m.secretsManager.EXPECT().GetSecretValue(gomock.Any(), "arn:aws:secretsmanager:us-west-2:123456789:secret:mysecret").Return("secretsmanager", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "myparam").Return("default", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"SSM":                   newVar("ssm", false, true),
					"SECRETS_MANAGER":       newVar("secretsmanager", false, true),
					"DEFAULT":               newVar("default", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
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
			setupMocks: func(m *runLocalExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "shared").Return("shared-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "foo").Return("foo-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "bar").Return("bar-value", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"ONE":                   newVar("shared-value", false, true),
					"TWO":                   newVar("foo-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
				},
				"bar": {
					"THREE":                 newVar("shared-value", false, true),
					"FOUR":                  newVar("bar-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
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
			setupMocks: func(m *runLocalExecuteMocks) {
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "shared").Return("shared-value", nil)
				m.ssm.EXPECT().GetSecretValue(gomock.Any(), "foo").Return("foo-value", nil)
			},
			want: map[string]containerEnv{
				"foo": {
					"ONE":                   newVar("one-overridden", true, false),
					"TWO":                   newVar("foo-value", false, true),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
				},
				"bar": {
					"ONE":                   newVar("one-overridden", true, false),
					"THREE":                 newVar("shared-value", false, true),
					"FOUR":                  newVar("four-overridden", true, false),
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
				},
			},
		},
		"error getting creds": {
			taskDef:    &awsecs.TaskDefinition{},
			credsError: errors.New("some error"),
			wantError:  `get IAM credentials: some error`,
		},
		"region env vars set": {
			taskDef: &awsecs.TaskDefinition{
				ContainerDefinitions: []*sdkecs.ContainerDefinition{
					{
						Name:        aws.String("foo"),
						Environment: []*sdkecs.KeyValuePair{},
					},
				},
			},
			region: aws.String("myRegion"),
			want: map[string]containerEnv{
				"foo": {
					"AWS_ACCESS_KEY_ID":     newVar("myID", false, true),
					"AWS_SECRET_ACCESS_KEY": newVar("mySecret", false, true),
					"AWS_SESSION_TOKEN":     newVar("myToken", false, true),
					"AWS_REGION":            newVar("myRegion", false, true),
					"AWS_DEFAULT_REGION":    newVar("myRegion", false, true),
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &runLocalExecuteMocks{
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

			o := &runLocalOpts{
				runLocalVars: runLocalVars{
					envOverrides: tc.envOverrides,
				},
				sess: &session.Session{
					Config: &aws.Config{
						Credentials: credentials.NewCredentials(m.sessCreds),
						Region:      tc.region,
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

type taggedResourceGetterDouble struct {
	GetResourcesByTagsFn func(string, map[string]string) ([]*resourcegroups.Resource, error)
}

func (d *taggedResourceGetterDouble) GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error) {
	if d.GetResourcesByTagsFn == nil {
		return nil, nil
	}
	return d.GetResourcesByTagsFn(resourceType, tags)
}

type rdsDescriberDouble struct {
	DescribeDBInstancesPagesWithContextFn func(context.Context, *rds.DescribeDBInstancesInput, func(*rds.DescribeDBInstancesOutput, bool) bool, ...request.Option) error
	DescribeDBClustersPagesWithContextFn  func(context.Context, *rds.DescribeDBClustersInput, func(*rds.DescribeDBClustersOutput, bool) bool, ...request.Option) error
}

func (d *rdsDescriberDouble) DescribeDBInstancesPagesWithContext(ctx context.Context, in *rds.DescribeDBInstancesInput, fn func(*rds.DescribeDBInstancesOutput, bool) bool, opts ...request.Option) error {
	if d.DescribeDBInstancesPagesWithContextFn == nil {
		return nil
	}
	return d.DescribeDBInstancesPagesWithContextFn(ctx, in, fn, opts...)
}

func (d *rdsDescriberDouble) DescribeDBClustersPagesWithContext(ctx context.Context, in *rds.DescribeDBClustersInput, fn func(*rds.DescribeDBClustersOutput, bool) bool, opts ...request.Option) error {
	if d.DescribeDBClustersPagesWithContextFn == nil {
		return nil
	}
	return d.DescribeDBClustersPagesWithContextFn(ctx, in, fn, opts...)
}

func TestRunLocal_HostDiscovery(t *testing.T) {
	type testMocks struct {
		ecs *mocks.MockecsClient
		rg  *taggedResourceGetterDouble
		rds *rdsDescriberDouble
	}
	ecsServices := []*awsecs.Service{
		{
			Deployments: []*sdkecs.Deployment{
				{
					Status: aws.String("ACTIVE"),
					ServiceConnectConfiguration: &sdkecs.ServiceConnectConfiguration{
						Enabled: aws.Bool(true),
						Services: []*sdkecs.ServiceConnectService{
							{
								ClientAliases: []*sdkecs.ServiceConnectClientAlias{
									{
										DnsName: aws.String("old"),
										Port:    aws.Int64(80),
									},
								},
							},
						},
					},
				},
				{
					Status: aws.String("PRIMARY"),
					ServiceConnectConfiguration: &sdkecs.ServiceConnectConfiguration{
						Enabled: aws.Bool(true),
						Services: []*sdkecs.ServiceConnectService{
							{
								ClientAliases: []*sdkecs.ServiceConnectClientAlias{
									{
										DnsName: aws.String("primary"),
										Port:    aws.Int64(80),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Deployments: []*sdkecs.Deployment{
				{
					Status: aws.String("INACTIVE"),
					ServiceConnectConfiguration: &sdkecs.ServiceConnectConfiguration{
						Enabled: aws.Bool(true),
						Services: []*sdkecs.ServiceConnectService{
							{
								ClientAliases: []*sdkecs.ServiceConnectClientAlias{
									{
										DnsName: aws.String("inactive"),
										Port:    aws.Int64(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tests := map[string]struct {
		setupMocks func(t *testing.T, m *testMocks)

		wantHosts []orchestrator.Host
		wantError string
	}{
		"error getting services": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantError: "get service connect services: some error",
		},
		"ignores non-primary deployments": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
			},
			wantHosts: []orchestrator.Host{
				{
					Name: "primary",
					Port: 80,
				},
			},
		},
		"error getting rds resources": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return nil, errors.New("some error")
				}
			},
			wantError: "get rds hosts: get tagged resources: some error",
		},
		"no db instances found": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return nil, nil
				}
			},
			wantHosts: []orchestrator.Host{
				{
					Name: "primary",
					Port: 80,
				},
			},
		},
		"invalid db arn": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return []*resourcegroups.Resource{
						{
							ARN: "arn:invalid",
						},
					}, nil
				}
			},
			wantError: `get rds hosts: invalid arn "arn:invalid": arn: not enough sections`,
		},
		"error describing rds instances": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return []*resourcegroups.Resource{
						{
							ARN: "arn:aws:rds:us-west-2:123456789:db:instanceID",
						},
					}, nil
				}
				m.rds.DescribeDBInstancesPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBInstancesInput, f func(*rds.DescribeDBInstancesOutput, bool) bool, o ...request.Option) error {
					return errors.New("some error")
				}
			},
			wantError: "get rds hosts: describe instances: some error",
		},
		"gets rds instance": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return []*resourcegroups.Resource{
						{
							ARN: "arn:aws:rds:us-west-2:123456789:db:instanceID",
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:subgrp:subgrpID",
						},
					}, nil
				}
				m.rds.DescribeDBInstancesPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBInstancesInput, f func(*rds.DescribeDBInstancesOutput, bool) bool, o ...request.Option) error {
					f(&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								Endpoint: &rds.Endpoint{
									Address: aws.String("db"),
									Port:    aws.Int64(3306),
								},
							},
						},
					}, true)
					return nil
				}
			},
			wantHosts: []orchestrator.Host{
				{
					Name: "primary",
					Port: 80,
				},
				{
					Name: "db",
					Port: 3306,
				},
			},
		},
		"error describing db cluster": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return []*resourcegroups.Resource{
						{
							ARN: "arn:aws:rds:us-west-2:123456789:db:instanceID",
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:subgrp:subgrpID",
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:cluster:clusterID",
						},
					}, nil
				}
				m.rds.DescribeDBInstancesPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBInstancesInput, f func(*rds.DescribeDBInstancesOutput, bool) bool, o ...request.Option) error {
					f(&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								Endpoint: &rds.Endpoint{
									Address: aws.String("db"),
									Port:    aws.Int64(3306),
								},
							},
						},
					}, true)
					return nil
				}
				m.rds.DescribeDBClustersPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBClustersInput, f func(*rds.DescribeDBClustersOutput, bool) bool, o ...request.Option) error {
					return errors.New("some error")
				}
			},
			wantError: "get rds hosts: describe clusters: some error",
		},
		"gets db cluster, skips other service resources": {
			setupMocks: func(t *testing.T, m *testMocks) {
				m.ecs.EXPECT().ServiceConnectServices(gomock.Any(), gomock.Any(), gomock.Any()).Return(ecsServices, nil)
				m.rg.GetResourcesByTagsFn = func(s string, m map[string]string) ([]*resourcegroups.Resource, error) {
					return []*resourcegroups.Resource{
						{
							ARN: "arn:aws:rds:us-west-2:123456789:db:instanceID",
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:subgrp:subgrpID",
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:cluster:clusterID",
							Tags: map[string]string{
								deploy.ServiceTagKey: "foo",
							},
						},
						{
							ARN: "arn:aws:rds:us-west-2:123456789:cluster:otherServiceCluster",
							Tags: map[string]string{
								deploy.ServiceTagKey: "bar",
							},
						},
					}, nil
				}
				m.rds.DescribeDBInstancesPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBInstancesInput, f func(*rds.DescribeDBInstancesOutput, bool) bool, o ...request.Option) error {
					f(&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								Endpoint: &rds.Endpoint{
									Address: aws.String("db"),
									Port:    aws.Int64(3306),
								},
							},
						},
					}, true)
					return nil
				}
				m.rds.DescribeDBClustersPagesWithContextFn = func(ctx context.Context, ddi *rds.DescribeDBClustersInput, f func(*rds.DescribeDBClustersOutput, bool) bool, o ...request.Option) error {
					require.NotContains(t, ddi.Filters[0].Values, aws.String("arn:aws:rds:us-west-2:123456789:cluster:otherServiceCluster"))

					f(&rds.DescribeDBClustersOutput{
						DBClusters: []*rds.DBCluster{
							{
								Endpoint:        aws.String("cluster"),
								Port:            aws.Int64(5432),
								ReaderEndpoint:  aws.String("cluster-ro"),
								CustomEndpoints: []*string{aws.String("cluster-custom")},
							},
						},
					}, true)
					return nil
				}
			},
			wantHosts: []orchestrator.Host{
				{
					Name: "primary",
					Port: 80,
				},
				{
					Name: "db",
					Port: 3306,
				},
				{
					Name: "cluster",
					Port: 5432,
				},
				{
					Name: "cluster-ro",
					Port: 5432,
				},
				{
					Name: "cluster-custom",
					Port: 5432,
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &testMocks{
				ecs: mocks.NewMockecsClient(ctrl),
				rg:  &taggedResourceGetterDouble{},
				rds: &rdsDescriberDouble{},
			}
			tc.setupMocks(t, m)

			h := &hostDiscoverer{
				wkld: "foo",
				ecs:  m.ecs,
				rg:   m.rg,
				rds:  m.rds,
			}

			hosts, err := h.Hosts(context.Background())
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantHosts, hosts)
			}
		})
	}
}
