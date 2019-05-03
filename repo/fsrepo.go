package repo

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	badgerds "github.com/ipfs/go-ds-badger"
	lockfile "github.com/ipfs/go-fs-lock"
	keystore "github.com/ipfs/go-ipfs-keystore"
	logging "github.com/ipfs/go-log"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/config"
)

const (
	// apiFile is the filename containing the filecoin node's api address.
	apiFile                = "api"
	configFilename         = "config.json"
	tempConfigFilename     = ".config.json.temp"
	lockFile               = "repo.lock"
	versionFilename        = "version"
	walletDatastorePrefix  = "wallet"
	chainDatastorePrefix   = "chain"
	dealsDatastorePrefix   = "deals"
	snapshotStorePrefix    = "snapshots"
	snapshotFilenamePrefix = "snapshot"

	// DefaultRepoDir is the default directory of the filecoin repo
	DefaultRepoDir = "repo"
)

// NoRepoError is returned when trying to open a repo where one does not exist
type NoRepoError struct {
	Path string
}

var log = logging.Logger("repo")

func (err NoRepoError) Error() string {
	return fmt.Sprintf("no filecoin repo found in %s.\nplease run: 'go-filecoin init [--repodir=%s]'", err.Path, err.Path)
}

// FSRepo is a repo implementation backed by a filesystem.
type FSRepo struct {
	path string

	version uint

	// lk protects the config file
	lk       sync.RWMutex
	cfg      *config.Config
	ds       Datastore
	keystore keystore.Keystore
	walletDs Datastore
	chainDs  Datastore
	dealsDs  Datastore

	// lockfile is the file system lock to prevent others from opening the same repo.
	lockfile io.Closer
}

var _ Repo = (*FSRepo)(nil)

// CreateRepo provides a quick shorthand for initializing and opening a repo
func CreateRepo(repoPath string, cfg *config.Config) (*FSRepo, error) {
	if err := InitFSRepo(repoPath, cfg); err != nil {
		return nil, err
	}
	return OpenFSRepo(repoPath)
}

// OpenFSRepo opens an already initialized fsrepo at the given path
func OpenFSRepo(repoPath string) (*FSRepo, error) {
	repoPath, err := homedir.Expand(repoPath)
	if err != nil {
		return nil, err
	}

	isInit, err := isInitialized(repoPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if repo was initialized")
	}

	if !isInit {
		return nil, &NoRepoError{repoPath}
	}

	r := &FSRepo{path: repoPath}

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

	if localVersion < Version {
		return fmt.Errorf("out of date repo version, got %d expected %d. Migrate with tools/migration/go-filecoin-migrate", localVersion, Version)
	}

	if localVersion > Version {
		return fmt.Errorf("binary needs update to handle repo version, got %d expected %d. Update binary to latest release", localVersion, Version)
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

	if err := r.openChainDatastore(); err != nil {
		return errors.Wrap(err, "failed to open chain datastore")
	}

	if err := r.openDealsDatastore(); err != nil {
		return errors.Wrap(err, "failed to open deals datastore")
	}
	return nil
}

// InitFSRepo initializes an fsrepo at the given path using the given configuration
func InitFSRepo(repoPath string, cfg *config.Config) error {
	repoPath, err := homedir.Expand(repoPath)
	if err != nil {
		return err
	}

	if err := checkWritable(repoPath); err != nil {
		return errors.Wrap(err, "checking writability of repo path failed")
	}

	init, err := isInitialized(repoPath)
	if err != nil {
		return err
	}

	if init {
		return fmt.Errorf("repo already initialized")
	}

	if err := initVersion(repoPath, Version); err != nil {
		return errors.Wrap(err, "initializing repo version failed")
	}

	if err := initConfig(repoPath, cfg); err != nil {
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
	if err := r.SnapshotConfig(r.Config()); err != nil {
		log.Warningf("failed to create snapshot: %s", err.Error())
	}
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

// SnapshotConfig stores a copy `cfg` in <repo_path>/snapshots/ appending the
// time of snapshot to the filename.
func (r *FSRepo) SnapshotConfig(cfg *config.Config) error {
	snapshotFile := filepath.Join(r.path, snapshotStorePrefix, genSnapshotFileName())
	if fileExists(snapshotFile) {
		// this should never happen
		return fmt.Errorf("file already exists: %s", snapshotFile)
	}
	return cfg.WriteFile(snapshotFile)
}

// Datastore returns the datastore.
func (r *FSRepo) Datastore() Datastore {
	return r.ds
}

// WalletDatastore returns the wallet datastore.
func (r *FSRepo) WalletDatastore() Datastore {
	return r.walletDs
}

// ChainDatastore returns the chain datastore.
func (r *FSRepo) ChainDatastore() Datastore {
	return r.chainDs
}

// DealsDatastore returns the deals datastore.
func (r *FSRepo) DealsDatastore() Datastore {
	return r.dealsDs
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
		return errors.Wrap(err, "failed to close wallet datastore")
	}

	if err := r.chainDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close chain datastore")
	}

	if err := r.dealsDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close miner deals datastore")
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
		return 0, errors.New("corrupt version file: version is not an integer")
	}

	return uint(version), nil
}

