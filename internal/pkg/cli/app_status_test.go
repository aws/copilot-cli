// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
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
		mockStoreReader  func(m *climocks.MockstoreReader)

		wantedError error
	}{
		"invalid project name": {
			inputProject: "my-project",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid application name": {
			inputProject:     "my-project",
			inputApplication: "my-app",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name: "my-project",
				}, nil)
				m.EXPECT().GetApplication("my-project", "my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputProject:     "my-project",
			inputEnvironment: "test",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
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

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetProject("my-project").Return(&archer.Project{
					Name: "my-project",
				}, nil)
				m.EXPECT().GetEnvironment("my-project", "test").Return(&archer.Environment{
					Name: "test",
				}, nil)
				m.EXPECT().GetApplication("my-project", "my-app").Return(&archer.Application{
					Name: "my-app",
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			tc.mockStoreReader(mockStoreReader)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName: tc.inputApplication,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				storeSvc: mockStoreReader,
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
		mockStoreReader     func(m *climocks.MockstoreReader)
		mockWebAppDescriber func(m *climocks.MockserviceArnGetter)
		mockPrompt          func(m *climocks.Mockprompter)

		wantedError error
	}{
		"skip asking": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},
		},
		"errors if failed to list project": {

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{}, mockError)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("list projects: some error"),
		},
		"errors if no project found": {

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("no project found: run `project init` please"),
		},
		"errors if failed to select project": {

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "mockProject",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appStatusProjectNamePrompt, appStatusProjectNameHelpPrompt, []string{"mockProject"}).Return("", mockError)
			},

			wantedError: fmt.Errorf("select project: some error"),
		},
		"errors if failed to list applications": {
			inputProject: "mockProject",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{}, mockError)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("list applications for project mockProject: some error"),
		},
		"errors if no available application found": {
			inputProject: "mockProject",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {},
			mockPrompt:          func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("no applications found in project mockProject"),
		},
		"errors if failed to check if app deployed in env": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(nil, mockError)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("check if app mockApp is deployed in env mockEnv: some error"),
		},
		"errors if no deployed application found": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(nil, mockStackNotFoundErr)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("no deployed apps found in project mockProject"),
		},
		"errors if failed to select deployed application": {
			inputProject: "mockProject",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					{
						Name: "mockApp",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv1").Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn("mockEnv2").Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogAppNamePrompt, applicationLogAppNameHelpPrompt,
					[]string{"mockApp (mockEnv1)", "mockApp (mockEnv2)"}).Return("", mockError)
			},

			wantedError: fmt.Errorf("select deployed applications for project mockProject: some error"),
		},
		"success": {
			inputProject: "mockProject",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().ListApplications("mockProject").Return([]*archer.Application{
					{
						Name: "mockApp",
					},
				}, nil)
				m.EXPECT().ListEnvironments("mockProject").Return([]*archer.Environment{
					{
						Name: "mockEnv1",
					},
					{
						Name: "mockEnv2",
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv1").Return(&mockServiceArn, nil)
				m.EXPECT().GetServiceArn("mockEnv2").Return(&mockServiceArn, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationLogAppNamePrompt, applicationLogAppNameHelpPrompt,
					[]string{"mockApp (mockEnv1)", "mockApp (mockEnv2)"}).Return("mockApp (mockEnv1)", nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockWebAppDescriber := climocks.NewMockserviceArnGetter(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockWebAppDescriber(mockWebAppDescriber)
			tc.mockPrompt(mockPrompt)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName: tc.inputApplication,
					envName: tc.inputEnvironment,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
						prompt:      mockPrompt,
					},
				},
				appDescriber:     mockWebAppDescriber,
				initAppDescriber: func(*appStatusOpts, string) error { return nil },
				storeSvc:         mockStoreReader,
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
	mockProvisioningAppStatus := &describe.WebAppStatusDesc{
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
	mockAppStatus := &describe.WebAppStatusDesc{
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

		mockStatusDescriber func(m *climocks.MockstatusDescriber)

		wantedContent string
		wantedError   error
	}{
		"errors if failed to describe the status of the app": {
			mockStatusDescriber: func(m *climocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe status of application mockApp: some error"),
		},
		"success with JSON output": {
			shouldOutputJSON: true,

			mockStatusDescriber: func(m *climocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(mockAppStatus, nil)
			},

			wantedContent: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":1136214245,\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":1136214245,\"stoppedAt\":1136217845,\"stoppedReason\":\"some reason\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"reason\":\"Threshold Crossed\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":1584129030}]}\n",
		},
		"success with human output": {
			mockStatusDescriber: func(m *climocks.MockstatusDescriber) {
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
			mockStatusDescriber: func(m *climocks.MockstatusDescriber) {
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
			mockStatusDescriber := climocks.NewMockstatusDescriber(ctrl)
			tc.mockStatusDescriber(mockStatusDescriber)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName:          "mockApp",
					envName:          "mockEnv",
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						projectName: "mockProject",
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
