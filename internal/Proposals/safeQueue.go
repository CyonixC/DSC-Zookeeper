package proposals

// This file contains related definitions for the SafeQueue structure, a thread-safe queue used to
// handle the queueing of recorded proposals.

import "sync"

// SafeQueue structure
type SafeQueue[T any] struct {
	sync.RWMutex
	items []T
}

func (q *SafeQueue[T]) elements() []T {
	q.RLock()
	defer q.RUnlock()
	return q.items
}

// enqueue adds an element to the end of the queue
func (q *SafeQueue[T]) enqueue(item T) {
	q.Lock()
	defer q.Unlock()
	q.items = append(q.items, item)
}

// dequeue removes an element from the front of the queue
func (q *SafeQueue[T]) dequeue() (T, bool) {
	q.Lock()
	defer q.Unlock()
	var zeroVal T
	if len(q.items) == 0 {
		return zeroVal, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// isEmpty checks if the queue is empty
func (q *SafeQueue[T]) isEmpty() bool {
	q.RLock()
	defer q.RUnlock()
	return len(q.items) == 0
}

// peek returns the front element without removing it
func (q *SafeQueue[T]) peek() (T, bool) {
	q.RLock()
	defer q.RUnlock()
	var zeroVal T
	if len(q.items) == 0 {
		return zeroVal, false
	}
	return q.items[0], true
}

// size returns the number of elements in the queue
func (q *SafeQueue[T]) size() int {
	q.RLock()
	defer q.RUnlock()
	return len(q.items)
}
