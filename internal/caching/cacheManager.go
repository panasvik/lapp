package caching

import (
	"ImageCacheProject/assets"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/h2non/bimg"
	"golang.org/x/sync/singleflight"
)

var (
	OriginalsDir string
	CacheDir     string
)

const (
	megabyte = 1024 * 1024
	initCap  = 4 * 1024 * megabyte
)

type lazyCacher interface {
	Start(startStop <-chan struct{})
	LazyWrite(imgBytes []byte, dst string)
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
	//usageMap   map[string]*usage
	cap          int64
	size         atomic.Int64
	cc           cacheCleaner
	lc           lazyCacher
	cleanReq     chan<- struct{}
	fmu          *CacheTable
	requestGroup singleflight.Group
}

func (c *CacheManager) GetImg(imgName string) (string, error) {
	cachePath := filepath.Join(CacheDir, imgName)

	if !c.fmu.Exists(cachePath) {
		return "", fmt.Errorf("cache: %w", ErrImgNotFound)
	}

	//c.usageMap[imgPath].timesUsed += 1
	//c.usageMap[imgPath].lastUsed = time.Now()
	return cachePath, nil
}

func (c *CacheManager) LoadNGetImg(imgName string) ([]byte, error) {
	origImgPath, err := GetOrigPath(imgName)

	if err != nil {
		return nil, err
	}
	res, err, _ := c.requestGroup.Do(origImgPath, func() (interface{}, error) {
		return c.loadImg(origImgPath)
	})

	if err != nil {
		return nil, fmt.Errorf("could not get img: %w", err)
	}

	imgBytes, ok := res.([]byte)
	if !ok {
		return nil, fmt.Errorf("internal error: singleflight returned unexpected type: %T", res)
	}

	cacheImgPath := filepath.Join(CacheDir, imgName)
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
	cleanChan := make(chan struct{}, 1)
	startStop := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(ctxP)
	cm := &CacheManager{
		cap:      initCap,
		cleanReq: cleanChan,
		fmu:      fmu}
	ce := &cacheEvictor{
		ctx:       ctx,
		cleanReq:  cleanChan,
		cancel:    cancel,
		fmu:       fmu,
		startStop: startStop,
		cm:        cm}
	wb := &WriteBehind{
		ctx:         ctx,
		restartChan: make(chan *restartInfo, 10),
		cancel:      cancel,
		errChan:     make(chan error, 10),
		fmu:         fmu,
		cm:          cm}
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
	CacheDir = filepath.Join(basePath, "assets", "cache")
}

func (c *CacheManager) LockFile(path string) (err error) {
	return c.fmu.CacheLockFile(path)
}

func (c *CacheManager) UnlockFile(path string) {
	c.fmu.CacheUnlockFile(path)
}

func GetOrigPath(imgName string) (string, error) {
	ext := filepath.Ext(imgName)
	onlyName := strings.Trim(imgName, ext)
	name64, err := strconv.ParseUint(onlyName, 10, 64)
	if err != nil {
		return "", fmt.Errorf("unsupported file name: %v", err)
	}
	subdir64 := name64 % assets.NumDirs
	subdir := strconv.FormatUint(subdir64, 10)
	fmt.Println(filepath.Join(OriginalsDir, subdir, imgName))
	return filepath.Join(OriginalsDir, subdir, imgName), nil
}
