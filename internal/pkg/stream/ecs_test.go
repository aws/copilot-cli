// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/require"
)

type mockECS struct {
	out       *ecs.Service
	tasks     []*ecs.Task
	err       error
	taskError error
}

type mockCW struct {
	out []cloudwatch.AlarmStatus
	err error
}

func (m mockECS) Service(clusterName, serviceName string) (*ecs.Service, error) {
	return m.out, m.err
}
func (m mockECS) StoppedServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error) {
	return m.tasks, m.taskError
}

func (m mockCW) AlarmStatuses(opts ...cloudwatch.DescribeAlarmOpts) ([]cloudwatch.AlarmStatus, error) {
	return m.out, m.err
}

func TestECSDeploymentStreamer_Subscribe(t *testing.T) {
	t.Run("allow new subscriptions if stack streamer is still active", func(t *testing.T) {
		// GIVEN
		streamer := &ECSDeploymentStreamer{}

		// WHEN
		_ = streamer.Subscribe()
		_ = streamer.Subscribe()

		// THEN
		require.Equal(t, 2, len(streamer.subscribers), "expected number of subscribers to match")
	})
	t.Run("new subscriptions on a finished stack streamer should return closed channels", func(t *testing.T) {
		// GIVEN
		streamer := &ECSDeploymentStreamer{isDone: true}

		// WHEN
		ch := streamer.Subscribe()
		_, ok := <-ch

		// THEN
		require.False(t, ok, "channel should be closed")
	})
}

