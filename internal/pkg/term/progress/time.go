// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import "time"

type clock interface {
	now() time.Time
}

type realClock struct{}

func (c realClock) now() time.Time {
	return time.Now()
}

type stopWatch struct {
	startTime time.Time
	stopTime  time.Time

	started bool
	stopped bool
	clock   clock
}

func newStopWatch() *stopWatch {
	return &stopWatch{
		clock: realClock{},
	}
}

// start records the current time when the watch started.
// If the watch is already in progress, then calling start multiple times does nothing.
func (sw *stopWatch) start() {
	if sw.started {
		return
	}
	sw.startTime = sw.clock.now()
	sw.started = true
}

// stop records the current time when the watch stopped.
// If the watch never started, then calling stop doesn't do anything.
// If the watch is already stopped, then calling stop another time doesn't do anything.
func (sw *stopWatch) stop() {
	if !sw.started {
		return
	}
	if sw.stopped {
		return
	}
	sw.stopTime = sw.clock.now()
	sw.stopped = true
}

// reset should be called before starting a timer always.
func (sw *stopWatch) reset() {
	sw.startTime = time.Time{}
	sw.stopTime = time.Time{}
	sw.stopped = false
	sw.started = false
}

// elapsed returns the time since starting the stopWatch.
// If the stopWatch never started, then returns false for the boolean.
func (sw *stopWatch) elapsed() (time.Duration, bool) {
	if !sw.started { // The stopWatch didn't start, so no time elapsed.
		return 0, false
	}
	if sw.stopped {
		return sw.stopTime.Sub(sw.startTime), true
	}
	return sw.clock.now().Sub(sw.startTime), true
}
