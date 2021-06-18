// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/progress"

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

	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type serviceStatusDescriberMocks struct {
	appRunnerSvcDescriber *mocks.MockapprunnerSvcDescriber
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
					m.alarmStatusGetter.EXPECT().AlarmStatus([]string{"mockAlarmName"}).Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get auto scaling CloudWatch alarms: some error"),
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
					m.alarmStatusGetter.EXPECT().AlarmStatus(gomock.Any()).Return([]cloudwatch.AlarmStatus{}, nil),
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
				Alarms: nil,
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						ID:        "1234567890123456789",
						StartedAt: startTime,
					},
				},
				StoppedTasks:             nil,
				TargetHealthDescriptions: nil,
				//rendererConfigurer:       &barRendererConfigurer{},
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
				Alarms: nil,
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
				//rendererConfigurer: &barRendererConfigurer{},
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

	updateTime, _ := time.Parse(time.RFC3339, "2020-03-13T19:50:30+00:00")
	stoppedTime, _ := time.Parse(time.RFC3339, "2020-03-13T20:00:30+00:00")

	testCases := map[string]struct {
		desc                 *ecsServiceStatus
		setUpMockBarRenderer func(length int, data []int, representations []string, emptyRepresentation string) (progress.Renderer, error)
		human                string
		json                 string
	}{
		"while provisioning (some primary, some active)": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 10,
					RunningCount: 3,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							Id:             "active-1",
							DesiredCount:   1,
							RunningCount:   1,
							Status:         "ACTIVE",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5",
						},
						{
							Id:             "active-2",
							DesiredCount:   2,
							RunningCount:   1,
							Status:         "ACTIVE",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4",
						},
						{
							Id:             "id-4",
							DesiredCount:   10,
							RunningCount:   1,
							Status:         "PRIMARY",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
						},
						{
							Id:     "id-5",
							Status: "INACTIVE",
						},
					},
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
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:         "HEALTHY",
						LastStatus:     "RUNNING",
						ID:             "111111111111111",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5",
					},
					{
						Health:         "UNKNOWN",
						LastStatus:     "RUNNING",
						ID:             "111111111111111",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4",
					},
					{
						Health:         "HEALTHY",
						LastStatus:     "PROVISIONING",
						ID:             "1234567890123456789",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
				},
			},
			human: `Task Summary

  Running      ██████████  3/10 desired tasks are running
  Deployments  █░░░░░░░░░  1/10 running tasks for primary (rev 6)
               ██████████  1/1 running tasks for active (rev 5)
               █████░░░░░  1/2 running tasks for active (rev 4)
  Health       █░░░░░░░░░  1/10 passes container health checks (rev 6)

Tasks

  ID        Status        Revision    Started At  Cont. Health
  --        ------        --------    ----------  ------------
  11111111  RUNNING       5           -           HEALTHY
  11111111  RUNNING       4           -           UNKNOWN
  12345678  PROVISIONING  6           -           HEALTHY

Alarms

  Name                            Condition                       Last Updated       Health
  ----                            ---------                       ------------       ------
  mySupercalifragilisticexpialid  RequestCount > 100.00 for 3 da  2 months from now  OK
  ociousAlarm                     tapoints within 25 minutes                         
                                                                                     
  Um-dittle-ittl-um-dittle-I-Ala  CPUUtilization > 70.00 for 3 d  2 months from now  OK
  rm                              atapoints within 3 minutes                         
                                                                                     
`,
			json: `{"Service":{"desiredCount":10,"runningCount":3,"status":"ACTIVE","deployments":[{"id":"active-1","desiredCount":1,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5","status":"ACTIVE"},{"id":"active-2","desiredCount":2,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4","status":"ACTIVE"},{"id":"id-4","desiredCount":10,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"},{"id":"id-5","desiredCount":0,"runningCount":0,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"","status":"INACTIVE"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"HEALTHY","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5"},{"health":"UNKNOWN","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4"},{"health":"HEALTHY","id":"1234567890123456789","images":null,"lastStatus":"PROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"}],"alarms":[{"arn":"mockAlarmArn1","name":"mySupercalifragilisticexpialidociousAlarm","condition":"RequestCount \u003e 100.00 for 3 datapoints within 25 minutes","status":"OK","type":"Metric","updatedTimes":"2020-03-13T19:50:30Z"},{"arn":"mockAlarmArn2","name":"Um-dittle-ittl-um-dittle-I-Alarm","condition":"CPUUtilization \u003e 70.00 for 3 datapoints within 3 minutes","status":"OK","type":"Metric","updatedTimes":"2020-03-13T19:50:30Z"}],"stoppedTasks":null,"targetHealthDescriptions":null}
`,
		},
		"while running with both health check (all primary)": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 3,
					RunningCount: 3,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							Status:         "PRIMARY",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
							DesiredCount:   3,
							RunningCount:   3,
						},
					},
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:         "HEALTHY",
						LastStatus:     "RUNNING",
						ID:             "111111111111111",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
					{
						Health:         "UNHEALTHY",
						LastStatus:     "RUNNING",
						ID:             "2222222222222222",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
					{
						Health:         "HEALTHY",
						LastStatus:     "PROVISIONING",
						ID:             "3333333333333333",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
				},
				TargetHealthDescriptions: []taskTargetHealth{
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:     "1.1.1.1",
							HealthState:  "unhealthy",
							HealthReason: "some reason",
						},
						TaskID:         "111111111111111",
						TargetGroupARN: "group-1",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "2.2.2.2",
							HealthState: "healthy",
						},
						TaskID:         "2222222222222222",
						TargetGroupARN: "group-1",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "3.3.3.3",
							HealthState: "healthy",
						},
						TaskID:         "3333333333333333",
						TargetGroupARN: "group-1",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "4.4.4.4",
							HealthState: "healthy",
						},
						TaskID:         "",
						TargetGroupARN: "group-1",
					},
				},
			},
			human: `Task Summary

  Running   ██████████  3/3 desired tasks are running
  Health    ███████░░░  2/3 passes HTTP health checks
            ███████░░░  2/3 passes container health checks

Tasks

  ID        Status        Revision    Started At  Cont. Health  HTTP Health
  --        ------        --------    ----------  ------------  -----------
  11111111  RUNNING       6           -           HEALTHY       UNHEALTHY
  22222222  RUNNING       6           -           UNHEALTHY     HEALTHY
  33333333  PROVISIONING  6           -           HEALTHY       HEALTHY
`,
			json: `{"Service":{"desiredCount":3,"runningCount":3,"status":"ACTIVE","deployments":[{"id":"","desiredCount":3,"runningCount":3,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"HEALTHY","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"},{"health":"UNHEALTHY","id":"2222222222222222","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"},{"health":"HEALTHY","id":"3333333333333333","images":null,"lastStatus":"PROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"}],"alarms":null,"stoppedTasks":null,"targetHealthDescriptions":[{"healthStatus":{"targetID":"1.1.1.1","description":"","state":"unhealthy","reason":"some reason"},"taskID":"111111111111111","targetGroup":"group-1"},{"healthStatus":{"targetID":"2.2.2.2","description":"","state":"healthy","reason":""},"taskID":"2222222222222222","targetGroup":"group-1"},{"healthStatus":{"targetID":"3.3.3.3","description":"","state":"healthy","reason":""},"taskID":"3333333333333333","targetGroup":"group-1"},{"healthStatus":{"targetID":"4.4.4.4","description":"","state":"healthy","reason":""},"taskID":"","targetGroup":"group-1"}]}
`,
		},
		"while some tasks are stopping": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 5,
					RunningCount: 3,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							Status:         "PRIMARY",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
							DesiredCount:   5,
							RunningCount:   3,
						},
					},
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:         "HEALTHY",
						LastStatus:     "RUNNING",
						ID:             "111111111111111",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
					{
						Health:         "UNHEALTHY",
						LastStatus:     "RUNNING",
						ID:             "2222222222222222",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
					{
						Health:         "HEALTHY",
						LastStatus:     "PROVISIONING",
						ID:             "3333333333333333",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
				},
				StoppedTasks: []awsecs.TaskStatus{
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S111111111111",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S2222222222222",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S333333333333333",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S44444444444",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S55555555555555",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
					{
						LastStatus:    "DEPROVISIONING",
						ID:            "S66666666666666",
						StoppedAt:     stoppedTime,
						Images:        []awsecs.Image{},
						StoppedReason: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
					},
				},
			},
			human: `Task Summary

  Running   ██████░░░░  3/5 desired tasks are running
  Health    ████░░░░░░  2/5 passes container health checks

Stopped Tasks

  Reason                          Task Count  Sample Task IDs
  ------                          ----------  ---------------
  April-is-the-cruellest-month-b  6           S1111111,S2222222,S3333333,S44
  reeding-Lilacs-out-of-the-dead              44444,S5555555
  -land-m                                     

Tasks

  ID        Status        Revision    Started At  Cont. Health
  --        ------        --------    ----------  ------------
  11111111  RUNNING       6           -           HEALTHY
  22222222  RUNNING       6           -           UNHEALTHY
  33333333  PROVISIONING  6           -           HEALTHY
`,
			json: `{"Service":{"desiredCount":5,"runningCount":3,"status":"ACTIVE","deployments":[{"id":"","desiredCount":5,"runningCount":3,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"HEALTHY","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"},{"health":"UNHEALTHY","id":"2222222222222222","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"},{"health":"HEALTHY","id":"3333333333333333","images":null,"lastStatus":"PROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"}],"alarms":null,"stoppedTasks":[{"health":"","id":"S111111111111","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""},{"health":"","id":"S2222222222222","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""},{"health":"","id":"S333333333333333","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""},{"health":"","id":"S44444444444","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""},{"health":"","id":"S55555555555555","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""},{"health":"","id":"S66666666666666","images":[],"lastStatus":"DEPROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"2020-03-13T20:00:30Z","stoppedReason":"April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m","capacityProvider":"","taskDefinitionARN":""}],"targetHealthDescriptions":null}
`,
		},
		"while running without health check": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 3,
					RunningCount: 2,
					Status:       "ACTIVE",
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:     "UNKNOWN",
						LastStatus: "RUNNING",
						ID:         "1111111111111111",
					},
					{
						Health:     "UNKNOWN",
						LastStatus: "RUNNING",
						ID:         "2222222222222222",
					},
				},
			},
			human: `Task Summary

  Running   ███████░░░  2/3 desired tasks are running

Tasks

  ID        Status      Revision    Started At
  --        ------      --------    ----------
  11111111  RUNNING     -           -
  22222222  RUNNING     -           -
`,
			json: `{"Service":{"desiredCount":3,"runningCount":2,"status":"ACTIVE","deployments":null,"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"UNKNOWN","id":"1111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":""},{"health":"UNKNOWN","id":"2222222222222222","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":""}],"alarms":null,"stoppedTasks":null,"targetHealthDescriptions":null}
`,
		},
		"should hide HTTP health from summary if no primary task has HTTP check": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 10,
					RunningCount: 3,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							Id:             "active-1",
							DesiredCount:   1,
							RunningCount:   1,
							Status:         "ACTIVE",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5",
						},
						{
							Id:             "active-2",
							DesiredCount:   2,
							RunningCount:   1,
							Status:         "ACTIVE",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4",
						},
						{
							Id:             "primary",
							DesiredCount:   10,
							RunningCount:   1,
							Status:         "PRIMARY",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
						},
					},
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:         "HEALTHY",
						LastStatus:     "RUNNING",
						ID:             "111111111111111",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5",
					},
					{
						Health:         "UNKNOWN",
						LastStatus:     "RUNNING",
						ID:             "22222222222222",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4",
					},
					{
						Health:         "HEALTHY",
						LastStatus:     "PROVISIONING",
						ID:             "3333333333333",
						TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
					},
				},
				TargetHealthDescriptions: []taskTargetHealth{
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:     "1.1.1.1",
							HealthState:  "unhealthy",
							HealthReason: "some reason",
						},
						TaskID:         "111111111111111",
						TargetGroupARN: "health check for active",
					},
					{
						HealthStatus: elbv2.HealthStatus{
							TargetID:    "2.2.2.2",
							HealthState: "healthy",
						},
						TaskID:         "22222222222222",
						TargetGroupARN: "health check for active",
					},
				},
			},
			human: `Task Summary

  Running      ██████████  3/10 desired tasks are running
  Deployments  █░░░░░░░░░  1/10 running tasks for primary (rev 6)
               ██████████  1/1 running tasks for active (rev 5)
               █████░░░░░  1/2 running tasks for active (rev 4)
  Health       █░░░░░░░░░  1/10 passes container health checks (rev 6)

Tasks

  ID        Status        Revision    Started At  Cont. Health  HTTP Health
  --        ------        --------    ----------  ------------  -----------
  11111111  RUNNING       5           -           HEALTHY       UNHEALTHY
  22222222  RUNNING       4           -           UNKNOWN       HEALTHY
  33333333  PROVISIONING  6           -           HEALTHY       -
`,
			json: `{"Service":{"desiredCount":10,"runningCount":3,"status":"ACTIVE","deployments":[{"id":"active-1","desiredCount":1,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5","status":"ACTIVE"},{"id":"active-2","desiredCount":2,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4","status":"ACTIVE"},{"id":"primary","desiredCount":10,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"HEALTHY","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5"},{"health":"UNKNOWN","id":"22222222222222","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4"},{"health":"HEALTHY","id":"3333333333333","images":null,"lastStatus":"PROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"}],"alarms":null,"stoppedTasks":null,"targetHealthDescriptions":[{"healthStatus":{"targetID":"1.1.1.1","description":"","state":"unhealthy","reason":"some reason"},"taskID":"111111111111111","targetGroup":"health check for active"},{"healthStatus":{"targetID":"2.2.2.2","description":"","state":"healthy","reason":""},"taskID":"22222222222222","targetGroup":"health check for active"}]}
`,
		},
		"while running with capacity providers": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 4,
					RunningCount: 3,
					Status:       "ACTIVE",
				},
				DesiredRunningTasks: []awsecs.TaskStatus{
					{
						Health:           "UNKNOWN",
						LastStatus:       "RUNNING",
						ID:               "11111111111111111",
						Images:           []awsecs.Image{},
						CapacityProvider: "FARGATE_SPOT",
					},
					{
						Health:           "UNKNOWN",
						LastStatus:       "RUNNING",
						ID:               "22222222222222",
						Images:           []awsecs.Image{},
						CapacityProvider: "FARGATE",
					},
					{
						Health:           "UNKNOWN",
						LastStatus:       "RUNNING",
						ID:               "333333333333",
						Images:           []awsecs.Image{},
						CapacityProvider: "",
					},
					{
						Health:           "UNKNOWN",
						LastStatus:       "ACTIVATING",
						ID:               "444444444444",
						Images:           []awsecs.Image{},
						CapacityProvider: "",
					},
				},
			},
			human: `Task Summary

  Running            ████████░░  3/4 desired tasks are running
  Capacity Provider  ▒▒▒▒▒▒▒▓▓▓  2/3 on Fargate, 1/3 on Fargate Spot

Tasks

  ID        Status      Revision    Started At  Capacity
  --        ------      --------    ----------  --------
  11111111  RUNNING     -           -           FARGATE_SPOT
  22222222  RUNNING     -           -           FARGATE
  33333333  RUNNING     -           -           FARGATE (Launch type)
  44444444  ACTIVATING  -           -           FARGATE (Launch type)
`,
			json: `{"Service":{"desiredCount":4,"runningCount":3,"status":"ACTIVE","deployments":null,"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"UNKNOWN","id":"11111111111111111","images":[],"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"FARGATE_SPOT","taskDefinitionARN":""},{"health":"UNKNOWN","id":"22222222222222","images":[],"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"FARGATE","taskDefinitionARN":""},{"health":"UNKNOWN","id":"333333333333","images":[],"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":""},{"health":"UNKNOWN","id":"444444444444","images":[],"lastStatus":"ACTIVATING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":""}],"alarms":null,"stoppedTasks":null,"targetHealthDescriptions":null}
`,
		},
		"hide tasks section if there is no desired running task": {
			desc: &ecsServiceStatus{
				Service: awsecs.ServiceStatus{
					DesiredCount: 0,
					RunningCount: 0,
					Status:       "ACTIVE",
					Deployments: []awsecs.Deployment{
						{
							Id:             "id-4",
							DesiredCount:   0,
							RunningCount:   0,
							Status:         "PRIMARY",
							TaskDefinition: "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6",
						},
					},
				},
				DesiredRunningTasks: []awsecs.TaskStatus{},
			},
			human: `Task Summary

  Running   ░░░░░░░░░░  0/0 desired tasks are running
`,
			json: `{"Service":{"desiredCount":0,"runningCount":0,"status":"ACTIVE","deployments":[{"id":"id-4","desiredCount":0,"runningCount":0,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[],"alarms":null,"stoppedTasks":null,"targetHealthDescriptions":null}
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			json, err := tc.desc.JSONString()
			require.NoError(t, err)
			require.Equal(t, tc.json, json)

			human := tc.desc.HumanString()
			require.Equal(t, tc.human, human)
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

			wantedError: fmt.Errorf("get AppRunner service description for App Runner service frontend in environment test: some error"),
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

			mockSvcDesc := mocks.NewMockapprunnerSvcDescriber(ctrl)
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

Last deployment

  Updated At        2 months ago
  Service ID        frontend/8a2b343f658144d885e47d10adb4845e
  Source            hello

System Logs

  2021-05-18T19:26:25Z  [AppRunner] Service creation started.
`,
			json: `{"arn":"arn:aws:apprunner:us-east-1:1111:service/frontend/8a2b343f658144d885e47d10adb4845e","status":"RUNNING","createdAt":"2020-01-01T00:00:00Z","updatedAt":"2020-03-01T00:00:00Z","source":{"imageId":"hello"}}` + "\n",
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
			taskDefinition:   "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:42",

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
			taskDefinition:   "arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:42",

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
			wantTaskStatus: "-\tRUNNING\t-\t-\tFARGATE (Launch type)\tHEALTHY",
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
