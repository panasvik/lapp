package caching

import (
	"ImageCacheProject/internal/util"
	"context"
	"log"
	"os"
	"path/filepath"
)

type cacheEvictor struct {
	*util.Paths
	ctx context.Context
}

func (ce *cacheEvictor) CleanAll() {
	select {
	case <-ce.ctx.Done():
		return
	default:
		err := filepath.WalkDir(ce.CacheDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if err != nil {
				return nil
			}
			removeErr := os.Remove(path)
			if removeErr != nil {
				log.Printf("error deleting file %s: %v\n", path, removeErr)
			} else {
				log.Printf("deleted: %s\n", path)
			}
			return removeErr
		})
		if err != nil {
			log.Printf("error scanning cache: %v\n", err)
		}
	}
}