func TestECSDeploymentStreamer_Fetch(t *testing.T) {
	t.Run("returns a wrapped error on describe service call failure", func(t *testing.T) {
		// GIVEN
		m := mockECS{
			err: errors.New("some error"),
		}
		cw := mockCW{}
		streamer := NewECSDeploymentStreamer(m, cw, "my-cluster", "my-svc", time.Now())

		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.EqualError(t, err, "fetch service description: some error")
	})
	t.Run("returns a wrapped error on alarm statuses call failure", func(t *testing.T) {
		// GIVEN
		m := mockECS{
			out: &ecs.Service{
				DeploymentConfiguration: &awsecs.DeploymentConfiguration{
					Alarms: &awsecs.DeploymentAlarms{
						AlarmNames: []*string{aws.String("alarm1"), aws.String("alarm2")},
						Enable:     aws.Bool(true),
						Rollback:   aws.Bool(true),
					},
				},
			},
		}
		cw := mockCW{
			err: errors.New("some error"),
		}
		streamer := NewECSDeploymentStreamer(m, cw, "my-cluster", "my-svc", time.Now())

		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.EqualError(t, err, "retrieve alarm statuses: some error")
	})
	t.Run("returns a wrapped error on stopped tasks call failure", func(t *testing.T) {
		// GIVEN
		m := mockECS{
			out: &ecs.Service{
				DeploymentConfiguration: &awsecs.DeploymentConfiguration{
					Alarms: &awsecs.DeploymentAlarms{
						AlarmNames: []*string{aws.String("alarm1"), aws.String("alarm2")},
						Enable:     aws.Bool(true),
						Rollback:   aws.Bool(true),
					},
				},
			},
			tasks: []*ecs.Task{
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/testbugbash-testenv-Cluster-qrvEB"),
					DesiredStatus: aws.String("Stopped"),
					LastStatus:    aws.String("Deprovisioning"),
					StoppedReason: aws.String("unable to pull secrets"),
				},
			},
			taskError: errors.New("some error"),
		}
		cw := mockCW{}
		streamer := NewECSDeploymentStreamer(m, cw, "my-cluster", "my-svc", time.Now())

		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.EqualError(t, err, "fetch stopped tasks: some error")
	})
	t.Run("stores events, alarms, and failures until deployment is done", func(t *testing.T) {
		// GIVEN
		oldStartDate := time.Date(2020, time.November, 23, 17, 0, 0, 0, time.UTC)
		startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
		m := mockECS{
			out: &ecs.Service{
				Deployments: []*awsecs.Deployment{
					{
						DesiredCount:   aws.Int64(10),
						FailedTasks:    aws.Int64(0),
						PendingCount:   aws.Int64(0),
						RolloutState:   aws.String("COMPLETED"),
						RunningCount:   aws.Int64(10),
						Status:         aws.String("PRIMARY"),
						TaskDefinition: aws.String("arn:aws:ecs:us-west-2:1111:task-definition/myapp-test-mysvc:2"),
						UpdatedAt:      aws.Time(startDate),
						Id:             aws.String("ecs-svc/123"),
					},
					{
						DesiredCount:   aws.Int64(10),
						FailedTasks:    aws.Int64(10),
						PendingCount:   aws.Int64(0),
						RolloutState:   aws.String("FAILED"),
						RunningCount:   aws.Int64(0),
						Status:         aws.String("ACTIVE"),
						TaskDefinition: aws.String("arn:aws:ecs:us-west-2:1111:task-definition/myapp-test-mysvc:1"),
						UpdatedAt:      aws.Time(oldStartDate),
						Id:             aws.String("ecs-svc/456"),
					},
				},
				DeploymentConfiguration: &awsecs.DeploymentConfiguration{
					Alarms: &awsecs.DeploymentAlarms{
						AlarmNames: []*string{aws.String("alarm1"), aws.String("alarm2")},
						Enable:     aws.Bool(true),
						Rollback:   aws.Bool(true),
					},
				},
				Events: []*awsecs.ServiceEvent{
					{
						CreatedAt: aws.Time(startDate),
						Id:        aws.String("id1"),
						Message:   aws.String("deployment failed: alarm detected"),
					},
					{
						CreatedAt: aws.Time(startDate),
						Id:        aws.String("id2"),
						Message:   aws.String("rolling back to deployment X"),
					},
				},
			},
			tasks: []*ecs.Task{
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEB"),
					DesiredStatus: aws.String("Stopped"),
					LastStatus:    aws.String("Deprovisioning"),
					StoppedReason: aws.String("unable to pull secrets"),
					StoppingAt:    aws.Time(startDate.Add(10 * time.Second)),
					StartedBy:     aws.String("ecs-svc/123"),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBt"),
					DesiredStatus: aws.String("Stopped"),
					LastStatus:    aws.String("Stopped"),
					StoppedReason: aws.String("unable to pull secrets"),
					StoppingAt:    aws.Time(oldStartDate),
					StartedBy:     aws.String("ecs-svc/123"),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBs"),
					DesiredStatus: aws.String("Stopped"),
					LastStatus:    aws.String("Deprovisioning"),
					StoppedReason: aws.String("ELB healthcheck failed"),
					StoppingAt:    aws.Time(startDate.Add(20 * time.Second)),
					StartedBy:     aws.String("ecs-svc/123"),
				},
				{
					TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBu"),
					DesiredStatus: aws.String("Stopped"),
					LastStatus:    aws.String("Deprovisioning"),
					StoppedReason: aws.String("Scaling activity initiated by deployment ecs-svc/mocktaskid"),
					StoppingAt:    aws.Time(startDate.Add(30 * time.Second)),
					StartedBy:     aws.String("ecs-svc/123"),
				},
			},
		}
		cw := mockCW{
			out: []cloudwatch.AlarmStatus{
				{
					Name:   "alarm1",
					Status: "OK",
				},
				{
					Name:   "alarm2",
					Status: "ALARM",
				},
			},
		}
		streamer := NewECSDeploymentStreamer(m, cw, "my-cluster", "my-svc", startDate)

		// WHEN
		_, done, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, []ECSService{
			{
				Deployments: []ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						RunningCount:    10,
						FailedCount:     0,
						PendingCount:    0,
						RolloutState:    "COMPLETED",
						UpdatedAt:       startDate,
						Id:              "ecs-svc/123",
					},
					{
						Status:          "ACTIVE",
						TaskDefRevision: "1",
						DesiredCount:    10,
						RunningCount:    0,
						FailedCount:     10,
						PendingCount:    0,
						RolloutState:    "FAILED",
						UpdatedAt:       oldStartDate,
						Id:              "ecs-svc/456",
					},
				},
				Alarms: []cloudwatch.AlarmStatus{
					{
						Name:   "alarm1",
						Status: "OK",
					},
					{
						Name:   "alarm2",
						Status: "ALARM",
					},
				},
				LatestFailureEvents: []string{"deployment failed: alarm detected", "rolling back to deployment X"},
				StoppedTasks: []ecs.Task{
					{
						TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEBs"),
						DesiredStatus: aws.String("Stopped"),
						LastStatus:    aws.String("Deprovisioning"),
						StoppedReason: aws.String("ELB healthcheck failed"),
						StoppingAt:    aws.Time(startDate.Add(20 * time.Second)),
					},
					{
						TaskArn:       aws.String("arn:aws:ecs:us-east-2:197732814171:task/bugbash-test-Cluster-qrvEB"),
						DesiredStatus: aws.String("Stopped"),
						LastStatus:    aws.String("Deprovisioning"),
						StoppedReason: aws.String("unable to pull secrets"),
						StoppingAt:    aws.Time(startDate.Add(10 * time.Second)),
					},
				},
			},
		}, streamer.eventsToFlush)
		require.True(t, done, "there should be no more work to do since the deployment is completed")
	})
	t.Run("stores events until deployment is done without circuit breaker", func(t *testing.T) {
		// GIVEN
		oldStartDate := time.Date(2020, time.November, 23, 17, 0, 0, 0, time.UTC)
		startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
		m := mockECS{
			out: &ecs.Service{
				Deployments: []*awsecs.Deployment{
					{
						DesiredCount:   aws.Int64(10),
						FailedTasks:    aws.Int64(0),
						PendingCount:   aws.Int64(0),
						RunningCount:   aws.Int64(10),
						Status:         aws.String("PRIMARY"),
						TaskDefinition: aws.String("arn:aws:ecs:us-west-2:1111:task-definition/myapp-test-mysvc:2"),
						UpdatedAt:      aws.Time(startDate),
					},
					{
						DesiredCount:   aws.Int64(10),
						FailedTasks:    aws.Int64(10),
						PendingCount:   aws.Int64(0),
						RunningCount:   aws.Int64(0),
						Status:         aws.String("ACTIVE"),
						TaskDefinition: aws.String("arn:aws:ecs:us-west-2:1111:task-definition/myapp-test-mysvc:1"),
						UpdatedAt:      aws.Time(oldStartDate),
					},
				},
				DeploymentConfiguration: &awsecs.DeploymentConfiguration{
					DeploymentCircuitBreaker: &awsecs.DeploymentCircuitBreaker{
						Enable:   aws.Bool(false),
						Rollback: aws.Bool(true),
					},
				},
			},
		}
		cw := mockCW{}
		streamer := NewECSDeploymentStreamer(m, cw, "my-cluster", "my-svc", startDate)

		// WHEN
		_, done, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, []ECSService{
			{
				Deployments: []ECSDeployment{
					{
						Status:          "PRIMARY",
						TaskDefRevision: "2",
						DesiredCount:    10,
						RunningCount:    10,
						FailedCount:     0,
						PendingCount:    0,
						UpdatedAt:       startDate,
					},
					{
						Status:          "ACTIVE",
						TaskDefRevision: "1",
						DesiredCount:    10,
						RunningCount:    0,
						FailedCount:     10,
						PendingCount:    0,
						UpdatedAt:       oldStartDate,
					},
				},
				LatestFailureEvents: nil,
			},
		}, streamer.eventsToFlush)
		require.True(t, done, "there should be no more work to do since the deployment is completed")
	})
	t.Run("stores only failure event messages", func(t *testing.T) {
		// GIVEN
		startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
		m := mockECS{
			out: &ecs.Service{
				Events: []*awsecs.ServiceEvent{
					{
						// Failure event
						Id:        aws.String("1"),
						Message:   aws.String("(service my-svc) deployment ecs-svc/0205655736282798388 deployment failed: alarm detected."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("2"),
						Message:   aws.String("(service my-svc) rolling back to deployment ecs-svc/9086637243870003494."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("3"),
						Message:   aws.String("(service my-svc) failed to register targets in (target-group 1234) with (error some-error)"),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Success event
						Id:        aws.String("4"),
						Message:   aws.String("(service my-svc) registered 1 targets in (target-group 1234)"),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("5"),
						Message:   aws.String("(service my-svc) failed to launch a task with (error some-error)."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("6"),
						Message:   aws.String("(service my-svc) (task 1234) failed container health checks."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Success event
						Id:        aws.String("7"),
						Message:   aws.String("(service my-svc) has started 1 tasks: (task 1234)."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("8"),
						Message:   aws.String("(service my-svc) (deployment 123) deployment failed: some-error."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("9"),
						Message:   aws.String("(service my-svc) was unable to place a task."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("10"),
						Message:   aws.String("(service my-svc) (port 80) is unhealthy in (target-group 1234) due to (reason some-error)."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
				},
			},
		}
		streamer := &ECSDeploymentStreamer{
			client:                 m,
			clock:                  fakeClock{startDate},
			rand:                   func(n int) int { return n },
			cluster:                "my-cluster",
			service:                "my-svc",
			deploymentCreationTime: startDate,
			pastEventIDs:           make(map[string]bool),
		}
		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, []ECSService{
			{
				LatestFailureEvents: []string{
					"(service my-svc) deployment ecs-svc/0205655736282798388 deployment failed: alarm detected.",
					"(service my-svc) rolling back to deployment ecs-svc/9086637243870003494.",
					"(service my-svc) failed to register targets in (target-group 1234) with (error some-error)",
					"(service my-svc) failed to launch a task with (error some-error).",
					"(service my-svc) (task 1234) failed container health checks.",
					"(service my-svc) (deployment 123) deployment failed: some-error.",
					"(service my-svc) was unable to place a task.",
					"(service my-svc) (port 80) is unhealthy in (target-group 1234) due to (reason some-error).",
				},
			},
		}, streamer.eventsToFlush)
	})
	t.Run("ignores failure events before deployment creation time", func(t *testing.T) {
		// GIVEN
		startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
		m := mockECS{
			out: &ecs.Service{
				Events: []*awsecs.ServiceEvent{
					{
						// Failure event
						Id:        aws.String("1"),
						Message:   aws.String("(service my-svc) failed to register targets in (target-group 1234) with (error some-error)"),
						CreatedAt: aws.Time(time.Date(2020, time.November, 23, 17, 0, 0, 0, time.UTC)),
					},
				},
			},
		}
		streamer := &ECSDeploymentStreamer{
			client:                 m,
			clock:                  fakeClock{startDate},
			rand:                   func(n int) int { return n },
			cluster:                "my-cluster",
			service:                "my-svc",
			deploymentCreationTime: startDate,
			pastEventIDs:           make(map[string]bool),
		}
		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, len(streamer.eventsToFlush), "should have only event to flush")
		require.Nil(t, streamer.eventsToFlush[0].LatestFailureEvents, "there should be no failed events emitted")
	})
	t.Run("ignores events that have already been registered", func(t *testing.T) {
		// GIVEN
		startDate := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)
		m := mockECS{
			out: &ecs.Service{
				Events: []*awsecs.ServiceEvent{
					{
						// Failure event
						Id:        aws.String("1"),
						Message:   aws.String("(service my-svc) failed to register targets in (target-group 1234) with (error some-error)"),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
				},
			},
		}
		streamer := &ECSDeploymentStreamer{
			client:                 m,
			clock:                  fakeClock{startDate},
			rand:                   func(n int) int { return n },
			cluster:                "my-cluster",
			service:                "my-svc",
			deploymentCreationTime: startDate,
			pastEventIDs:           make(map[string]bool),
		}
		streamer.pastEventIDs["1"] = true

		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, len(streamer.eventsToFlush), "should have only one event to flush")
		require.Nil(t, streamer.eventsToFlush[0].LatestFailureEvents, "there should be no failed events emitted")
	})
}

func TestECSDeploymentStreamer_Notify(t *testing.T) {
	// GIVEN
	wantedEvents := []ECSService{
		{
			Deployments: []ECSDeployment{
				{
					Status: "PRIMARY",
				},
				{
					Status: "ACTIVE",
				},
			},
		},
	}
	sub := make(chan ECSService, 2)
	streamer := &ECSDeploymentStreamer{
		subscribers:   []chan ECSService{sub},
		eventsToFlush: wantedEvents,
		clock:         fakeClock{fakeNow: time.Now()},
		rand:          func(n int) int { return n },
	}

	// WHEN
	streamer.Notify()
	close(sub) // Close the channel to stop expecting to receive new events.

	// THEN
	var actualEvents []ECSService
	for event := range sub {
		actualEvents = append(actualEvents, event)
	}
	require.ElementsMatch(t, wantedEvents, actualEvents)
}

func TestECSDeploymentStreamer_Close(t *testing.T) {
	// GIVEN
	streamer := &ECSDeploymentStreamer{}
	c := streamer.Subscribe()

	// WHEN
	streamer.Close()

	// THEN
	_, isOpen := <-c
	require.False(t, isOpen, "expected subscribed channels to be closed")
	require.True(t, streamer.isDone, "should mark the streamer that it won't allow new subscribers")
}
