// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
)

// mockStackSetClient implements the StackSetDescriber interface.
type mockStackSetClient struct {
	instanceSummariesFn func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error)
	describeOpFn        func(name, opID string) (stackset.Operation, error)
}

func (m mockStackSetClient) InstanceSummaries(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
	return m.instanceSummariesFn(name, opts...)
}

func (m mockStackSetClient) DescribeOperation(name, opID string) (stackset.Operation, error) {
	return m.describeOpFn(name, opID)
}

func TestStackSetStreamer_InstanceStreamers(t *testing.T) {
	t.Run("should return a wrapped error when instance summaries cannot be found", func(t *testing.T) {
		// GIVEN
		mockStackSet := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				return nil, errors.New("some error")
			},
		}
		mockStackLocator := func(_ string) StackEventsDescriber {
			return mockStackClient{}
		}
		streamer := NewStackSetStreamer(mockStackSet, "demo-infrastructure", "1", time.Now())

		// WHEN
		_, err := streamer.InstanceStreamers(mockStackLocator)

		// THEN
		require.EqualError(t, err, `describe in progress stack instances for stack set "demo-infrastructure": some error`)
	})
	t.Run("should return immediately if there are stack instances in progress", func(t *testing.T) {
		// GIVEN
		mockStackSet := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				return []stackset.InstanceSummary{
					{
						StackID: "1111",
						Region:  "us-west-2",
						Status:  "RUNNING",
					},
					{
						StackID: "2222",
						Region:  "us-east-1",
						Status:  "RUNNING",
					},
				}, nil
			},
		}
		regionalStreamers := make(map[string]int)
		mockStackLocator := func(region string) StackEventsDescriber {
			regionalStreamers[region] += 1
			return mockStackClient{}
		}
		streamer := NewStackSetStreamer(mockStackSet, "demo-infrastructure", "1", time.Now())

		// WHEN
		children, err := streamer.InstanceStreamers(mockStackLocator)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 2, len(regionalStreamers), "expected a separate streamer for each region")
		require.Equal(t, 2, len(children), "expected as many streamers as instance summaries")
	})
	t.Run("should return a wrapped error when describing the operation fails", func(t *testing.T) {
		// GIVEN
		mockStackSet := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				return []stackset.InstanceSummary{
					{
						StackID: "1111",
						Region:  "us-west-2",
						Status:  "SUCCEEDED",
					},
				}, nil
			},
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{}, errors.New("some error")
			},
		}
		regionalStreamers := make(map[string]int)
		mockStackLocator := func(region string) StackEventsDescriber {
			regionalStreamers[region] += 1
			return mockStackClient{}
		}
		streamer := NewStackSetStreamer(mockStackSet, "demo-infrastructure", "1", time.Now())

		// WHEN
		_, err := streamer.InstanceStreamers(mockStackLocator)

		// THEN
		require.EqualError(t, err, `describe operation "1" for stack set "demo-infrastructure": some error`)
	})
	t.Run("should keep calling InstanceSummary until in progress instances are found", func(t *testing.T) {
		// GIVEN
		var callCount int
		mockStackSet := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				defer func() { callCount += 1 }()
				if callCount == 0 {
					return []stackset.InstanceSummary{
						{
							StackID: "1111",
							Region:  "us-west-2",
							Status:  "SUCCEEDED",
						},
					}, nil
				}
				return []stackset.InstanceSummary{
					{
						StackID: "1111",
						Region:  "us-west-2",
						Status:  "SUCCEEDED",
					},
					{
						StackID: "2222",
						Region:  "us-east-1",
						Status:  "RUNNING",
					},
				}, nil
			},
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{
					Status: "RUNNING",
				}, nil
			},
		}
		regionalStreamers := make(map[string]int)
		mockStackLocator := func(region string) StackEventsDescriber {
			regionalStreamers[region] += 1
			return mockStackClient{}
		}
		streamer := NewStackSetStreamer(mockStackSet, "demo-infrastructure", "1", time.Now())
		streamer.instanceSummariesInterval = 0 // override time to wait interval.

		// WHEN
		children, err := streamer.InstanceStreamers(mockStackLocator)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, len(regionalStreamers), "expected a separate streamer for each region")
		require.Equal(t, 1, len(children), "expected as many streamers as instance summaries")
	})
}

func TestStackSetStreamer_Subscribe(t *testing.T) {
	t.Run("subscribing to a closed streamer should return a closed channel", func(t *testing.T) {
		// GIVEN
		client := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				return nil, nil
			},
		}
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", time.Now())
		streamer.Close()

		// WHEN
		ch := streamer.Subscribe()

		// THEN
		_, more := <-ch
		require.False(t, more, "there should not be any more messages to send in the channel")
	})
}

