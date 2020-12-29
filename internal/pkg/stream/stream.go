// Package stream implements streamers that publish AWS events periodically.
// A streamer fetches AWS events periodically and notifies subscribed channels of them.
package stream

import (
	"context"
	"time"
)

// Fetcher is the interface that wraps the Fetch method.
// Fetcher fetches events, updates its internal state with new events and returns the next time
// the Fetch call should be attempted. On failure, Fetch returns an error.
type Fetcher interface {
	Fetch() (next time.Time, err error)
}

// Notifier is the interface that wraps the Notify method.
// Notify publishes all new event updates to subscribers..
type Notifier interface {
	Notify()
}

// Stopper is the interface that wraps the Stop method.
// Stop notifies all subscribers that no more events will be sent.
type Stopper interface {
	Stop()
}

// FetchNotifyStopper is the interface that groups a Fetcher, Notifier, and Stopper.
type FetchNotifyStopper interface {
	Fetcher
	Notifier
	Stopper
}

// Stream streams event updates by calling Fetch followed with Notify until the context is canceled or Fetch errors.
// Once the context is canceled, a best effort Fetch and Notify is called followed with stopping the streamer.
func Stream(ctx context.Context, streamer FetchNotifyStopper) error {
	defer streamer.Stop()

	var next time.Time
	var err error
	for {
		var fetchDelay time.Duration // By default there is no delay.
		if now := time.Now(); next.After(now) {
			fetchDelay = next.Sub(now)
		}

		select {
		case <-ctx.Done():
			// The parent context is canceled. Best-effort publish latest events.
			_, _ = streamer.Fetch()
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
