package caching

import (
	"errors"
	"os"
	"sync"
)

type FileState struct {
	readers  int  // количество текущих горутин-читателей
	toDelete bool // флаг, что файл заказан на удаление
}

// CacheTable TODO: implement states into writeBehind, cacheCleaner, workers DONE
type CacheTable struct {
	mu            sync.Mutex
	items         map[string]FileState
	diskSemaphore chan struct{}
}

func NewTable() *CacheTable {
	return &CacheTable{
		items:         make(map[string]FileState),
		diskSemaphore: make(chan struct{}, 10),
	}
}

func (c *CacheTable) AddReader(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, exists := c.items[path]
	if !exists {
		state = c.AddFileState(path, 0)
	}
	state.readers++
	c.items[path] = state
}

func (c *CacheTable) AddFileState(path string, numReaders int) FileState {
	fs := FileState{numReaders, false}
	c.items[path] = fs
	return fs
}

func (c *CacheTable) CacheLockFile(path string) error {
	c.mu.Lock()
	state, exists := c.items[path]

	if !exists || state.toDelete {
		c.mu.Unlock()
		return errors.New("file not found or being deleted")
	}
	c.diskSemaphore <- struct{}{}
	state.readers++
	c.items[path] = state
	c.mu.Unlock()
	return nil
}

func (c *CacheTable) CacheUnlockFile(path string) {
	c.mu.Lock()
	st := c.items[path]
	st.readers--
	c.items[path] = st

	needDelete := st.toDelete && st.readers == 0
	if needDelete {
		delete(c.items, path)
	}
	c.mu.Unlock()

	if needDelete {
		os.Remove(path)
	}
	<-c.diskSemaphore

}

func (c *CacheTable) CleanUpFile(path string) error {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()

	c.mu.Lock()
	state, exists := c.items[path]

	if !exists || state.toDelete {
		c.mu.Unlock()
		return nil
	}

	if state.readers == 0 {
		delete(c.items, path)
		c.mu.Unlock()
		return os.Remove(path)
	}

	state.toDelete = true
	c.items[path] = state
	c.mu.Unlock()

	return nil
}

func (c *CacheTable) ReadFile(path string) ([]byte, error) {
	c.diskSemaphore <- struct{}{}
	defer func() { <-c.diskSemaphore }()
	return os.ReadFile(path)
}
