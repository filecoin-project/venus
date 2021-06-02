package fskeystore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("fskeystore")

// ErrNoSuchKey is returned if a key of the given name is not found in the store
var ErrNoSuchKey = xerrors.Errorf("no key by the given name was found")

// ErrKeyExists is returned when writing a key would overwrite an existing key
var ErrKeyExists = xerrors.Errorf("key by that name already exists, refusing to overwrite")

// ErrKeyFmt is returned when the key's format is invalid
var ErrKeyFmt = xerrors.Errorf("key has invalid format")

// FSKeystore is a keystore backed by files in a given directory stored on disk.
type FSKeystore struct {
	dir string
}

// NewFSKeystore returns a new filesystem keystore at directory `dir`
func NewFSKeystore(dir string) (*FSKeystore, error) {
	_, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}

	return &FSKeystore{dir}, nil
}

// Has returns whether or not a key exists in the Keystore
func (ks *FSKeystore) Has(name string) (bool, error) {
	kp := filepath.Join(ks.dir, name)

	_, err := os.Stat(kp)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if err := validateName(name); err != nil {
		return false, err
	}

	return true, nil
}

// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
func (ks *FSKeystore) Put(name string, data []byte) error {
	if err := validateName(name); err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	_, err := os.Stat(kp)
	if err == nil {
		return ErrKeyExists
	} else if !os.IsNotExist(err) {
		return err
	}

	fi, err := os.Create(kp)
	if err != nil {
		return err
	}
	defer func() {
		_ = fi.Close()
	}()

	_, err = fi.Write(data)

	return err
}

// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
// otherwise.
func (ks *FSKeystore) Get(name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	kp := filepath.Join(ks.dir, name)

	data, err := ioutil.ReadFile(kp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSuchKey
		}
		return nil, err
	}

	return data, nil
}

// Delete removes a key from the Keystore
func (ks *FSKeystore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	return os.Remove(kp)
}

// List returns a list of key identifiers
func (ks *FSKeystore) List() ([]string, error) {
	dir, err := os.Open(ks.dir)
	if err != nil {
		return nil, err
	}

	dirs, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(dirs))

	for _, name := range dirs {
		err := validateName(name)
		if err == nil {
			list = append(list, name)
		} else {
			log.Warnf("Ignoring the invalid keyfile: %s", name)
		}
	}

	return list, nil
}

func validateName(name string) error {
	if name == "" {
		return xerrors.Errorf("key names must be at least one character: %v", ErrKeyFmt)
	}

	if strings.Contains(name, "/") {
		return xerrors.Errorf("key names may not contain slashes: %v", ErrKeyFmt)
	}

	if strings.HasPrefix(name, ".") {
		return xerrors.Errorf("key names may not begin with a period: %v", ErrKeyFmt)
	}

	return nil
}
