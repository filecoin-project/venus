package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"github.com/filecoin-project/go-filecoin/config"
)

const configFilename = "config"

// NoRepoError is returned when trying to open a repo where one does not exist
type NoRepoError struct {
	Path string
}

func (err NoRepoError) Error() string {
	return fmt.Sprintf("no filecoin repo found in %s.\nplease run: 'go-filecoin init'", err.Path)
}

// FSRepo is a repo implementation backed by a filesystem.
type FSRepo struct {
	path string

	cfg *config.Config
	ds  Datastore
}

var _ Repo = (*FSRepo)(nil)

// Open opens an already initialized fsrepo at the given path
func Open(p string) (*FSRepo, error) {
	expath, err := homedir.Expand(p)
	if err != nil {
		return nil, err
	}

	r := &FSRepo{path: expath}

	isInit, err := r.isInitialized()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if repo was initialized")
	}

	if !isInit {
		return nil, &NoRepoError{p}
	}

	if err := r.loadConfig(); err != nil {
		return nil, errors.Wrap(err, "failed to load config file")
	}

	if err := r.openDatastore(); err != nil {
		return nil, errors.Wrap(err, "failed to open datastore")
	}

	return r, nil
}

// Init initializes an fsrepo at the given path using the given configuration
func Init(p string, cfg *config.Config) error {
	// TODO: write config file
	// Create datastore as described in config
	// write repo version file
	return nil
}

// Config returns the configuration object.
func (r *FSRepo) Config() *config.Config {
	return r.cfg
}

// Datastore returns the datastore.
func (r *FSRepo) Datastore() Datastore {
	return r.ds
}

func (r *FSRepo) isInitialized() (bool, error) {
	configPath := filepath.Join(r.path, configFilename)

	_, err := os.Lstat(configPath)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err == nil:
		return true, nil
	default:
		return false, err
	}
}

func (r *FSRepo) loadConfig() error {
	// TODO: read config file from disk
	r.cfg = nil // make linting okay
	panic("NYI")
}

func (r *FSRepo) openDatastore() error {
	// TODO: read datastore info from config, use that to open it up
	r.ds = datastore.NewMapDatastore()

	return nil
}
