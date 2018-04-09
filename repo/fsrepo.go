package repo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	badgerds "gx/ipfs/QmPAiAmc3qhTFwzWnKpxr6WCXGZ5mqpaQ2YEwSTnwyduHo/go-ds-badger"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/keystore"
)

const configFilename = "config.toml"
const versionFilename = "version"

// NoRepoError is returned when trying to open a repo where one does not exist
type NoRepoError struct {
	Path string
}

func (err NoRepoError) Error() string {
	return fmt.Sprintf("no filecoin repo found in %s.\nplease run: 'go-filecoin init [--repodir=%s]'", err.Path, err.Path)
}

// FSRepo is a repo implementation backed by a filesystem.
type FSRepo struct {
	path    string
	version uint

	cfg      *config.Config
	ds       Datastore
	keystore keystore.Keystore
}

var _ Repo = (*FSRepo)(nil)

// OpenFSRepo opens an already initialized fsrepo at the given path
func OpenFSRepo(p string) (*FSRepo, error) {
	expath, err := homedir.Expand(p)
	if err != nil {
		return nil, err
	}

	isInit, err := isInitialized(expath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if repo was initialized")
	}

	if !isInit {
		return nil, &NoRepoError{p}
	}

	r := &FSRepo{path: expath}

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

	if err := r.openKeystore(); err != nil {
		return nil, errors.Wrap(err, "failed to open keystore")
	}

	return r, nil
}

// InitFSRepo initializes an fsrepo at the given path using the given configuration
func InitFSRepo(p string, cfg *config.Config) error {
	expath, err := homedir.Expand(p)
	if err != nil {
		return err
	}

	init, err := isInitialized(expath)
	if err != nil {
		return err
	}

	if init {
		return fmt.Errorf("repo already initialized")
	}

	if err := checkWritable(expath); err != nil {
		return errors.Wrap(err, "checking writability failed")
	}

	if err := initVersion(expath, Version); err != nil {
		return errors.Wrap(err, "initializing repo version failed")
	}

	if err := initConfig(expath, cfg); err != nil {
		return errors.Wrap(err, "initializing config file failed")
	}

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

// Keystore returns the keystore
func (r *FSRepo) Keystore() keystore.Keystore {
	return r.keystore
}

// Close closes the repo.
func (r *FSRepo) Close() error {
	if err := r.ds.Close(); err != nil {
		return errors.Wrap(err, "failed to close datastore")
	}

	return nil
}

func isInitialized(p string) (bool, error) {
	configPath := filepath.Join(p, configFilename)

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
	configFile := filepath.Join(r.path, configFilename)

	cfg, err := config.ReadFile(configFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read config file at %q", configFile)
	}

	r.cfg = cfg
	return nil
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
	switch r.cfg.Datastore.Type {
	case "badgerds":
		ds, err := badgerds.NewDatastore(filepath.Join(r.path, r.cfg.Datastore.Path), nil)
		if err != nil {
			return err
		}
		r.ds = ds
	default:
		return fmt.Errorf("unknown datastore type in config: %s", r.cfg.Datastore.Type)
	}

	return nil
}

func (r *FSRepo) openKeystore() error {
	ksp := filepath.Join(r.path, "keystore")

	ks, err := keystore.NewFSKeystore(ksp)
	if err != nil {
		return err
	}
	r.keystore = ks

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
