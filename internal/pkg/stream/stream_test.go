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

	next func() time.Time
}

func (s *counterStreamer) Fetch() (time.Time, bool, error) {
	s.fetchCount += 1
	return s.next(), false, nil
}

func (s *counterStreamer) Notify() {
	s.notifyCount += 1
}

func (s *counterStreamer) Close() {}

// errStreamer returns an error when Fetch is invoked.
type errStreamer struct {
	err error
}

func (s *errStreamer) Fetch() (time.Time, bool, error) {
	return time.Now(), false, s.err
}

func (s *errStreamer) Notify() {}

func (s *errStreamer) Close() {}

func TestStream(t *testing.T) {
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
	t.Run("nextFetchDate works correctly to grab times before the timeout.", func(t *testing.T) {
		clock := fakeClock{fakeNow: time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC)}
		rand := func(n int) int { return n }
		intervalNS := int(streamerFetchIntervalDurationMs * time.Millisecond)
		for r := 0; r < 4; r++ {
			a := nextFetchDate(clock, rand, r)
			require.Equal(t, a, time.Date(2020, time.November, 1, 0, 0, 0, intervalNS*(1<<r), time.UTC), "require that the given date for 0 retries is less than %dms in the future", streamerFetchIntervalDurationMs*(1<<r))
		}
		maxIntervalNS := int(streamerMaxFetchIntervalDurationMs * time.Millisecond)
		b := nextFetchDate(clock, rand, 10)
		require.Equal(t, b, time.Date(2020, time.November, 1, 0, 0, 0, maxIntervalNS, time.UTC), "require that the given date for 10 retries is never more than the max interval")
	})
}
