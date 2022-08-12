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

	mu         sync.Mutex
	subs       []chan StackSetOpEvent
	done       chan struct{}
	isDone     bool
	lastSentOp stackset.Operation
	curOp      stackset.Operation

	clock   clock
	rand    func(int) int
	retries int
}

// NewStackSetStreamer creates a StackSetStreamer for the given stack set name and operation.
func NewStackSetStreamer(cfn StackSetDescriber, ssName, opID string, opStartTime time.Time) *StackSetStreamer {
	return &StackSetStreamer{
		stackset:    cfn,
		ssName:      ssName,
		opID:        opID,
		opStartTime: opStartTime,

		done: make(chan struct{}),

		clock: realClock{},
		rand:  rand.Intn,
	}
}

// InstanceStreamers initializes Streamers for each stack instance that's in progress part of the stack set.
func (s *StackSetStreamer) InstanceStreamers(cfnClientFor func(region string) StackEventsDescriber) ([]Streamer, error) {
	instances, err := s.stackset.InstanceSummaries(s.ssName,
		stackset.FilterSummariesByDetailedStatus(stackset.ProgressInstanceStatuses()))
	if err != nil {
		return nil, fmt.Errorf("describe in progress stack instances for stack set %q: %w", s.ssName, err)
	}
	streamers := make([]Streamer, len(instances))
	for i, instance := range instances {
		streamers[i] = NewStackStreamer(cfnClientFor(instance.Region), instance.StackID, s.opStartTime)
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs = append(s.subs, c)
	return c
}

// Fetch retrieves the latest stack set operation and buffers it.
func (s *StackSetStreamer) Fetch() (next time.Time, err error) {
	op, err := s.stackset.DescribeOperation(s.ssName, s.opID)
	if err != nil {
		// Check for throttles and wait to try again using the StackSetStreamer's interval.
		if request.IsErrorThrottle(err) {
			s.retries += 1
			return nextFetchDate(s.clock, s.rand, s.retries), nil
		}
		return next, fmt.Errorf("describe operation %q for stack set %q: %w", s.opID, s.ssName, err)
	}

	if op.Status.IsCompleted() {
		// There are no more stack set events, notify there is no need to Fetch again.
		close(s.done)
	}
	s.retries = 0
	s.curOp = op
	return nextFetchDate(s.clock, s.rand, s.retries), nil
}

// Notify publishes the stack set's operation description to subscribers only
// if the content changed from the last time Notify was called.
func (s *StackSetStreamer) Notify() {
	// Copy current list of subscribers over, so that we can we add more subscribers while
	// notifying previous subscribers of operations.
	if s.lastSentOp == s.curOp {
		return
	}

	s.mu.Lock()
	subs := make([]chan StackSetOpEvent, len(s.subs))
	copy(subs, s.subs)
	s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subs {
		close(sub)
	}
	s.isDone = true
}

// Done returns a channel that's closed when there are no more events that can be fetched.
func (s *StackSetStreamer) Done() <-chan struct{} {
	return s.done
}
