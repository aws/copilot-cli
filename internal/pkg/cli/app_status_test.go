// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/aws-sdk-go/aws/awserr"
	humanize "github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppStatus_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		inputEnvironment string
		mockStoreReader  func(m *mocks.Mockstore)

		wantedError error
	}{
		"invalid project name": {
			inputProject: "my-project",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-project").Return(&config.Application{
					Name: "my-project",
				}, nil)
				m.EXPECT().GetService("my-project", "my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputProject:     "my-project",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-project").Return(&config.Application{
					Name: "my-project",
				}, nil)
				m.EXPECT().GetEnvironment("my-project", "test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			inputProject:     "my-project",
			inputApplication: "my-app",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-project").Return(&config.Application{
					Name: "my-project",
				}, nil)
				m.EXPECT().GetEnvironment("my-project", "test").Return(&config.Environment{
					Name: "test",
				}, nil)
				m.EXPECT().GetService("my-project", "my-app").Return(&config.Service{
					Name: "my-app",
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			tc.mockStoreReader(mockStoreReader)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName: tc.inputApplication,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputProject,
					},
				},
				store: mockStoreReader,
			}

			// WHEN
			err := appStatus.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppStatus_Ask(t *testing.T) {
	mockServiceArn := ecs.ServiceArn("mockArn")
	mockError := errors.New("some error")
	mockStackNotFoundErr := fmt.Errorf("describe stack my-project-test-my-app: %w",
		awserr.New("ValidationError", "Stack with id my-project-test-my-app does not exist", nil))
	testCases := map[string]struct {
		inputProject        string
		inputApplication    string
		inputEnvironment    string
		mockStoreReader     func(m *mocks.Mockstore)
		mockWebAppDescriber func(m *mocks.MockserviceArnGetter)
		mockPrompt          func(m *mocks.Mockprompter)

		wantedError error
	}{
		"skip asking": {
			inputProject:     "mockApp",
			inputApplication: "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},
		},
		"errors if failed to list project": {

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListApplications().Return([]*config.Application{}, mockError)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("list projects: some error"),
		},
		"errors if no project found": {

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListApplications().Return([]*config.Application{}, nil)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no project found: run `project init` please"),
		},
		"errors if failed to select project": {

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "mockApp",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appStatusProjectNamePrompt, appStatusProjectNameHelpPrompt, []string{"mockApp"}).Return("", mockError)
			},

			wantedError: fmt.Errorf("select project: some error"),
		},
		"errors if failed to list applications": {
			inputProject: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{}, mockError)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("list applications for project mockApp: some error"),
		},
		"errors if no available application found": {
			inputProject: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{}, nil)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no applications found in project mockApp"),
		},
		"errors if failed to check if app deployed in env": {
			inputProject:     "mockApp",
			inputApplication: "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(nil, mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("check if app mockSvc is deployed in env mockEnv: some error"),
		},
		"errors if no deployed application found": {
			inputProject:     "mockApp",
			inputApplication: "mockSvc",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *mocks.Mockstore) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(nil, mockStackNotFoundErr)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("no deployed apps found in project mockApp"),
		},
		"errors if failed to select deployed application": {
			inputProject: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogAppNamePrompt, applicationLogAppNameHelpPrompt,
					[]string{"mockSvc (mockEnv1)", "mockSvc (mockEnv2)"}).Return("", mockError)
			},

			wantedError: fmt.Errorf("select deployed applications for project mockApp: some error"),
		},
		"success": {
			inputProject: "mockApp",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("mockApp").Return([]*config.Service{
					{
						Name: "mockSvc",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockApp").Return([]*config.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogAppNamePrompt, applicationLogAppNameHelpPrompt,
					[]string{"mockSvc (mockEnv1)", "mockSvc (mockEnv2)"}).Return("mockSvc (mockEnv1)", nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockWebAppDescriber := mocks.NewMockserviceArnGetter(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockWebAppDescriber(mockWebAppDescriber)
			tc.mockPrompt(mockPrompt)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName: tc.inputApplication,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputProject,
						prompt:  mockPrompt,
					},
				},
				appDescriber:     mockWebAppDescriber,
				initAppDescriber: func(*appStatusOpts, string, string) error { return nil },
				store:            mockStoreReader,
			}

			// WHEN
			err := appStatus.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppStatus_Execute(t *testing.T) {
	mockError := errors.New("some error")
	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	updateTime := time.Unix(1584129030, 0)
	mockProvisioningAppStatus := &describe.AppStatusDesc{
		Service: ecs.ServiceStatus{
			DesiredCount:     1,
			RunningCount:     0,
			Status:           "ACTIVE",
			LastDeploymentAt: startTime.Unix(),
			TaskDefinition:   "mockTaskDefinition",
		},
		Alarms: []cloudwatch.AlarmStatus{
			{
				Arn:          "mockAlarmArn",
				Name:         "mockAlarm",
				Reason:       "Threshold Crossed",
				Status:       "OK",
				Type:         "Metric",
				UpdatedTimes: updateTime.Unix(),
			},
		},
		Tasks: []ecs.TaskStatus{
			{
				Health:     "HEALTHY",
				LastStatus: "PROVISIONING",
				ID:         "1234567890123456789",
			},
		},
	}
	mockAppStatus := &describe.AppStatusDesc{
		Service: ecs.ServiceStatus{
			DesiredCount:     1,
			RunningCount:     1,
			Status:           "ACTIVE",
			LastDeploymentAt: startTime.Unix(),
			TaskDefinition:   "mockTaskDefinition",
		},
		Alarms: []cloudwatch.AlarmStatus{
			{
				Arn:          "mockAlarmArn",
				Name:         "mockAlarm",
				Reason:       "Threshold Crossed",
				Status:       "OK",
				Type:         "Metric",
				UpdatedTimes: updateTime.Unix(),
			},
		},
		Tasks: []ecs.TaskStatus{
			{
				Health:     "HEALTHY",
				LastStatus: "RUNNING",
				ID:         "1234567890123456789",
				Images: []ecs.Image{
					{
						Digest: "69671a968e8ec3648e2697417750e",
						ID:     "mockImageID1",
					},
					{
						ID:     "mockImageID2",
						Digest: "ca27a44e25ce17fea7b07940ad793",
					},
				},
				StartedAt:     startTime.Unix(),
				StoppedAt:     stopTime.Unix(),
				StoppedReason: "some reason",
			},
		},
	}
	testCases := map[string]struct {
		shouldOutputJSON bool

		mockStatusDescriber func(m *mocks.MockstatusDescriber)

		wantedContent string
		wantedError   error
	}{
		"errors if failed to describe the status of the app": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe status of application mockSvc: some error"),
		},
		"success with JSON output": {
			shouldOutputJSON: true,

			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockAppStatus, nil)
			},

			wantedContent: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":1136214245,\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":1136214245,\"stoppedAt\":1136217845,\"stoppedReason\":\"some reason\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"reason\":\"Threshold Crossed\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":1584129030}]}\n",
		},
		"success with human output": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockAppStatus, nil)
			},

			wantedContent: fmt.Sprintf(`Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At        %s
  Task Definition   mockTaskDefinition

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  12345678          69671a96,ca27a44e   RUNNING             HEALTHY             %s        %s

Alarms

  Name              Health              Last Updated        Reason
  mockAlarm         OK                  %s         Threshold Crossed
`, humanize.Time(startTime), humanize.Time(startTime), humanize.Time(stopTime), humanize.Time(updateTime)),
		},
		"success with human output when task is provisioning": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockProvisioningAppStatus, nil)
			},

			wantedContent: fmt.Sprintf(`Service Status

  ACTIVE 0 / 1 running tasks (1 pending)

Last Deployment

  Updated At        %s
  Task Definition   mockTaskDefinition

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  12345678          -                   PROVISIONING        HEALTHY             -                   -

Alarms

  Name              Health              Last Updated        Reason
  mockAlarm         OK                  %s         Threshold Crossed
`, humanize.Time(startTime), humanize.Time(updateTime)),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStatusDescriber := mocks.NewMockstatusDescriber(ctrl)
			tc.mockStatusDescriber(mockStatusDescriber)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName:          "mockSvc",
					envName:          "mockEnv",
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						appName: "mockApp",
					},
				},
				statusDescriber:     mockStatusDescriber,
				initStatusDescriber: func(*appStatusOpts) error { return nil },
				w:                   b,
			}

			// WHEN
			err := appStatus.Execute()

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
