package util

import (
	"path/filepath"
)

type Paths struct {
	OriginalsDir  string
	CacheDir      string
	CacheLibDir   string
	CacheManDir   string
	CacheModalDir string
}

func (p *Paths) UpdateEnv(key string, val string) {
	switch key {
	case "CACHE_DIR":
		p.CacheDir = val
	case "CACHE_LIB_DIR":
		p.CacheLibDir = val
	case "CACHE_MOD_DIR":
		p.CacheModalDir = val
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
