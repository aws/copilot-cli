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
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type serviceStatusDescriberMocks struct {
	appRunnerSvcDescriber *mocks.MockapprunnerDescriber
	ecsServiceGetter      *mocks.MockecsServiceGetter
	alarmStatusGetter     *mocks.MockalarmStatusGetter
	serviceDescriber      *mocks.MockserviceDescriber
	aas                   *mocks.MockautoscalingAlarmNamesGetter
	logGetter             *mocks.MocklogGetter
	targetHealthGetter    *mocks.MocktargetHealthGetter
	s3Client              *mocks.MockbucketNameGetter
	bucketDataGetter      *mocks.MockbucketDataGetter
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
		setupMocks func(mocks serviceStatusDescriberMocks)

		wantedError   error
		wantedContent *ecsServiceStatus
	}{
		"errors if failed to describe a service": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get ECS service description for mockSvc: some error"),
		},
		"errors if failed to get the ECS service": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get service mockService: some error"),
		},
		"errors if failed to get running tasks status": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
			setupMocks: func(m serviceStatusDescriberMocks) {
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
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get tagged CloudWatch alarms: some error"),
		},
		"errors if failed to get auto scaling CloudWatch alarm names": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{"mockAlarmName"}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get auto scaling CloudWatch alarms: some error"),
		},
		"errors if failed to get Copilot-created CloudWatch rollback alarm status": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.serviceDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(mockServiceDesc, nil),
					m.ecsServiceGetter.EXPECT().Service(mockCluster, mockService).Return(&awsecs.Service{}, nil),
					m.alarmStatusGetter.EXPECT().AlarmsWithTags(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{"mockAlarmName"}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("get Copilot-created CloudWatch alarms: some error"),
		},
		"do not get status of extraneous alarms": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
					}).Return(nil, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return(nil, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, nil),
				)
			},

			wantedContent: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 1,
					RunningCount: 1,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							UpdatedAt:      startTime,
							TaskDefinition: "mockTaskDefinition",
						},
					},
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:     "HEALTHY",
						LastStatus: "RUNNING",
						ID:         "1234567890123456789",
						Images: []awsecs.Image{
							{
								Digest: "69671a968e8ec3648e2697417750e",
								ID:     "mockImageID1",
							},
						},
						StartedAt:     startTime,
						StoppedAt:     stopTime,
						StoppedReason: "some reason",
					},
				},
			},
		},
		"do not error out if failed to get a service's target group health": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, nil),
					m.targetHealthGetter.EXPECT().TargetsHealth("group-1").Return(nil, errors.New("some error")),
				)
			},
			wantedContent: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					Deployments: []awsecs.Deployment{
						{
							UpdatedAt: startTime,
						},
					},
					LastDeploymentAt: startTime,
				},
				Alarms: []cloudwatch.AlarmStatus{},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						ID:        "1234567890123456789",
						StartedAt: startTime,
					},
				},
				StoppedTasks:             nil,
				TargetHealthDescriptions: nil,
			},
		},
		"retrieve all target health information in service": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(nil, nil),
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
					DesiredCount: 1,
					RunningCount: 1,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							UpdatedAt:      startTime,
							TaskDefinition: "mockTaskDefinition",
						},
					},
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						ID: "task-with-private-ip-being-target",
					},
					{
						ID: "task-with-private-ip-not-a-target",
					},
				},
				TargetHealthDescriptions: []taskTargetHealth{
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "4.3.2.1",
							HealthState: "healthy",
						},
						TaskID:         "",
						TargetGroupARN: "group-1",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:     "1.2.3.4",
							HealthState:  "unhealthy",
							HealthReason: "Target.ResponseCodeMismatch",
						},
						TaskID:         "task-with-private-ip-being-target",
						TargetGroupARN: "group-1",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "4.3.2.1",
							HealthState: "healthy",
						},
						TaskID:         "",
						TargetGroupARN: "group-2",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "1.2.3.4",
							HealthState: "healthy",
						},
						TaskID:         "task-with-private-ip-being-target",
						TargetGroupARN: "group-2",
					},
				},
				//rendererConfigurer: &barRendererConfigurer{},
			},
		},
		"success": {
			setupMocks: func(m serviceStatusDescriberMocks) {
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
							Name:         "AMetricAlarm",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
						{
							Arn:          "mockAlarmArn11",
							Name:         "BMetricAlarm",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
					}, nil),
					m.aas.EXPECT().ECSServiceAlarmNames(mockCluster, mockService).Return([]string{"mockAlarm2"}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return([]cloudwatch.AlarmStatus{
						{
							Arn:          "mockAlarmArn22",
							Name:         "BAutoScalingAlarm",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
						{
							Arn:          "mockAlarmArn2",
							Name:         "AAutoScalingAlarm",
							Condition:    "mockCondition",
							Status:       "OK",
							Type:         "Metric",
							UpdatedTimes: updateTime,
						},
					}, nil),
					m.alarmStatusGetter.EXPECT().AlarmStatuses(gomock.Any()).Return(
						[]cloudwatch.AlarmStatus{
							{
								Arn:          "mockAlarmArn33",
								Name:         "BmockApp-mockEnv-mockSvc-CopilotRollbackAlarm",
								Condition:    "mockCondition",
								Status:       "OK",
								Type:         "Metric",
								UpdatedTimes: updateTime,
							},
							{
								Arn:          "mockAlarmArn3",
								Name:         "AmockApp-mockEnv-mockSvc-CopilotRollbackAlarm",
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
					DesiredCount: 1,
					RunningCount: 1,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							UpdatedAt:      startTime,
							TaskDefinition: "mockTaskDefinition",
						},
					},
					LastDeploymentAt: startTime,
					TaskDefinition:   "mockTaskDefinition",
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Arn:          "mockAlarmArn2",
						Condition:    "mockCondition",
						Name:         "AAutoScalingAlarm",
						Status:       "OK",
						Type:         "Auto Scaling",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn22",
						Name:         "BAutoScalingAlarm",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Auto Scaling",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn1",
						Name:         "AMetricAlarm",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn11",
						Name:         "BMetricAlarm",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Metric",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn3",
						Name:         "AmockApp-mockEnv-mockSvc-CopilotRollbackAlarm",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Rollback",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn33",
						Name:         "BmockApp-mockEnv-mockSvc-CopilotRollbackAlarm",
						Condition:    "mockCondition",
						Status:       "OK",
						Type:         "Rollback",
						UpdatedTimes: updateTime,
					},
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
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
			mocks := serviceStatusDescriberMocks{
				ecsServiceGetter:   mockecsSvc,
				alarmStatusGetter:  mockcwSvc,
				serviceDescriber:   mockSvcDescriber,
				aas:                mockaasClient,
				targetHealthGetter: mockTargetHealthGetter,
			}

			tc.setupMocks(mocks)

			svcStatus := &ecsStatusDescriber{
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
		setupMocks func(mocks serviceStatusDescriberMocks)
		desc       *appRunnerServiceStatus

		wantedError   error
		wantedContent *appRunnerServiceStatus
	}{
		"errors if failed to describe a service": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				gomock.InOrder(
					m.appRunnerSvcDescriber.EXPECT().Service().Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get App Runner service description for App Runner service frontend in environment test: some error"),
		},
		"success": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				m.appRunnerSvcDescriber.EXPECT().Service().Return(&mockAppRunnerService, nil)
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

			mockSvcDesc := mocks.NewMockapprunnerDescriber(ctrl)
			mockLogsSvc := mocks.NewMocklogGetter(ctrl)
			mocks := serviceStatusDescriberMocks{
				appRunnerSvcDescriber: mockSvcDesc,
				logGetter:             mockLogsSvc,
			}
			tc.setupMocks(mocks)

			svcStatus := &appRunnerStatusDescriber{
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

func TestStaticSiteStatusDescriber_Describe(t *testing.T) {
	appName := "testapp"
	envName := "test"
	svcName := "frontend"
	mockBucket := "jimmyBuckets"
	mockError := errors.New("some error")

	testCases := map[string]struct {
		setupMocks func(mocks serviceStatusDescriberMocks)
		desc       *staticSiteServiceStatus

		wantedError   error
		wantedContent *staticSiteServiceStatus
	}{
		"success": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				m.s3Client.EXPECT().BucketName(appName, envName, svcName).Return(mockBucket, nil)
				m.bucketDataGetter.EXPECT().BucketSizeAndCount(mockBucket).Return("mockSize", 123, nil)
			},
			wantedContent: &staticSiteServiceStatus{
				BucketName: mockBucket,
				Size:       "mockSize",
				Count:      123,
			},
		},
		"error getting bucket name": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				m.s3Client.EXPECT().BucketName(appName, envName, svcName).Return("", mockError)
			},
			wantedError: fmt.Errorf(`get bucket name for "frontend" Static Site service in "test" environment: %w`, mockError),
		},
		"error getting bucket size and count": {
			setupMocks: func(m serviceStatusDescriberMocks) {
				m.s3Client.EXPECT().BucketName(appName, envName, svcName).Return(mockBucket, nil)
				m.bucketDataGetter.EXPECT().BucketSizeAndCount(mockBucket).Return("", 0, mockError)
			},
			wantedError: fmt.Errorf(`get size and count data for "jimmyBuckets" S3 bucket: %w`, mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			//mockSvcDesc := mocks.NewMockstaticSiteDescriber(ctrl)
			mocks := serviceStatusDescriberMocks{
				s3Client:         mocks.NewMockbucketNameGetter(ctrl),
				bucketDataGetter: mocks.NewMockbucketDataGetter(ctrl),
			}
			tc.setupMocks(mocks)

			d := &staticSiteStatusDescriber{
				app: appName,
				env: envName,
				svc: svcName,
				initS3Client: func(string) (bucketDataGetter, bucketNameGetter, error) {
					return mocks.bucketDataGetter, mocks.s3Client, nil
				},
			}

			statusDesc, err := d.Describe()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, statusDesc, "expected output content match")
			}
		})
	}
}

