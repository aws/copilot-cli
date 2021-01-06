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

func (sw *stopWatch) start() {
	if sw.stopped {
		sw.reset()
	}

	sw.startTime = sw.clock.now()
	sw.started = true
}

func (sw *stopWatch) stop() {
	sw.stopTime = sw.clock.now()

	if !sw.started {
		sw.start()
		sw.stopTime = sw.startTime
	}
	sw.stopped = true
}

func (sw *stopWatch) reset() {
	sw.startTime = time.Time{}
	sw.stopTime = time.Time{}
	sw.stopped = false
	sw.started = false
}

// elapsed returns the time since starting the stopWatch.
// If the stopWatch never run, then returns false for the boolean.
func (sw *stopWatch) elapsed() (time.Duration, bool) {
	if !sw.started { // The stopWatch didn't start, so no time elapsed.
		return 0, false
	}
	if sw.stopped {
		return sw.stopTime.Sub(sw.startTime), true
	}
	return sw.clock.now().Sub(sw.startTime), true
}
