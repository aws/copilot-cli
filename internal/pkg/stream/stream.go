// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package stream implements streamers that publish AWS events periodically.
// A streamer fetches AWS events periodically and notifies subscribed channels of them.
package stream

import (
	"context"
	"time"
)

const (
	streamerFetchIntervalDuration = 3 * time.Second // How long to wait until Fetch is called again for a Streamer.
)

// Streamer is the interface that groups methods to periodically retrieve events,
// publish them to subscribers, and stop publishing once there are no more events left.
type Streamer interface {
	// Fetcher fetches events, updates the internal state of the Streamer with new events and returns the next time
	// the Fetch call should be attempted. On failure, Fetch returns an error.
	Fetch() (next time.Time, err error)

	// Notify publishes all new event updates to subscribers.
	Notify()

	// Close notifies all subscribers that no more events will be sent.
	Close()

	// Done returns a channel that's closed when there are no more events to Fetch.
	Done() <-chan struct{}
}

// Stream streams event updates by calling Fetch followed with Notify until there are no more events left.
// If the context is canceled or Fetch errors, then Stream short-circuits and returns the error.
func Stream(ctx context.Context, streamer Streamer) error {
	defer streamer.Close()

	var next time.Time
	var err error
	for {
		var fetchDelay time.Duration // By default there is no delay.
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-streamer.Done():
			// No more events to Fetch, flush and exit successfully.
			streamer.Notify()
			return nil
		case <-time.After(fetchDelay):
			next, err = streamer.Fetch()
			if err != nil {
				return err
			}
			streamer.Notify()
		}
	}
}
