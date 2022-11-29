// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

type mockStackClient struct {
	out *cloudformation.DescribeStackEventsOutput
	err error
}

func (m mockStackClient) DescribeStackEvents(*cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
	return m.out, m.err
}

func TestStackStreamer_Region(t *testing.T) {
	testCases := map[string]struct {
		stackID string

		wantedRegion string
		wantedOK     bool
	}{
		"should return false when the stack id isn't an ARN": {
			stackID:      "StackSet-demo-infrastructure-7382d3ee-6823-4967-9bcf-8a9118259998",
			wantedRegion: "",
			wantedOK:     false,
		},
		"should return the region when the stack id is an ARN": {
			stackID:      "arn:aws:cloudformation:ap-northeast-1:1111:stack/StackSet-demo-infrastructure-7382d3ee-6823-4967-9bcf-8a9118259998/23f0ecb0-1d7e-11ed-af45-06a7c29c9545",
			wantedRegion: "ap-northeast-1",
			wantedOK:     true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			streamer := NewStackStreamer(nil, tc.stackID, time.Now())

			// WHEN
			region, ok := streamer.Region()

			// THEN
			require.Equal(t, tc.wantedOK, ok)
			require.Equal(t, tc.wantedRegion, region)
		})
	}
}

func TestStackStreamer_Subscribe(t *testing.T) {
	t.Run("allow new subscriptions if stack streamer is still active", func(t *testing.T) {
		// GIVEN
		streamer := &StackStreamer{}

		// WHEN
		_ = streamer.Subscribe()
		_ = streamer.Subscribe()

		// THEN
		require.Equal(t, 2, len(streamer.subscribers), "expected number of subscribers to match")
	})
	t.Run("new subscriptions on a finished stack streamer should return closed channels", func(t *testing.T) {
		// GIVEN
		streamer := &StackStreamer{isDone: true}

		// WHEN
		ch := streamer.Subscribe()
		_, ok := <-ch

		// THEN
		require.False(t, ok, "channel should be closed")
	})
}

func TestStackStreamer_Fetch(t *testing.T) {
	t.Run("stores all events in chronological order on fetch and closes done when the stack is no longer in progress", testStackStreamer_Fetch_Success)
	t.Run("stores only events after the changeset creation time", testStackStreamer_Fetch_PostChangeSet)
	t.Run("stores only events that have not been seen yet", testStackStreamer_Fetch_WithSeenEvents)
	t.Run("returns wrapped error if describe call fails", testStackStreamer_Fetch_WithError)
	t.Run("throttle results in a gracefully handled error and exponential backoff", testStackStreamer_Fetch_withThrottle)
}

func TestStackStreamer_Notify(t *testing.T) {
	// GIVEN
	wantedEvents := []StackEvent{
		{
			LogicalResourceID:  "Cluster",
			PhysicalResourceID: "Cluster1",
			ResourceType:       "AWS::ECS::Cluster",
			ResourceStatus:     "CREATE_COMPLETE",
		},
		{
			LogicalResourceID:  "PublicLoadBalancer",
			PhysicalResourceID: "ELB1",
			ResourceType:       "AWS::ElasticLoadBalancingV2::LoadBalancer",
			ResourceStatus:     "CREATE_COMPLETE",
		},
	}
	sub := make(chan StackEvent, 2)
	streamer := &StackStreamer{
		subscribers:   []chan StackEvent{sub},
		eventsToFlush: wantedEvents,
	}

	// WHEN
	streamer.Notify()
	close(sub) // Close the channel to stop expecting to receive new events.

	// THEN
	var actualEvents []StackEvent
	for event := range sub {
		actualEvents = append(actualEvents, event)
	}
	require.ElementsMatch(t, wantedEvents, actualEvents)
}

