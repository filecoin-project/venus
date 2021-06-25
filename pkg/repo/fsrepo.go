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

	"github.com/filecoin-project/venus/pkg/repo/fskeystore"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-multistore"
	badgerds "github.com/ipfs/go-ds-badger2"
	lockfile "github.com/ipfs/go-fs-lock"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/config"
)

// Version is the version of repo schema that this code understands.
const LatestVersion uint = 3

const (
	// apiFile is the filename containing the filecoin node's api address.
	apiToken               = "token"
	apiFile                = "api"
	configFilename         = "config.json"
	tempConfigFilename     = ".config.json.temp"
	lockFile               = "repo.lock"
	versionFilename        = "version"
	walletDatastorePrefix  = "wallet"
	chainDatastorePrefix   = "chain"
	metaDatastorePrefix    = "metadata"
	paychDatastorePrefix   = "paych"
	snapshotStorePrefix    = "snapshots"
	snapshotFilenamePrefix = "snapshot"
	dataTransfer           = "data-transfer"
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

	ds        *blockstoreutil.BadgerBlockstore
	stagingDs Datastore
	mds       *multistore.MultiStore
	keystore  fskeystore.Keystore
	walletDs  Datastore
	chainDs   Datastore
	metaDs    Datastore
	//marketDs  Datastore
	paychDs Datastore
	// lockfile is the file system lock to prevent others from opening the same repo.
	lockfile io.Closer
}

var _ Repo = (*FSRepo)(nil)

// InitFSRepo initializes a new repo at the target path with the provided configuration.
// The successful result creates a symlink at targetPath pointing to a sibling directory
// named with a timestamp and repo version number.
// The link path must be empty prior. If the computed actual directory exists, it must be empty.
func InitFSRepo(targetPath string, version uint, cfg *config.Config) error {
	repoPath, err := homedir.Expand(targetPath)
	if err != nil {
		return err
	}

	if repoPath == "" { // path contained no separator
		repoPath = "./"
	}

	exists, err := fileExists(repoPath)
	if err != nil {
		return errors.Wrapf(err, "error inspecting repo path %s", repoPath)
	} else if exists {
		return errors.Errorf("repo at %s, file exists", repoPath)
	}

	// Create the actual directory and then the link to it.
	if err = InitFSRepoDirect(repoPath, version, cfg); err != nil {
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
	if err := initDataTransfer(repoPath); err != nil {
		return errors.Wrap(err, "initializing data-transfer directory failed")
	}
	return nil
}

func Exists(repoPath string) (bool, error) {
	_, err := os.Stat(filepath.Join(repoPath, walletDatastorePrefix))
	notExist := os.IsNotExist(err)
	if notExist {
		err = nil

		_, err = os.Stat(filepath.Join(repoPath, configFilename))
		notExist = os.IsNotExist(err)
		if notExist {
			err = nil
		}
	}
	return !notExist, err
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
		return nil, errors.Errorf("no repo found at %s; run: 'venus init [--repodir=%s]'", repoPath, repoPath)
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

	if localVersion > r.version {
		return fmt.Errorf("binary needs update to handle repo version, got %d expected %d. Update binary to latest release", localVersion, LatestVersion)
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

	if err := r.openMetaDatastore(); err != nil {
		return errors.Wrap(err, "failed to open metadata datastore")
	}

	if err := r.openMultiStore(); err != nil {
		return errors.Wrap(err, "failed to open staging datastore")
	}

	if err := r.openPaychDataStore(); err != nil {
		return errors.Wrap(err, "failed to open paych datastore")
	}

	/*if err := r.openMarketDataStore(); err != nil {
		return errors.Wrap(err, "failed to open market datastore")
	}*/
	return nil
}

// configModule returns the configuration object.
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
func (r *FSRepo) Datastore() blockstoreutil.Blockstore {
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

func (r *FSRepo) MetaDatastore() Datastore {
	return r.metaDs
}

/*func (r *FSRepo) MarketDatastore() Datastore {
	return r.marketDs
}*/

func (r *FSRepo) PaychDatastore() Datastore {
	return r.paychDs
}

// Version returns the version of the repo
func (r *FSRepo) Version() uint {
	return r.version
}

// Keystore returns the keystore
func (r *FSRepo) Keystore() fskeystore.Keystore {
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

	if err := r.metaDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close meta datastore")
	}

	if err := r.mds.Close(); err != nil {
		return errors.Wrap(err, "failed to close mds datastore")
	}

	if err := r.stagingDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close stagingDs datastore")
	}

	if err := r.paychDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close paych datastore")
	}

	/*if err := r.marketDs.Close(); err != nil {
		return errors.Wrap(err, "failed to close market datastore")
	}*/

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
	return ReadVersion(r.path)
}

