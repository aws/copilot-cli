// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package queue provides a generic priority queue.
package queue

import "container/heap"

// Lesser is an interface to enable generic structs to be elements of a priority queue.
type Lesser[T any] interface {
	LessThan(T) bool
}

// PriorityQueue implements a priority queue.
type PriorityQueue[T Lesser[T]] struct {
	pq pq[T]
}

func (p *PriorityQueue[T]) Push(x T) {
	heap.Push(&p.pq, x)
}

func (p *PriorityQueue[T]) Pop() (*T, bool) {
	if p.pq.Len() == 0 {
		return nil, false
	}
	v := heap.Pop(&p.pq).(T)
	return &v, true
}

func (p *PriorityQueue[T]) Len() int {
	return p.pq.Len()
}

type pq[T Lesser[T]] []T

// NewPriorityQueue returns an empty priority queue.
func NewPriorityQueue[T Lesser[T]]() *PriorityQueue[T] {
	var arr pq[T] = make([]T, 0)
	heap.Init(&arr)
	return &PriorityQueue[T]{
		pq: arr,
	}
}

// Len returns the length of the data structure.
func (p *pq[T]) Len() int { return len(*p) }

// Less returns whether element i is less than element j, using the generic type's LessThan function.
func (p *pq[T]) Less(i, j int) bool { return (*p)[i].LessThan((*p)[j]) }

// Swap swaps the positions of two elements in the priority queue.
func (p *pq[T]) Swap(i, j int) {
	(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
}

// Push appends a new element to the back of the underlying array.
func (p *pq[T]) Push(x any) {
	*p = append(*p, x.(T))
}

// Pop removes the last element from the array.
func (p *pq[T]) Pop() any {
	old := *p
	n := len(old)
	res := old[n-1]
	*p = old[:n-1]
	return res
}

// Compile-time check that the PriorityQueue type works on a generic type.
type comparableInt int

func (c comparableInt) LessThan(a comparableInt) bool {
	return c < a
}

var _ heap.Interface = (*pq[comparableInt])(nil)
