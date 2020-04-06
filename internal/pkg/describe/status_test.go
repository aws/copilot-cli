// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/aws/aws-sdk-go/aws"
	ECSAPI "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWebAppStatus_Describe(t *testing.T) {
	badMockServiceArn := ecs.ServiceArn("badMockArn")
	mockServiceArn := ecs.ServiceArn("arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService")
	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	mockError := errors.New("some error")
	testCases := map[string]struct {
		mockecsSvc          func(m *mocks.MockecsServiceGetter)
		mockcwSvc           func(m *mocks.MockalarmStatusGetter)
		mockWebAppDescriber func(m *mocks.MockserviceArnGetter)

		wantedError   error
		wantedContent *WebAppStatusDesc
	}{
		"errors if failed to get service ARN": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *mocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get service ARN: some error"),
		},
		"errors if failed to get cluster name": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *mocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&badMockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get cluster name: arn: invalid prefix"),
		},
		"errors if failed to get ECS service info": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get ECS service mockService: some error"),
		},
		"errors if failed to get ECS running tasks info": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get ECS tasks for service mockService: some error"),
		},
		"errors if failed to get ECS running tasks status": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn: aws.String("badMockTaskArn"),
					},
				}, nil)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get status for task badMockTaskArn: arn: invalid prefix"),
		},
		"errors if failed to get CloudWatch alarms": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn:   aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
						StartedAt: &startTime,
					},
				}, nil)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {
				m.EXPECT().GetAlarmsWithTags(map[string]string{
					"ecs-project":     "mockProject",
					"ecs-environment": "mockEnv",
					"ecs-application": "mockApp",
				}).Return(nil, mockError)
			},
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get CloudWatch alarms: some error"),
		},
		"success": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{
					Status:       aws.String("ACTIVE"),
					DesiredCount: aws.Int64(1),
					RunningCount: aws.Int64(1),
					Deployments: []*ECSAPI.Deployment{
						{
							UpdatedAt:      &startTime,
							TaskDefinition: aws.String("mockTaskDefinition"),
						},
					},
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
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {
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
			mockWebAppDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn("mockEnv").Return(&mockServiceArn, nil)
			},

			wantedContent: &WebAppStatusDesc{
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
						UpdatedTimes: 1584129030,
					},
				},
				Tasks: []ecs.TaskStatus{
					{
						DesiredStatus: "RUNNING",
						LastStatus:    "RUNNING",
						ID:            "1234567890123456789",
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
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockecsSvc := mocks.NewMockecsServiceGetter(ctrl)
			mockcwSvc := mocks.NewMockalarmStatusGetter(ctrl)
			mockWebAppDescriber := mocks.NewMockserviceArnGetter(ctrl)
			tc.mockecsSvc(mockecsSvc)
			tc.mockcwSvc(mockcwSvc)
			tc.mockWebAppDescriber(mockWebAppDescriber)

			appStatus := &WebAppStatus{
				AppName:     "mockApp",
				EnvName:     "mockEnv",
				ProjectName: "mockProject",
				CwSvc:       mockcwSvc,
				EcsSvc:      mockecsSvc,
				Describer:   mockWebAppDescriber,
				// initDescriber: func(*WebAppStatus, string) error { return nil },
				// initcwSvc:     func(*WebAppStatus, *archer.Environment) error { return nil },
				// initecsSvc:    func(*WebAppStatus, *archer.Environment) error { return nil },
			}

			// WHEN
			statusDesc, err := appStatus.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, statusDesc, "expected output content match")
			}
		})
	}
}
