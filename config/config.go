package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

	"github.com/filecoin-project/go-filecoin/types"
)

// Config is an in memory representation of the filecoin configuration file
type Config struct {
	API       *APIConfig       `toml:"api"`
	Bootstrap *BootstrapConfig `toml:"bootstrap"`
	Datastore *DatastoreConfig `toml:"datastore"`
	Swarm     *SwarmConfig     `toml:"swarm"`
	Mining    *MiningConfig    `toml:"mining"`
	Wallet    *WalletConfig    `toml:"wallet"`
}

// APIConfig holds all configuration options related to the api.
type APIConfig struct {
	Address                       string   `toml:"address"`
	AccessControlAllowOrigin      []string `toml:"accessControlAllowOrigin"`
	AccessControlAllowCredentials bool     `toml:"accessControlAllowCredentials"`
	AccessControlAllowMethods     []string `toml:"accessControlAllowMethods"`
}

func newDefaultAPIConfig() *APIConfig {
	return &APIConfig{
		Address: ":3453",
		AccessControlAllowOrigin: []string{
			"http://localhost",
			"https://localhost",
			"http://127.0.0.1",
			"https://127.0.0.1",
		},
		AccessControlAllowMethods: []string{"GET", "POST", "PUT"},
	}
}

// DatastoreConfig holds all the configuration options for the datastore.
// TODO: use the advanced datastore configuration from ipfs
type DatastoreConfig struct {
	Type string `toml:"type"`
	Path string `toml:"path"`
}

func newDefaultDatastoreConfig() *DatastoreConfig {
	return &DatastoreConfig{
		Type: "badgerds",
		Path: "badger",
	}
}

// SwarmConfig holds all configuration options related to the swarm.
type SwarmConfig struct {
	Address string `toml:"address"`
}

func newDefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		Address: "/ip4/127.0.0.1/tcp/6000",
	}
}

// BootstrapConfig holds all configuration options related to bootstrap nodes
type BootstrapConfig struct {
	Addresses []string `toml:"addresses"`
}

// TODO: provide bootstrap node addresses
func newDefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		Addresses: []string{},
	}
}

// MiningConfig holds all configuration options related to mining.
type MiningConfig struct {
	RewardAddress  types.Address   `toml:"rewardAddress,omitempty"`
	MinerAddresses []types.Address `toml:"minerAddresses"`
}

func newDefaultMiningConfig() *MiningConfig {
	return &MiningConfig{
		MinerAddresses: make([]types.Address, 0),
	}
}

// WalletConfig holds all configuration options related to the wallet.
type WalletConfig struct {
	DefaultAddress types.Address `toml:"defaultAddress,omitempty"`
}

func newDefaultWalletConfig() *WalletConfig {
	return &WalletConfig{
		DefaultAddress: types.Address{},
	}
}

// NewDefaultConfig returns a config object with all the fields filled out to
// their default values
func NewDefaultConfig() *Config {
	return &Config{
		API:       newDefaultAPIConfig(),
		Bootstrap: newDefaultBootstrapConfig(),
		Datastore: newDefaultDatastoreConfig(),
		Swarm:     newDefaultSwarmConfig(),
		Mining:    newDefaultMiningConfig(),
		Wallet:    newDefaultWalletConfig(),
	}
}

// WriteFile writes the config to the given filepath.
func (cfg *Config) WriteFile(file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if err := toml.NewEncoder(f).Encode(*cfg); err != nil {
		return err
	}

	return f.Close()
}

