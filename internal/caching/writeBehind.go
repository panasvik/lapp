package caching

import (
	"ImageCacheProject/internal/util"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	MaxRetry    = 2
	MaxWaitTime = 60 * time.Second
)

var (
	ErrTmpWrite  = errors.New("unable to create tmp file")
	ErrTmpRename = errors.New("unable to rename tmp file, deleting")
)

type cacheManager interface {
	AddSize(n int64)
	GetSize() int64
	GetCap() int64
}

type restartInfo struct {
	imgBytes     []byte
	imgCachePath string
	retryCount   int
}

type WriteBehind struct {
	util.Paths
	ctx         context.Context
	restartChan chan *restartInfo
	fmu         *util.FileMutex
	cm          cacheManager
	cancel      context.CancelFunc
	errChan     chan error
	startStop   <-chan struct{}
}

func (wb *WriteBehind) LazyWrite(imgBytes []byte, dst string) {
	go wb.lazyWrite(imgBytes, dst, 0)
}

func (wb *WriteBehind) lazyWrite(imgBytes []byte, imgCachePath string, retryCount int) {
	ri := &restartInfo{
		imgBytes:     imgBytes,
		imgCachePath: imgCachePath,
		retryCount:   retryCount}

	imgSize := int64(len(imgBytes))
	nextSize := imgSize + wb.cm.GetSize()
	if nextSize > wb.cm.GetCap() {
		wb.restartChan <- ri
		return
	}
	wb.cm.AddSize(imgSize)
	tmpFile, err := os.CreateTemp(wb.CacheLibDir, "cached-*.tmp")
	if err != nil {
		wb.restartChan <- ri
		wb.cm.AddSize(-imgSize)
		wb.errChan <- fmt.Errorf("%s %w", "error creating .tmp file: ", ErrTmpWrite)
		return
	}
	tmpName := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	err = wb.fmu.WriteFile(tmpName, imgBytes)
	if err != nil {
		wb.restartChan <- ri
		wb.cm.AddSize(-imgSize)
		wb.errChan <- fmt.Errorf("%s %w", tmpName, ErrTmpWrite)
		return
	}

	err = os.Rename(tmpName, imgCachePath)
	if err != nil {
		wb.errChan <- fmt.Errorf("%s %w", tmpName, ErrTmpRename)
		err = wb.fmu.Remove(tmpName)
		if err != nil {
			wb.errChan <- fmt.Errorf("%s %w", tmpName, ErrTmpRename)
			return
		}
		wb.cm.AddSize(-imgSize)
		return
	}
	wb.fmu.AddFileState(imgCachePath, 0)
}

func (wb *WriteBehind) restarter() {
	for ri := range wb.restartChan {
		select {
		case <-wb.ctx.Done():
			return
		default:
			if ri.retryCount > MaxRetry {
				continue
			}
			imgSize := int64(len(ri.imgBytes))
			nextSize := imgSize + wb.cm.GetSize()
			if nextSize > highMark {
				select {
				case <-wb.startStop:
				case <-time.After(MaxWaitTime):
					continue
				}
			}
			nextSize = imgSize + wb.cm.GetSize()
			if nextSize > wb.cm.GetCap() {
				continue
			}
			go wb.lazyWrite(ri.imgBytes, ri.imgCachePath, ri.retryCount+1)
		}

	}
}

func (wb *WriteBehind) Start() {
	go wb.restarter()
	go wb.ErrLogger()

}

func (wb *WriteBehind) ErrLogger() {
	for err := range wb.errChan {
		log.Printf(err.Error())
	}
}

func (wb *WriteBehind) Stop() {
	wb.cancel()
	close(wb.restartChan)
	close(wb.errChan)
}
