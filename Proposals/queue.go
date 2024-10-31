package proposals

import "sync"

// SafeQueue structure
type SafeQueue[T any] struct {
	sync.RWMutex
	items []T
}

// Enqueue adds an element to the end of the queue
func (q *SafeQueue[T]) Enqueue(item T) {
	q.Lock()
	defer q.Unlock()
	q.items = append(q.items, item)
}

// Dequeue removes an element from the front of the queue
func (q *SafeQueue[T]) Dequeue() (T, bool) {
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

// IsEmpty checks if the queue is empty
func (q *SafeQueue[T]) IsEmpty() bool {
	q.RLock()
	defer q.RUnlock()
	return len(q.items) == 0
}

// Peek returns the front element without removing it
func (q *SafeQueue[T]) Peek() (T, bool) {
	q.RLock()
	defer q.RUnlock()
	var zeroVal T
	if len(q.items) == 0 {
		return zeroVal, false
	}
	return q.items[0], true
}

// Size returns the number of elements in the queue
func (q *SafeQueue[T]) Size() int {
	q.RLock()
	defer q.RUnlock()
	return len(q.items)
}