func testStackStreamer_Fetch_Success(t *testing.T) {
	// GIVEN
	startTime := time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC)
	client := mockStackClient{
		// Events are in reverse chronological order.
		out: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []*cloudformation.StackEvent{
				{
					EventId:            aws.String("5"),
					LogicalResourceId:  aws.String("phonetool-test"),
					PhysicalResourceId: aws.String("phonetool-test"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(startTime.Add(time.Hour)),
				},
				{
					EventId:              aws.String("4"),
					LogicalResourceId:    aws.String("CloudformationExecutionRole"),
					PhysicalResourceId:   aws.String("CloudformationExecutionRole-123a"),
					ResourceStatus:       aws.String("CREATE_FAILED"),
					ResourceStatusReason: aws.String("phonetool-test-CFNExecutionRole already exists"),
					Timestamp:            aws.Time(startTime.Add(time.Hour + 30*time.Minute)),
				},
				{
					EventId:            aws.String("3"),
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("Cluster-6574"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(startTime.Add(2 * time.Hour)),
				},
				{
					EventId:            aws.String("2"),
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("Cluster-6574"),
					ResourceStatus:     aws.String("CREATE_IN_PROGRESS"),
					Timestamp:          aws.Time(startTime.Add(3 * time.Hour)),
				},
				{
					EventId:            aws.String("1"),
					LogicalResourceId:  aws.String("PublicLoadBalancer"),
					PhysicalResourceId: aws.String("PublicLoadBalancer-2139"),
					ResourceStatus:     aws.String("CREATE_COMPLETE"),
					Timestamp:          aws.Time(startTime.Add(4 * time.Hour)),
				},
			},
		},
	}
	streamer := NewStackStreamer(client, "arn:aws:cloudformation:us-west-2:111111:stack/phonetool-test/b3184400-1429-11ed-a574-0a587ce78f9b", startTime)

	// WHEN
	beforeFetch := time.Now()
	expected, done, err := streamer.Fetch()
	// THEN
	require.NoError(t, err)
	require.False(t, beforeFetch.Add(streamerMinFetchIntervalDurationMs*time.Millisecond).After(expected))
	require.Equal(t, []StackEvent{
		{
			LogicalResourceID:  "PublicLoadBalancer",
			PhysicalResourceID: "PublicLoadBalancer-2139",
			ResourceStatus:     "CREATE_COMPLETE",
			Timestamp:          startTime.Add(4 * time.Hour),
		},
		{
			LogicalResourceID:  "Cluster",
			PhysicalResourceID: "Cluster-6574",
			ResourceStatus:     "CREATE_COMPLETE",
			Timestamp:          startTime.Add(2 * time.Hour),
		},
		{
			LogicalResourceID:    "CloudformationExecutionRole",
			PhysicalResourceID:   "CloudformationExecutionRole-123a",
			ResourceStatus:       "CREATE_FAILED",
			ResourceStatusReason: "phonetool-test-CFNExecutionRole already exists",
			Timestamp:            startTime.Add(time.Hour + 30*time.Minute),
		},
		{
			LogicalResourceID:  "phonetool-test",
			PhysicalResourceID: "phonetool-test",
			ResourceStatus:     "CREATE_COMPLETE",
			Timestamp:          startTime.Add(time.Hour),
		},
	}, streamer.compress(streamer.eventsToFlush), "expected eventsToFlush to appear in chronological order")
	require.True(t, done, "there should be no more work to do since the stack is created")
}

func testStackStreamer_Fetch_PostChangeSet(t *testing.T) {
	// GIVEN
	client := mockStackClient{
		out: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []*cloudformation.StackEvent{
				{
					EventId:           aws.String("abc"),
					LogicalResourceId: aws.String("Cluster"),
					ResourceStatus:    aws.String("CREATE_COMPLETE"),
					Timestamp:         aws.Time(time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)),
				},
			},
		},
	}
	streamer := &StackStreamer{
		client:                client,
		clock:                 fakeClock{fakeNow: time.Now()},
		rand:                  func(n int) int { return n },
		stackID:               "phonetool-test",
		stackName:             "phonetool-test",
		changeSetCreationTime: time.Date(2020, time.November, 23, 19, 0, 0, 0, time.UTC), // An hour after the last event.
	}

	// WHEN
	_, _, err := streamer.Fetch()

	// THEN
	require.NoError(t, err)
	require.Empty(t, streamer.eventsToFlush, "expected eventsToFlush to be empty")
}

