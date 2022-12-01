// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
)

// StackSetDescriber is the CloudFormation interface needed to describe the health of a stack set operation.
type StackSetDescriber interface {
	InstanceSummaries(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error)
	DescribeOperation(name, opID string) (stackset.Operation, error)
}

// StackSetOpEvent represents a stack set operation status update message.
type StackSetOpEvent struct {
	Name      string // The name of the stack set.
	Operation stackset.Operation
}

// StackSetStreamer is a [Streamer] emitting [StackSetOpEvent] messages for instances under modification.
type StackSetStreamer struct {
	stackset    StackSetDescriber
	ssName      string
	opID        string
	opStartTime time.Time

	subsMu     sync.Mutex
	subs       []chan StackSetOpEvent
	isDone     bool
	lastSentOp stackset.Operation
	curOp      stackset.Operation

	// Overridden in tests.
	clock                     clock
	rand                      func(int) int
	retries                   int
	instanceSummariesInterval time.Duration
}

// NewStackSetStreamer creates a StackSetStreamer for the given stack set name and operation.
func NewStackSetStreamer(cfn StackSetDescriber, ssName, opID string, opStartTime time.Time) *StackSetStreamer {
	return &StackSetStreamer{
		stackset:                  cfn,
		ssName:                    ssName,
		opID:                      opID,
		opStartTime:               opStartTime,
		clock:                     realClock{},
		rand:                      rand.Intn,
		instanceSummariesInterval: 250 * time.Millisecond,
	}
}

// Name returns the CloudFormation stack set's name.
func (s *StackSetStreamer) Name() string {
	return s.ssName
}

// InstanceStreamers initializes Streamers for each stack instance that's in progress part of the stack set.
// As long as the operation is in progress, [InstanceStreamers] will keep
// looking for at least one stack instance that's outdated and return only then.
func (s *StackSetStreamer) InstanceStreamers(cfnClientFor func(region string) StackEventsDescriber) ([]*StackStreamer, error) {
	var streamers []*StackStreamer
	for {
		instances, err := s.stackset.InstanceSummaries(s.ssName)
		if err != nil {
			return nil, fmt.Errorf("describe in progress stack instances for stack set %q: %w", s.ssName, err)
		}

		for _, instance := range instances {
			if !instance.Status.InProgress() || instance.StackID == "" /* new instances won't immediately have an ID */ {
				continue
			}
			streamers = append(streamers, NewStackStreamer(cfnClientFor(instance.Region), instance.StackID, s.opStartTime))
		}
		if len(streamers) > 0 {
			break
		}

		// It's possible that instance statuses aren't updated immediately after a stack set operation is started.
		// If the operation is still ongoing, there must be at least one stack instance that's outdated.
		op, err := s.stackset.DescribeOperation(s.ssName, s.opID)
		if err != nil {
			return nil, fmt.Errorf("describe operation %q for stack set %q: %w", s.opID, s.ssName, err)
		}
		if !op.Status.InProgress() {
			break
		}
		<-time.After(s.instanceSummariesInterval) // Empirically, instances appear within this timeframe.
	}
	return streamers, nil
}

// Subscribe returns a read-only channel to receive stack set operation events.
func (s *StackSetStreamer) Subscribe() <-chan StackSetOpEvent {
	c := make(chan StackSetOpEvent)
	if s.isDone {
		close(c)
		return c
	}

	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	s.subs = append(s.subs, c)
	return c
}

// Fetch retrieves and stores the latest CloudFormation stack set operation.
// If an error occurs from describing stack set operation, returns a wrapped error.
// Otherwise, returns the time the next Fetch should be attempted and whether or not there are more operations to fetch.
func (s *StackSetStreamer) Fetch() (next time.Time, done bool, err error) {
	op, err := s.stackset.DescribeOperation(s.ssName, s.opID)
	if err != nil {
		// Check for throttles and wait to try again using the StackSetStreamer's interval.
		if request.IsErrorThrottle(err) {
			s.retries += 1
			return nextFetchDate(s.clock, s.rand, s.retries), false, nil
		}
		return next, false, fmt.Errorf("describe operation %q for stack set %q: %w", s.opID, s.ssName, err)
	}
	if op.Status.IsCompleted() {
		done = true
	}
	s.retries = 0
	s.curOp = op
	return nextFetchDate(s.clock, s.rand, s.retries), done, nil
}

// Notify publishes the stack set's operation description to subscribers only
// if the content changed from the last time Notify was called.
func (s *StackSetStreamer) Notify() {
	// Copy current list of subscribers over, so that we can we add more subscribers while
	// notifying previous subscribers of operations.
	if s.lastSentOp == s.curOp {
		return
	}

	s.subsMu.Lock()
	subs := make([]chan StackSetOpEvent, len(s.subs))
	copy(subs, s.subs)
	s.subsMu.Unlock()

	for _, sub := range subs {
		sub <- StackSetOpEvent{
			Name:      s.ssName,
			Operation: s.curOp,
		}
	}
	s.lastSentOp = s.curOp
}

// Close closes all subscribed channels notifying them that no more events will be sent
// and causes the streamer to no longer accept any new subscribers.
func (s *StackSetStreamer) Close() {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()

	for _, sub := range s.subs {
		close(sub)
	}
	s.isDone = true
}
