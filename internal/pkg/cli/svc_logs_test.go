// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSvcLogs_Validate(t *testing.T) {
	const (
		mockLimit        = 3
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

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			// default value for limit and since flags
			inputLimit: 10,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: nil,
		},
		"invalid project name": {
			inputApp: "my-app",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			tc.mockstore(mockstore)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					follow:         tc.inputFollow,
					limit:          tc.inputLimit,
					envName:        tc.inputEnvName,
					humanStartTime: tc.inputStartTime,
					humanEndTime:   tc.inputEndTime,
					since:          tc.inputSince,
					svcName:        tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				store:         mockstore,
				initCwLogsSvc: func(*svcLogsOpts, *config.Environment) error { return nil },
			}

			// WHEN
			err := svcLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestSvcLogs_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputEnvName string

		mockstore        func(m *mocks.Mockstore)
		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService
		mockSelector     func(m *mocks.MockconfigSelector)
		mockPrompter     func(m *mocks.Mockprompter)

		wantedError error
	}{
		"with all flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("mockApp", "mockSvc").Return(&config.Service{
					Name: "mockSvc",
				}, nil)
				m.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc")).Return(true, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: nil,
		},
		"with all flag set and return error if fail to get service": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("mockApp", "mockSvc").Return(nil, errors.New("some error"))
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("get service: some error"),
		},
		"with all flag set and return error if fail to get environment": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("mockApp", "mockSvc").Return(&config.Service{
					Name: "mockSvc",
				}, nil)
				m.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(nil, errors.New("some error"))
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("get environment: some error"),
		},
		"with only service flag set and not deployed in one of envs": {
			inputApp: "mockApp",
			inputSvc: "mockSvc",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("mockApp", "mockSvc").Return(&config.Service{
					Name: "mockSvc",
				}, nil)
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv",
					},
					{
						Name: "mockTestEnv",
					},
					{
						Name: "mockProdEnv",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockTestEnv", "mockSvc")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockProdEnv", "mockSvc")).Return(false, nil)
				cwlogServices["mockEnv"] = m
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(svcLogNamePrompt), svcLogNameHelpPrompt, []string{"mockSvc (mockEnv)", "mockSvc (mockTestEnv)"}).Return("mockSvc (mockTestEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"with only env flag set": {
			inputApp:     "mockApp",
			inputEnvName: "mockEnv",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
					Name: "mockEnv",
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockFrontend",
					},
					{
						Name: "mockBackend",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockFrontend")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockBackend")).Return(true, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(svcLogNamePrompt), svcLogNameHelpPrompt, []string{"mockFrontend (mockEnv)", "mockBackend (mockEnv)"}).Return("mockFrontend (mockEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"retrieve service name from ssm store": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockTestEnv",
					},
					{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockTestEnv", "mockSvc")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockProdEnv", "mockSvc")).Return(true, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(svcLogNamePrompt), svcLogNameHelpPrompt, []string{"mockSvc (mockTestEnv)", "mockSvc (mockProdEnv)"}).Return("mockSvc (mockTestEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"skip selecting if only one deployed service found": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockTestEnv",
					},
					{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockTestEnv", "mockSvc")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockProdEnv", "mockSvc")).Return(false, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: nil,
		},
		"returns error if fail to select application": {
			mockstore: func(m *mocks.Mockstore) {},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"returns error if fail to retrieve services": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return(nil, errors.New("some error"))
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("list services for application mockApp: some error"),
		},
		"returns error if no services found": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no services found in application %s", color.HighlightUserInput("mockApp")),
		},
		"returns error if fail to list environments": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return(nil, errors.New("some error"))
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("list environments: some error"),
		},
		"returns error if no environment found": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no environments found in application %s", color.HighlightUserInput("mockApp")),
		},
		"returns error if fail to check service deployed or not": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv",
					},
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc")).Return(false, errors.New("some error"))
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("check if the log group /copilot/mockApp-mockEnv-mockSvc exists: some error"),
		},
		"returns error if no deployed service found": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv",
					},
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc")).Return(false, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no deployed services found in application %s", color.HighlightUserInput("mockApp")),
		},
		"returns error if fail to select service env name": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockTestEnv",
					},
					{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
			},
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockTestEnv", "mockSvc")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockProdEnv", "mockSvc")).Return(true, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(svcLogNamePrompt), svcLogNameHelpPrompt, []string{"mockSvc (mockTestEnv)", "mockSvc (mockProdEnv)"}).Return("", errors.New("some error")).Times(1)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockconfigSelector(ctrl)
			tc.mockstore(mockstore)
			tc.mockPrompter(mockPrompter)
			tc.mockSelector(mockSel)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					envName: tc.inputEnvName,
					svcName: tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
						prompt:  mockPrompter,
					},
				},
				store:         mockstore,
				sel:           mockSel,
				initCwLogsSvc: func(*svcLogsOpts, *config.Environment) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
			}

			// WHEN
			err := svcLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestSvcLogs_Execute(t *testing.T) {
	mockLastEventTime := map[string]int64{
		"mockLogStreamName": 123456,
	}
	logEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -`,
		},
	}
	moreLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	logEventsHumanString := `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
	logEventsJSONString := "{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputFollow  bool
		inputEnvName string
		inputJSON    bool

		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService

		wantedError   error
		wantedContent string
	}{
		"with no optional flags set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil)

				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError:   nil,
			wantedContent: logEventsHumanString,
		},
		"with json flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",
			inputJSON:    true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil)

				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError:   nil,
			wantedContent: logEventsJSONString,
		},
		"with follow flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",
			inputFollow:  true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        logEvents,
					LastEventTime: mockLastEventTime,
				}, nil)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), mockLastEventTime, gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        moreLogEvents,
					LastEventTime: nil,
				}, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError: nil,
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
		"returns error if fail to get event logs": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).Return(nil, errors.New("some error"))
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					follow:           tc.inputFollow,
					envName:          tc.inputEnvName,
					svcName:          tc.inputSvc,
					shouldOutputJSON: tc.inputJSON,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				initCwLogsSvc: func(*svcLogsOpts, *config.Environment) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
				w:             b,
			}

			// WHEN
			err := svcLogs.Execute()

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
