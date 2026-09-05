package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrImgNotFound = errors.New("img not found")
)

type FileEntry struct {
	rwMu     sync.RWMutex
	toDelete bool
	refCount int
}

// FileMutex TODO: implement states into writeBehind, cacheCleaner, workers DONE
type FileMutex struct {
	mu            sync.Mutex
	items         map[string]*FileEntry
	diskSemaphore chan struct{}
}

func NewTable() *FileMutex {
	return &FileMutex{
		items:         make(map[string]*FileEntry),
		diskSemaphore: make(chan struct{}, 10),
	}
}

func (fm *FileMutex) AddFileState(path string, numReaders int) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fe := &FileEntry{refCount: numReaders, toDelete: false}
	fm.items[path] = fe
}

func (fm *FileMutex) CacheLockFile(path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	entry, exists := fm.items[path]
	if !exists || entry.toDelete {
		return errors.New("file not found or being deleted")
	}
	fm.diskSemaphore <- struct{}{}
	entry.refCount++
	return nil
}

func (fm *FileMutex) CacheUnlockFile(path string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	entry := fm.items[path]
	entry.refCount--

	needDelete := entry.toDelete && entry.refCount == 0

	defer func() {
		if needDelete {
			os.Remove(path)
		}
	}()
	defer func() { <-fm.diskSemaphore }()
}

func (fm *FileMutex) CleanUpFile(path string) error {
	fm.mu.Lock()
	entry, exists := fm.items[path]
	if !exists || entry.toDelete {
		fm.mu.Unlock()
		return nil
	}

	entry.toDelete = true

	if entry.refCount == 0 {
		delete(fm.items, path)
		fm.mu.Unlock()

		entry.rwMu.Lock()
		defer entry.rwMu.Unlock()
		return os.Remove(path)
	}

	fm.mu.Unlock()
	return nil
}

func (fm *FileMutex) getEntry(path string) (*FileEntry, error) {
	entry, exists := fm.items[path]
	if !exists || entry.toDelete {
		return nil, fmt.Errorf("file mutex: %v", ErrImgNotFound)
	}
	return entry, nil
}

func (fm *FileMutex) incrementEntry(path string) (*FileEntry, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	entry, err := fm.getEntry(path)
	if err != nil {
		return nil, err
	}
	entry.refCount++
	return entry, nil
}

func (fm *FileMutex) ReadCacheFile(path string) ([]byte, error) {
	entry, err := fm.incrementEntry(path)
	if err != nil {
		return nil, err
	}

	defer func() {
		fm.mu.Lock()
		entry.refCount--
		if entry.toDelete && entry.refCount == 0 {
			delete(fm.items, path)
			go os.Remove(path)
		}
		fm.mu.Unlock()
	}()
	entry.rwMu.RLock()
	defer entry.rwMu.RUnlock()

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

type Paths struct {
	OriginalsDir string
	CacheDir     string
	CacheLibDir  string
	CacheManDir  string
}

func (p *Paths) UpdateEnv(key string, val string) {
	switch key {
	case "CACHE_DIR":
		p.CacheDir = val
	case "CACHE_LIB_DIR":
		p.CacheLibDir = val
	case "CACHE_MAN_DIR":
		p.CacheManDir = val
	case "UPLOADS_DIR":
		p.OriginalsDir = val
	default:
	}
}

func (p *Paths) GetOrigPath(imgName string) string {
	subdir := imgName[:2]
	return filepath.Join(p.OriginalsDir, subdir, imgName)
}
