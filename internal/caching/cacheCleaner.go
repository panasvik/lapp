package caching

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	MaxAge   = 24 * time.Hour
	highMark = int64(0.9 * float64(initCap))
	lowMark  = int64(0.7 * float64(initCap))
)

type cacheEvictor struct {
	ctx       context.Context
	cleanReq  <-chan struct{}
	startStop chan<- struct{}
	cm        cacheManager
	cancel    context.CancelFunc
	fmu       *CacheTable
}

func (ce *cacheEvictor) Start() {
	go ce.runCleanUpWorker()
}

func (ce *cacheEvictor) runCleanUpWorker() {
	for {
		select {
		case <-ce.ctx.Done():
			return
		case <-ce.cleanReq:
			ce.clean()
		}
	}
}

func (ce *cacheEvictor) clean() {
	select {
	case <-ce.ctx.Done():
		return
	default:
		cutoffTime := time.Now().Add(-MaxAge)
		err := filepath.WalkDir(CacheDir, func(path string, d os.DirEntry, err error) error {
			return ce.eval(cutoffTime, path, d, err)
		})
		if err != nil {
			log.Printf("error scanning cache: %v\n", err)
		}
		ce.startStop <- struct{}{}
	}

}

func (ce *cacheEvictor) eval(cutoffTime time.Time, path string, d os.DirEntry, err error) error {
	if ce.cm.GetSize() < lowMark {
		return filepath.SkipAll
	}
	if err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}
	info, err := d.Info()
	if err != nil {
		return nil
	}
	fSize := info.Size()
	if info.ModTime().Before(cutoffTime) {
		removeErr := ce.fmu.CleanUpFile(path)
		if removeErr != nil {
			log.Printf("error deleting file %s: %v\n", path, removeErr)
		} else {
			log.Printf("deleted: %s\n", path)
			ce.cm.AddSize(-fSize)
			ce.cm.ConfirmRemoveCache(path)
		}
	}
	return nil
}

func (ce *cacheEvictor) CleanAll() {
	err := filepath.WalkDir(CacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		removeErr := ce.fmu.CleanUpFile(path)
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fSize := info.Size()
		if removeErr != nil {
			log.Printf("error deleting file %s: %v\n", path, removeErr)
		} else {
			log.Printf("deleted: %s\n", path)
			ce.cm.ConfirmRemoveCache(path)
			ce.cm.AddSize(-fSize)
		}
		return nil
	})
	if err != nil {
		log.Printf("error scanning cache: %v\n", err)
	}
}

func (ce *cacheEvictor) Stop() {
	ce.cancel()
	close(ce.startStop)
}
