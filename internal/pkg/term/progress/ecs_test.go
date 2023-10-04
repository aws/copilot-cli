// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/stream"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/stretchr/testify/require"
)

func TestRollingUpdateComponent_Listen(t *testing.T) {
	t.Run("should update deployments to latest event and respect max number of failed msgs", func(t *testing.T) {
		// GIVEN
		events := make(chan stream.ECSService)
		done := make(chan struct{})
		c := &rollingUpdateComponent{
			padding:           0,
			maxLenFailureMsgs: 2,
			stream:            events,
			done:              done,
		}

		// WHEN
		go c.Listen()
		go func() {
			events <- stream.ECSService{
				Deployments: []stream.ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						PendingCount:    10,
						RolloutState:    "IN_PROGRESS",
					},
					{
						Status:          "ACTIVE",
						TaskDefRevision: "1",
						DesiredCount:    10,
						RunningCount:    10,
						RolloutState:    "COMPLETED",
					},
				},
				LatestFailureEvents: []string{"event1", "event2", "event3"},
			}
			events <- stream.ECSService{
				Deployments: []stream.ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						RunningCount:    10,
						RolloutState:    "COMPLETED",
					},
				},
				LatestFailureEvents: []string{"event4"},
			}
			close(events)
		}()

		// THEN
		<-done // Listen should have closed the channel.
		require.Equal(t, []stream.ECSDeployment{
			{
				Status:          "PRIMARY",
				TaskDefRevision: "2",
				DesiredCount:    10,
				RunningCount:    10,
				RolloutState:    "COMPLETED",
			},
		}, c.deployments, "expected only the latest deployment to be stored")
		require.Equal(t, []string{"event3", "event4"}, c.failureMsgs, "expected max len failure msgs to be respected")
	})
}