func TestStackSetStreamer_Close(t *testing.T) {
	t.Run("should close all subscribed channels", func(t *testing.T) {
		// GIVEN
		client := mockStackSetClient{
			instanceSummariesFn: func(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
				return nil, nil
			},
		}
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", time.Now())
		first := streamer.Subscribe()
		second := streamer.Subscribe()

		// WHEN
		streamer.Close()

		// THEN
		_, more := <-first
		require.False(t, more, "there should not be any more messages to send in the first channel")
		_, more = <-second
		require.False(t, more, "there should not be any more messages to send in the second channel")
	})
}

func TestStackSetStreamer_Fetch(t *testing.T) {
	t.Run("Fetch should return a later timestamp if a throttling error occurs", func(t *testing.T) {
		// GIVEN
		client := &mockStackSetClient{
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{}, awserr.New("RequestThrottled", "throttle err", errors.New("abc"))
			},
		}
		startTime := time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC)
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", startTime)
		streamer.clock = fakeClock{fakeNow: startTime}
		streamer.rand = func(n int) int { return n }
		wantedTime := startTime.Add(2 * streamerFetchIntervalDurationMs * time.Millisecond)

		// WHEN
		next, _, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, wantedTime, next)
	})

	t.Run("Fetch should return an error when the operation cannot be described", func(t *testing.T) {
		// GIVEN
		client := &mockStackSetClient{
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{}, errors.New("some error")
			},
		}
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", time.Now())

		// WHEN
		_, _, err := streamer.Fetch()

		// THEN
		require.EqualError(t, err, `describe operation "1" for stack set "demo-infrastructure": some error`)
	})

	t.Run("Fetch should return the next immediate date on success", func(t *testing.T) {
		client := &mockStackSetClient{
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{Status: "hello"}, nil
			},
		}
		startTime := time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC)
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", startTime)
		streamer.clock = fakeClock{fakeNow: startTime}
		streamer.rand = func(n int) int { return n }

		// WHEN
		next, _, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.Equal(t, startTime.Add(streamerFetchIntervalDurationMs*time.Millisecond), next)
	})
}

func TestStackSetStreamer_Integration(t *testing.T) {
	t.Run("Done if Fetch retrieved a final status", func(t *testing.T) {
		// GIVEN
		client := &mockStackSetClient{
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				return stackset.Operation{
					ID:     "1",
					Status: "SUCCEEDED",
				}, nil
			},
		}
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", time.Now())

		// WHEN
		_, done, err := streamer.Fetch()

		// THEN
		require.NoError(t, err)
		require.True(t, done)
	})
	t.Run("should only broadcast unique operations to subscribers", func(t *testing.T) {
		// GIVEN
		var callCount int
		responses := [5]stackset.Operation{
			{ID: "1", Status: "QUEUED"},
			{ID: "1", Status: "QUEUED"},
			{ID: "1", Status: "RUNNING"},
			{ID: "1", Status: "STOPPING"},
			{ID: "1", Status: "STOPPED", Reason: "manually stopped"},
		}
		wanted := [4]StackSetOpEvent{
			{Name: "demo-infrastructure", Operation: stackset.Operation{ID: "1", Status: "QUEUED"}},
			{Name: "demo-infrastructure", Operation: stackset.Operation{ID: "1", Status: "RUNNING"}},
			{Name: "demo-infrastructure", Operation: stackset.Operation{ID: "1", Status: "STOPPING"}},
			{Name: "demo-infrastructure", Operation: stackset.Operation{ID: "1", Status: "STOPPED", Reason: "manually stopped"}},
		}
		client := &mockStackSetClient{
			describeOpFn: func(_, _ string) (stackset.Operation, error) {
				defer func() { callCount += 1 }()
				return responses[callCount], nil
			},
		}
		streamer := NewStackSetStreamer(client, "demo-infrastructure", "1", time.Now())
		sub := streamer.Subscribe()

		// WHEN
		go func() {
			for i := 0; i < 5; i += 1 {
				_, _, err := streamer.Fetch()
				require.NoError(t, err, "fetch %d should succeed", i)
				streamer.Notify()
			}
			streamer.Close()
		}()
		done := make(chan struct{})
		var actual []StackSetOpEvent
		go func() {
			for msg := range sub {
				actual = append(actual, msg)
			}
			close(done)
		}()

		// THEN
		<-done
		require.ElementsMatch(t, wanted, actual)
	})
}
