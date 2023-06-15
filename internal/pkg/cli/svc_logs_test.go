// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type wkldLogsMock struct {
	configStore  *mocks.Mockstore
	sel          *mocks.MockdeploySelector
	sessProvider *mocks.MocksessionProvider
	ecs          *mocks.MockserviceDescriber
	logSvcWriter *mocks.MocklogEventsWriter
}

func TestSvcLogs_Validate(t *testing.T) {
	const (
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputApp       string
		inputSvc       string
		inputLimit     int
		inputFollow    bool
		inputEnvName   string
		inputStartTime string
		inputEndTime   string
		inputSince     time.Duration
		inputPrevious  bool
		inputTaskIDs   []string

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			mockstore: func(m *mocks.Mockstore) {},
		},
		"returns error if since and startTime flags are set together": {
			inputSince:     mockSince,
			inputStartTime: mockStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --since or --start-time may be used"),
		},
		"returns error if follow and endTime flags are set together": {
			inputFollow:  true,
			inputEndTime: mockEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --follow or --end-time may be used"),
		},
		"returns error if invalid start time flag value": {
			inputStartTime: mockBadStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badStartTime for \"--start-time\" flag: reading time value badStartTime: parsing time \"badStartTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badStartTime\" as \"2006\""),
		},
		"returns error if invalid end time flag value": {
			inputEndTime: mockBadEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badEndTime for \"--end-time\" flag: reading time value badEndTime: parsing time \"badEndTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badEndTime\" as \"2006\""),
		},
		"returns error if invalid since flag value": {
			inputSince: -mockSince,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--since must be greater than 0"),
		},
		"returns error if limit value is below limit": {
			inputLimit: -1,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit -1 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if limit value is above limit": {
			inputLimit: 10001,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit 10001 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if both previous and tasks flags are defined": {
			inputPrevious: true,
			inputTaskIDs:  []string{"taskId"},

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("cannot specify both --previous and --tasks"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			tc.mockstore(mockstore)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					wkldLogsVars: wkldLogsVars{
						follow:         tc.inputFollow,
						limit:          tc.inputLimit,
						envName:        tc.inputEnvName,
						humanStartTime: tc.inputStartTime,
						humanEndTime:   tc.inputEndTime,
						since:          tc.inputSince,
						name:           tc.inputSvc,
						appName:        tc.inputApp,
						taskIDs:        tc.inputTaskIDs,
					},
					previous: tc.inputPrevious,
				},
				wkldLogOpts: wkldLogOpts{
					configStore: mockstore,
				},
			}

			// WHEN
			err := svcLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcLogs_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputSvc = "my-svc"
	)
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputEnvName string
		inputTaskIDs []string

		setupMocks func(mocks wkldLogsMock)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"validate app env and svc with all flags passed in": {
			inputApp:     inputApp,
			inputSvc:     inputSvc,
			inputEnvName: inputEnv,
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.configStore.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
					m.configStore.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{}, nil),
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
						Return(&selector.DeployedService{
							Env:  "my-env",
							Name: "my-svc",
						}, nil), // Let prompter handles the case when svc(env) is definite.
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"return error if name of Static Site passed in": {
			inputApp:     inputApp,
			inputSvc:     inputSvc,
			inputEnvName: inputEnv,
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.configStore.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
					m.configStore.EXPECT().GetService("my-app", "my-svc").Return(
						&config.Workload{
							Type: manifestinfo.StaticSiteType,
						}, nil))
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
					Env:     "my-env",
					Name:    "my-svc",
					SvcType: manifestinfo.StaticSiteType,
				}, nil)
			},
			wantedError: errors.New("`svc logs` unavailable for Static Site services"),
		},
		"prompt for app name": {
			inputSvc:     inputSvc,
			inputEnvName: inputEnv,
			setupMocks: func(m wkldLogsMock) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil)
				m.configStore.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
					Env:     "my-env",
					Name:    "my-svc",
					SvcType: manifestinfo.BackendServiceType,
				}, nil).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"returns error if fail to select app": {
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for svc and env": {
			inputApp: "my-app",
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"return error if fail to select deployed services": {
			inputApp: inputApp,
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, inputApp, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("select deployed services for application my-app: some error"),
		},
		"return error if task ID is used for an RDWS": {
			inputApp:     inputApp,
			inputTaskIDs: []string{"mockTask1, mockTask2"},
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, inputApp, gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						SvcType: manifestinfo.RequestDrivenWebServiceType,
					}, nil)
			},
			wantedError: errors.New("cannot use `--tasks` for App Runner service logs"),
		},
		"return error if selected svc is of Static Site type": {
			inputApp: inputApp,
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, inputApp, gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						SvcType: manifestinfo.StaticSiteType,
					}, nil)
			},
			wantedError: errors.New("`svc logs` unavailable for Static Site services"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)

			mocks := wkldLogsMock{
				configStore: mockstore,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					wkldLogsVars: wkldLogsVars{
						envName: tc.inputEnvName,
						name:    tc.inputSvc,
						appName: tc.inputApp,
						taskIDs: tc.inputTaskIDs,
					},
				},
				wkldLogOpts: wkldLogOpts{
					configStore: mockstore,
					sel:         mockSel,
				},
			}

			// WHEN
			err := svcLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, svcLogs.appName, "expected app name to match")
				require.Equal(t, tc.wantedSvc, svcLogs.name, "expected service name to match")
				require.Equal(t, tc.wantedEnv, svcLogs.envName, "expected service name to match")
			}
		})
	}
}

