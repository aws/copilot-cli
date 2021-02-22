// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

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
	done        chan struct{}

	next func() time.Time
}

func (s *counterStreamer) Fetch() (time.Time, error) {
	s.fetchCount += 1
	return s.next(), nil
}

func (s *counterStreamer) Notify() {
	s.notifyCount += 1
}

func (s *counterStreamer) Close() {}

func (s *counterStreamer) Done() <-chan struct{} {
	if s.done == nil {
		s.done = make(chan struct{})
	}
	return s.done
}

// errStreamer returns an error when Fetch is invoked.
type errStreamer struct {
	err  error
	done chan struct{}
}

func (s *errStreamer) Fetch() (time.Time, error) {
	return time.Now(), s.err
}

func (s *errStreamer) Notify() {}

func (s *errStreamer) Close() {}

func (s *errStreamer) Done() <-chan struct{} {
	if s.done == nil {
		s.done = make(chan struct{})
	}
	return s.done
}

func TestStream(t *testing.T) {
	t.Run("short-circuits immediately if context is canceled", func(t *testing.T) {
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
		require.EqualError(t, err, ctx.Err().Error(), "the error returned should be context canceled")
		require.Equal(t, 0, streamer.fetchCount, "expected number of Fetch calls to match")
		require.Equal(t, 0, streamer.notifyCount, "expected number of Notify calls to match")
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
		t.Parallel()
		// GIVEN
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		streamer := &counterStreamer{
			next: func() time.Time {
				return time.Now().Add(100 * time.Millisecond)
			},
		}

		// WHEN
		err := Stream(ctx, streamer)

		// THEN
		require.EqualError(t, err, ctx.Err().Error(), "the error returned should be context canceled")
		require.Greater(t, streamer.fetchCount, 1, "expected more than one call to Fetch within timeout")
		require.Greater(t, streamer.notifyCount, 1, "expected more than one call to Notify within timeout")
	})

	t.Run("calls Fetch and Notify multiple times until there is no more work left", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		streamer := &counterStreamer{
			next: func() time.Time {
				return time.Now().Add(100 * time.Millisecond)
			},
			done: done,
		}
		go func() {
			// Stop the streamer after 1s of work.
			<-time.After(300 * time.Millisecond)
			close(done)
		}()

		// WHEN
		err := Stream(context.Background(), streamer)

		// THEN
		require.NoError(t, err)
		require.Greater(t, streamer.fetchCount, 1, "expected more than one call to Fetch within timeout")
		require.Greater(t, streamer.notifyCount, 1, "expected more than one call to Notify within timeout")
	})

	t.Run("nextFetchDate works correctly to grab times before the timeout.", func(t *testing.T) {
		for r := 0; r < 1000; r++ {
			now := time.Now()
			a := nextFetchDate(now, 0)
			require.True(t, a.Before(now.Add(4*time.Second)), "require that the given date for 0 retries is less than 4s in the future")
			b := nextFetchDate(now, 10)
			require.True(t, b.Before(now.Add(32*time.Second)), "require that the given date for 10 retries is never more than the max interval")
		}

	})
}
