package cfg

import (
	"github.com/filecoin-project/go-filecoin/repo"
)

// Config is plumbing implementation for setting and retrieving values from local config.
type Config struct {
	repo repo.Repo
}

// NewConfig returns a new Config.
func NewConfig(repo repo.Repo) *Config {
	return &Config{repo: repo}
}

// Set sets a value in config
func (s *Config) Set(dottedKey string, jsonString string) error {
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
