package util

import (
	"sync"
	"sync/atomic"
)

type entry[T any] struct {
	prev  *entry[T]
	next  *entry[T]
	data  T
	index int32
}

type Queue[T any] struct {
	head *entry[T]
	tail *entry[T]
	size atomic.Int32
	mu   sync.Mutex
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		head: nil,
		tail: nil}
}

func (q *Queue[T]) Push(data T) {
	newEntry := &entry[T]{
		data: data}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.tail != nil {
		q.tail.next = newEntry
		newEntry.prev = q.tail
	} else {
		q.head = newEntry
	}
	q.tail = newEntry
	newEntry.index = q.size.Load()
	q.size.Add(1)
}

func (q *Queue[T]) Pop() *T {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head == nil {
		return nil
	}
	val := q.head.data
	q.head = q.head.next
	if q.head == nil {
		q.tail = nil
	} else {
		q.head.prev = nil
	}
	q.size.Add(-1)
	return &val
}

func (q *Queue[T]) GetSize() int32 {
	return q.size.Load()
}
