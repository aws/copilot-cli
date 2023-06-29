// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"

	"github.com/dustin/go-humanize"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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
						Type:         "Auto Scaling",
						UpdatedTimes: updateTime,
					},
					{
						Arn:          "mockAlarmArn2",
						Name:         "Um-dittle-ittl-um-dittle-I-Alarm",
						Condition:    "CPUUtilization > 70.00 for 3 datapoints within 3 minutes",
						Status:       "OK",
						Type:         "Rollback",
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

  Name                            Type          Condition                       Last Updated       Health
  ----                            ----          ---------                       ------------       ------
  mySupercalifragilisticexpialid  Auto Scaling  RequestCount > 100.00 for 3 da  2 months from now  OK
  ociousAlarm                                   tapoints within 25 minutes                         
                                                                                                   
  Um-dittle-ittl-um-dittle-I-Ala  Rollback      CPUUtilization > 70.00 for 3 d  2 months from now  OK
  rm                                            atapoints within 3 minutes                         
                                                                                                   
`,
			json: `{"Service":{"desiredCount":10,"runningCount":3,"status":"ACTIVE","deployments":[{"id":"active-1","desiredCount":1,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5","status":"ACTIVE"},{"id":"active-2","desiredCount":2,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4","status":"ACTIVE"},{"id":"id-4","desiredCount":10,"runningCount":1,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6","status":"PRIMARY"},{"id":"id-5","desiredCount":0,"runningCount":0,"updatedAt":"0001-01-01T00:00:00Z","launchType":"","taskDefinition":"","status":"INACTIVE"}],"lastDeploymentAt":"0001-01-01T00:00:00Z","taskDefinition":""},"tasks":[{"health":"HEALTHY","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:5"},{"health":"UNKNOWN","id":"111111111111111","images":null,"lastStatus":"RUNNING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:4"},{"health":"HEALTHY","id":"1234567890123456789","images":null,"lastStatus":"PROVISIONING","startedAt":"0001-01-01T00:00:00Z","stoppedAt":"0001-01-01T00:00:00Z","stoppedReason":"","capacityProvider":"","taskDefinitionARN":"arn:aws:ecs:us-east-1:000000000000:task-definition/some-task-def:6"}],"alarms":[{"arn":"mockAlarmArn1","name":"mySupercalifragilisticexpialidociousAlarm","condition":"RequestCount \u003e 100.00 for 3 datapoints within 25 minutes","status":"OK","type":"Auto Scaling","updatedTimes":"2020-03-13T19:50:30Z"},{"arn":"mockAlarmArn2","name":"Um-dittle-ittl-um-dittle-I-Alarm","condition":"CPUUtilization \u003e 70.00 for 3 datapoints within 3 minutes","status":"OK","type":"Rollback","updatedTimes":"2020-03-13T19:50:30Z"}],"stoppedTasks":null,"targetHealthDescriptions":null}
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

  Updated At  2 months ago
  Service ID  frontend/8a2b343f658144d885e47d10adb4845e
  Source      hello

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

func TestServiceStatusDesc_StaticSiteServiceString(t *testing.T) {
	testCases := map[string]struct {
		desc  *staticSiteServiceStatus
		human string
		json  string
	}{
		"success": {
			desc: &staticSiteServiceStatus{
				BucketName: "Jimmy Buckets",
				Size:       "999 MB",
				Count:      22,
			},
			human: `Bucket Summary

  Bucket Name     Jimmy Buckets
  Total Objects   22
  Total Size      999 MB
`,
			json: `{"bucketName":"Jimmy Buckets","totalSize":"999 MB","totalObjects":22}` + "\n",
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
