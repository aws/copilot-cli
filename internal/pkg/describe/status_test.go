// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/ecs"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type serviceStatusMocks struct {
	ecsServiceGetter  *mocks.MockecsServiceGetter
	alarmStatusGetter *mocks.MockalarmStatusGetter
	serviceDescriber  *mocks.MockserviceDescriber
	aas               *mocks.MockautoscalingAlarmNamesGetter
}

func TestServiceStatus_Describe(t *testing.T) {
	const (
		mockCluster = "mockCluster"
		mockService = "mockService"
	)
	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
	updateTime, _ := time.Parse(time.RFC3339, "2020-03-13T19:50:30+00:00")
	mockServiceDesc := &ecs.ServiceDesc{
		ClusterName: mockCluster,
		Name:        mockService,
		Tasks: []*awsecs.Task{
			{
				TaskArn:   aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
				StartedAt: &startTime,
			},
		},
	}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks serviceStatusMocks)

		wantedError   error
		wantedContent *ServiceStatusDesc
	}{
		"errors if failed to describe a service": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get ECS service description for mockSvc: some error"),
		},
		"errors if failed to get the ECS service": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get service mockService: some error"),
		},
		"errors if failed to get running tasks status": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: mockCluster,
						Name:        mockService,
						Tasks: []*awsecs.Task{
							{
								TaskArn: aws.String("badMockTaskArn"),
							},
						},
					}, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
				)
			},

			wantedError: fmt.Errorf("get status for task badMockTaskArn: parse ECS task ARN: arn: invalid prefix"),
		},
		"errors if failed to get tagged CloudWatch alarms": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get tagged CloudWatch alarms: some error"),
		},
		"errors if failed to get auto scaling CloudWatch alarm names": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("retrieve auto scaling alarm names for ECS service mockCluster/mockService: some error"),
		},
		"errors if failed to get auto scaling CloudWatch alarm status": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{"mockAlarmName"}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatus([]string{"mockAlarmName"}).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get auto scaling CloudWatch alarms: some error"),
		},
		"success": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: mockCluster,
						Name:        mockService,
						Tasks: []*awsecs.Task{
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
						},
					}, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{
						Status:       aws.String("ACTIVE"),
						DesiredCount: aws.Int64(1),
						RunningCount: aws.Int64(1),
						Deployments: []*ecsapi.Deployment{
							{
								UpdatedAt:      &startTime,
								TaskDefinition: aws.String("mockTaskDefinition"),
							},
						},
					}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(map[string]string{
						"copilot-application": "mockApp",
						"copilot-environment": "mockEnv",
						"copilot-service":     "mockSvc",
					}).Return([]cloudwatch.AlarmStatus{
						{
							Arn:          "mockAlarmArn1",
							Name:         "mockAlarm1",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
					}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{"mockAlarm2"}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatus([]string{"mockAlarm2"}).Return([]cloudwatch.AlarmStatus{
						{
							Arn:          "mockAlarmArn2",
							Name:         "mockAlarm2",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
					}, nil),
				)
			},

			wantedContent: &ServiceStatusDesc{
				Service: awsecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     1,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn1",
						Name:         "mockAlarm1",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn2",
						Condition:    "mockCondition",
						Name:         "mockAlarm2",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
				},
				Tasks: []awsecs.TaskStatus{
					{
						Health:     "HEALTHY",
						LastStatus: "RUNNING",
						ID:         "1234567890123456789",
						Images: []awsecs.Image{
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
			mockSvcDescriber := mocks.NewMockserviceDescriber(ctrl)
			mockaasClient := mocks.NewMockautoscalingAlarmNamesGetter(ctrl)
			mocks := serviceStatusMocks{
				ecsServiceGetter:  mockecsSvc,
				alarmStatusGetter: mockcwSvc,
				serviceDescriber:  mockSvcDescriber,
				aas:               mockaasClient,
			}

			tc.setupMocks(mocks)

			svcStatus := &ServiceStatus{
				svc:          "mockSvc",
				env:          "mockEnv",
				app:          "mockApp",
				cwSvc:        mockcwSvc,
				ecsSvc:       mockecsSvc,
				svcDescriber: mockSvcDescriber,
				aasSvc:       mockaasClient,
			}

			// WHEN
			statusDesc, err := svcStatus.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
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
	updateTime, _ := time.Parse(time.RFC3339, "2020-03-13T19:50:30+00:00")

	testCases := map[string]struct {
		desc  *ServiceStatusDesc
		human string
		json  string
	}{
		"while provisioning": {
			desc: &ServiceStatusDesc{
				Service: awsecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     0,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn1",
						Name:         "mySupercalifragilisticexpialidociousAlarm",
						Condition:    "RequestCount > 100.00 for 3 datapoints within 25 minutes",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn2",
						Name:         "Um-dittle-ittl-um-dittle-I-Alarm",
						Condition:    "CPUUtilization > 70.00 for 3 datapoints within 3 minutes",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
				},
				Tasks: []awsecs.TaskStatus{
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

  Updated At         14 years ago
  Task Definition    mockTaskDefinition

Task Status

  ID                Image Digest        Last Status         Started At          Stopped At          Capacity Provider    Health Status
  --                ------------        -----------         ----------          ----------          -----------------    -------------
  12345678          -                   PROVISIONING        -                   -                   -                    HEALTHY

Alarms

  Name                              Condition                         Last Updated         Health
  ----                              ---------                         ------------         ------
  mySupercalifragilisticexpialid    RequestCount > 100.00 for 3 da    2 months from now    OK
  ociousAlarm                       tapoints within 25 minutes                             
                                                                                           
  Um-dittle-ittl-um-dittle-I-Ala    CPUUtilization > 70.00 for 3 d    2 months from now    OK
  rm                                atapoints within 3 minutes                             
                                                                                           
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":0,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":null,\"lastStatus\":\"PROVISIONING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"0001-01-01T00:00:00Z\",\"stoppedReason\":\"\",\"capacityProvider\":\"\"}],\"alarms\":[{\"arn\":\"mockAlarmArn1\",\"name\":\"mySupercalifragilisticexpialidociousAlarm\",\"condition\":\"RequestCount \\u003e 100.00 for 3 datapoints within 25 minutes\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"},{\"arn\":\"mockAlarmArn2\",\"name\":\"Um-dittle-ittl-um-dittle-I-Alarm\",\"condition\":\"CPUUtilization \\u003e 70.00 for 3 datapoints within 3 minutes\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}]}\n",
		},
		"running": {
			desc: &ServiceStatusDesc{
				Service: awsecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     1,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn",
						Condition:    "mockCondition",
						Name:         "mockAlarm",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
				},
				Tasks: []awsecs.TaskStatus{
					{
						Health:     "HEALTHY",
						LastStatus: "RUNNING",
						ID:         "1234567890123456789",
						Images: []awsecs.Image{
							{
								Digest: "69671a968e8ec3648e2697417750e",
								ID:     "mockImageID1",
							},
							{
								ID:     "mockImageID2",
								Digest: "ca27a44e25ce17fea7b07940ad793",
							},
						},
						StoppedReason: "some reason",
					},
				},
			},
			human: `Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At         14 years ago
  Task Definition    mockTaskDefinition

Task Status

  ID                Image Digest         Last Status         Started At          Stopped At          Capacity Provider    Health Status
  --                ------------         -----------         ----------          ----------          -----------------    -------------
  12345678          69671a96,ca27a44e    RUNNING             -                   -                   -                    HEALTHY

Alarms

  Name              Condition           Last Updated         Health
  ----              ---------           ------------         ------
  mockAlarm         mockCondition       2 months from now    OK
                                                             
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"0001-01-01T00:00:00Z\",\"stoppedReason\":\"some reason\",\"capacityProvider\":\"\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"condition\":\"mockCondition\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}]}\n",
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
