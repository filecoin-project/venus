package config

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	repo2 "github.com/filecoin-project/venus/pkg/repo"
	"sync"
)

// configModule is plumbing implementation for setting and retrieving values from local config.
type ConfigModule struct { //nolint
	repo repo2.Repo
	lock sync.Mutex
}

// NewConfig returns a new configModule.
func NewConfigModule(repo repo2.Repo) *ConfigModule {
	return &ConfigModule{repo: repo}
}

// Set sets a value in config
func (s *ConfigModule) Set(dottedKey string, jsonString string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	cfg := s.repo.Config()
	if err := cfg.Set(dottedKey, jsonString); err != nil {
		return err
	}

	return s.repo.ReplaceConfig(cfg)
}

// Get gets a value from config
func (s *ConfigModule) Get(dottedKey string) (interface{}, error) {
	return s.repo.Config().Get(dottedKey)
}

//API create a new config api implement
func (s *ConfigModule) API() apiface.IConfig {
	return &configAPI{config: s}
}
