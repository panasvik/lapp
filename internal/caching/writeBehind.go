package caching

import (
	"ImageCacheProject/internal/util"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

const (
	MaxRetry = 2
)

var (
	ErrTmpWrite  = errors.New("unable to write tmp file")
	ErrTmpRename = errors.New("unable to rename tmp file, deleting")
)

type restartInfo struct {
	imgBytes     []byte
	imgCachePath string
	retryCount   int
}

type WriteBehind struct {
	*util.Paths
	ctx         context.Context
	restartChan chan *restartInfo
	storage     *cacheStorage
	cancel      context.CancelFunc
	errChan     chan error
	wg          sync.WaitGroup
	limit       chan struct{}
}

func (wb *WriteBehind) sendToRestart(ri *restartInfo) {
	select {
	case <-wb.ctx.Done():
		return
	case wb.restartChan <- ri:
	}
}

func (wb *WriteBehind) sendErr(err error) {
	select {
	case <-wb.ctx.Done():
		return
	case wb.errChan <- err:
	}
}

func (wb *WriteBehind) LazyWrite(imgBytes []byte, dst string) {
	wb.wg.Go(func() {
		wb.lazyWrite(imgBytes, dst, 0)
	})
}

func (wb *WriteBehind) lazyWrite(imgBytes []byte, imgCachePath string, retryCount int) {
	wb.limit <- struct{}{}
	defer func() { <-wb.limit }()

	ri := &restartInfo{
		imgBytes:     imgBytes,
		imgCachePath: imgCachePath,
		retryCount:   retryCount,
	}

	imgSize := int64(len(imgBytes))
	nextSize := imgSize + wb.storage.GetSize()
	if nextSize > wb.storage.GetCap() {
		wb.sendToRestart(ri)
		return
	}

	tmpFile, err := os.CreateTemp(wb.CacheLibDir, "cached-*.tmp")
	if err != nil {
		wb.sendToRestart(ri)
		wb.sendErr(fmt.Errorf("error creating .tmp file: %w", err))
		return
	}
	tmpName := tmpFile.Name()

	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = os.Remove(tmpName)
		}
	}()

	_, err = tmpFile.Write(imgBytes)
	closeErr := tmpFile.Close()
	if err != nil {
		wb.sendToRestart(ri)
		wb.sendErr(fmt.Errorf("%s: %w", tmpName, ErrTmpWrite))
		return
	}
	if closeErr != nil {
		wb.sendToRestart(ri)
		wb.sendErr(fmt.Errorf("error closing tmp file: %w", closeErr))
		return
	}

	_ = os.Remove(imgCachePath)

	err = os.Rename(tmpName, imgCachePath)
	if err != nil {
		wb.sendErr(fmt.Errorf("%s -> %s: %w", tmpName, imgCachePath, ErrTmpRename))
		return
	}

	cleanupNeeded = false
	wb.storage.AddCacheFile(imgCachePath, imgSize)
}

func (wb *WriteBehind) restarter() {
	defer wb.wg.Done()
	for {
		select {
		case <-wb.ctx.Done():
			return
		case ri, ok := <-wb.restartChan:
			if !ok {
				return
			}
			if ri.retryCount >= MaxRetry {
				continue
			}
			imgSize := int64(len(ri.imgBytes))
			nextSize := imgSize + wb.storage.GetSize()
			if nextSize > wb.storage.GetCap() {
				// Если места всё еще нет, откладываем на следующую попытку
				continue
			}

			wb.wg.Add(1)
			go func(info *restartInfo) {
				defer wb.wg.Done()
				wb.lazyWrite(info.imgBytes, info.imgCachePath, info.retryCount+1)
			}(ri)
		}
	}
}

func (wb *WriteBehind) Start() {
	wb.wg.Add(2)
	go wb.restarter()
	go wb.ErrLogger()
}

func (wb *WriteBehind) ErrLogger() {
	defer wb.wg.Done()
	for {
		select {
		case <-wb.ctx.Done():
			return
		case err, ok := <-wb.errChan:
			if !ok {
				return
			}
			log.Println(err.Error())
		}
	}
}

func (wb *WriteBehind) Stop() {
	wb.cancel()
	wb.wg.Wait()
	close(wb.restartChan)
	close(wb.errChan)
}
