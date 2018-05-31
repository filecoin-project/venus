package repo

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	badgerds "gx/ipfs/QmPAiAmc3qhTFwzWnKpxr6WCXGZ5mqpaQ2YEwSTnwyduHo/go-ds-badger"
	lockfile "gx/ipfs/QmPdqSMmiwtQCBC515gFtMW2mP14HsfgnyQ2k5xPQVxMge/go-fs-lock"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	keystore "gx/ipfs/QmXjHfhUzN9W57ajPh6N1wQvPYGuRDQAmjqhxFSSqeEjuc/go-ipfs-keystore"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"github.com/filecoin-project/go-filecoin/config"
)

const apiFile = "api"
const configFilename = "config.toml"
const tempConfigFilename = ".config.toml.temp"
const lockFile = "repo.lock"
const versionFilename = "version"
const walletDatastorePrefix = "wallet"

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

	// lk protects the config file
	lk       sync.RWMutex
	cfg      *config.Config
	ds       Datastore
	keystore keystore.Keystore
	walletDs Datastore

	// lockfile is the file system lock to prevent others from opening the same repo.
	lockfile io.Closer
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

	r.lockfile, err = lockfile.Lock(r.path, lockFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to take repo lock")
	}

	if err := r.loadFromDisk(); err != nil {
		r.lockfile.Close() // nolint: errcheck
		return nil, err
	}

	return r, nil
}

func (r *FSRepo) loadFromDisk() error {
	localVersion, err := r.loadVersion()
	if err != nil {
		return errors.Wrap(err, "failed to load version")
	}

	if localVersion != Version {
		return fmt.Errorf("invalid repo version, got %d expected %d", localVersion, Version)
	}

	r.version = localVersion

	if err := r.loadConfig(); err != nil {
		return errors.Wrap(err, "failed to load config file")
	}

	if err := r.openDatastore(); err != nil {
		return errors.Wrap(err, "failed to open datastore")
	}

	if err := r.openKeystore(); err != nil {
		return errors.Wrap(err, "failed to open keystore")
	}

	if err := r.openWalletDatastore(); err != nil {
		return errors.Wrap(err, "failed to open wallet datastore")
	}
	return nil
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
	r.lk.RLock()
	defer r.lk.RUnlock()

	return r.cfg
}

// ReplaceConfig replaces the current config with the newly passed in one.
func (r *FSRepo) ReplaceConfig(cfg *config.Config) error {
	r.lk.Lock()
	defer r.lk.Unlock()

	r.cfg = cfg
	tmp := filepath.Join(r.path, tempConfigFilename)
	err := os.RemoveAll(tmp)
	if err != nil {
		return err
	}
	err = r.cfg.WriteFile(tmp)
	if err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(r.path, configFilename))

}

// Datastore returns the datastore.
func (r *FSRepo) Datastore() Datastore {
	return r.ds
}

// WalletDatastore returns the wallet datastore.
func (r *FSRepo) WalletDatastore() Datastore {
	return r.walletDs
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

	if err := r.walletDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close datastore")
	}

	if err := r.removeAPIFile(); err != nil {
		return errors.Wrap(err, "error removing API file")
	}

	return r.lockfile.Close()
}

func (r *FSRepo) removeFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (r *FSRepo) removeAPIFile() error {
	return r.removeFile(filepath.Join(r.path, apiFile))
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

func (r *FSRepo) openWalletDatastore() error {
	// TODO: read wallet datastore info from config, use that to open it up
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, walletDatastorePrefix), nil)
	if err != nil {
		return err
	}

	r.walletDs = ds

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

// StagingDir satisfies node.SectorDirs
func (r *FSRepo) StagingDir() string {
	return path.Join(r.path, "staging")
}

// SealedDir satisfies node.SectorDirs
func (r *FSRepo) SealedDir() string {
	return path.Join(r.path, "sealed")
}

// SetAPIAddr writes the address to the API file.
func (r *FSRepo) SetAPIAddr(addr string) error {
	f, err := os.Create(filepath.Join(r.path, apiFile))
	if err != nil {
		return errors.Wrap(err, "could not create API file")
	}

	defer f.Close() // nolint: errcheck

	_, err = f.WriteString(addr)
	if err != nil {
		// If we encounter an error writing to the API file,
		// delete the API file. The error encountered while
		// deleting the API file will be returned (if one
		// exists) instead of the write-error.
		if err := r.removeAPIFile(); err != nil {
			return errors.Wrap(err, "failed to remove API file")
		}

		return errors.Wrap(err, "failed to write to API file")
	}

	return nil
}

// APIAddr reads the address from the API file.
func (r *FSRepo) APIAddr() (string, error) {
	apiFilePath := filepath.Join(filepath.Clean(r.path), apiFile)

	contents, err := ioutil.ReadFile(apiFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read API file")
	}

	return string(contents), nil
}
