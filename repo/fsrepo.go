package repo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"github.com/filecoin-project/go-filecoin/config"
)

const configFilename = "config.toml"
const versionFilename = "version"

// NoRepoError is returned when trying to open a repo where one does not exist
type NoRepoError struct {
	Path string
}

func (err NoRepoError) Error() string {
	return fmt.Sprintf("no filecoin repo found in %s.\nplease run: 'go-filecoin init'", err.Path)
}

// FSRepo is a repo implementation backed by a filesystem.
type FSRepo struct {
	path    string
	version uint

	cfg *config.Config
	ds  Datastore
}

var _ Repo = (*FSRepo)(nil)

// OpenFSRepo opens an already initialized fsrepo at the given path
func OpenFSRepo(p string) (*FSRepo, error) {
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

	localVersion, err := r.loadVersion()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load version")
	}

	if localVersion != Version {
		return nil, fmt.Errorf("invalid repo version, got %d expected %d", localVersion, Version)
	}

	r.version = localVersion

	if err := r.loadConfig(); err != nil {
		return nil, errors.Wrap(err, "failed to load config file")
	}

	if err := r.openDatastore(); err != nil {
		return nil, errors.Wrap(err, "failed to open datastore")
	}

	return r, nil
}

// InitFSRepo initializes an fsrepo at the given path using the given configuration
func InitFSRepo(p string, cfg *config.Config) error {
	expath, err := homedir.Expand(p)
	if err != nil {
		return err
	}

	if err := checkWritable(p); err != nil {
		return err
	}

	if err := initVersion(expath, Version); err != nil {
		return err
	}

	if err := initConfig(expath, cfg); err != nil {
		return err
	}
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

// Version returns the version of the repo
func (r *FSRepo) Version() uint {
	return r.version
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

func (r *FSRepo) loadVersion() (uint, error) {
	// TODO: limited file reading, to avoid attack vector
	file, err := ioutil.ReadFile(filepath.Join(r.path, versionFilename))
	if err != nil {
		return 0, err
	}

	version, err := strconv.Atoi(strings.Trim(string(file), "\n"))
	if err != nil {
		return 0, err
	}

	return uint(version), nil
}

func (r *FSRepo) openDatastore() error {
	// TODO: read datastore info from config, use that to open it up
	r.ds = datastore.NewMapDatastore()

	return nil
}

func initVersion(p string, version uint) error {
	return ioutil.WriteFile(filepath.Join(p, versionFilename), []byte(strconv.Itoa(int(version))), 0644)
}

func initConfig(p string, cfg *config.Config) error {
	configFile := filepath.Join(p, configFilename)
	if fileExists(configFile) {
		return fmt.Errorf("file already exists: %s", configFile)
	}

	return cfg.WriteFile(configFile)
}

func checkWritable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		// dir doesnt exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return errors.Wrapf(err, "cannot write to %s, incorrect permissions", dir)
	}

	return err
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
