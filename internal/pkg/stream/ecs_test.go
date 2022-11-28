// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/require"
)

type mockECS struct {
	out *ecs.Service
	err error
}

func (m mockECS) Service(clusterName, serviceName string) (*ecs.Service, error) {
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
		streamer := NewECSDeploymentStreamer(m, "my-cluster", "my-svc", time.Now())

		// WHEN
		_, err := streamer.Fetch()

		// THEN
		require.EqualError(t, err, "fetch service description: some error")
	})
	t.Run("stores events until deployment is done", func(t *testing.T) {
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
					},
				},
			},
		}
		streamer := NewECSDeploymentStreamer(m, "my-cluster", "my-svc", startDate)

		// WHEN
		_, err := streamer.Fetch()

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
					},
				},
				LatestFailureEvents: nil,
			},
		}, streamer.eventsToFlush)
		_, isOpen := <-streamer.Done()
		require.False(t, isOpen, "there should be no more work to do since the deployment is completed")
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
			},
		}
		streamer := NewECSDeploymentStreamer(m, "my-cluster", "my-svc", startDate)

		// WHEN
		_, err := streamer.Fetch()

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
		_, isOpen := <-streamer.Done()
		require.False(t, isOpen, "there should be no more work to do since the deployment is completed")
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
						Message:   aws.String("(service my-svc) failed to register targets in (target-group 1234) with (error some-error)"),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Success event
						Id:        aws.String("2"),
						Message:   aws.String("(service my-svc) registered 1 targets in (target-group 1234)"),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("3"),
						Message:   aws.String("(service my-svc) failed to launch a task with (error some-error)."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("4"),
						Message:   aws.String("(service my-svc) (task 1234) failed container health checks."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Success event
						Id:        aws.String("5"),
						Message:   aws.String("(service my-svc) has started 1 tasks: (task 1234)."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("6"),
						Message:   aws.String("(service my-svc) (deployment 123) deployment failed: some-error."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("7"),
						Message:   aws.String("(service my-svc) was unable to place a task."),
						CreatedAt: aws.Time(startDate.Add(1 * time.Minute)),
					},
					{
						// Failure event
						Id:        aws.String("8"),
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
			done:                   make(chan struct{}),
			pastEventIDs:           make(map[string]bool),
		}
		// WHEN
		_, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, []ECSService{
			{
				LatestFailureEvents: []string{
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
			done:                   make(chan struct{}),
			pastEventIDs:           make(map[string]bool),
		}
		// WHEN
		_, err := streamer.Fetch()

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
			done:                   make(chan struct{}),
			pastEventIDs:           make(map[string]bool),
		}
		streamer.pastEventIDs["1"] = true

		// WHEN
		_, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, len(streamer.eventsToFlush), "should have only event to flush")
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
