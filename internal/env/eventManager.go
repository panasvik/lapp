package env

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type observer interface {
	UpdateEnv(key string, val string)
}

type EventManager struct {
	obss map[string][]observer
}

func (e *EventManager) Attach(obs observer, envName string) {
	envSubs := e.obss[envName]
	envSubs = append(envSubs, obs)
}

func (e *EventManager) Detach(obs observer, envName string) {
	envSubs, exists := e.obss[envName]
	if !exists {
		return
	}

	idx := slices.Index(envSubs, obs)
	if idx == -1 {
		return
	}

	envSubs[idx] = envSubs[len(envSubs)-1]
	envSubs = envSubs[:len(envSubs)-1]
}

func (e *EventManager) SetEnv(envName string, envVal string) error {
	err := os.Setenv(envName, envVal)
	if err != nil {
		return fmt.Errorf("error setting env: %v", err)
	}
	e.sendEvent(envName, envVal)
	return err
}

func (e *EventManager) sendEvent(envName string, envVal string) {
	envSubs, exists := e.obss[envName]
	if !exists {
		return
	}

	for _, obs := range envSubs {
		obs.UpdateEnv(envName, envVal)
	}
}

func (e *EventManager) InitPaths(basePath string) error {
	if basePath == "" {
		var err error
		basePath, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	err := e.SetEnv("CACHE_DIR", filepath.Join(basePath, "assets", "cache"))
	if err != nil {
		return err
	}
	err = e.SetEnv("UPLOADS_DIR", filepath.Join(basePath, "assets", "uploads"))
	if err != nil {
		return err
	}
	return nil
}

func NewEventManager() *EventManager {
	return &EventManager{make(map[string][]observer)}
}
