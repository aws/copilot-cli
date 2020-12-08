// Package stream implements streamers that publish AWS events periodically.
// A streamer fetches AWS events periodically and notifies subscribed channels of them.
package stream

import (
	"context"
	"time"
)

// Fetcher fetches events, updates its internal state with new events and returns the next time
// the Fetch call should be attempted. On failure, Fetch returns an error.
type Fetcher interface {
	Fetch() (next time.Time, err error)
}

// Notifier notifies all of its subscribers of any new event updates.
type Notifier interface {
	Notify()
}

// FetchNotifier is the interface that groups the Fetch and a Notify methods.
type FetchNotifier interface {
	Fetcher
	Notifier
}

// Stream streams event updates by calling Fetch followed with Notify until the context is canceled or Fetch errors.
// Once the context is canceled, a best effort Fetch and Notify is called one last time.
func Stream(ctx context.Context, fn FetchNotifier) error {
	var next time.Time
	var err error
	for {
		var fetchDelay time.Duration // By default there is no delay.
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}

		select {
		case <-ctx.Done():
			// The parent context is canceled. Try Fetch and Notify one last time and exit successfully.
			fn.Fetch()
			fn.Notify()
			return nil
		case <-time.After(fetchDelay):
			next, err = fn.Fetch()
			if err != nil {
				return err
			}
			fn.Notify()
		}
	}
}
