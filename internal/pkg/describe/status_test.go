// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	ecsapi "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestServiceStatus_Describe(t *testing.T) {
	badMockServiceArn := ecs.ServiceArn("badMockArn")
	mockServiceArn := ecs.ServiceArn("arn:aws:ecs:us-west-2:1234567890:service/mockCluster/mockService")
	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	updateTime, _ := time.Parse(time.RFC3339, "2020-03-13T19:50:30+00:00")
	mockError := errors.New("some error")
	testCases := map[string]struct {
		mockecsSvc           func(m *mocks.MockecsServiceGetter)
		mockcwSvc            func(m *mocks.MockalarmStatusGetter)
		mockServiceDescriber func(m *mocks.MockserviceArnGetter)

		wantedError   error
		wantedContent *ServiceStatusDesc
	}{
		"errors if failed to get service ARN": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *mocks.MockalarmStatusGetter) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(nil, mockError)
			},

			wantedError: fmt.Errorf("get service ARN: some error"),
		},
		"errors if failed to get cluster name": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {},
			mockcwSvc:  func(m *mocks.MockalarmStatusGetter) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&badMockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get cluster name: arn: invalid prefix"),
		},
		"errors if failed to get service info": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get service mockService: some error"),
		},
		"errors if failed to get running tasks info": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return(nil, mockError)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get tasks for service mockService: some error"),
		},
		"errors if failed to get running tasks status": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn: aws.String("badMockTaskArn"),
					},
				}, nil)
			},
			mockcwSvc: func(m *mocks.MockalarmStatusGetter) {},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
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
					"copilot-application": "mockApp",
					"copilot-environment": "mockEnv",
					"copilot-service":     "mockSvc",
				}).Return(nil, mockError)
			},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},

			wantedError: fmt.Errorf("get CloudWatch alarms: some error"),
		},
		"success": {
			mockecsSvc: func(m *mocks.MockecsServiceGetter) {
				m.EXPECT().Service("mockCluster", "mockService").Return(&ecs.Service{
					Status:       aws.String("ACTIVE"),
					DesiredCount: aws.Int64(1),
					RunningCount: aws.Int64(1),
					Deployments: []*ecsapi.Deployment{
						{
							UpdatedAt:      &startTime,
							TaskDefinition: aws.String("mockTaskDefinition"),
						},
					},
				}, nil)
				m.EXPECT().ServiceTasks("mockCluster", "mockService").Return([]*ecs.Task{
					{
						TaskArn:      aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
						StartedAt:    &startTime,
						HealthStatus: aws.String("HEALTHY"),
						LastStatus:   aws.String("RUNNING"),
						Containers: []*ecsapi.Container{
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
					"copilot-application": "mockApp",
					"copilot-environment": "mockEnv",
					"copilot-service":     "mockSvc",
				}).Return([]cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
				}, nil)
			},
			mockServiceDescriber: func(m *mocks.MockserviceArnGetter) {
				m.EXPECT().GetServiceArn().Return(&mockServiceArn, nil)
			},

			wantedContent: &ServiceStatusDesc{
				Service: ecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     1,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
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
						StartedAt:     startTime,
						StoppedAt:     stopTime,
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
			mockServiceDescriber := mocks.NewMockserviceArnGetter(ctrl)
			tc.mockecsSvc(mockecsSvc)
			tc.mockcwSvc(mockcwSvc)
			tc.mockServiceDescriber(mockServiceDescriber)

			svcStatus := &ServiceStatus{
				SvcName:   "mockSvc",
				EnvName:   "mockEnv",
				AppName:   "mockApp",
				CwSvc:     mockcwSvc,
				EcsSvc:    mockecsSvc,
				Describer: mockServiceDescriber,
			}

			// WHEN
			statusDesc, err := svcStatus.Describe()

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

func TestServiceStatusDesc_String(t *testing.T) {
	// from the function changes (ex: from "1 month ago" to "2 months ago"). To make our tests stable,
	oldHumanize := humanizeTime
	humanizeTime = func(then time.Time) string {
		now, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00+00:00")
		return humanize.RelTime(then, now, "ago", "from now")
	}
	defer func() {
		humanizeTime = oldHumanize
	}()

	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	updateTime, _ := time.Parse(time.RFC3339, "2020-03-13T19:50:30+00:00")

	testCases := map[string]struct {
		desc  *ServiceStatusDesc
		human string
		json  string
	}{
		"while provisioning": {
			desc: &ServiceStatusDesc{
				Service: ecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     0,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
				},
				Tasks: []ecs.TaskStatus{
					{
						Health:     "HEALTHY",
						LastStatus: "PROVISIONING",
						ID:         "1234567890123456789",
					},
				},
			},
			human: `Service Status

  ACTIVE 0 / 1 running tasks (1 pending)

Last Deployment

  Updated At        14 years ago
  Task Definition   mockTaskDefinition

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  12345678          -                   PROVISIONING        HEALTHY             -                   -

Alarms

  Name              Health              Last Updated        Reason
  mockAlarm         OK                  2 months from now   Threshold Crossed
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":0,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":null,\"lastStatus\":\"PROVISIONING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"0001-01-01T00:00:00Z\",\"stoppedReason\":\"\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"reason\":\"Threshold Crossed\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}]}\n",
		},
		"running": {
			desc: &ServiceStatusDesc{
				Service: ecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     1,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Name:         "mockAlarm",
						Reason:       "Threshold Crossed",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
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
						StartedAt:     startTime,
						StoppedAt:     stopTime,
						StoppedReason: "some reason",
					},
				},
			},
			human: `Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At        14 years ago
  Task Definition   mockTaskDefinition

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  12345678          69671a96,ca27a44e   RUNNING             HEALTHY             14 years ago        14 years ago

Alarms

  Name              Health              Last Updated        Reason
  mockAlarm         OK                  2 months from now   Threshold Crossed
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":\"2006-01-02T15:04:05Z\",\"stoppedAt\":\"2006-01-02T16:04:05Z\",\"stoppedReason\":\"some reason\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"reason\":\"Threshold Crossed\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}]}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			json, err := tc.desc.JSONString()
			require.NoError(t, err)
			require.Equal(t, tc.human, tc.desc.HumanString())
			require.Equal(t, tc.json, json)
		})
	}
}
