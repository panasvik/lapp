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
	highMark = int64(initCap * 9 / 10)
	lowMark  = int64(initCap * 7 / 10)
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
			{
				cutoffTime := time.Now().Add(-MaxAge)
				ce.clean(cutoffTime)
				ce.startStop <- struct{}{}
			}
		}
	}
}

func (ce *cacheEvictor) clean(cutoffTime time.Time) {
	select {
	case <-ce.ctx.Done():
		return
	default:
		err := filepath.WalkDir(CacheDir, func(path string, d os.DirEntry, err error) error {
			return ce.eval(cutoffTime, path, d, err)
		})
		if err != nil {
			log.Printf("error scanning cache: %v\n", err)
		}
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
		}
	}
	return nil
}

func (ce *cacheEvictor) CleanAll() {
	ce.clean(time.Now())
}

func (ce *cacheEvictor) Stop() {
	ce.cancel()
	close(ce.startStop)
}
