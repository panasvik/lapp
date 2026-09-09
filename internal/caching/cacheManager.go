package caching

import (
	"ImageCacheProject/internal/util"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/h2non/bimg"
	"golang.org/x/sync/singleflight"
)

const wLimit = 50

var (
	ErrWrongPhotoType = errors.New("unknown photo type")
)

type lazyCacher interface {
	Start()
	LazyWrite(imgBytes []byte, dst string)
	Stop()
}

type cacheCleaner interface {
	CleanAll()
}

type CacheManager struct {
	*util.Paths
	cc           cacheCleaner
	lc           lazyCacher
	storage      *cacheStorage
	requestGroup singleflight.Group
}

func (c *CacheManager) GetImg(imgName string, ImgType ImageCategory) (string, error) {
	cachePath, err := c.formImgPath(imgName, ImgType)
	if err != nil {
		return "", fmt.Errorf("error in Options ImgType (%d): %w", ImgType, err)
	}
	_, ok := c.storage.get(cachePath)
	if !ok {
		return "", ErrNotFound
	}
	return cachePath, nil
}

func (c *CacheManager) LoadNGetImg(imgName string, options Options) ([]byte, error) {
	origImgPath := c.GetOrigPath(imgName)
	sfKey := fmt.Sprintf("%s:%d:%d:%d:%d", origImgPath, options.Category, options.BimgOpt.Width, options.BimgOpt.Height, options.BimgOpt.Quality)
	res, err, _ := c.requestGroup.Do(sfKey, func() (any, error) {
		return c.loadImg(origImgPath, options)
	})

	if err != nil {
		return nil, fmt.Errorf("could not get img: %w", err)
	}

	imgBytes, ok := res.([]byte)
	if !ok {
		return nil, fmt.Errorf("internal error: singleflight returned unexpected type: %T", res)
	}

	cacheImgPath, err := c.formImgPath(imgName, options.Category)
	if err != nil {
		return nil, fmt.Errorf("error in Options ImgType (%d): %w", options.Category, err)
	}
	c.lc.LazyWrite(imgBytes, cacheImgPath)
	return imgBytes, nil
}

func (c *CacheManager) loadImg(origImgPath string, options Options) ([]byte, error) {
	buffer, err := os.ReadFile(origImgPath)
	if err != nil {
		return nil, fmt.Errorf("error reading img: %w", err)
	}

	newImage, err := bimg.NewImage(buffer).Process(options.BimgOpt)
	if err != nil {
		err = fmt.Errorf("unable to load img: %w", err)
		return nil, err
	}
	return newImage, nil
}

func InitCache(ctxP context.Context, p *util.Paths) *CacheManager {
	storage := newCacheStorage()
	ctx, cancel := context.WithCancel(ctxP)
	cm := &CacheManager{
		Paths:   p,
		storage: storage,
		cc: &cacheEvictor{
			Paths: p,
			ctx:   ctx},
		lc: &WriteBehind{
			Paths:       p,
			ctx:         ctx,
			restartChan: make(chan *restartInfo, wLimit),
			cancel:      cancel,
			errChan:     make(chan error, wLimit),
			storage:     storage,
			limit:       make(chan struct{}, wLimit)},
	}
	return cm
}

func (c *CacheManager) StartBGProcesses() {
	//c.cc.CleanAll()
	c.lc.Start()
}

func (c *CacheManager) Close() {
	c.lc.Stop()
	//c.cc.CleanAll()
}

func (c *CacheManager) UseFile(path string, r func(path string)) (err error) {
	return c.storage.UseCacheFile(path, r)
}

type ImageCategory int

const (
	ManifestImage ImageCategory = 0
	LibImage      ImageCategory = 1
	ModalImage    ImageCategory = 2
)

type Options struct {
	Category ImageCategory
	BimgOpt  bimg.Options
}

func (c *CacheManager) formImgPath(imgName string, imgType ImageCategory) (string, error) {
	switch imgType {
	case ManifestImage:
		return filepath.Join(c.CacheManDir, imgName), nil
	case LibImage:
		return filepath.Join(c.CacheLibDir, imgName), nil
	case ModalImage:
		return filepath.Join(c.CacheModalDir, imgName), nil
	}
	return "", ErrWrongPhotoType
}
