package caching

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/h2non/bimg"
)

var (
	OriginalsDir = "./assets/uploads"
	CacheDir     = "./assets/image_cache"
)

const (
	megabyte = 1024 * 1024
	initCap  = 4 * 1024 * megabyte
)

var (
	ErrImgNotFound = errors.New("img not found")
)

type lazyCacher interface {
	Start(startStop <-chan struct{})
	LazyWrite(imgBytes []byte, cacheImgPath string)
	Stop()
}

type cacheCleaner interface {
	Start()
	CleanAll()
	Stop()
}

//type usage struct {
//	lastUsed  time.Time
//	timesUsed int
//}

type CacheManager struct {
	fastSearchMU sync.RWMutex
	fastSearch   map[string]bool
	//usageMap   map[string]*usage
	cap      int64
	size     atomic.Int64
	cc       cacheCleaner
	lc       lazyCacher
	cleanReq chan<- struct{}
	fmu      *CacheTable
}

func (c *CacheManager) GetImg(imgPath string) (string, error) {
	if !c.fastSearch[imgPath] {
		return "", fmt.Errorf("cache: %w", ErrImgNotFound)
	}
	cachePath := filepath.Join(CacheDir, imgPath)
	//c.usageMap[imgPath].timesUsed += 1
	//c.usageMap[imgPath].lastUsed = time.Now()
	return cachePath, nil
}

func (c *CacheManager) LoadNGetImg(imgPath string) ([]byte, error) {
	origImgPath := filepath.Join(OriginalsDir, imgPath)
	imgBytes, err, _ := requestGroup.Do(imgPath, func() (interface{}, error) {
		return c.loadImg(origImgPath)
	})

	if err != nil {
		return nil, fmt.Errorf("could not get img: %w", err)
	}
	cacheImgPath := filepath.Join(CacheDir, imgPath)
	c.lc.LazyWrite(imgBytes, cacheImgPath)
	return imgBytes, nil
}

func (c *CacheManager) loadImg(origImgPath string) ([]byte, error) {
	buffer, err := c.fmu.ReadFile(origImgPath)
	if err != nil {
		return nil, fmt.Errorf("error reading img: %w", err)
	}

	options := bimg.Options{
		// Width:   800,       // Раскомментируйте, если нужно изменить ширину (сохранит пропорции)
		// Height:  600,       // Если указать и Width и Height, картинка обрежется (Crop: true)
		Quality:       75,        // Сжатие до 75% (отлично подходит для JPEG/WebP)
		Type:          bimg.JPEG, // Принудительно конвертируем на выходе в JPEG
		StripMetadata: true,      // Удаляем EXIF-данные (геолокацию, модель камеры), чтобы уменьшить вес
	}

	newImage, err := bimg.NewImage(buffer).Process(options)
	if err != nil {
		err = fmt.Errorf("unable to load img: %w", err)
		return nil, err
	}
	return newImage, nil
}

func (c *CacheManager) ConfirmAddCache(cacheImgPath string) {
	c.fastSearchMU.Lock()
	defer c.fastSearchMU.Unlock()
	c.fastSearch[cacheImgPath] = true
}

func (c *CacheManager) ConfirmRemoveCache(cacheImgPath string) {
	c.fastSearchMU.Lock()
	defer c.fastSearchMU.Unlock()
	c.fastSearch[cacheImgPath] = false
}

func (c *CacheManager) AddSize(n int64) {
	c.size.Add(n)
	if c.GetSize() > highMark {
		c.cleanReq <- struct{}{}
	}
}

func (c *CacheManager) GetSize() int64 {
	return c.size.Load()
}

func (c *CacheManager) GetCap() int64 {
	return c.cap
}

func InitCache(ctxP context.Context, fmu *CacheTable) *CacheManager {
	cleanChan := make(chan struct{})
	startStop := make(chan struct{})
	ctx, cancel := context.WithCancel(ctxP)
	cm := &CacheManager{
		fastSearch: make(map[string]bool),
		cap:        initCap,
		cleanReq:   cleanChan,
		fmu:        fmu}
	ce := &cacheEvictor{
		ctx:       ctx,
		cleanReq:  cleanChan,
		cancel:    cancel,
		fmu:       fmu,
		startStop: startStop}
	wb := &WriteBehind{
		ctx:         ctx,
		restartChan: make(chan *restartInfo, 10),
		cancel:      cancel,
		errChan:     make(chan error, 10)}
	cm.cc = ce
	cm.lc = wb
	cm.cc.CleanAll()
	cm.cc.Start()
	cm.lc.Start(startStop)
	return cm
}

func (c *CacheManager) Close() {
	c.lc.Stop()
	c.cc.CleanAll()
	c.cc.Stop()
}

func InitPaths(basePath string) {
	if basePath == "" {
		var err error
		basePath, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}

	OriginalsDir = filepath.Join(basePath, "assets", "uploads")
	CacheDir = filepath.Join(basePath, "assets", "image_cache")
}
