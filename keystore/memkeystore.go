package keystore

import (
	"fmt"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

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
