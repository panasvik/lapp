package util

type RingQueue[T any] struct {
	buf  []T
	head int
	tail int
	size int
}

func NewRingQueue[T any](capacity int) *RingQueue[T] {
	return &RingQueue[T]{buf: make([]T, capacity)}
}

func (q *RingQueue[T]) Push(val T) {
	if q.size == len(q.buf) {
		newBuf := make([]T, len(q.buf)*2)
		for i := 0; i < q.size; i++ {
			newBuf[i] = q.buf[(q.head+i)%len(q.buf)]
		}
		q.buf = newBuf
		q.head = 0
		q.tail = q.size
	}
	q.buf[q.tail] = val
	q.tail = (q.tail + 1) % len(q.buf)
	q.size++
}

func (q *RingQueue[T]) Pop() *T {
	var zero T
	if q.size == 0 {
		return nil
	}
	val := q.buf[q.head]
	q.buf[q.head] = zero
	q.head = (q.head + 1) % len(q.buf)
	q.size--
	return &val
}