func TestSvcLogs_Execute(t *testing.T) {
	mockTaskARN := "arn:aws:ecs:us-west-2:123456789:task/mockCluster/mockTaskID"
	mockOtherTaskARN := "arn:aws:ecs:us-west-2:123456789:task/mockCluster/mockTaskID1"
	mockStartTime := int64(123456789)
	mockEndTime := int64(987654321)
	mockLimit := int64(10)
	var mockNilLimit *int64
	testCases := map[string]struct {
		inputSvc          string
		inputApp          string
		inputEnv          string
		follow            bool
		limit             int
		endTime           int64
		startTime         int64
		taskIDs           []string
		inputPreviousTask bool
		container         string
		logGroup          string

		setupMocks func(mocks wkldLogsMock)

		wantedError error
	}{
		"success": {
			inputSvc:  "mockSvc",
			endTime:   mockEndTime,
			startTime: mockStartTime,
			follow:    true,
			limit:     10,
			taskIDs:   []string{"mockTaskID"},
			container: "datadog",
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.logSvcWriter.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
						require.Equal(t, param.TaskIDs, []string{"mockTaskID"})
						require.Equal(t, param.EndTime, &mockEndTime)
						require.Equal(t, param.StartTime, &mockStartTime)
						require.Equal(t, param.Follow, true)
						require.Equal(t, param.Limit, &mockLimit)
						require.Equal(t, param.ContainerName, "datadog")
					}).Return(nil),
				)
			},
			wantedError: nil,
		},
		"success with no limit set": {
			inputSvc:  "mockSvc",
			endTime:   mockEndTime,
			startTime: mockStartTime,
			follow:    true,
			taskIDs:   []string{"mockTaskID"},

			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.logSvcWriter.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
						require.Equal(t, param.TaskIDs, []string{"mockTaskID"})
						require.Equal(t, param.EndTime, &mockEndTime)
						require.Equal(t, param.StartTime, &mockStartTime)
						require.Equal(t, param.Follow, true)
						require.Equal(t, param.Limit, mockNilLimit)
					}).Return(nil),
				)
			},
			wantedError: nil,
		},
		"success with system log group for RDWS": {
			inputSvc:  "mockSvc",
			startTime: mockStartTime,
			endTime:   mockEndTime,
			logGroup:  "system",
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.logSvcWriter.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
						require.Equal(t, param.TaskIDs, ([]string)(nil))
						require.Equal(t, param.EndTime, &mockEndTime)
						require.Equal(t, param.StartTime, &mockStartTime)
						require.Equal(t, param.Follow, false)
						require.Equal(t, param.Limit, (*int64)(nil))
						require.Equal(t, param.ContainerName, "")
						require.Equal(t, param.LogGroup, "system")
					}).Return(nil),
				)
			},
			wantedError: nil,
		},
		"returns error if fail to get event logs": {
			inputSvc: "mockSvc",
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.logSvcWriter.EXPECT().WriteLogEvents(gomock.Any()).
						Return(errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("write log events for service mockSvc: some error"),
		},
		"retrieve previously stopped task's logs": {
			inputSvc:          "mockSvc",
			inputPreviousTask: true,
			inputApp:          "my-app",
			inputEnv:          "my-env",
			endTime:           mockEndTime,
			startTime:         mockStartTime,

			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.ecs.EXPECT().DescribeService("my-app", "my-env", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: "mockCluster",
						StoppedTasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("STOPPED"),
								StoppingAt: aws.Time(time.Now()),
							},
							{
								TaskArn:    aws.String(mockOtherTaskARN),
								LastStatus: aws.String("STOPPED"),
								StoppingAt: aws.Time(time.Now()),
							},
						},
						Tasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
						},
					}, nil),

					m.logSvcWriter.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
						require.Equal(t, param.TaskIDs, []string{"mockTaskID1"})
						require.Equal(t, param.EndTime, &mockEndTime)
						require.Equal(t, param.StartTime, &mockStartTime)
						require.Equal(t, param.Limit, mockNilLimit)
					}).Return(nil),
				)
			},

			wantedError: nil,
		},
		"retrieve warning no previously stopped tasks found, when no stopped task or logs available": {
			inputSvc:          "mockSvc",
			inputPreviousTask: true,
			inputApp:          "my-app",
			inputEnv:          "my-env",
			endTime:           mockEndTime,
			startTime:         mockStartTime,

			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.ecs.EXPECT().DescribeService("my-app", "my-env", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName:  "mockCluster",
						StoppedTasks: []*awsecs.Task{},
						Tasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
						},
					}, nil),
				)
			},

			wantedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockConfigStoreReader := mocks.NewMockstore(ctrl)
			mockSelector := mocks.NewMockdeploySelector(ctrl)
			mockSvcDescriber := mocks.NewMockserviceDescriber(ctrl)
			mockSessionProvider := mocks.NewMocksessionProvider(ctrl)
			mockLogsSvc := mocks.NewMocklogEventsWriter(ctrl)

			mocks := wkldLogsMock{
				configStore:  mockConfigStoreReader,
				sessProvider: mockSessionProvider,
				sel:          mockSelector,
				ecs:          mockSvcDescriber,
				logSvcWriter: mockLogsSvc,
			}

			tc.setupMocks(mocks)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					wkldLogsVars: wkldLogsVars{
						name:    tc.inputSvc,
						appName: tc.inputApp,
						envName: tc.inputEnv,
						follow:  tc.follow,
						limit:   tc.limit,
						taskIDs: tc.taskIDs,
					},
					previous:      tc.inputPreviousTask,
					containerName: tc.container,
					logGroup:      tc.logGroup,
				},

				wkldLogOpts: wkldLogOpts{
					startTime:          &tc.startTime,
					endTime:            &tc.endTime,
					initRuntimeClients: func() error { return nil },
					logsSvc:            mockLogsSvc,
					configStore:        mockConfigStoreReader,
					sel:                mockSelector,
					sessProvider:       mockSessionProvider,
					ecs:                mockSvcDescriber,
				},
			}

			// WHEN
			err := svcLogs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
