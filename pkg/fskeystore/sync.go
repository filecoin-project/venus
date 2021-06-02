package fskeystore

import (
	"sync"
)

// MutexKeystore contains a child keystore and a mutex.
// used for coarse sync
type MutexKeystore struct {
	sync.RWMutex

	child Keystore
}

// MutexWrap constructs a keystore with a coarse lock around
// the entire keystore, for every single operation
func MutexWrap(k Keystore) *MutexKeystore {
	return &MutexKeystore{child: k}
}

// Children implements Shim
func (mk *MutexKeystore) Children() []Keystore {
	return []Keystore{mk.child}
}

// Put implements Keystore.Put
func (mk *MutexKeystore) Put(k string, data []byte) error {
	mk.Lock()
	defer mk.Unlock()
	return mk.child.Put(k, data)
}

// Get implements Keystore.Get
func (mk *MutexKeystore) Get(k string) ([]byte, error) {
	mk.RLock()
	defer mk.RUnlock()
	return mk.child.Get(k)
}

// Has implements Keystore.Has
func (mk *MutexKeystore) Has(k string) (bool, error) {
	mk.RLock()
	defer mk.RUnlock()
	return mk.child.Has(k)
}

// Delete implements Keystore.Delete
func (mk *MutexKeystore) Delete(k string) error {
	mk.Lock()
	defer mk.Unlock()
	return mk.child.Delete(k)
}

// List implements Keystore.List
func (mk *MutexKeystore) List() ([]string, error) {
	mk.RLock()
	defer mk.RUnlock()
	return mk.child.List()
}
