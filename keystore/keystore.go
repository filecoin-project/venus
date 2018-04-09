package keystore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

// Keystore provides a key management interface
type Keystore interface {
	// Has returns whether or not a key exist in the Keystore
	Has(string) (bool, error)
	// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
	Put(string, ci.PrivKey) error
	// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
	// otherwise.
	Get(string) (ci.PrivKey, error)
	// Delete removes a key from the Keystore
	Delete(string) error
	// List returns a list of key identifier
	List() ([]string, error)
}

var ErrNoSuchKey = fmt.Errorf("no key by the given name was found")                     // nolint: golint
var ErrKeyExists = fmt.Errorf("key by that name already exists, refusing to overwrite") // nolint: golint
var ErrKeyFmt = fmt.Errorf("Key has invalid format")                                    // nolint: golint

type keyMap map[string]ci.PrivKey

// MemKeystore is a keystore backed by an in-memory map
type MemKeystore struct {
	values keyMap
}

// NewMemKeystore returns a new map-based keystore
func NewMemKeystore() (mk *MemKeystore) {
	return &MemKeystore{
		values: keyMap{},
	}
}

// Has returns whether or not a key exist in the keystore map
func (mk *MemKeystore) Has(k string) (bool, error) {
	if err := validateName(k); err != nil {
		return false, err
	}
	_, found := mk.values[k]
	return found, nil
}

// Put stores a key in the map, if a key with the same name already exists, returns ErrKeyExists
func (mk *MemKeystore) Put(k string, v ci.PrivKey) error {
	if err := validateName(k); err != nil {
		return err
	}
	_, found := mk.values[k]
	if found {
		return ErrKeyExists
	}
	mk.values[k] = v
	return nil
}

// Get retrieves a key from the mape if it exists, and returns ErrNoSuchKey
// otherwise.
func (mk *MemKeystore) Get(k string) (ci.PrivKey, error) {
	if err := validateName(k); err != nil {
		return nil, err
	}
	val, found := mk.values[k]
	if !found {
		return nil, ErrNoSuchKey
	}
	return val, nil

}

// Delete removes a key from the map
func (mk *MemKeystore) Delete(k string) error {
	if err := validateName(k); err != nil {
		return err
	}
	if _, found := mk.values[k]; !found {
		return ErrNoSuchKey
	}
	delete(mk.values, k)
	return nil
}

// List returns a list of key identifier in random order
func (mk *MemKeystore) List() ([]string, error) {
	list := make([]string, 0, len(mk.values))
	for k := range mk.values {
		err := validateName(k)
		if err == nil {
			list = append(list, k)
		} else {
			// TODO log a warning here
			fmt.Printf("Ignoring the invalid keyfile: %s", k) // nolint: vet
		}
	}
	return list, nil
}

// FSKeystore is a keystore backed by files in a given directory stored on disk.
type FSKeystore struct {
	dir string
}

func validateName(name string) error {
	if name == "" {
		return errors.Wrap(ErrKeyFmt, "key names must be at least one character")
	}

	if strings.Contains(name, "/") {
		return errors.Wrap(ErrKeyFmt, "key names may not contain slashes")
	}

	if strings.HasPrefix(name, ".") {
		return errors.Wrap(ErrKeyFmt, "key names may not begin with a period")
	}

	return nil
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

// Has returns whether or not a key exist in the Keystore
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
func (ks *FSKeystore) Put(name string, k ci.PrivKey) error {
	if err := validateName(name); err != nil {
		return err
	}

	b, err := k.Bytes()
	if err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	_, err = os.Stat(kp)
	if err == nil {
		return ErrKeyExists
	} else if !os.IsNotExist(err) {
		return err
	}

	fi, err := os.Create(kp)
	if err != nil {
		return err
	}
	defer fi.Close() // nolint: errcheck

	_, err = fi.Write(b)

	return err
}

// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
// otherwise.
func (ks *FSKeystore) Get(name string) (ci.PrivKey, error) {
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

	pk, err := ci.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

// Delete removes a key from the Keystore
func (ks *FSKeystore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	kp := filepath.Join(ks.dir, name)

	return os.Remove(kp)
}

// List return a list of key identifier
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
			// TODO log a warning here
			fmt.Printf("Ignoring the invalid keyfile: %s", name) // nolint: vet, megacheck
		}
	}

	return list, nil
}
