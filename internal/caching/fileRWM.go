package caching

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	ErrImgNotFound = errors.New("img not found")
)

type fileState struct {
	readers  int  // количество текущих горутин-читателей
	toDelete bool // флаг, что файл заказан на удаление
}

// CacheTable TODO: implement states into writeBehind, cacheCleaner, workers DONE
type CacheTable struct {
	mu            sync.Mutex
	items         map[string]fileState
	diskSemaphore chan struct{}
}

func NewTable() *CacheTable {
	return &CacheTable{
		items:         make(map[string]fileState),
		diskSemaphore: make(chan struct{}, 10),
	}
}

func (c *CacheTable) AddFileState(path string, numReaders int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fs := fileState{numReaders, false}
	c.items[path] = fs
}

func (c *CacheTable) CacheLockFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, exists := c.items[path]

	if !exists || state.toDelete {
		return errors.New("file not found or being deleted")
	}
	c.diskSemaphore <- struct{}{}
	state.readers++
	c.items[path] = state
	return nil
}

func (c *CacheTable) CacheUnlockFile(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.items[path]
	st.readers--
	c.items[path] = st

	needDelete := st.toDelete && st.readers == 0

	defer func() {
		if needDelete {
			os.Remove(path)
		}
	}()
	defer func() { <-c.diskSemaphore }()
}

func (c *CacheTable) CleanUpFile(path string) error {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	c.mu.Lock()
	defer c.mu.Unlock()
	state, exists := c.items[path]

	if !exists || state.toDelete {
		return nil
	}

	if state.readers == 0 {
		delete(c.items, path)
		return os.Remove(path)
	}

	state.toDelete = true
	c.items[path] = state

	return nil
}

func (c *CacheTable) ReadCacheFile(path string) ([]byte, error) {
	state, exists := c.items[path]
	if !exists {
		return nil, fmt.Errorf("cache table: %v", ErrImgNotFound)
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	state.readers++
	defer func() { c.items[path] = state }()
	defer func() { state.readers-- }()
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	return os.ReadFile(path)
}

// ReadFile for NOT cache files, for cache files use ReadCacheFile
func (c *CacheTable) ReadFile(path string) ([]byte, error) {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	return os.ReadFile(path)
}

func (c *CacheTable) WriteFile(path string, data []byte) error {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	return os.WriteFile(path, data, 0644)
}

func (c *CacheTable) Remove(path string) error {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	return os.Remove(path)
}

func (c *CacheTable) Exists(path string) bool {
	_, exists := c.items[path]
	return exists
}
