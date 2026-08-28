package upload

import (
	"ImageCacheProject/internal/caching"
	"io"
	"os"
	"path/filepath"
)

type Manager struct {
	caching.Paths
	fmu *caching.FileMutex
}

func (m *Manager) SaveUploadedFile(fileName string, src io.Reader) error {
	uploadPath := filepath.Join(m.OriginalsDir, fileName)
	tmpPath := filepath.Join(uploadPath, ".tmp")
	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	err = m.fmu.Copy(dst, src)
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, uploadPath)
}

func NewManager(fmu *caching.FileMutex) *Manager {
	return &Manager{
		caching.Paths{"", ""},
		fmu}
}