func (r *FSRepo) openDatastore() error {
	switch r.cfg.Datastore.Type {
	case "badgerds":
		path := filepath.Join(r.path, r.cfg.Datastore.Path)
		opts, err := blockstoreutil.BadgerBlockstoreOptions(path, false)
		if err != nil {
			return err
		}
		opts.Prefix = bstore.BlockPrefix.String()
		ds, err := blockstoreutil.Open(opts)
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

	ks, err := fskeystore.NewFSKeystore(ksp)
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

func (r *FSRepo) openMetaDatastore() error {
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, metaDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}

	r.metaDs = ds

	return nil
}
func (r *FSRepo) openPaychDataStore() error {
	var err error
	r.paychDs, err = badgerds.NewDatastore(filepath.Join(r.path, paychDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}
	return nil
}

/*func (r *FSRepo) openMarketDataStore() error {
	var err error
	r.marketDs, err = badgerds.NewDatastore(filepath.Join(r.path, marketDatastoreProfix), badgerOptions())
	if err != nil {
		return err
	}
	return nil
}*/
func (r *FSRepo) openWalletDatastore() error {
	// TODO: read wallet datastore info from config, use that to open it up
	ds, err := badgerds.NewDatastore(filepath.Join(r.path, walletDatastorePrefix), badgerOptions())
	if err != nil {
		return err
	}

	r.walletDs = ds

	return nil
}

func (r *FSRepo) openMultiStore() error {
	var err error
	r.stagingDs, err = badgerds.NewDatastore(filepath.Join(r.path, "/staging"), badgerOptions())
	if err != nil {
		return err
	}

	mds, err := multistore.NewMultiDstore(r.stagingDs)
	if err != nil {
		return err
	}
	r.mds = mds
	return nil
}

// WriteVersion writes the given version to the repo version file.
func WriteVersion(p string, version uint) error {
	return ioutil.WriteFile(filepath.Join(p, versionFilename), []byte(strconv.Itoa(int(version))), 0644)
}

// ReadVersion returns the unparsed (string) version
// from the version file in the specified repo.
func ReadVersion(repoPath string) (uint, error) {
	file, err := ioutil.ReadFile(filepath.Join(repoPath, versionFilename))
	if err != nil {
		return 0, err
	}
	verStr := strings.Trim(string(file), "\n")
	version, err := strconv.ParseUint(verStr, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(version), nil
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

func initDataTransfer(p string) error {
	dataTransferDir := filepath.Join(p, dataTransfer)
	state, err := os.Stat(dataTransferDir)
	if err == nil {
		if state.IsDir() {
			return nil
		}
		return errors.New("error must be a directory")
	}
	if !os.IsNotExist(err) {
		return err
	}
	//create data-transfer state
	return os.MkdirAll(dataTransferDir, 0777)
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
	return apiAddrFromFile(repoPath)
}

// APIAddrFromRepoPath returns the token from the filecoin repo
func APITokenFromRepoPath(repoPath string) (string, error) {
	repoPath, err := homedir.Expand(repoPath)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("can't resolve local repo path %s", repoPath))
	}
	return apiTokenFromFile(repoPath)
}

// APIAddrFromFile reads the address from the API file at the given path.
// A relevant comment from a similar function at go-ipfs/repo/fsrepo/fsrepo.go:
// This is a concurrent operation, meaning that any process may read this file.
// Modifying this file, therefore, should use "mv" to replace the whole file
// and avoid interleaved read/writes
func apiAddrFromFile(repoPath string) (string, error) {
	jsonrpcFile := filepath.Join(repoPath, apiFile)
	jsonrpcAPI, err := ioutil.ReadFile(jsonrpcFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read API file")
	}

	return string(jsonrpcAPI), nil
}

// apiTokenFromFile reads the token from the token file at the given path.
func apiTokenFromFile(repoPath string) (string, error) {
	tokenFile := filepath.Join(repoPath, apiToken)
	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read API file")
	}

	return strings.TrimSpace(string(token)), nil
}

// APIAddr reads the FSRepo's api file and returns the api address
func (r *FSRepo) APIAddr() (string, error) {
	return apiAddrFromFile(filepath.Clean(r.path))
}

func (r *FSRepo) SetAPIToken(token []byte) error {
	return ioutil.WriteFile(filepath.Join(r.path, "token"), token, 0600)
}

func (r *FSRepo) APIToken() (string, error) {
	tkBuff, err := ioutil.ReadFile(filepath.Join(r.path, "token"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(tkBuff)), nil
}

func badgerOptions() *badgerds.Options {
	result := &badgerds.DefaultOptions
	result.Truncate = true
	result.MaxTableSize = 64 << 21
	return result
}

func (r *FSRepo) Repo() Repo {
	return r
}