func Test_targetHealthForTasks(t *testing.T) {
	testCases := map[string]struct {
		inTargetsHealth  []*elbv2.TargetHealth
		inTasks          []*awsecs.Task
		inTargetGroupARN string

		wanted []taskTargetHealth
	}{
		"include target health in output even if it's not matchable to a task": {
			inTargetsHealth: []*elbv2.TargetHealth{
				{
					Target: &elbv2api.TargetDescription{
						Id: aws.String("42.42.42.42"),
					},
					TargetHealth: &elbv2api.TargetHealth{},
				},
				{
					Target: &elbv2api.TargetDescription{
						Id: aws.String("24.24.24.24"),
					},
					TargetHealth: &elbv2api.TargetHealth{},
				},
			},
			inTasks: []*awsecs.Task{
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
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/task-with-private-ip-being-target"),
					Attachments: []*ecsapi.Attachment{
						{
							Type: aws.String("ElasticNetworkInterface"),
							Details: []*ecsapi.KeyValuePair{
								{
									Name:  aws.String("privateIPv4Address"),
									Value: aws.String("4.3.2.1"),
								},
							},
						},
					},
				},
			},
			inTargetGroupARN: "group-1",
			wanted: []taskTargetHealth{
				{
					HealthStatus: elbv2.HealthStatus{
						TargetID: "42.42.42.42",
					},
					TargetGroupARN: "group-1",
				},
				{
					HealthStatus: elbv2.HealthStatus{
						TargetID: "24.24.24.24",
					},
					TargetGroupARN: "group-1",
				},
			},
		},
		"target health should be matched to a task if applicable": {
			inTargetsHealth: []*elbv2.TargetHealth{
				{
					Target: &elbv2api.TargetDescription{
						Id: aws.String("42.42.42.42"),
					},
					TargetHealth: &elbv2api.TargetHealth{
						Description: aws.String("unhealthy because this and that"),
						State:       aws.String("unhealthy"),
						Reason:      aws.String("Target.Timeout"),
					},
				},
				{
					Target: &elbv2api.TargetDescription{
						Id: aws.String("24.24.24.24"),
					},
					TargetHealth: &elbv2api.TargetHealth{
						State: aws.String("healthy"),
					},
				},
			},
			inTasks: []*awsecs.Task{
				{
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/task-with-private-ip-being-target"),
					Attachments: []*ecsapi.Attachment{
						{
							Type: aws.String("ElasticNetworkInterface"),
							Details: []*ecsapi.KeyValuePair{
								{
									Name:  aws.String("privateIPv4Address"),
									Value: aws.String("42.42.42.42"),
								},
							},
						},
					},
				},
				{
					TaskArn: aws.String("arn:aws:ecs:us-west-2:123456789012:task/mockCluster/not-target"),
					Attachments: []*ecsapi.Attachment{
						{
							Type: aws.String("ElasticNetworkInterface"),
							Details: []*ecsapi.KeyValuePair{
								{
									Name:  aws.String("privateIPv4Address"),
									Value: aws.String("4.3.2.1"),
								},
							},
						},
					},
				},
			},
			inTargetGroupARN: "group-1",
			wanted: []taskTargetHealth{
				{
					TaskID: "task-with-private-ip-being-target",
					HealthStatus: elbv2.HealthStatus{
						TargetID:          "42.42.42.42",
						HealthReason:      "Target.Timeout",
						HealthState:       "unhealthy",
						HealthDescription: "unhealthy because this and that",
					},
					TargetGroupARN: "group-1",
				},
				{
					TaskID: "",
					HealthStatus: elbv2.HealthStatus{
						TargetID:    "24.24.24.24",
						HealthState: "healthy",
					},
					TargetGroupARN: "group-1",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := targetHealthForTasks(tc.inTargetsHealth, tc.inTasks, tc.inTargetGroupARN)
			require.Equal(t, tc.wanted, got)
		})
	}
}
