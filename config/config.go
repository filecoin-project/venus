package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
)

// Config is an in memory representation of the filecoin configuration file
type Config struct {
	API       *APIConfig       `json:"api"`
	Bootstrap *BootstrapConfig `json:"bootstrap"`
	Datastore *DatastoreConfig `json:"datastore"`
	Swarm     *SwarmConfig     `json:"swarm"`
	Mining    *MiningConfig    `json:"mining"`
	Wallet    *WalletConfig    `json:"wallet"`
	Heartbeat *HeartbeatConfig `json:"heartbeat"`
}

// APIConfig holds all configuration options related to the api.
type APIConfig struct {
	Address                       string   `json:"address"`
	AccessControlAllowOrigin      []string `json:"accessControlAllowOrigin"`
	AccessControlAllowCredentials bool     `json:"accessControlAllowCredentials"`
	AccessControlAllowMethods     []string `json:"accessControlAllowMethods"`
}

func newDefaultAPIConfig() *APIConfig {
	return &APIConfig{
		Address: "/ip4/127.0.0.1/tcp/3453",
		AccessControlAllowOrigin: []string{
			"http://localhost:8080",
			"https://localhost:8080",
			"http://127.0.0.1:8080",
			"https://127.0.0.1:8080",
		},
		AccessControlAllowMethods: []string{"GET", "POST", "PUT"},
	}
}

// DatastoreConfig holds all the configuration options for the datastore.
// TODO: use the advanced datastore configuration from ipfs
type DatastoreConfig struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

func newDefaultDatastoreConfig() *DatastoreConfig {
	return &DatastoreConfig{
		Type: "badgerds",
		Path: "badger",
	}
}

// SwarmConfig holds all configuration options related to the swarm.
type SwarmConfig struct {
	Address string `json:"address"`
}

func newDefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		Address: "/ip4/0.0.0.0/tcp/6000",
	}
}

// BootstrapConfig holds all configuration options related to bootstrap nodes
type BootstrapConfig struct {
	Relays           []string `json:"relays,omitempty"`
	Addresses        []string `json:"addresses"`
	MinPeerThreshold int      `json:"minPeerThreshold"`
	Period           string   `json:"period,omitempty"`
}

// TODO: provide bootstrap node addresses
func newDefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		Addresses:        []string{},
		MinPeerThreshold: 0, // TODO: we don't actually have an bootstrap peers yet.
		Period:           "1m",
	}
}

// MiningConfig holds all configuration options related to mining.
type MiningConfig struct {
	MinerAddress            address.Address `json:"minerAddress"`
	AutoSealIntervalSeconds uint            `json:"autoSealIntervalSeconds"`
}

func newDefaultMiningConfig() *MiningConfig {
	return &MiningConfig{
		MinerAddress:            address.Address{},
		AutoSealIntervalSeconds: 120,
	}
}

// WalletConfig holds all configuration options related to the wallet.
type WalletConfig struct {
	DefaultAddress address.Address `json:"defaultAddress,omitempty"`
}

func newDefaultWalletConfig() *WalletConfig {
	return &WalletConfig{
		DefaultAddress: address.Address{},
	}
}

// HeartbeatConfig holds all configuration options related to node heartbeat.
type HeartbeatConfig struct {
	// BeatTarget represents the address the filecoin node will send heartbeats to.
	BeatTarget string `json:"beatTarget"`
	// BeatPeriod represents how frequently heartbeats are sent.
	// Golang duration units are accepted.
	BeatPeriod string `json:"beatPeriod"`
	// ReconnectPeriod represents how long the node waits before attempting to reconnect.
	// Golang duration units are accepted.
	ReconnectPeriod string `json:"reconnectPeriod"`
	// Nickname represents the nickname of the filecoin node,
	Nickname string `json:"nickname"`
}

func newDefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		BeatTarget:      "",
		BeatPeriod:      "3s",
		ReconnectPeriod: "10s",
		Nickname:        "",
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
		Heartbeat: newDefaultHeartbeatConfig(),
	}
}

// WriteFile writes the config to the given filepath.
func (cfg *Config) WriteFile(file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close() // nolint: errcheck

	configString, err := json.MarshalIndent(*cfg, "", "\t")
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(f, string(configString))
	return err
}

// ReadFile reads a config file from disk.
func ReadFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	cfg := NewDefaultConfig()
	rawConfig, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(rawConfig) == 0 {
		return cfg, nil
	}

	err = json.Unmarshal(rawConfig, &cfg)
	if err != nil {
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
				jsonTag := strings.Split(
					v.Type().Field(i).Tag.Get("json"),
					",")[0]
				if jsonTag == keyTag {
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

// prependKey includes the JSON key in the jsonVal blob necessary for correct
// marshaling.  Ordinary tables require "[key]\n" prepended.  All others,
// including inline tables and arrays require "k = " prepended, where k is the
// last period separated substring of key. This function assumes all tables
// within an array are specified in inline format.
// TODO(dont merge): kill
func prependKey(jsonValue string, key string) string {
	ks := strings.Split(key, ".")
	k := ks[len(ks)-1]

	if !json.Valid([]byte(jsonValue)) {
		return fmt.Sprintf(`{ "%s": "%s" }`, k, jsonValue)
	}

	return fmt.Sprintf(`{ "%s": %s }`, k, jsonValue)
}

// fieldToSet calculates the reflector Value to set the config at the given key
// based on the user provided json blob.
func fieldToSet(key string, jsonVal string, fieldT reflect.Type) (reflect.Value, error) {
	// set up a struct with this field for unmarshaling
	jsonValKey := prependKey(jsonVal, key)
	ks := strings.Split(key, ".")
	k := ks[len(ks)-1]

	field := reflect.StructField{
		Name: "Field",
		Type: fieldT,
		Tag:  reflect.StructTag("json:\"" + k + "\""),
	}
	recvT := reflect.StructOf([]reflect.StructField{field})
	valToRecv := reflect.New(recvT)

	err := json.Unmarshal([]byte(jsonValKey), valToRecv.Interface())
	if err != nil {
		msg := fmt.Sprintf("input could not be marshaled to sub-config at: %s", key)
		return valToRecv, errors.Wrap(err, msg)
	}
	return valToRecv.Elem().Field(0), nil
}

// Set sets the config sub-struct referenced by `key`, e.g. 'api.address'
// or 'datastore' to the json key value pair encoded in jsonVal.  Note, Set
// only handles arrays of tables specified in inline format
func (cfg *Config) Set(key string, jsonVal string) (interface{}, error) {
	f := func(v reflect.Value, key string) (interface{}, error) {
		// dereference pointer types for marshaling
		setT := v.Type()
		var recvT reflect.Type
		if setT.Kind() == reflect.Ptr {
			recvT = setT.Elem()
		} else {
			recvT = setT
		}

		if key == "heartbeat.nickname" {
			match, _ := regexp.MatchString("^\"?[a-zA-Z]+\"?$", jsonVal)
			if !match {
				return nil, errors.New("node nickname must only contain letters")
			}
		}

		valToSet, err := fieldToSet(key, jsonVal, recvT)
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
