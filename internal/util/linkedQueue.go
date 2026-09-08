package util

type entry[T any] struct {
	prev *entry[T]
	next *entry[T]
	data T
}

type LinkedQueue[T any] struct {
	head *entry[T]
	tail *entry[T]
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
}

func (q *LinkedQueue[T]) Pop() *T {
	if q.head == nil {
		return nil
	}
	tmp := q.head
	val := tmp.data
	q.head = q.head.next
	if q.head == nil {
		q.tail = nil
	} else {
		q.head.prev = nil
	}
	tmp.next = nil
	tmp.prev = nil
	return &val
}
