package caching

import (
	"errors"
	"fmt"
	"io"
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

// FileMutex TODO: implement states into writeBehind, cacheCleaner, workers DONE
type FileMutex struct {
	mu            sync.Mutex
	items         map[string]fileState
	diskSemaphore chan struct{}
}

func NewTable() *FileMutex {
	return &FileMutex{
		items:         make(map[string]fileState),
		diskSemaphore: make(chan struct{}, 10),
	}
}

func (fm *FileMutex) AddFileState(path string, numReaders int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fs := fileState{numReaders, false}
	fm.items[path] = fs
}

func (fm *FileMutex) CacheLockFile(path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	state, exists := fm.items[path]

	if !exists || state.toDelete {
		return errors.New("file not found or being deleted")
	}
	fm.diskSemaphore <- struct{}{}
	state.readers++
	fm.items[path] = state
	return nil
}

func (fm *FileMutex) CacheUnlockFile(path string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	st := fm.items[path]
	st.readers--
	fm.items[path] = st

	needDelete := st.toDelete && st.readers == 0

	defer func() {
		if needDelete {
			os.Remove(path)
		}
	}()
	defer func() { <-fm.diskSemaphore }()
}

func (fm *FileMutex) CleanUpFile(path string) error {
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	fm.mu.Lock()
	defer fm.mu.Unlock()
	state, exists := fm.items[path]

	if !exists || state.toDelete {
		return nil
	}

	if state.readers == 0 {
		delete(fm.items, path)
		return os.Remove(path)
	}

	state.toDelete = true
	fm.items[path] = state

	return nil
}

func (fm *FileMutex) ReadCacheFile(path string) ([]byte, error) {
	state, exists := fm.items[path]
	if !exists {
		return nil, fmt.Errorf("cache table: %v", ErrImgNotFound)
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()

	state.readers++
	defer func() { fm.items[path] = state }()
	defer func() { state.readers-- }()
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	return os.ReadFile(path)
}

// ReadFile for NOT cache files, for cache files use ReadCacheFile
func (fm *FileMutex) ReadFile(path string) ([]byte, error) {
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	return os.ReadFile(path)
}

func (fm *FileMutex) WriteFile(path string, data []byte) error {
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	return os.WriteFile(path, data, 0644)
}

func (fm *FileMutex) Remove(path string) error {
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	return os.Remove(path)
}

func (fm *FileMutex) Exists(path string) bool {
	_, exists := fm.items[path]
	return exists
}

func (fm *FileMutex) Copy(dst io.Writer, src io.Reader) error {
	fm.diskSemaphore <- struct{}{}
	defer func() { <-fm.diskSemaphore }()
	_, err := io.Copy(dst, src)
	return err
}
