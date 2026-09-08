package util

type FifoMap[T any] struct {
	nodeMap map[string]*FifoNode[T]
	head    *FifoNode[T]
	tail    *FifoNode[T]
}

func NewFifoMap[T any]() *FifoMap[T] {
	return &FifoMap[T]{
		nodeMap: make(map[string]*FifoNode[T]),
		head:    nil,
		tail:    nil,
	}
}

type FifoNode[T any] struct {
	key  string
	val  *T
	prev *FifoNode[T]
	next *FifoNode[T]
}

func newNode[T any](val *T, key string) *FifoNode[T] {
	return &FifoNode[T]{key: key, val: val}
}

func (f *FifoMap[T]) Push(val *T, key string) (*T, bool) {
	oldVal, replaced := f.PopAt(key)

	node := newNode(val, key)
	if f.tail != nil {
		f.tail.next = node
		node.prev = f.tail
	} else {
		f.head = node
	}
	f.tail = node
	f.nodeMap[key] = node

	return oldVal, replaced
}

func (f *FifoMap[T]) PopAt(key string) (*T, bool) {
	node, ok := f.nodeMap[key]
	if !ok {
		return nil, false
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		f.tail = node.prev
	}
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		f.head = node.next
	}
	node.prev = nil
	node.next = nil
	val := node.val
	node.val = nil
	delete(f.nodeMap, key)
	return val, true
}

func (f *FifoMap[T]) Pop() (*T, string, bool) {
	if f.head == nil {
		return nil, "", false
	}
	tmp := f.head
	val := tmp.val
	f.head = f.head.next
	if f.head == nil {
		f.tail = nil
	} else {
		f.head.prev = nil
	}
	tmp.next = nil
	delete(f.nodeMap, tmp.key)
	return val, tmp.key, true
}

func (f *FifoMap[T]) Peek() (*T, bool) {
	if f.head == nil {
		return nil, false
	}
	return f.head.val, true
}

func (f *FifoMap[T]) Get(key string) (*T, bool) {
	node, ok := f.nodeMap[key]
	if !ok {
		return nil, false
	}
	return node.val, true
}
