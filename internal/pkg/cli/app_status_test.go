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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	ECSAPI "github.com/aws/aws-sdk-go/service/ecs"
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
	mockServiceArn := describe.ServiceArn("mockArn")
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
				describer:     mockWebAppDescriber,
				initDescriber: func(*appStatusOpts, string) error { return nil },
				storeSvc:      mockStoreReader,
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
	badMockServiceArn := describe.ServiceArn("badMockArn")
	mockServiceArn := describe.ServiceArn("arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService")
	mockError := errors.New("some error")
	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	testCases := map[string]struct {
		inputProject     string
		inputApplication string
		inputEnvironment string
		shouldOutputJSON bool

		mockStoreReader     func(m *climocks.MockstoreReader)
		mockecsSvc          func(m *climocks.MockecsServiceGetter)
		mockcwSvc           func(m *climocks.MockalarmStatusGetter)
		mockWebAppDescriber func(m *climocks.MockserviceArnGetter)

		wantedContent string
		wantedError   error
	}{
		"errors if failed to get environment": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(nil, mockError)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
			},

			wantedError: fmt.Errorf("get environment mockEnv: some error"),
		},
		"errors if failed to get service ARN": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get service ARN: some error"),
		},
		"errors if failed to get cluster name": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&badMockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get cluster name: arn: invalid prefix"),
		},
		"errors if failed to get ECS service info": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get ECS service mockService: some error"),
		},
		"errors if failed to get ECS running tasks info": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get ECS tasks for service mockService: some error"),
		},
		"errors if failed to get ECS running tasks status": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn: aws.String("badMockTaskArn"),
					},
				}, nil)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get status for task badMockTaskArn: arn: invalid prefix"),
		},
		"errors if failed to get CloudWatch alarms": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn:   aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
						StartedAt: &startTime,
					},
				}, nil)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {
				m.EXPECT().GetAlarmsWithTags(map[string]string{
					"ecs-project":     "mockProject",
					"ecs-environment": "mockEnv",
					"ecs-application": "mockApp",
				}).Return(nil, mockError)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get CloudWatch alarms: some error"),
		},
		"success with JSON output": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",
			shouldOutputJSON: true,

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{
					Status:       aws.String("ACTIVE"),
					DesiredCount: aws.Int64(1),
					RunningCount: aws.Int64(1),
				}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn:       aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
						StartedAt:     &startTime,
						DesiredStatus: aws.String("RUNNING"),
						LastStatus:    aws.String("RUNNING"),
						Containers: []*ECSAPI.Container{
							{
								Image:       aws.String("mockImageID1"),
								ImageDigest: aws.String("69671a968e8ec3648e2697417750e"),
							},
							{
								Image:       aws.String("mockImageID2"),
								ImageDigest: aws.String("ca27a44e25ce17fea7b07940ad793"),
							},
						},
						StoppedAt:     &stopTime,
						StoppedReason: aws.String("some reason"),
					},
				}, nil)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {
				m.EXPECT().GetAlarmsWithTags(map[string]string{
					"ecs-project":     "mockProject",
					"ecs-environment": "mockEnv",
					"ecs-application": "mockApp",
				}).Return([]cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: 1584129030,
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedContent: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\"},\"tasks\":[{\"desiredStatus\":\"RUNNING\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":1136214245,\"stoppedAt\":1136217845,\"stoppedReason\":\"some reason\"}],\"metrics\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"reason\":\"Threshold Crossed\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":1584129030}]}\n",
		},
		"success with human output": {
			inputProject:     "mockProject",
			inputApplication: "mockApp",
			inputEnvironment: "mockEnv",

			mockStoreReader: func(m *climocks.MockstoreReader) {
				m.EXPECT().GetEnvironment("mockProject", "mockEnv").Return(&archer.Environment{
					Name: "mockEnv",
				}, nil)
			},
			mockecsSvc: func(m *climocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{
					Status:       aws.String("ACTIVE"),
					DesiredCount: aws.Int64(1),
					RunningCount: aws.Int64(1),
				}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn:       aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
						StartedAt:     &startTime,
						DesiredStatus: aws.String("RUNNING"),
						LastStatus:    aws.String("RUNNING"),
						Containers: []*ECSAPI.Container{
							{
								Image:       aws.String("mockImageID1"),
								ImageDigest: aws.String("69671a968e8ec3648e2697417750e"),
							},
							{
								Image:       aws.String("mockImageID2"),
								ImageDigest: aws.String("ca27a44e25ce17fea7b07940ad793"),
							},
						},
						StoppedAt:     &stopTime,
						StoppedReason: aws.String("some reason"),
					},
				}, nil)
			},
			mockcwSvc: func(m *climocks.MockalarmStatusGetter) {
				m.EXPECT().GetAlarmsWithTags(map[string]string{
					"ecs-project":     "mockProject",
					"ecs-environment": "mockEnv",
					"ecs-application": "mockApp",
				}).Return([]cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: 1584129030,
					},
				}, nil)
			},
			mockWebAppDescriber: func(m *climocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedContent: `Service Status

  Status            ACTIVE
  DesiredCount      1
  RunningCount      1

Task Status

  ID                ImageDigest         LastStatus          DesiredStatus       StartedAt           StoppedAt
  12345678          69671a96,ca27a44e   RUNNING             RUNNING             14 years ago        14 years ago

Metrics

  Name              Health              UpdatedTimes        Reason
  mockAlarm         OK                  2 weeks ago         Threshold Crossed
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := climocks.NewMockstoreReader(ctrl)
			mockecsSvc := climocks.NewMockecsServiceGetter(ctrl)
			mockcwSvc := climocks.NewMockalarmStatusGetter(ctrl)
			mockWebAppDescriber := climocks.NewMockserviceArnGetter(ctrl)
			tc.mockStoreReader(mockStoreReader)
			tc.mockecsSvc(mockecsSvc)
			tc.mockcwSvc(mockcwSvc)
			tc.mockWebAppDescriber(mockWebAppDescriber)

			appStatus := &appStatusOpts{
				appStatusVars: appStatusVars{
					appName:          tc.inputApplication,
					envName:          tc.inputEnvironment,
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						projectName: tc.inputProject,
					},
				},
				cwSvc:         mockcwSvc,
				ecsSvc:        mockecsSvc,
				describer:     mockWebAppDescriber,
				initDescriber: func(*appStatusOpts, string) error { return nil },
				initcwSvc:     func(*appStatusOpts, *archer.Environment) error { return nil },
				initecsSvc:    func(*appStatusOpts, *archer.Environment) error { return nil },
				storeSvc:      mockStoreReader,
				w:             b,
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