// ReadFile reads a config file from disk.
func ReadFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	cfg := NewDefaultConfig()
	if _, err := toml.DecodeReader(f, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// traverseConfig contains the shared traversal logic for getting and setting
// config values.  It uses reflection to find the sub-struct referenced by `key`
// and applies a processing function to the referenced struct
func (cfg *Config) traverseConfig(key string,
	f func(reflect.Value, string) (interface{}, error)) (interface{}, error) {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	keyTags := strings.Split(key, ".")
OUTER:
	for j, keyTag := range keyTags {
		switch v.Type().Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				tomlTag := strings.Split(
					v.Type().Field(i).Tag.Get("toml"),
					",")[0]
				if tomlTag == keyTag {
					v = v.Field(i)
					if j == len(keyTags)-1 {
						return f(v, key)
					}
					v = reflect.Indirect(v) // only attempt one dereference
					continue OUTER
				}
			}
		case reflect.Array, reflect.Slice:
			i64, err := strconv.ParseUint(keyTag, 0, 0)
			if err != nil {
				return nil, fmt.Errorf("non-integer key into slice")
			}
			i := int(i64)
			if i > v.Len()-1 {
				return nil, fmt.Errorf("key into slice out of range")
			}
			v = v.Index(i)
			if j == len(keyTags)-1 {
				return f(v, key)
			}
			v = reflect.Indirect(v) // only attempt one dereference
			continue OUTER
		}

		return nil, fmt.Errorf("key: %s invalid for config", key)
	}
	// Cannot get here as len(strings.Split(s, sep)) >= 1 with non-empty sep
	return nil, fmt.Errorf("empty key is invalid")
}

// prependKey includes the TOML key in the tomlVal blob necessary for correct
// marshaling.  Ordinary tables require "[key]\n" prepended.  All others,
// including inline tables and arrays require "k = " prepended, where k is the
// last period separated substring of key. This function assumes all tables
// within an array are specified in inline format.
func prependKey(tomlVal string, key string, fieldT reflect.Type) string {
	ks := strings.Split(key, ".")
	k := ks[len(ks)-1]
	fieldK := fieldT.Kind()
	if fieldK == reflect.Ptr {
		fieldK = fieldT.Elem().Kind() // only attempt one dereference
	}

	switch fieldK {
	case reflect.Struct:
		tomlVal = strings.TrimSpace(tomlVal)
		// inline table
		if strings.HasPrefix(tomlVal, "{") {
			return fmt.Sprintf("%s=%s", k, tomlVal)
		}
		return fmt.Sprintf("[%s]\n%s", key, tomlVal)
	default:
		return fmt.Sprintf("%s=%s", k, tomlVal)
	}
}

// fieldToSet calculates the reflector Value to set the config at the given key
// based on the user provided toml blob.
func fieldToSet(key string, tomlVal string, fieldT reflect.Type) (reflect.Value, error) {
	// set up a struct with this field for unmarshaling
	tomlValKey := prependKey(tomlVal, key, fieldT)
	ks := strings.Split(key, ".")
	k := ks[len(ks)-1]

	field := reflect.StructField{
		Name: "Field",
		Type: fieldT,
		Tag:  reflect.StructTag("toml:" + "\"" + k + "\""),
	}
	recvT := reflect.StructOf([]reflect.StructField{field})
	valToRecv := reflect.New(recvT)

	_, err := toml.Decode(tomlValKey, valToRecv.Interface())
	if err != nil {
		msg := fmt.Sprintf("input could not be marshaled to sub-config at: %s", key)
		return valToRecv, errors.Wrap(err, msg)
	}
	return valToRecv.Elem().Field(0), nil
}

// Set sets the config sub-struct referenced by `key`, e.g. 'api.address'
// or 'datastore' to the toml key value pair encoded in tomlVal.  Note, Set
// only handles arrays of tables specified in inline format
func (cfg *Config) Set(key string, tomlVal string) (interface{}, error) {
	f := func(v reflect.Value, key string) (interface{}, error) {
		// dereference pointer types for marshaling
		setT := v.Type()
		var recvT reflect.Type
		if setT.Kind() == reflect.Ptr {
			recvT = setT.Elem()
		} else {
			recvT = setT
		}

		valToSet, err := fieldToSet(key, tomlVal, recvT)
		if err != nil {
			return nil, err
		}
		// add pointers back for setting
		if setT.Kind() == reflect.Ptr {
			valToSet = valToSet.Addr()
		}

		v.Set(valToSet)

		return v.Interface(), nil
	}

	return cfg.traverseConfig(key, f)
}

// Get gets the config sub-struct referenced by `key`, e.g. 'api.address'
func (cfg *Config) Get(key string) (interface{}, error) {
	f := func(v reflect.Value, key string) (interface{}, error) {
		return v.Interface(), nil
	}

	return cfg.traverseConfig(key, f)
}
