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
	elbv2api "github.com/aws/aws-sdk-go/service/elbv2"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/ecs"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type serviceStatusMocks struct {
	apprunnerSvcDescriber *mocks.MockapprunnerSvcDescriber
	ecsServiceGetter      *mocks.MockecsServiceGetter
	alarmStatusGetter     *mocks.MockalarmStatusGetter
	serviceDescriber      *mocks.MockserviceDescriber
	aas                   *mocks.MockautoscalingAlarmNamesGetter
	logGetter             *mocks.MocklogGetter
	targetHealthGetter    *mocks.MocktargetHealthGetter
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
		wantedContent *ecsServiceStatus
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
		"errors if failed to get stopped task status": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: mockCluster,
						Name:        mockService,
						Tasks: []*awsecs.Task{
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/1234567890123456789"),
							},
						},
						StoppedTasks: []*awsecs.Task{
							{
								TaskArn: aws.String("badMockTaskArn"),
							},
						},
					}, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
				)
			},

			wantedError: fmt.Errorf("get status for stopped task badMockTaskArn: parse ECS task ARN: arn: invalid prefix"),
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
		"do not error out if failed to get a service's target group health": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{
						Deployments: []*ecsapi.Deployment{
							{
								UpdatedAt: aws.Time(startTime),
							},
						},
						LoadBalancers: []*ecsapi.LoadBalancer{
							{
								TargetGroupArn: aws.String("group-1"),
							},
						},
					}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(gomock.Any(), gomock.Any()).Return([]string{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatus(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.targetHealthGetter.EXPECT().TargetsHealth("group-1").Return(nil, errors.New("some error")),
				)
			},
			wantedContent: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					LastDeploymentAt: startTime,
				},
				Alarms: nil,
				Tasks: []awsecs.TaskStatus{
					{
						ID:        "1234567890123456789",
						StartedAt: startTime,
					},
				},
				StoppedTasks:      nil,
				TasksTargetHealth: nil,
			},
		},
		"retrieve all target health information in service": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: mockCluster,
						Name:        mockService,
						Tasks: []*awsecs.Task{
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/task-with-private-ip-being-target"),
								Attachments: []*ecsapi.Attachment{
									{
										Type: aws.String("ElasticNetworkInterface"),
										Details: []*ecsapi.KeyValuePair{
											{
												Name:  aws.String("privateIPv4Address"),
												Value: aws.String("1.2.3.4"),
											},
										},
									},
								},
							},
							{
								TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/task-with-private-ip-not-a-target"),
								Attachments: []*ecsapi.Attachment{
									{
										Type: aws.String("ElasticNetworkInterface"),
										Details: []*ecsapi.KeyValuePair{
											{
												Name:  aws.String("privateIPv4Address"),
												Value: aws.String("5.6.7.8"),
											},
										},
									},
								},
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
						LoadBalancers: []*ecsapi.LoadBalancer{
							{
								TargetGroupArn: aws.String("group-1"),
							},
							{
								TargetGroupArn: aws.String("group-2"),
							},
						},
					}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(map[string]string{
						"copilot-application": "mockApp",
						"copilot-environment": "mockEnv",
						"copilot-service":     "mockSvc",
					}).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatus([]string{}).Return([]cloudwatch.AlarmStatus{}, nil),
					m.targetHealthGetter.EXPECT().TargetsHealth("group-1").Return([]*elbv2.TargetHealth{
						{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("1.2.3.4"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State:  aws.String("unhealthy"),
								Reason: aws.String("Target.ResponseCodeMismatch"),
							},
						},
						{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("4.3.2.1"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State: aws.String("healthy"),
							},
						},
					}, nil),
					m.targetHealthGetter.EXPECT().TargetsHealth("group-2").Return([]*elbv2.TargetHealth{
						{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("1.2.3.4"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State: aws.String("healthy"),
							},
						},
						{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("4.3.2.1"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State: aws.String("healthy"),
							},
						},
					}, nil),
				)
			},

			wantedContent: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount:     1,
					RunningCount:     1,
					Status:           "ACTIVE",
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: nil,
				Tasks: []awsecs.TaskStatus{
					{
						ID: "task-with-private-ip-being-target",
					},
					{
						ID: "task-with-private-ip-not-a-target",
					},
				},
				TasksTargetHealth: []taskTargetHealth{
					{
						TargetHealthDescription: elbv2.TargetHealth{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("1.2.3.4"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State:  aws.String("unhealthy"),
								Reason: aws.String("Target.ResponseCodeMismatch"),
							},
						},
						TaskID:         "task-with-private-ip-being-target",
						TargetGroupARN: "group-1",
					},
					{
						TargetHealthDescription: elbv2.TargetHealth{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("1.2.3.4"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State: aws.String("healthy"),
							},
						},
						TaskID:         "task-with-private-ip-being-target",
						TargetGroupARN: "group-2",
					},
				},
			},
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

			wantedContent: &ecsServiceStatus{
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
			mockTargetHealthGetter := mocks.NewMocktargetHealthGetter(ctrl)
			mocks := serviceStatusMocks{
				ecsServiceGetter:   mockecsSvc,
				alarmStatusGetter:  mockcwSvc,
				serviceDescriber:   mockSvcDescriber,
				aas:                mockaasClient,
				targetHealthGetter: mockTargetHealthGetter,
			}

			tc.setupMocks(mocks)

			svcStatus := &ECSStatusDescriber{
				svc:                "mockSvc",
				env:                "mockEnv",
				app:                "mockApp",
				cwSvcGetter:        mockcwSvc,
				ecsSvcGetter:       mockecsSvc,
				svcDescriber:       mockSvcDescriber,
				aasSvcGetter:       mockaasClient,
				targetHealthGetter: mockTargetHealthGetter,
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
	stoppedTime, _ := time.Parse(time.RFC3339, "2020-03-13T20:00:30+00:00")

	testCases := map[string]struct {
		desc  *ecsServiceStatus
		human string
		json  string
	}{
		"while provisioning": {
			desc: &ecsServiceStatus{
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

  ID                Image Digest        Last Status         Started At          Capacity Provider    Health Status
  --                ------------        -----------         ----------          -----------------    -------------
  12345678          -                   PROVISIONING        -                   -                    HEALTHY

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
			desc: &ecsServiceStatus{
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

  ID                Image Digest         Last Status         Started At          Capacity Provider    Health Status
  --                ------------         -----------         ----------          -----------------    -------------
  12345678          69671a96,ca27a44e    RUNNING             -                   -                    HEALTHY

Alarms

  Name              Condition           Last Updated         Health
  ----              ---------           ------------         ------
  mockAlarm         mockCondition       2 months from now    OK
                                                             
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"0001-01-01T00:00:00Z\",\"stoppedReason\":\"some reason\",\"capacityProvider\":\"\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"condition\":\"mockCondition\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}]}\n",
		},
		"some are tasks deprovisioning": {
			desc: &ecsServiceStatus{
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
				StoppedTasks: []awsecs.TaskStatus{
					{
						LastStatus: "DEPROVISIONING",
						ID:         "0102030490123123123",
						StoppedAt:  stoppedTime,
						Images: []awsecs.Image{
							{
								Digest: "30dkd891jdk9s8d350e932k390093",
								ID:     "mockImageID1",
							},
							{
								ID:     "mockImageID2",
								Digest: "41flf902kfl0d9f461r043l411104",
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

  ID                Image Digest         Last Status         Started At          Capacity Provider    Health Status
  --                ------------         -----------         ----------          -----------------    -------------
  12345678          69671a96,ca27a44e    RUNNING             -                   -                    HEALTHY

Stopped Tasks

  ID                Image Digest         Last Status         Started At          Stopped At           Stopped Reason
  --                ------------         -----------         ----------          ----------           --------------
  01020304          30dkd891,41flf902    DEPROVISIONING      -                   2 months from now    some reason

Alarms

  Name              Condition           Last Updated         Health
  ----              ---------           ------------         ------
  mockAlarm         mockCondition       2 months from now    OK
                                                             
`,
			json: "{\"Service\":{\"desiredCount\":1,\"runningCount\":1,\"status\":\"ACTIVE\",\"lastDeploymentAt\":\"2006-01-02T15:04:05Z\",\"taskDefinition\":\"mockTaskDefinition\"},\"tasks\":[{\"health\":\"HEALTHY\",\"id\":\"1234567890123456789\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"69671a968e8ec3648e2697417750e\"},{\"ID\":\"mockImageID2\",\"Digest\":\"ca27a44e25ce17fea7b07940ad793\"}],\"lastStatus\":\"RUNNING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"0001-01-01T00:00:00Z\",\"stoppedReason\":\"some reason\",\"capacityProvider\":\"\"}],\"alarms\":[{\"arn\":\"mockAlarmArn\",\"name\":\"mockAlarm\",\"condition\":\"mockCondition\",\"status\":\"OK\",\"type\":\"Metric\",\"updatedTimes\":\"2020-03-13T19:50:30Z\"}],\"stoppedTasks\":[{\"health\":\"\",\"id\":\"0102030490123123123\",\"images\":[{\"ID\":\"mockImageID1\",\"Digest\":\"30dkd891jdk9s8d350e932k390093\"},{\"ID\":\"mockImageID2\",\"Digest\":\"41flf902kfl0d9f461r043l411104\"}],\"lastStatus\":\"DEPROVISIONING\",\"startedAt\":\"0001-01-01T00:00:00Z\",\"stoppedAt\":\"2020-03-13T20:00:30Z\",\"stoppedReason\":\"some reason\",\"capacityProvider\":\"\"}]}\n",
		},
		"shows target health such that target healths for the same task are shown in consecutive rows": {
			desc: &ecsServiceStatus{
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
					},
				},
				TasksTargetHealth: []taskTargetHealth{
					{
						TargetHealthDescription: elbv2.TargetHealth{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("5.6.7.8"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State:  aws.String("unhealthy"),
								Reason: aws.String("some reason"),
							},
						},
						TaskID: "a-task-with-private-ip-being-target",
					},
					{
						TargetHealthDescription: elbv2.TargetHealth{
							Target: &elbv2api.TargetDescription{
								Id: aws.String("1.1.1.1"),
							},
							TargetHealth: &elbv2api.TargetHealth{
								State: aws.String("healthy"),
							},
						},
						TaskID: "another-task-with-private-ip-being-target",
					},
				},
			},
			human: `Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At         14 years ago
  Task Definition    mockTaskDefinition

Task Status

  ID                Image Digest         Last Status         Started At          Capacity Provider    Health Status
  --                ------------         -----------         ----------          -----------------    -------------
  12345678          69671a96,ca27a44e    RUNNING             -                   -                    HEALTHY

HTTP Health

  Task              Target              Reason              Health Status
  ----              ------              ------              -------------
  a-task-w          5.6.7.8             some reason         UNHEALTHY
  another-          1.1.1.1             -                   HEALTHY

Alarms

  Name              Condition           Last Updated         Health
  ----              ---------           ------------         ------
  mockAlarm         mockCondition       2 months from now    OK
                                                             
`,
			json: `{"Service":{"desiredCount":1,"runningCount":1,"status":"ACTIVE","lastDeploymentAt":"2006-01-02T15:04:05Z","taskDefinition":"mockTaskDefinition"},"tasks":[{"health":"HEALTHY","id":"1234567890123456789","images":[{"ID":"mockImageID1","Digest":"69671a968e8ec3648e2697417750e"},{"ID":"mockImageID2","Digest":"ca27a44e25ce17fea7b07940ad793"}],"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":""}],"alarms":[{"arn":"mockAlarmArn","name":"mockAlarm","condition":"mockCondition","status":"OK","type":"Metric","updatedTimes":"2020-03-13T19:50:30Z"}],"targetsHealth":[{"healthDescription":{"HealthCheckPort":null,"Target":{"AvailabilityZone":null,"Id":"5.6.7.8","Port":null},"TargetHealth":{"Description":null,"Reason":"some reason","State":"unhealthy"}},"taskID":"a-task-with-private-ip-being-target"},{"healthDescription":{"HealthCheckPort":null,"Target":{"AvailabilityZone":null,"Id":"1.1.1.1","Port":null},"TargetHealth":{"Description":null,"Reason":null,"State":"healthy"}},"taskID":"another-task-with-private-ip-being-target"}]}
`,
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

func TestAppRunnerStatusDescriber_Describe(t *testing.T) {
	appName := "testapp"
	envName := "test"
	svcName := "frontend"
	updateTime := time.Unix(int64(1613145765), 0)
	mockError := errors.New("some error")
	mockAppRunnerService := apprunner.Service{
		ServiceARN:  "arn:aws:apprunner:us-west-2:1234567890:service/testapp-test-frontend/fc1098ac269245959ba78fd58bdd4bf",
		Name:        "testapp-test-frontend",
		ID:          "fc1098ac269245959ba78fd58bdd4bf",
		Status:      "RUNNING",
		DateUpdated: updateTime,
	}
	logEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "events",
			Message:       `[AppRunner] Service creation started.`,
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks serviceStatusMocks)
		desc       *appRunnerServiceStatus

		wantedError   error
		wantedContent *appRunnerServiceStatus
	}{
		"errors if failed to describe a service": {
			setupMocks: func(m serviceStatusMocks) {
				gomock.InOrder(
					m.apprunnerSvcDescriber.EXPECT().Service().Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get AppRunner service description for App Runner service frontend in environment test: some error"),
		},
		"success": {
			setupMocks: func(m serviceStatusMocks) {
				m.apprunnerSvcDescriber.EXPECT().Service().Return(&mockAppRunnerService, nil)
				m.logGetter.EXPECT().LogEvents(cloudwatchlogs.LogEventsOpts{LogGroup: "/aws/apprunner/testapp-test-frontend/fc1098ac269245959ba78fd58bdd4bf/service", Limit: aws.Int64(10)}).Return(&cloudwatchlogs.LogEventsOutput{
					Events: logEvents,
				}, nil)
			},
			wantedContent: &appRunnerServiceStatus{
				Service:   mockAppRunnerService,
				LogEvents: logEvents,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcDesc := mocks.NewMockapprunnerSvcDescriber(ctrl)
			mockLogsSvc := mocks.NewMocklogGetter(ctrl)
			mocks := serviceStatusMocks{
				apprunnerSvcDescriber: mockSvcDesc,
				logGetter:             mockLogsSvc,
			}
			tc.setupMocks(mocks)

			svcStatus := &AppRunnerStatusDescriber{
				app:          appName,
				env:          envName,
				svc:          svcName,
				svcDescriber: mockSvcDesc,
				eventsGetter: mockLogsSvc,
			}

			statusDesc, err := svcStatus.Describe()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, statusDesc, "expected output content match")
			}
		})
	}
}

func TestServiceStatusDesc_AppRunnerServiceString(t *testing.T) {
	oldHumanize := humanizeTime
	humanizeTime = func(then time.Time) string {
		now, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00+00:00")
		return humanize.RelTime(then, now, "from now", "ago")
	}
	defer func() {
		humanizeTime = oldHumanize
	}()

	createTime, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00+00:00")
	updateTime, _ := time.Parse(time.RFC3339, "2020-03-01T00:00:00+00:00")

	logEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "events",
			Message:       `[AppRunner] Service creation started.`,
			Timestamp:     1621365985294,
		},
	}

	testCases := map[string]struct {
		desc  *appRunnerServiceStatus
		human string
		json  string
	}{
		"RUNNING": {
			desc: &appRunnerServiceStatus{
				Service: apprunner.Service{
					Name:        "frontend",
					ID:          "8a2b343f658144d885e47d10adb4845e",
					ServiceARN:  "arn:aws:apprunner:us-east-1:1111:service/frontend/8a2b343f658144d885e47d10adb4845e",
					Status:      "RUNNING",
					DateCreated: createTime,
					DateUpdated: updateTime,
					ImageID:     "hello",
				},
				LogEvents: logEvents,
			},
			human: `Service Status

 Status RUNNING 

Last Deployment

  Updated At        2 months ago
  Service ID        frontend/8a2b343f658144d885e47d10adb4845e
  Source            hello

System Logs

  2021-05-18T19:26:25Z    [AppRunner] Service creation started.
`,
			json: `{"arn":"arn:aws:apprunner:us-east-1:1111:service/frontend/8a2b343f658144d885e47d10adb4845e","status":"RUNNING","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-03-01T00:00:00Z","source":{"imageId":"hello"}}` + "\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			json, err := tc.desc.JSONString()
			require.NoError(t, err)
			print(tc.desc.HumanString())
			require.Equal(t, tc.human, tc.desc.HumanString())
			require.Equal(t, tc.json, json)
		})
	}
}

func TestECSTaskStatus_humanString(t *testing.T) {
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
	mockImageDigest := "18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7"
	testCases := map[string]struct {
		id               string
		health           string
		lastStatus       string
		imageDigest      string
		startedAt        time.Time
		stoppedAt        time.Time
		capacityProvider string
		taskDefinition   string

		inConfigs []ecsTaskStatusConfigOpts

		wantTaskStatus string
	}{
		"show only basic fields": {
			health:           "HEALTHY",
			id:               "aslhfnqo39j8qomimvoiqm89349",
			lastStatus:       "RUNNING",
			startedAt:        startTime,
			stoppedAt:        stopTime,
			imageDigest:      mockImageDigest,
			capacityProvider: "FARGATE",
			taskDefinition:   "arn:aws:ecs:us-east-1:568623488001:task-definition/some-task-def:42",

			wantTaskStatus: "aslhfnqo\tRUNNING\t42\t14 years ago",
		},
		"show all": {
			health:           "HEALTHY",
			id:               "aslhfnqo39j8qomimvoiqm89349",
			lastStatus:       "RUNNING",
			startedAt:        startTime,
			stoppedAt:        stopTime,
			imageDigest:      mockImageDigest,
			capacityProvider: "FARGATE",
			taskDefinition:   "arn:aws:ecs:us-east-1:568623488001:task-definition/some-task-def:42",

			inConfigs: []ecsTaskStatusConfigOpts{
				withCapProviderShown,
				withContainerHealthShow,
			},

			wantTaskStatus: "aslhfnqo\tRUNNING\t42\t14 years ago\tFARGATE\tHEALTHY",
		},
		"show all while having missing params": {
			health:     "HEALTHY",
			lastStatus: "RUNNING",
			inConfigs: []ecsTaskStatusConfigOpts{
				withCapProviderShown,
				withContainerHealthShow,
			},
			wantTaskStatus: "-\tRUNNING\t-\t-\t-\tHEALTHY",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			task := ecsTaskStatus{
				Health: tc.health,
				ID:     tc.id,
				Images: []awsecs.Image{
					{
						Digest: tc.imageDigest,
					},
				},
				LastStatus:       tc.lastStatus,
				StartedAt:        tc.startedAt,
				StoppedAt:        tc.stoppedAt,
				CapacityProvider: tc.capacityProvider,
				TaskDefinition:   tc.taskDefinition,
			}

			gotTaskStatus := task.humanString(tc.inConfigs...)

			require.Equal(t, tc.wantTaskStatus, gotTaskStatus)
		})

	}
}

//func TestECSStoppedTaskStatus_humanString(t *testing.T) {
//	// from the function changes (ex: from "1 month ago" to "2 months ago"). To make our tests stable,
//	oldHumanize := humanizeTime
//	humanizeTime = func(then time.Time) string {
//		now, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00+00:00")
//		return humanize.RelTime(then, now, "ago", "from now")
//	}
//	defer func() {
//		humanizeTime = oldHumanize
//	}()
//	startTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
//	stopTime, _ := time.Parse(time.RFC3339, "2006-01-02T16:04:05+00:00")
//	mockImageDigest := "18f7eb6cff6e63e5f5273fb53f672975fe6044580f66c354f55d2de8dd28aec7"
//	testCases := map[string]struct {
//		id            string
//		health        string
//		lastStatus    string
//		imageDigest   string
//		startedAt     time.Time
//		stoppedAt     time.Time
//		stoppedReason string
//
//		wantTaskStatus string
//	}{
//		"all params": {
//			health:        "HEALTHY",
//			id:            "aslhfnqo39j8qomimvoiqm89349",
//			lastStatus:    "RUNNING",
//			startedAt:     startTime,
//			stoppedAt:     stopTime,
//			imageDigest:   mockImageDigest,
//			stoppedReason: "Stopped by reasons",
//
//			wantTaskStatus: "aslhfnqo\t18f7eb6c\tRUNNING\t14 years ago\t14 years ago\tStopped by reasons",
//		},
//		"missing params": {
//			lastStatus: "RUNNING",
//
//			wantTaskStatus: "-\t-\tRUNNING\t-\t-\t-",
//		},
//	}
//
//	for name, tc := range testCases {
//		t.Run(name, func(t *testing.T) {
//			// GIVEN
//			ctrl := gomock.NewController(t)
//			defer ctrl.Finish()
//
//			task := ecsStoppedTaskStatus{
//				ID: tc.id,
//				Images: []awsecs.Image{
//					{
//						Digest: tc.imageDigest,
//					},
//				},
//				LastStatus:    tc.lastStatus,
//				StartedAt:     tc.startedAt,
//				StoppedAt:     tc.stoppedAt,
//				StoppedReason: tc.stoppedReason,
//			}
//
//			gotTaskStatus := task.humanString()
//
//			require.Equal(t, tc.wantTaskStatus, gotTaskStatus)
//		})
//
//	}
//}

//func TestTargetHealth_humanString(t *testing.T) {
//	testCases := map[string]struct {
//		id     string
//		state  string
//		reason string
//
//		wantedOut string
//	}{
//		"healthy": {
//			id:    "target-1",
//			state: elbv2api.TargetHealthStateEnumHealthy,
//
//			wantedOut: `target-1	-	HEALTHY`,
//		},
//		"unhealthy with reason": {
//			id:     "target-1",
//			state:  elbv2api.TargetHealthStateEnumUnhealthy,
//			reason: elbv2api.TargetHealthReasonEnumTargetResponseCodeMismatch,
//
//			wantedOut: `target-1	Target.ResponseCodeMismatch	UNHEALTHY`,
//		},
//	}
//
//	for name, tc := range testCases {
//		t.Run(name, func(t *testing.T) {
//			th := elbv2TargetHealth{
//				Target: &elbv2api.TargetDescription{
//					Id: aws.String(tc.id),
//				},
//				TargetHealth: &elbv2api.TargetHealth{
//					State:  aws.String(tc.state),
//					Reason: aws.String(tc.reason),
//				},
//			}
//
//			got := th.humanString()
//			require.Equal(t, tc.wantedOut, got)
//		})
//	}
//}

func Test_Temporary(t *testing.T) {
	t.Run("temporary", func(t *testing.T) {
		stoppedTime, _ := time.Parse(time.RFC3339, "2020-03-13T20:00:30+00:00")

		desc := &ecsServiceStatus{
			Service: awsecs.ServiceStatus{
				RunningCount:   3,
				DesiredCount:   4,
				TaskDefinition: "arn:aws:ecs:us-east-1:111122222333:task-definition/some-task-def:6",
			},
			Tasks: []awsecs.TaskStatus{
				{
					Health: "HEALTHY",
					//Health:     "UNKNOWN",
					LastStatus: "RUNNING",
					ID:         "a-task-with-private-ip-being-target",
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
					StoppedReason:    "some reason",
					CapacityProvider: "FARGATE",
					TaskDefinition:   "arn:aws:ecs:us-east-1:111122222333:task-definition/some-task-def:6",
				},
				{
					Health:     "UNKNOWN",
					LastStatus: "RUNNING",
					ID:         "another-task-with-private-ip-being-target",
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
					//CapacityProvider: "FARGATE_SPOT",
					TaskDefinition: "arn:aws:ecs:us-east-1:111122222333:task-definition/some-task-def:1",
				},
				{
					Health:     "UNKNOWN",
					LastStatus: "RUNNING",
					ID:         "aaa",
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
					StoppedReason:    "some reason",
					CapacityProvider: "FARGATE_SPOT",
					TaskDefinition:   "arn:aws:ecs:us-east-1:111122222333:task-definition/some-task-def:1",
				},
			},
			StoppedTasks: []awsecs.TaskStatus{
				{
					LastStatus: "DEPROVISIONING",
					ID:         "0102030490123123123",
					StoppedAt:  stoppedTime,
					Images: []awsecs.Image{
						{
							Digest: "30dkd891jdk9s8d350e932k390093",
							ID:     "mockImageID1",
						},
						{
							ID:     "mockImageID2",
							Digest: "41flf902kfl0d9f461r043l411104",
						},
					},
					StoppedReason: "some reason",
				},
				{
					LastStatus: "DEPROVISIONING",
					ID:         "1234567899",
					StoppedAt:  stoppedTime,
					Images: []awsecs.Image{
						{
							Digest: "30dkd891jdk9s8d350e932k390093",
							ID:     "mockImageID1",
						},
						{
							ID:     "mockImageID2",
							Digest: "41flf902kfl0d9f461r043l411104",
						},
					},
					StoppedReason: "some reason",
				},
				{
					LastStatus: "DEPROVISIONING",
					ID:         "1111111111",
					StoppedAt:  stoppedTime,
					Images: []awsecs.Image{
						{
							Digest: "30dkd891jdk9s8d350e932k390093",
							ID:     "mockImageID1",
						},
						{
							ID:     "mockImageID2",
							Digest: "41flf902kfl0d9f461r043l411104",
						},
					},
					StoppedReason: "some other reason",
				},
			},
			TasksTargetHealth: []taskTargetHealth{
				{
					TargetHealthDescription: elbv2.TargetHealth{
						Target: &elbv2api.TargetDescription{
							Id: aws.String("5.6.7.8"),
						},
						TargetHealth: &elbv2api.TargetHealth{
							State:  aws.String("unhealthy"),
							Reason: aws.String("some reason"),
						},
					},
					TaskID: "a-task-with-private-ip-being-target",
				},
				{
					TargetHealthDescription: elbv2.TargetHealth{
						Target: &elbv2api.TargetDescription{
							Id: aws.String("1.1.1.1"),
						},
						TargetHealth: &elbv2api.TargetHealth{
							State: aws.String("healthy"),
						},
					},
					TaskID: "another-task-with-private-ip-being-target",
				},
				{
					TargetHealthDescription: elbv2.TargetHealth{
						Target: &elbv2api.TargetDescription{
							Id: aws.String("1.1.1.1"),
						},
						TargetHealth: &elbv2api.TargetHealth{
							State: aws.String("unhealthy"),
						},
					},
					TaskID: "another-task-with-private-ip-being-target",
				},
			},
		}

		human := desc.HumanString()
		fmt.Print(human)
	})
}
