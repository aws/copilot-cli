// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	cfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
)

// StackEventsDescriber is the CloudFormation interface needed to describe stack events.
type StackEventsDescriber interface {
	DescribeStackEvents(*cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)
}

// StackEvent is a CloudFormation stack event.
type StackEvent struct {
	LogicalResourceID    string
	PhysicalResourceID   string
	ResourceType         string
	ResourceStatus       string
	ResourceStatusReason string
	Timestamp            time.Time
}

type clock interface {
	now() time.Time
}

type realClock struct{}

func (c realClock) now() time.Time {
	return time.Now()
}

type fakeClock struct{ fakeNow time.Time }

func (c fakeClock) now() time.Time {
	return c.fakeNow
}

// StackStreamer is a Streamer for StackEvent events started by a change set.
type StackStreamer struct {
	client                StackEventsDescriber
	clock                 clock
	rand                  func(int) int
	stackName             string
	changeSetCreationTime time.Time

	subscribers   []chan StackEvent
	done          chan struct{}
	isDone        bool
	pastEventIDs  map[string]bool
	eventsToFlush []StackEvent
	mu            sync.Mutex

	retries int
}

// NewStackStreamer creates a StackStreamer from a cloudformation client, stack name, and the change set creation timestamp.
func NewStackStreamer(cfn StackEventsDescriber, stackName string, csCreationTime time.Time) *StackStreamer {
	return &StackStreamer{
		clock:                 realClock{},
		rand:                  rand.Intn,
		client:                cfn,
		stackName:             stackName,
		changeSetCreationTime: csCreationTime,
		pastEventIDs:          make(map[string]bool),
		done:                  make(chan struct{}),
	}
}

// Subscribe returns a read-only channel that will receive stack events from the StackStreamer.
func (s *StackStreamer) Subscribe() <-chan StackEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := make(chan StackEvent)
	s.subscribers = append(s.subscribers, c)

	if s.isDone {
		// If the streamer is already done streaming, any new subscription requests should just return a closed channel.
		close(c)
	}
	return c
}

// Fetch retrieves and stores any new CloudFormation stack events since the ChangeSetCreationTime in chronological order.
// If an error occurs from describe stack events, returns a wrapped error.
// Otherwise, returns the time the next Fetch should be attempted.
func (s *StackStreamer) Fetch() (next time.Time, err error) {
	var events []StackEvent
	var nextToken *string
	for {
		// DescribeStackEvents returns events in reverse chronological order,
		// so we retrieve new events until we go past the ChangeSetCreationTime or we see an already seen event ID.
		// This logic is taken from the AWS CDK:
		// https://github.com/aws/aws-cdk/blob/43f3f09cc561fd32d651b2c327e877ad81c2ddb2/packages/aws-cdk/lib/api/util/cloudformation/stack-activity-monitor.ts#L230-L234
		out, err := s.client.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			NextToken: nextToken,
			StackName: aws.String(s.stackName),
		})
		if err != nil {
			// Check for throttles and wait to try again using the StackStreamer's interval.
			if request.IsErrorThrottle(err) {
				s.retries += 1
				return nextFetchDate(s.clock, s.rand, s.retries), nil
			}
			return next, fmt.Errorf("describe stack events %s: %w", s.stackName, err)
		}

		s.retries = 0

		var finished bool
		for _, event := range out.StackEvents {
			if event.Timestamp.Before(s.changeSetCreationTime) {
				finished = true
				break
			}
			if _, seen := s.pastEventIDs[aws.StringValue(event.EventId)]; seen {
				finished = true
				break
			}

			logicalID, resourceStatus := aws.StringValue(event.LogicalResourceId), aws.StringValue(event.ResourceStatus)
			if logicalID == s.stackName && !cfn.StackStatus(resourceStatus).InProgress() {
				// The stack is done, notify that there is no need for another Fetch call beyond this point.
				close(s.done)
			}
			events = append(events, StackEvent{
				LogicalResourceID:    logicalID,
				PhysicalResourceID:   aws.StringValue(event.PhysicalResourceId),
				ResourceType:         aws.StringValue(event.ResourceType),
				ResourceStatus:       resourceStatus,
				ResourceStatusReason: aws.StringValue(event.ResourceStatusReason),
				Timestamp:            aws.TimeValue(event.Timestamp),
			})
			s.pastEventIDs[aws.StringValue(event.EventId)] = true
		}
		if finished || out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	// Store events to flush in chronological order.
	reverse(events)
	s.eventsToFlush = append(s.eventsToFlush, events...)
	// Only use exponential backoff if there's an error.
	return nextFetchDate(s.clock, s.rand, 0), nil
}

// Notify flushes all new events to the streamer's subscribers.
func (s *StackStreamer) Notify() {
	// Copy current list of subscribers over, so that we can we add more subscribers while
	// notifying previous subscribers of older events.
	s.mu.Lock()
	var subs []chan StackEvent
	subs = append(subs, s.subscribers...)
	s.mu.Unlock()

	for _, event := range s.eventsToFlush {
		for _, sub := range subs {
			sub <- event
		}
	}
	s.eventsToFlush = nil // reset after flushing all events.
}

// Close closes all subscribed channels notifying them that no more events will be sent.
func (s *StackStreamer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subscribers {
		close(sub)
	}
	s.isDone = true
}

// Done returns a channel that's closed when there are no more events that can be fetched.
func (s *StackStreamer) Done() <-chan struct{} {
	return s.done
}

// Taken from https://github.com/golang/go/wiki/SliceTricks#reversing
func reverse(arr []StackEvent) {
	for i := len(arr)/2 - 1; i >= 0; i-- {
		opp := len(arr) - 1 - i
		arr[i], arr[opp] = arr[opp], arr[i]
	}
}
