// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatchlogs"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppLogs_Validate(t *testing.T) {
	const (
		mockLimit        = 3
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		inputLimit       int
		inputFollow      bool
		inputEnvName     string
		inputStartTime   string
		inputEndTime     string
		inputSince       time.Duration

		mockStoreReader  func(m *climocks.MockstoreReader)
		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService

		wantedError error
	}{
		"with no flag set": {
			// default value for limit and since flags
			inputLimit: 10,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: nil,
		},
		"invalid project name": {
			inputProject: "my-project",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(nil, errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("some error"),
		},
		"returns error if since and startTime flags are set together": {
			inputSince:     mockSince,
			inputStartTime: mockStartTime,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("only one of --since or --start-time may be used"),
		},
		"returns error if follow and endTime flags are set together": {
			inputFollow:  true,
			inputEndTime: mockEndTime,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("only one of --follow or --end-time may be used"),
		},
		"returns error if invalid start time flag value": {
			inputStartTime: mockBadStartTime,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("invalid argument badStartTime for \"--start-time\" flag: reading time value badStartTime: parsing time \"badStartTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badStartTime\" as \"2006\""),
		},
		"returns error if invalid end time flag value": {
			inputEndTime: mockBadEndTime,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("invalid argument badEndTime for \"--end-time\" flag: reading time value badEndTime: parsing time \"badEndTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badEndTime\" as \"2006\""),
		},
		"returns error if invalid since flag value": {
			inputSince: -mockSince,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("--since must be greater than 0"),
		},
		"returns error if limit value is below limit": {
			inputLimit: -1,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("--limit -1 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if limit value is above limit": {
			inputLimit: 10001,

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},

			wantedError: fmt.Errorf("--limit 10001 is out-of-bounds, value must be between 1 and 10000"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			tc.mockStoreReader(mockStoreReader)

			appLogs := &appLogsOpts{
				appLogsVars: appLogsVars{
					follow:         tc.inputFollow,
					limit:          tc.inputLimit,
					envName:        tc.inputEnvName,
					humanStartTime: tc.inputStartTime,
					humanEndTime:   tc.inputEndTime,
					since:          tc.inputSince,
					appName:        tc.inputApplication,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				storeSvc:      mockStoreReader,
				initCwLogsSvc: func(*appLogsOpts, *archer.Environment) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
			}

			// WHEN
			err := appLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppLogs_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		inputEnvName     string

		mockStoreReader  func(m *climocks.MockstoreReader)
		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService
		mockPrompter     func(m *climocks.Mockprompter)

		wantedError error
	}{
		"with all flag set": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("mockProject", "mockApp").Return(&archer.Application{
					Name: "mockApp",
				}, nil)
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp")).Return(true, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {},

			wantedError: nil,
		},
		"with all flag set and return error if fail to get application": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("mockProject", "mockApp").Return(nil, errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("get application: some error"),
		},
		"with all flag set and return error if fail to get environment": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("mockProject", "mockApp").Return(&archer.Application{
					Name: "mockApp",
				}, nil)
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(nil, errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("get environment: some error"),
		},
		"with only app flag set and not deployed in one of envs": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetApplication("mockProject", "mockApp").Return(&archer.Application{
					Name: "mockApp",
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockEnv",
					},
					&archer.Environment{
						Name: "mockTestEnv",
					},
					&archer.Environment{
						Name: "mockProdEnv",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockTestEnv", "mockApp")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockProdEnv", "mockApp")).Return(false, nil)
				cwlogServices["mockEnv"] = m
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(applicationLogAppNamePrompt), applicationLogAppNameHelpPrompt, []string{"mockApp (mockEnv)", "mockApp (mockTestEnv)"}).Return("mockApp (mockTestEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"with only env flag set": {
			inputProject: "mockProject",
			inputEnvName: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockFrontend",
					},
					&archer.Application{
						Name: "mockBackend",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockFrontend")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockBackend")).Return(true, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(fmt.Sprintf(applicationLogAppNamePrompt), applicationLogAppNameHelpPrompt, []string{"mockFrontend (mockEnv)", "mockBackend (mockEnv)"}).Return("mockFrontend (mockEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"retrieve app name from ssm store": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockTestEnv",
					},
					&archer.Environment{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockTestEnv", "mockApp")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockProdEnv", "mockApp")).Return(true, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
				m.EXPECT().SelectOne(fmt.Sprintf(applicationLogAppNamePrompt), applicationLogAppNameHelpPrompt, []string{"mockApp (mockTestEnv)", "mockApp (mockProdEnv)"}).Return("mockApp (mockTestEnv)", nil).Times(1)
			},

			wantedError: nil,
		},
		"skip selecting if only one deployed app found": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockTestEnv",
					},
					&archer.Environment{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockTestEnv", "mockApp")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockProdEnv", "mockApp")).Return(false, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: nil,
		},
		"returns error if fail to list projects": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return(nil, errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("list projects: some error"),
		},
		"returns error if no project found": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("no project found: run %s please", color.HighlightCode("project init")),
		},
		"returns error if fail to select project": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("", errors.New("some error"))
			},

			wantedError: fmt.Errorf("select projects: some error"),
		},
		"returns error if fail to retrieve application": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return(nil, errors.New("some error"))
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("list applications for project mockProject: some error"),
		},
		"returns error if no applications found": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("no applications found in project %s", color.HighlightUserInput("mockProject")),
		},
		"returns error if fail to list environments": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return(nil, errors.New("some error"))
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("list environments: some error"),
		},
		"returns error if no environment found": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				return nil
			}, mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("no environments found in project %s", color.HighlightUserInput("mockProject")),
		},
		"returns error if fail to check application deployed or not": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockEnv",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp")).Return(false, errors.New("some error"))
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("check if the log group exists: some error"),
		},
		"returns error if no deployed application found": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockEnv",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp")).Return(false, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
			},

			wantedError: fmt.Errorf("no deployed applications found in project %s", color.HighlightUserInput("mockProject")),
		},
		"returns error if fail to select app env name": {
			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{
						Name: "mockProject",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					&archer.Environment{
						Name: "mockTestEnv",
					},
					&archer.Environment{
						Name: "mockProdEnv",
					},
				}, nil)
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					&archer.Application{
						Name: "mockApp",
					},
				}, nil)
			},
			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockTestEnv", "mockApp")).Return(true, nil)
				m.EXPECT().LogGroupExists(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockProdEnv", "mockApp")).Return(true, nil)
				cwlogServices["mockTestEnv"] = m
				cwlogServices["mockProdEnv"] = m
				return cwlogServices
			},
			mockPrompter: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogProjectNamePrompt, applicationLogProjectNameHelpPrompt, []string{"mockProject"}).Return("mockProject", nil)
				m.EXPECT().SelectOne(fmt.Sprintf(applicationLogAppNamePrompt), applicationLogAppNameHelpPrompt, []string{"mockApp (mockTestEnv)", "mockApp (mockProdEnv)"}).Return("", errors.New("some error")).Times(1)
			},

			wantedError: fmt.Errorf("select deployed applications for project mockProject: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockPrompter(mockPrompter)

			appLogs := &appLogsOpts{
				appLogsVars: appLogsVars{
					envName: tc.inputEnvName,
					appName: tc.inputApplication,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
						prompt:      mockPrompter,
					},
				},
				storeSvc:      mockStoreReader,
				initCwLogsSvc: func(*appLogsOpts, *archer.Environment) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
			}

			// WHEN
			err := appLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppLogs_Execute(t *testing.T) {
	mockLastEventTime := map[string]int64{
		"mockLogStreamName": 123456,
	}
	logEvents := []*cloudwatchlogs.Event{
		&cloudwatchlogs.Event{
			TaskID:  "123456789",
			Message: `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -`,
		},
		&cloudwatchlogs.Event{
			TaskID:  "123456789",
			Message: `10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -`,
		},
		&cloudwatchlogs.Event{
			TaskID:  "123456789",
			Message: `10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -`,
		},
	}
	moreLogEvents := []*cloudwatchlogs.Event{
		&cloudwatchlogs.Event{
			TaskID:  "123456789",
			Message: `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	logEventsHumanString := `1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
	logEventsJSONString := "{\"taskID\":\"123456789\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"taskID\":\"123456789\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"taskID\":\"123456789\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		inputFollow      bool
		inputEnvName     string
		inputJSON        bool

		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService

		wantedError   error
		wantedContent string
	}{
		"with no optional flags set": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp"), make(map[string]int64), gomock.Any()).
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
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",
			inputJSON:        true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp"), make(map[string]int64), gomock.Any()).
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
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",
			inputFollow:      true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp"), make(map[string]int64), gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        logEvents,
					LastEventTime: mockLastEventTime,
				}, nil)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp"), mockLastEventTime, gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        moreLogEvents,
					LastEventTime: nil,
				}, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError: nil,
			wantedContent: `1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
1234567 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
		"returns error if fail to get event logs": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvName:     "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := climocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockProject", "mockEnv", "mockApp"), make(map[string]int64), gomock.Any()).Return(nil, errors.New("some error"))
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
			appLogs := &appLogsOpts{
				appLogsVars: appLogsVars{
					follow:           tc.inputFollow,
					envName:          tc.inputEnvName,
					appName:          tc.inputApplication,
					shouldOutputJSON: tc.inputJSON,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				initCwLogsSvc: func(*appLogsOpts, *archer.Environment) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
				w:             b,
			}

			// WHEN
			err := appLogs.Execute()

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
