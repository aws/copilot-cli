package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// counterStreamer counts the number of times Fetch and Notify are invoked.
type counterStreamer struct {
	fetchCount  int
	notifyCount int

	next func() time.Time
}

func (s *counterStreamer) Fetch() (time.Time, error) {
	s.fetchCount += 1
	return s.next(), nil
}

func (s *counterStreamer) Notify() {
	s.notifyCount += 1
}

func (s *counterStreamer) Stop() {}

// errStreamer returns an error when Fetch is invoked.
type errStreamer struct {
	err error
}

func (s *errStreamer) Fetch() (time.Time, error) {
	return time.Now(), s.err
}

func (s *errStreamer) Notify() {}

func (s *errStreamer) Stop() {}

func TestStream(t *testing.T) {
	t.Run("calls Fetch and Notify if context is canceled", func(t *testing.T) {
		// GIVEN
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // call cancel immediately.
		streamer := &counterStreamer{
			next: func() time.Time {
				return time.Now()
			},
		}

		// WHEN
		err := Stream(ctx, streamer)

		// THEN
		require.NoError(t, err)
		require.Equal(t, 1, streamer.fetchCount, "expected number of Fetch calls to match")
		require.Equal(t, 1, streamer.notifyCount, "expected number of Notify calls to match")
	})

	t.Run("returns error from Fetch", func(t *testing.T) {
		// GIVEN
		wantedErr := errors.New("unexpected fetch error")
		streamer := &errStreamer{err: wantedErr}

		// WHEN
		actualErr := Stream(context.Background(), streamer)

		// THEN
		require.EqualError(t, actualErr, wantedErr.Error())
	})

	t.Run("calls Fetch and Notify multiple times until context is canceled", func(t *testing.T) {
		// GIVEN
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		streamer := &counterStreamer{
			next: func() time.Time {
				return time.Now().Add(100 * time.Millisecond)
			},
		}

		// WHEN
		err := Stream(ctx, streamer)

		// THEN
		require.NoError(t, err)
		require.Greater(t, streamer.fetchCount, 1, "expected more than one call to Fetch within a second")
		require.Greater(t, streamer.notifyCount, 1, "expected more than one call to Notify within a second")
	})
}
