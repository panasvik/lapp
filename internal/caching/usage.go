package caching

import (
	"ImageCacheProject/internal/util"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

var (
	ErrNotFound = errors.New("file not found")
)

const (
	maxVal     = int32(3)
	shardCount = 16
	shardMask  = shardCount - 1
	shardCap   = 1 * 1024 * 1024
	gLimit     = uint32(1e4)
)

type cacheStorage struct {
	shards [shardCount]*shard
	cap    int64
}

func newCacheStorage() *cacheStorage {
	var shards [shardCount]*shard
	for i := range shardCount {
		shards[i] = newShard()
	}
	return &cacheStorage{shards: shards, cap: shardCap * shardCount}
}

type fileNode struct {
	path string
	size int64
	freq atomic.Int32
}

type fileData struct {
	size int64
	path string
}

type shard struct {
	s          *util.FifoMap[fileNode]
	sSize      atomic.Int64
	m          *util.FifoMap[fileNode]
	mSize      atomic.Int64
	g          *util.FifoMap[struct{}]
	gCount     atomic.Uint32
	cap        int64
	size       atomic.Int64
	mu         sync.RWMutex
	isCleaning atomic.Bool
}

func newShard() *shard {
	return &shard{
		s:   util.NewFifoMap[fileNode](),
		m:   util.NewFifoMap[fileNode](),
		g:   util.NewFifoMap[struct{}](),
		cap: shardCap,
	}
}

func (s *shard) get(key string) (*fileNode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.s.Get(key)
	if ok {
		node.freq.Store(min(node.freq.Load()+1, maxVal))
		return node, true
	}

	node, ok = s.m.Get(key)
	if ok {
		node.freq.Store(min(node.freq.Load()+1, maxVal))
		return node, true
	}

	return nil, false
}

func (s *shard) set(data fileData, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, isGhost := s.g.Get(key)
	node := &fileNode{size: data.size, path: data.path}

	if isGhost {
		s.g.PopAt(key)
		s.gCount.Add(^uint32(0)) // -1

		old, replaced := s.m.Push(node, key)
		if replaced {
			s.mSize.Add(-old.size)
			s.size.Add(-old.size)
		}
		s.mSize.Add(data.size)
	} else {
		old, replaced := s.s.Push(node, key)
		if replaced {
			s.sSize.Add(-old.size)
			s.size.Add(-old.size)
		}
		s.sSize.Add(data.size)
	}

	s.size.Add(data.size)
	return s.size.Load() > s.cap
}

func (s *shard) cleanS() {
	target := s.cap / 10
	for s.sSize.Load() > target {
		if !s.cleanSIter() {
			break
		}
	}
}

func (s *shard) cleanSIter() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, key, ok := s.s.Pop()
	if !ok {
		return false
	}
	s.sSize.Add(-node.size)

	if node.freq.Load() > 0 {
		node.freq.Store(0)
		old, replaced := s.m.Push(node, key)
		if replaced {
			s.mSize.Add(-old.size)
			s.size.Add(-old.size)
		}
		s.mSize.Add(node.size)
	} else {
		if s.gCount.Load() >= gLimit {
			s.g.Pop()
			s.gCount.Add(^uint32(0)) // -1
		}
		s.g.Push(&struct{}{}, key)
		s.gCount.Add(1)

		// Уменьшаем размер и удаляем файл
		s.size.Add(-node.size)
		err := os.Remove(node.path)
		if err != nil {
			slog.Error("unable to cleanup file from disk", "path", node.path, "err", err)
		}
	}
	return true
}

func (s *shard) cleanM() {
	target := s.cap * 9 / 10
	for s.mSize.Load() > target {
		if !s.cleanMIter() {
			break
		}
	}
}

func (s *shard) cleanMIter() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, key, ok := s.m.Pop()
	if !ok {
		return false
	}

	if node.freq.Load() > 0 {
		// Второй шанс: возвращаем в хвост M
		node.freq.Add(-1)
		s.m.Push(node, key)
	} else {
		// Окончательное вытеснение из кэша
		s.mSize.Add(-node.size)
		s.size.Add(-node.size)

		err := os.Remove(node.path)
		if err != nil {
			slog.Error("unable to cleanup file from disk", "path", node.path, "err", err)
		}
	}
	return true
}

func (s *shard) runCleanUp() {
	defer s.isCleaning.Store(false)
	s.cleanS()
	s.cleanM()
}

func fnv64(s string) uint64 {
	const offset64 = 14695981039346656037
	const prime64 = 1099511628211

	var h uint64 = offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

func (c *cacheStorage) getShard(key string) *shard {
	idx := fnv64(key) & shardMask
	return c.shards[idx]
}

func (c *cacheStorage) get(key string) (*fileNode, bool) {
	s := c.getShard(key)
	return s.get(key)
}

func (c *cacheStorage) set(data fileData, key string) {
	s := c.getShard(key)
	if needCleanup := s.set(data, key); needCleanup {
		if s.isCleaning.CompareAndSwap(false, true) {
			go s.runCleanUp()
		}
	}
}

func (c *cacheStorage) UseCacheFile(path string, r func(path string)) error {
	_, ok := c.get(path)
	if !ok {
		return ErrNotFound
	}

	r(path)
	return nil
}

func (c *cacheStorage) ReadCacheFile(path string) ([]byte, error) {
	var bytes []byte
	var err error
	r := func(p string) {
		bytes, err = os.ReadFile(p)
	}
	useErr := c.UseCacheFile(path, r)

	return bytes, errors.Join(err, useErr)
}

func (c *cacheStorage) AddCacheFile(path string, size int64) {
	c.set(fileData{path: path, size: size}, path)
}

func (c *cacheStorage) GetSize() int64 {
	var total int64
	for i := 0; i < shardCount; i++ {
		total += c.shards[i].size.Load()
	}
	return total
}

func (c *cacheStorage) GetCap() int64 {
	return c.cap
}
