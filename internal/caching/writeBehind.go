package caching

import (
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
	ConfirmAddCache(origImgPath string)
	ConfirmRemoveCache(origImgPath string)
}

type restartInfo struct {
	imgBytes     []byte
	imgCachePath string
	retryCount   int
}

type WriteBehind struct {
	ctx         context.Context
	restartChan chan *restartInfo
	cm          cacheManager
	cancel      context.CancelFunc
	errChan     chan error
}

func (wb *WriteBehind) LazyWrite(imgBytes []byte, imgCachePath string) {
	go wb.lazyWrite(imgBytes, imgCachePath, 0)
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
	tmpPath := imgCachePath + ".tmp"
	err := os.WriteFile(tmpPath, imgBytes, 0644)
	if err != nil {
		wb.restartChan <- ri
		wb.cm.AddSize(-imgSize)
		wb.errChan <- fmt.Errorf("%s %w", tmpPath, ErrTmpWrite)
		return
	}
	err = os.Rename(tmpPath, imgCachePath)
	if err != nil {
		wb.cm.AddSize(-imgSize)
		wb.errChan <- fmt.Errorf("%s %w", tmpPath, ErrTmpRename)
		err = os.Remove(tmpPath)
		if err != nil {
			wb.errChan <- fmt.Errorf("%s %w", tmpPath, ErrTmpRename)
		}
	}
	wb.cm.ConfirmAddCache(imgCachePath)
}

func (wb *WriteBehind) restarter(startStop <-chan struct{}) {
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
				case <-startStop:
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

func (wb *WriteBehind) Start(startStop <-chan struct{}) {
	go wb.restarter(startStop)
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
