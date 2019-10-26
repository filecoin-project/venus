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

	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger"
	lockfile "github.com/ipfs/go-fs-lock"
	keystore "github.com/ipfs/go-ipfs-keystore"
	logging "github.com/ipfs/go-log"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
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
)

var log = logging.Logger("repo")

// FSRepo is a repo implementation backed by a filesystem.
type FSRepo struct {
	// Path to the repo root directory.
	path    string
	version uint

	// lk protects the config file
	lk  sync.RWMutex
	cfg *config.Config

	ds       Datastore
	keystore keystore.Keystore
	walletDs Datastore
	chainDs  Datastore
	dealsDs  Datastore

	// lockfile is the file system lock to prevent others from opening the same repo.
	lockfile io.Closer
}

var _ Repo = (*FSRepo)(nil)

// InitFSRepo initializes a new repo at the target path with the provided configuration.
// The successful result creates a symlink at targetPath pointing to a sibling directory
// named with a timestamp and repo version number.
// The link path must be empty prior. If the computed actual directory exists, it must be empty.
func InitFSRepo(targetPath string, version uint, cfg *config.Config) error {
	linkPath, err := homedir.Expand(targetPath)
	if err != nil {
		return err
	}

	container, basename := filepath.Split(linkPath)
	if container == "" { // path contained no separator
		container = "./"
	}

	dirpath := container + MakeRepoDirName(basename, time.Now(), version, 0)

	exists, err := fileExists(linkPath)
	if err != nil {
		return errors.Wrapf(err, "error inspecting repo symlink path %s", linkPath)
	} else if exists {
		return errors.Errorf("refusing to init repo symlink at %s, file exists", linkPath)
	}

	// Create the actual directory and then the link to it.
	if err = InitFSRepoDirect(dirpath, version, cfg); err != nil {
		return err
	}
	if err = os.Symlink(dirpath, linkPath); err != nil {
		return err
	}

	return nil
}

// InitFSRepoDirect initializes a new repo at a target path, establishing a provided configuration.
// The target path must not exist, or must reference an empty, read/writable directory.
func InitFSRepoDirect(targetPath string, version uint, cfg *config.Config) error {
	repoPath, err := homedir.Expand(targetPath)
	if err != nil {
		return err
	}

	if err := ensureWritableDirectory(repoPath); err != nil {
		return errors.Wrap(err, "no writable directory")
	}

	empty, err := isEmptyDir(repoPath)
	if err != nil {
		return errors.Wrapf(err, "failed to list repo directory %s", repoPath)
	}
	if !empty {
		return fmt.Errorf("refusing to initialize repo in non-empty directory %s", repoPath)
	}

	if err := WriteVersion(repoPath, version); err != nil {
		return errors.Wrap(err, "initializing repo version failed")
	}

	if err := initConfig(repoPath, cfg); err != nil {
		return errors.Wrap(err, "initializing config file failed")
	}
	return nil
}

// OpenFSRepo opens an initialized fsrepo, expecting a specific version.
// The provided path may be to a directory, or a symbolic link pointing at a directory, which
// will be resolved just once at open.
func OpenFSRepo(repoPath string, version uint) (*FSRepo, error) {
	repoPath, err := homedir.Expand(repoPath)
	if err != nil {
		return nil, err
	}

	hasConfig, err := hasConfig(repoPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check for repo config")
	}

	if !hasConfig {
		return nil, errors.Errorf("no repo found at %s; run: 'go-filecoin init [--repodir=%s]'", repoPath, repoPath)
	}

	info, err := os.Stat(repoPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat repo link %s", repoPath)
	}

	// Resolve path if it's a symlink.
	var actualPath string
	if info.IsDir() {
		actualPath = repoPath
	} else {
		actualPath, err = os.Readlink(repoPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to follow repo symlink %s", repoPath)
		}
	}

	r := &FSRepo{path: actualPath, version: version}

	r.lockfile, err = lockfile.Lock(r.path, lockFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to take repo lock")
	}

	if err := r.loadFromDisk(); err != nil {
		_ = r.lockfile.Close()
		return nil, err
	}

	return r, nil
}

// MakeRepoDirName constructs a name for a concrete repo directory, which includes its
// version number and a timestamp. The name will begin with prefix and, if uniqueifier is
// non-zero, end with that (intended as an ordinal for finding a free name).
// E.g. ".filecoin-20190102-140425-012-1
// This is exported for use by migrations.
func MakeRepoDirName(prefix string, ts time.Time, version uint, uniqueifier uint) string {
	name := strings.Join([]string{
		prefix,
		ts.Format("20060102-150405"),
		fmt.Sprintf("v%03d", version),
	}, "-")
	if uniqueifier != 0 {
		name = name + fmt.Sprintf("-%d", uniqueifier)
	}
	return name
}

