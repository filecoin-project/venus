package cfg

import (
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/pkg/errors"
	"sync"
)

// Config is plumbing implementation for setting and retrieving values from local config.
type Config struct {
	repo repo.Repo
	lock sync.Mutex
}

// NewConfig returns a new Config.
func NewConfig(repo repo.Repo) *Config {
	return &Config{repo: repo}
}

// Set sets a value in config
func (s *Config) Set(dottedKey string, jsonString string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	cfg := s.repo.Config()
	if err := cfg.Set(dottedKey, jsonString); err != nil {
		return err
	}

	return s.repo.ReplaceConfig(cfg)
}

// Get gets a value from config
func (s *Config) Get(dottedKey string) (interface{}, error) {
	return s.repo.Config().Get(dottedKey)
}

type ProtocolParams struct {
	AutoSealInterval uint
}

func (s *Config) ProtocolParams() (*ProtocolParams, error) {
	autoSealIntervalInterface, err := s.Get("mining.autoSealIntervalSeconds")
	if err != nil {
		return nil, err
	}

	autoSealInterval, ok := autoSealIntervalInterface.(uint)
	if !ok {
		return nil, errors.New("Failed to read autoSealInterval from config")
	}

	return &ProtocolParams{
		AutoSealInterval: autoSealInterval,
	}, nil
}
