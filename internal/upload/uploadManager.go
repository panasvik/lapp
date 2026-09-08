package upload

import (
	"ImageCacheProject/internal/util"
	"encoding/hex"
	"hash"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
)

type Manager struct {
	*util.Paths
}

func (m *Manager) SaveUploadedFile(fileName string, src io.Reader) (finalPath string, callback func() error, err error) {
	tmpFile, err := os.CreateTemp(m.OriginalsDir, "upload-*.tmp")
	ext := filepath.Ext(fileName)
	if err != nil {
		return "", nil, err
	}
	tmpName := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	new64a := fnv.New64a()
	err = m.copy(tmpFile, new64a, src)
	if err != nil {
		return "", nil, err
	}
	finalPath, err = m.rename(new64a, tmpName, ext)
	callback = func() error {
		return m.callback(finalPath)
	}
	return finalPath, callback, err
}

func (m *Manager) rename(new64a hash.Hash64, tmpName string, ext string) (string, error) {
	hashSum := new64a.Sum(nil)
	hashName := hex.EncodeToString(hashSum)
	subDir := filepath.Join(m.OriginalsDir, hashName[:2])
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return "", err
	}
	finalPath := filepath.Join(subDir, hashName+ext)
	err := os.Rename(tmpName, finalPath)
	if err != nil {
		return "", err
	}
	return finalPath, nil
}

func (m *Manager) copy(tmpFile *os.File, new64a hash.Hash64, src io.Reader) error {
	mw := io.MultiWriter(tmpFile, new64a)
	_, err := io.Copy(mw, src)
	if err != nil {
		return err
	}
	if err = tmpFile.Close(); err != nil {
		return err
	}
	return nil
}

func NewManager(paths *util.Paths) *Manager {
	return &Manager{
		paths}
}

func (m *Manager) callback(imgPath string) error {
	origPath := m.GetOrigPath(imgPath)
	err := os.Remove(origPath)
	return err
}
