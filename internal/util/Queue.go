package util

import (
	"sync/atomic"
)

type entry[T any] struct {
	prev  *entry[T]
	next  *entry[T]
	data  T
	index int32
}

type LinkedQueue[T any] struct {
	head *entry[T]
	tail *entry[T]
	size atomic.Int32
}

func NewQueue[T any]() *LinkedQueue[T] {
	return &LinkedQueue[T]{
		head: nil,
		tail: nil}
}

func (q *LinkedQueue[T]) Push(data T) {
	newEntry := &entry[T]{
		data: data}
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

func (q *LinkedQueue[T]) Pop() *T {
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

func (q *LinkedQueue[T]) GetSize() int32 {
	return q.size.Load()
}