func (r *FSRepo) loadFromDisk() error {
	localVersion, err := r.readVersion()
	if err != nil {
		return errors.Wrap(err, "failed to read version")
	}

	if localVersion < r.version {
		return fmt.Errorf("out of date repo version, got %d expected %d. Migrate with tools/migration/go-filecoin-migrate", localVersion, Version)
	}

	if localVersion > r.version {
		return fmt.Errorf("binary needs update to handle repo version, got %d expected %d. Update binary to latest release", localVersion, Version)
	}

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

// Config returns the configuration object.
func (r *FSRepo) Config() *config.Config {
	r.lk.RLock()
	defer r.lk.RUnlock()

	return r.cfg
}

// ReplaceConfig replaces the current config with the newly passed in one.
func (r *FSRepo) ReplaceConfig(cfg *config.Config) error {
	if err := r.SnapshotConfig(r.Config()); err != nil {
		log.Warnf("failed to create snapshot: %s", err.Error())
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
	exists, err := fileExists(snapshotFile)
	if err != nil {
		return errors.Wrap(err, "error checking snapshot file")
	} else if exists {
		// this should never happen
		return fmt.Errorf("file already exists: %s", snapshotFile)
	}
	return cfg.WriteFile(snapshotFile)
}

// Datastore returns the datastore.
func (r *FSRepo) Datastore() ds.Batching {
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

// Tests whether a repo directory contains the expected config file.
func hasConfig(p string) (bool, error) {
	configPath := filepath.Join(p, configFilename)

	_, err := os.Lstat(configPath)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
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

// readVersion reads the repo's version file (but does not change r.version).
func (r *FSRepo) readVersion() (uint, error) {
	content, err := ReadVersion(r.path)
	if err != nil {
		return 0, err
	}

	version, err := strconv.Atoi(content)
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

// WriteVersion writes the given version to the repo version file.
func WriteVersion(p string, version uint) error {
	return ioutil.WriteFile(filepath.Join(p, versionFilename), []byte(strconv.Itoa(int(version))), 0644)
}

// ReadVersion returns the unparsed (string) version
// from the version file in the specified repo.
func ReadVersion(repoPath string) (string, error) {
	file, err := ioutil.ReadFile(filepath.Join(repoPath, versionFilename))
	if err != nil {
		return "", err
	}
	return strings.Trim(string(file), "\n"), nil
}

func initConfig(p string, cfg *config.Config) error {
	configFile := filepath.Join(p, configFilename)
	exists, err := fileExists(configFile)
	if err != nil {
		return errors.Wrap(err, "error inspecting config file")
	} else if exists {
		return fmt.Errorf("config file already exists: %s", configFile)
	}

	if err := cfg.WriteFile(configFile); err != nil {
		return err
	}

	// make the snapshot dir
	snapshotDir := filepath.Join(p, snapshotStorePrefix)
	return ensureWritableDirectory(snapshotDir)
}

func genSnapshotFileName() string {
	return fmt.Sprintf("%s-%d.json", snapshotFilenamePrefix, time.Now().UTC().UnixNano())
}

// Ensures that path points to a read/writable directory, creating it if necessary.
func ensureWritableDirectory(path string) error {
	// Attempt to create the requested directory, accepting that something might already be there.
	err := os.Mkdir(path, 0775)

	if err == nil {
		return nil // Skip the checks below, we just created it.
	} else if !os.IsExist(err) {
		return errors.Wrapf(err, "failed to create directory %s", path)
	}

	// Inspect existing directory.
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed to stat path \"%s\"", path)
	}
	if !stat.IsDir() {
		return errors.Errorf("%s is not a directory", path)
	}
	if (stat.Mode() & 0600) != 0600 {
		return errors.Errorf("insufficient permissions for path %s, got %04o need %04o", path, stat.Mode(), 0600)
	}
	return nil
}

// Tests whether the directory at path is empty
func isEmptyDir(path string) (bool, error) {
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(infos) == 0, nil
}

func fileExists(file string) (bool, error) {
	_, err := os.Stat(file)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
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

// JournalPath returns the path the journal is at.
func (r *FSRepo) JournalPath() string {
	return fmt.Sprintf("%s/journal.json", r.path)
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