func (r *FSRepo) openDatastore() error {
	switch r.cfg.Datastore.Type {
	case "badgerds":
		ds, err := badgerds.NewDatastore(filepath.Join(r.path, r.cfg.Datastore.Path), badgerOptions())
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

func (r *FSRepo) openChainDatastore() error {
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, chainDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}

	r.chainDs = ds

	return nil
}

func (r *FSRepo) openWalletDatastore() error {
	// TODO: read wallet datastore info from config, use that to open it up
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, walletDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}

	r.walletDs = ds

	return nil
}

func (r *FSRepo) openDealsDatastore() error {
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, dealsDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}

	r.dealsDs = ds

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

	if err := cfg.WriteFile(configFile); err != nil {
		return err
	}

	// make the snapshot dir
	snapshotDir := filepath.Join(p, snapshotStorePrefix)
	return checkWritable(snapshotDir)
}

func genSnapshotFileName() string {
	return fmt.Sprintf("%s-%d.json", snapshotFilenamePrefix, time.Now().UTC().UnixNano())
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

// SetAPIAddr writes the address to the API file. SetAPIAddr expects parameter
// `port` to be of the form `:<port>`.
func (r *FSRepo) SetAPIAddr(maddr string) error {
	f, err := os.Create(filepath.Join(r.path, apiFile))
	if err != nil {
		return errors.Wrap(err, "could not create API file")
	}

	defer f.Close() // nolint: errcheck

	_, err = f.WriteString(maddr)
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

// Path returns the path the fsrepo is at
func (r *FSRepo) Path() (string, error) {
	return r.path, nil
}

// APIAddrFromRepoPath returns the api addr from the filecoin repo
func APIAddrFromRepoPath(repoPath string) (string, error) {
	repoPath, err := homedir.Expand(repoPath)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("can't resolve local repo path %s", repoPath))
	}
	return apiAddrFromFile(filepath.Join(repoPath, apiFile))
}

// APIAddrFromFile reads the address from the API file at the given path.
// A relevant comment from a similar function at go-ipfs/repo/fsrepo/fsrepo.go:
// This is a concurrent operation, meaning that any process may read this file.
// Modifying this file, therefore, should use "mv" to replace the whole file
// and avoid interleaved read/writes
func apiAddrFromFile(apiFilePath string) (string, error) {
	contents, err := ioutil.ReadFile(apiFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read API file")
	}

	return string(contents), nil
}

// APIAddr reads the FSRepo's api file and returns the api address
func (r *FSRepo) APIAddr() (string, error) {
	return apiAddrFromFile(filepath.Join(filepath.Clean(r.path), apiFile))
}

func badgerOptions() *badgerds.Options {
	result := &badgerds.DefaultOptions
	result.Truncate = true
	return result
}

// VersionFilename returns the version filename for the repo
func VersionFilename() string {
	return versionFilename
}