func TestRollingUpdateComponent_Render(t *testing.T) {
	startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
	testCases := map[string]struct {
		inDeployments  []stream.ECSDeployment
		inFailureMsgs  []string
		inAlarms       []cloudwatch.AlarmStatus
		inStoppedTasks []ecs.Task

		wantedNumLines int
		wantedOut      string
	}{
		"should render only deployments if there are no failure messages": {
			inDeployments: []stream.ECSDeployment{
				{
					Status:          "PRIMARY",
					TaskDefRevision: "2",
					DesiredCount:    10,
					RunningCount:    10,
					RolloutState:    "COMPLETED",
				},
			},

			wantedNumLines: 3,
			wantedOut: `Deployments
           Revision  Rollout      Desired  Running  Failed  Pending
  PRIMARY  2         [completed]  10       10       0       0
`,
		},
		"should render a single failure event": {
			inFailureMsgs: []string{"(service my-svc) (task 1234) failed container health checks."},

			wantedNumLines: 3,
			wantedOut: `
✘ Latest failure event
  - (service my-svc) (task 1234) failed container health checks.
`,
		},
		"should split really long failure event messages": {
			inFailureMsgs: []string{
				"(service webapp-test-frontend-Service-ss036XlczgjO) (port 80) is unhealthy in (target-group arn:aws:elasticloadbalancing:us-west-2:1111: targetgroup/aaaaaaaaaaaa) due to (reason some-error).",
			},
			wantedNumLines: 5,
			wantedOut: `
✘ Latest failure event
  - (service webapp-test-frontend-Service-ss036XlczgjO) (port 80) is unhea
    lthy in (target-group arn:aws:elasticloadbalancing:us-west-2:1111: tar
    getgroup/aaaaaaaaaaaa) due to (reason some-error).
`,
		},
		"should render multiple failure messages in reverse order": {
			inFailureMsgs: []string{
				"(service my-svc) (task 1234) failed container health checks.",
				"(service my-svc) (task 5678) failed container health checks.",
			},
			wantedNumLines: 4,
			wantedOut: `
✘ Latest 2 failure events
  - (service my-svc) (task 5678) failed container health checks.
  - (service my-svc) (task 1234) failed container health checks.
`,
		},
		"should render rollback alarms and their statuses": {
			inAlarms: []cloudwatch.AlarmStatus{
				{
					Name:   "alarm1",
					Status: "OK",
				},
				{
					Name:   "alarm2",
					Status: "ALARM",
				},
			},
			wantedNumLines: 5,
			wantedOut: `
Alarms
  Name    State
  alarm1  [OK]
  alarm2  [ALARM]
`,
		},
		"should render stopped tasks and their statuses": {
			inStoppedTasks: []ecs.Task{
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/21479dca3393490a9d95f27353186bf6"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("DEPROVISIONING"),
					StoppedReason: aws.String("ELB healthcheck failed"),
					StoppingAt:    aws.Time(startDate.Add(20 * time.Second)),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/2243bac3ca1d4b3a8c66888348cba2e1"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("STOPPING"),
					StoppedReason: aws.String("unable to pull secrets"),
					StoppingAt:    aws.Time(startDate.Add(10 * time.Second)),
				},
			},
			wantedNumLines: 12,
			wantedOut: fmt.Sprintf(`Latest 2 stopped tasks
  TaskId    CurrentStatus   DesiredStatus
  21479dca  DEPROVISIONING  STOPPED
  2243bac3  STOPPING        STOPPED

✘ Latest 2 tasks stopped reason
  - [21479dca]: ELB healthcheck failed
  - [2243bac3]: unable to pull secrets

Troubleshoot task stopped reason
  1. You can run %s to see the logs of the last stopped task.
  2. You can visit this article: https://repost.aws/knowledge-center/ecs-task-stopped.
`, color.HighlightCode("copilot svc logs --previous")),
		},
		"render collapse taskids if task reasons are same": {
			inStoppedTasks: []ecs.Task{
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/21479dca3393490a9d95f27353186bf6"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("DEPROVISIONING"),
					StoppedReason: aws.String("Essential container in the task exited"),
					StoppingAt:    aws.Time(startDate.Add(20 * time.Second)),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/2243bac3ca1d4b3a8c66888348cba2e1"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("STOPPING"),
					StoppedReason: aws.String("Essential container in the task exited"),
					StoppingAt:    aws.Time(startDate.Add(10 * time.Second)),
				},
			},
			wantedNumLines: 11,
			wantedOut: fmt.Sprintf(`Latest 2 stopped tasks
  TaskId    CurrentStatus   DesiredStatus
  21479dca  DEPROVISIONING  STOPPED
  2243bac3  STOPPING        STOPPED

✘ Latest 2 tasks stopped reason
  - [21479dca,2243bac3]: Essential container in the task exited

Troubleshoot task stopped reason
  1. You can run %s to see the logs of the last stopped task.
  2. You can visit this article: https://repost.aws/knowledge-center/ecs-task-stopped.
`, color.HighlightCode("copilot svc logs --previous")),
		},
		"should render stopped tasks and split long stopped reasons": {
			inStoppedTasks: []ecs.Task{
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/21479dca3393490a9d95f27353186bf6"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("DEPROVISIONING"),
					StoppedReason: aws.String("ELB healthcheck failed"),
					StoppingAt:    aws.Time(startDate.Add(20 * time.Second)),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBaBlImsZ/2243bac3ca1d4b3a8c66888348cba2e1"),
					DesiredStatus: aws.String("STOPPED"),
					LastStatus:    aws.String("STOPPING"),
					StoppedReason: aws.String("ResourceInitializationError: unable to pull secrets or registry auth: execution resource retrieval failed: unable to retrieve secrets from ssm: service call has been retried 1 time(s)"),
					StoppingAt:    aws.Time(startDate.Add(10 * time.Second)),
				},
			},
			wantedNumLines: 14,
			wantedOut: fmt.Sprintf(`Latest 2 stopped tasks
  TaskId    CurrentStatus   DesiredStatus
  21479dca  DEPROVISIONING  STOPPED
  2243bac3  STOPPING        STOPPED

✘ Latest 2 tasks stopped reason
  - [21479dca]: ELB healthcheck failed
  - [2243bac3]: ResourceInitializationError: unable to pull secrets or reg
    istry auth: execution resource retrieval failed: unable to retrieve se
    crets from ssm: service call has been retried 1 time(s)

Troubleshoot task stopped reason
  1. You can run %s to see the logs of the last stopped task.
  2. You can visit this article: https://repost.aws/knowledge-center/ecs-task-stopped.
`, color.HighlightCode("copilot svc logs --previous")),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			buf := new(strings.Builder)
			c := &rollingUpdateComponent{
				deployments:  tc.inDeployments,
				failureMsgs:  tc.inFailureMsgs,
				alarms:       tc.inAlarms,
				stoppedTasks: tc.inStoppedTasks,
			}

			// WHEN
			nl, err := c.Render(buf)

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedNumLines, nl, "number of lines expected did not match")
			require.Equal(t, tc.wantedOut, buf.String(), "the content written did not match")
		})
	}
}
