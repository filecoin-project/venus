package fskeystore

type keyMap map[string][]byte

// MemKeystore is a keystore backed by an in-memory map
type MemKeystore struct {
	values keyMap
}

// NewMemKeystore returns a new map-based keystore
func NewMemKeystore() *MemKeystore {
	return &MemKeystore{
		values: keyMap{},
	}
}

// Has returns whether or not a key exists in the keystore map
func (mk *MemKeystore) Has(k string) (bool, error) {
	if err := validateName(k); err != nil {
		return false, err
	}
	_, found := mk.values[k]
	return found, nil
}

// Put stores a key in the map, if a key with the same name already exists,
// returns ErrKeyExists
func (mk *MemKeystore) Put(k string, b []byte) error {
	if err := validateName(k); err != nil {
		return err
	}

	_, found := mk.values[k]
	if found {
		return ErrKeyExists
	}

	mk.values[k] = b
	return nil
}

// Get retrieves a key from the map if it exists, else it returns ErrNoSuchKey
func (mk *MemKeystore) Get(k string) ([]byte, error) {
	if err := validateName(k); err != nil {
		return nil, err
	}

	v, found := mk.values[k]
	if !found {
		return nil, ErrNoSuchKey
	}

	return v, nil
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

// List returns a list of key identifiers in random order
func (mk *MemKeystore) List() ([]string, error) {
	out := make([]string, 0, len(mk.values))
	for k := range mk.values {
		err := validateName(k)
		if err == nil {
			out = append(out, k)
		} else {
			log.Warningf("ignoring the invalid keyfile: %s", k)
		}
	}
	return out, nil
}