func testStackStreamer_Fetch_WithSeenEvents(t *testing.T) {
	// GIVEN
	startTime := time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC)
	client := mockStackClient{

		out: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []*cloudformation.StackEvent{
				{
					EventId:           aws.String("abc"),
					LogicalResourceId: aws.String("Cluster"),
					ResourceStatus:    aws.String("CREATE_COMPLETE"),
					Timestamp:         aws.Time(startTime.Add(2 * time.Hour)),
				},
				{
					EventId:           aws.String("def"),
					LogicalResourceId: aws.String("PublicLoadBalancer"),
					ResourceStatus:    aws.String("CREATE_COMPLETE"),
					Timestamp:         aws.Time(startTime.Add(time.Hour)),
				},
			},
		},
	}
	streamer := &StackStreamer{
		client:                client,
		clock:                 fakeClock{fakeNow: time.Now()},
		rand:                  func(n int) int { return n },
		stackID:               "phonetool-test",
		stackName:             "phonetool-test",
		changeSetCreationTime: startTime,
		pastEventIDs: map[string]bool{
			"def": true,
		},
	}

	// WHEN
	_, _, err := streamer.Fetch()

	// THEN
	require.NoError(t, err)
	require.ElementsMatch(t, []StackEvent{
		{
			LogicalResourceID: "Cluster",
			ResourceStatus:    "CREATE_COMPLETE",
			Timestamp:         startTime.Add(2 * time.Hour),
		},
	}, streamer.eventsToFlush, "expected only the event not seen yet to be flushed")
}

func testStackStreamer_Fetch_WithError(t *testing.T) {
	// GIVEN
	client := mockStackClient{
		err: errors.New("some error"),
	}
	streamer := &StackStreamer{
		client:                client,
		clock:                 fakeClock{fakeNow: time.Now()},
		rand:                  func(n int) int { return n },
		stackID:               "phonetool-test",
		stackName:             "phonetool-test",
		changeSetCreationTime: time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC),
	}

	// WHEN
	_, _, err := streamer.Fetch()

	// THEN
	require.EqualError(t, err, "describe stack events phonetool-test: some error")
}

func testStackStreamer_Fetch_withThrottle(t *testing.T) {
	// GIVEN
	client := &mockStackClient{
		err: awserr.New("RequestThrottled", "throttle err", errors.New("abc")),
	}
	streamer := &StackStreamer{
		client:                *client,
		clock:                 fakeClock{fakeNow: time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC)},
		rand:                  func(n int) int { return n },
		stackID:               "phonetool-test",
		stackName:             "phonetool-test",
		changeSetCreationTime: time.Date(2020, time.November, 23, 16, 0, 0, 0, time.UTC),
		pastEventIDs:          map[string]bool{},
		retries:               0,
	}

	// WHEN
	nextDate, _, err := streamer.Fetch()
	maxDuration := 2 * streamerFetchIntervalDurationMs * time.Millisecond
	require.NoError(t, err, "expect no results and no error for throttle exception")
	require.Equal(t, nextDate, time.Date(2020, time.November, 23, 16, 0, 8, 0, time.UTC), "expect that the returned timeout (%s) is less than the maximum for backoff (%d)", time.Until(nextDate), maxDuration)
	require.Equal(t, 1, streamer.retries)
}

func TestStackStreamer_Close(t *testing.T) {
	// GIVEN
	streamer := &StackStreamer{}
	c := streamer.Subscribe()

	// WHEN
	streamer.Close()

	// THEN
	_, isOpen := <-c
	require.False(t, isOpen, "expected subscribed channels to be closed")
	require.True(t, streamer.isDone, "should mark the streamer that it won't allow new subscribers")
}
