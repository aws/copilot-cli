// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package queue

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	t.Run("sorts numbers using comparableInt", func(t *testing.T) {
		nums := []comparableInt{1, 6, 2, 10, 1000, -5}
		want := []comparableInt{-5, 1, 2, 6, 10, 1000}
		pq := NewPriorityQueue[comparableInt]()
		for _, n := range nums {
			pq.Push(n)
		}
		require.Equal(t, len(nums), pq.Len())
		out := make([]comparableInt, 0, pq.Len())
		for pq.Len() > 0 {
			v, ok := pq.Pop()
			require.Equal(t, true, ok)
			out = append(out, *v)
		}
		require.Equal(t, want, out)
	})

	t.Run("sorts arbitrary structs by length of member", func(t *testing.T) {
		items := []arbitraryStruct{
			{"aaaaaa"}, {"x"}, {"aaa"},
		}
		want := []arbitraryStruct{
			{"x"}, {"aaa"}, {"aaaaaa"},
		}
		pq := NewPriorityQueue[arbitraryStruct]()
		for _, n := range items {
			pq.Push(n)
		}
		require.Equal(t, len(items), pq.Len())
		out := make([]arbitraryStruct, 0, pq.Len())
		for pq.Len() > 0 {
			v, ok := pq.Pop()
			require.Equal(t, true, ok)
			out = append(out, *v)
		}
		require.Equal(t, want, out)
	})

	t.Run("can't pop from empty priority queue", func(t *testing.T) {
		pq := NewPriorityQueue[arbitraryStruct]()
		v, ok := pq.Pop()
		require.Nil(t, v)
		require.Equal(t, false, ok)
	})
}

type arbitraryStruct struct {
	fieldA string
}

func (a arbitraryStruct) LessThan(b arbitraryStruct) bool { return len(a.fieldA) < len(b.fieldA) }
