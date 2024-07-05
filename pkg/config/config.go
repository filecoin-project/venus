package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

const (
	scryptN = 1 << 21
	scryptP = 1
)

var DefaultDefaultMaxFee = types.MustParseFIL("10")

// Config is an in memory representation of the filecoin configuration file
type Config struct {
	API           *APIConfig           `json:"api"`
	Bootstrap     *BootstrapConfig     `json:"bootstrap"`
	Datastore     *DatastoreConfig     `json:"datastore"`
	Mpool         *MessagePoolConfig   `json:"mpool"`
	NetworkParams *NetworkParamsConfig `json:"parameters"`
	Observability *ObservabilityConfig `json:"observability"`
	Swarm         *SwarmConfig         `json:"swarm"`
	Wallet        *WalletConfig        `json:"walletModule"`
	SlashFilterDs *SlashFilterDsConfig `json:"slashFilter"`
	RateLimitCfg  *RateLimitCfg        `json:"rateLimit"`
	FevmConfig    *FevmConfig          `json:"fevm"`
	EventsConfig  *EventsConfig        `json:"events"`
	PubsubConfig  *PubsubConfig        `json:"pubsub"`
	FaultReporter *FaultReporterConfig `json:"faultReporter"`
}

// APIConfig holds all configuration options related to the api.
// nolint
type APIConfig struct {
	VenusAuthURL                  string   `json:"venusAuthURL"`
	VenusAuthToken                string   `json:"venusAuthToken"`
	APIAddress                    string   `json:"apiAddress"`
	AccessControlAllowOrigin      []string `json:"accessControlAllowOrigin"`
	AccessControlAllowCredentials bool     `json:"accessControlAllowCredentials"`
	AccessControlAllowMethods     []string `json:"accessControlAllowMethods"`
}

type RateLimitCfg struct {
	Endpoint string `json:"RedisEndpoint"`
	User     string `json:"user"`
	Pwd      string `json:"pwd"`
	Enable   bool   `json:"enable"`
}

func newDefaultAPIConfig() *APIConfig {
	return &APIConfig{
		APIAddress: "/ip4/127.0.0.1/tcp/3453",
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

// Validators hold the list of validation functions for each configuration
// property. Validators must take a key and json string respectively as
// arguments, and must return either an error or nil depending on whether or not
// the given key and value are valid. Validators will only be run if a property
// being set matches the name given in this map.
var Validators = map[string]func(string, string) error{
	"heartbeat.nickname": validateLettersOnly,
}

func newDefaultDatastoreConfig() *DatastoreConfig {
	return &DatastoreConfig{
		Type: "badgerds",
		Path: "badger",
	}
}

// SwarmConfig holds all configuration options related to the swarm.
type SwarmConfig struct {
	Address            string `json:"address"`
	PublicRelayAddress string `json:"public_relay_address,omitempty"`

	ProtectedPeers []string `json:"protectedPeers"`
	//ConnMgrLow is the number of connections that the basic connection manager
	// will trim down to.
	ConnMgrLow uint `json:"connMgrLow"`

	// ConnMgrHigh is the number of connections that, when exceeded, will trigger
	// a connection GC operation. Note: protected/recently formed connections don't
	// count towards this limit.
	ConnMgrHigh uint `json:"connMgrHigh"`

	// ConnMgrGrace is a time duration that new connections are immune from being
	// closed by the connection manager.
	ConnMgrGrace Duration `json:"connMgrGrace"`
}

func newDefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		Address:      "/ip4/0.0.0.0/tcp/0",
		ConnMgrLow:   150,
		ConnMgrHigh:  180,
		ConnMgrGrace: Duration(20 * time.Second),
	}
}

// BootstrapConfig holds all configuration options related to bootstrap nodes
type BootstrapConfig struct {
	Addresses []string `json:"addresses"`
	Period    string   `json:"period,omitempty"`
}

func (bsc *BootstrapConfig) AddPeers(peers ...string) {
	filter := map[string]struct{}{}
	for _, peer := range bsc.Addresses {
		filter[peer] = struct{}{}
	}

	notInclude := []string{}
	for _, peer := range peers {
		_, has := filter[peer]
		if has {
			continue
		}
		filter[peer] = struct{}{}
		notInclude = append(notInclude, peer)
	}
	bsc.Addresses = append(bsc.Addresses, notInclude...)
}

// TODO: provide bootstrap node addresses
func newDefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		Addresses: []string{},
		Period:    "1m",
	}
}

// WalletConfig holds all configuration options related to the wallet.
type WalletConfig struct {
	DefaultAddress   address.Address  `json:"defaultAddress,omitempty"`
	PassphraseConfig PassphraseConfig `json:"passphraseConfig,omitempty"`
	RemoteEnable     bool             `json:"remoteEnable"`
	RemoteBackend    string           `json:"remoteBackend"`
}

type PassphraseConfig struct {
	ScryptN int `json:"scryptN"`
	ScryptP int `json:"scryptP"`
}

func newDefaultWalletConfig() *WalletConfig {
	return &WalletConfig{
		DefaultAddress:   address.Undef,
		PassphraseConfig: DefaultPassphraseConfig(),
	}
}

func DefaultPassphraseConfig() PassphraseConfig {
	return PassphraseConfig{
		ScryptN: scryptN,
		ScryptP: scryptP,
	}
}

func TestPassphraseConfig() PassphraseConfig {
	return PassphraseConfig{
		ScryptN: 1 << 15,
		ScryptP: scryptP,
	}
}

// DrandConfig holds all configuration options related to pulling randomness from Drand servers
type DrandConfig struct {
	StartTimeUnix int64 `json:"startTimeUnix"`
	RoundSeconds  int   `json:"roundSeconds"`
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

// ObservabilityConfig is a container for configuration related to observables.
type ObservabilityConfig struct {
	Metrics *MetricsConfig `json:"metrics"`
	Tracing *TraceConfig   `json:"tracing"`
}

func newDefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		Metrics: newDefaultMetricsConfig(),
		Tracing: newDefaultTraceConfig(),
	}
}

// MetricsConfig holds all configuration options related to node metrics.
type MetricsConfig struct {
	// Enabled will enable prometheus metrics when true.
	PrometheusEnabled bool `json:"prometheusEnabled"`
	// ReportInterval represents how frequently filecoin will update its prometheus metrics.
	ReportInterval string `json:"reportInterval"`
	// PrometheusEndpoint represents the address filecoin will expose prometheus metrics at.
	PrometheusEndpoint string `json:"prometheusEndpoint"`
}

func newDefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		PrometheusEnabled:  false,
		ReportInterval:     "5s",
		PrometheusEndpoint: "/ip4/0.0.0.0/tcp/9400",
	}
}

// TraceConfig holds all configuration options related to enabling and exporting
// filecoin node traces.
type TraceConfig struct {
	// JaegerTracingEnabled will enable exporting traces to jaeger when true.
	JaegerTracingEnabled bool `json:"jaegerTracingEnabled"`
	// ProbabilitySampler will sample fraction of traces, 1.0 will sample all traces.
	ProbabilitySampler float64 `json:"probabilitySampler"`
	// JaegerEndpoint is the URL traces are collected on.
	JaegerEndpoint string `json:"jaegerEndpoint"`
	ServerName     string `json:"servername"`
}

func newDefaultTraceConfig() *TraceConfig {
	return &TraceConfig{
		JaegerEndpoint:       "localhost:6831",
		JaegerTracingEnabled: false,
		ProbabilitySampler:   1.0,
		ServerName:           "venus-node",
	}
}

// MessagePoolConfig holds all configuration options related to nodes message pool (mpool).
type MessagePoolConfig struct {
	// MaxNonceGap is the maximum nonce of a message past the last received on chain
	MaxNonceGap uint64 `json:"maxNonceGap"`
	// MaxFee
	MaxFee types.FIL `json:"maxFee"`
}

var DefaultMessagePoolParam = &MessagePoolConfig{
	MaxNonceGap: 100,
	MaxFee:      DefaultDefaultMaxFee,
}

func newDefaultMessagePoolConfig() *MessagePoolConfig {
	return &MessagePoolConfig{
		MaxNonceGap: 100,
		MaxFee:      DefaultDefaultMaxFee,
	}
}

// NetworkParamsConfig record netork parameters
type NetworkParamsConfig struct {
	DevNet                  bool                         `json:"-"`
	NetworkType             types.NetworkType            `json:"networkType"`
	AddressNetwork          address.Network              `json:"-"`
	GenesisNetworkVersion   network.Version              `json:"-"`
	ConsensusMinerMinPower  uint64                       `json:"-"` // uint64 goes up to 18 EiB
	MinVerifiedDealSize     int64                        `json:"-"`
	ReplaceProofTypes       []abi.RegisteredSealProof    `json:"-"`
	BlockDelay              uint64                       `json:"-"`
	DrandSchedule           map[abi.ChainEpoch]DrandEnum `json:"-"`
	ForkUpgradeParam        *ForkUpgradeConfig           `json:"-"`
	PreCommitChallengeDelay abi.ChainEpoch               `json:"-"`
	PropagationDelaySecs    uint64                       `json:"-"`
	AllowableClockDriftSecs uint64                       `json:"allowableClockDriftSecs"`
	// ChainId defines the chain ID used in the Ethereum JSON-RPC endpoint.
	// As per https://github.com/ethereum-lists/chains
	Eip155ChainID int `json:"-"`
	// NOTE: DO NOT change this unless you REALLY know what you're doing. This is consensus critical.
	ActorDebugging   bool           `json:"-"`
	F3Enabled        bool           `json:"f3Enabled"`
	F3BootstrapEpoch abi.ChainEpoch `json:"f3BootstrapEpoch"`
	ManifestServerID string         `json:"manifestServerID"`
}

// ForkUpgradeConfig record upgrade parameters
type ForkUpgradeConfig struct {
	UpgradeSmokeHeight                abi.ChainEpoch `json:"upgradeSmokeHeight"`
	UpgradeBreezeHeight               abi.ChainEpoch `json:"upgradeBreezeHeight"`
	UpgradeIgnitionHeight             abi.ChainEpoch `json:"upgradeIgnitionHeight"`
	UpgradeLiftoffHeight              abi.ChainEpoch `json:"upgradeLiftoffHeight"`
	UpgradeAssemblyHeight             abi.ChainEpoch `json:"upgradeActorsV2Height"`
	UpgradeRefuelHeight               abi.ChainEpoch `json:"upgradeRefuelHeight"`
	UpgradeTapeHeight                 abi.ChainEpoch `json:"upgradeTapeHeight"`
	UpgradeKumquatHeight              abi.ChainEpoch `json:"upgradeKumquatHeight"`
	UpgradePriceListOopsHeight        abi.ChainEpoch `json:"upgradePriceListOopsHeight"`
	BreezeGasTampingDuration          abi.ChainEpoch `json:"breezeGasTampingDuration"`
	UpgradeCalicoHeight               abi.ChainEpoch `json:"upgradeCalicoHeight"`
	UpgradePersianHeight              abi.ChainEpoch `json:"upgradePersianHeight"`
	UpgradeOrangeHeight               abi.ChainEpoch `json:"upgradeOrangeHeight"`
	UpgradeClausHeight                abi.ChainEpoch `json:"upgradeClausHeight"`
	UpgradeTrustHeight                abi.ChainEpoch `json:"upgradeActorsV3Height"`
	UpgradeNorwegianHeight            abi.ChainEpoch `json:"upgradeNorwegianHeight"`
	UpgradeTurboHeight                abi.ChainEpoch `json:"upgradeActorsV4Height"`
	UpgradeHyperdriveHeight           abi.ChainEpoch `json:"upgradeHyperdriveHeight"`
	UpgradeChocolateHeight            abi.ChainEpoch `json:"upgradeChocolateHeight"`
	UpgradeOhSnapHeight               abi.ChainEpoch `json:"upgradeOhSnapHeight"`
	UpgradeSkyrHeight                 abi.ChainEpoch `json:"upgradeSkyrHeight"`
	UpgradeSharkHeight                abi.ChainEpoch `json:"upgradeSharkHeight"`
	UpgradeHyggeHeight                abi.ChainEpoch `json:"upgradeHyggeHeight"`
	UpgradeLightningHeight            abi.ChainEpoch `json:"upgradeLightningHeight"`
	UpgradeThunderHeight              abi.ChainEpoch `json:"upgradeThunderHeight"`
	UpgradeWatermelonHeight           abi.ChainEpoch `json:"upgradeWatermelonHeight"`
	UpgradeWatermelonFixHeight        abi.ChainEpoch `json:"upgradeWatermelonFixHeight"`
	UpgradeWatermelonFix2Height       abi.ChainEpoch `json:"upgradeWatermelonFix2Height"`
	UpgradeDragonHeight               abi.ChainEpoch `json:"upgradeDragonHeight"`
	UpgradePhoenixHeight              abi.ChainEpoch `json:"upgradePhoenixHeight"`
	UpgradeCalibrationDragonFixHeight abi.ChainEpoch `json:"upgradeCalibrationDragonFixHeight"`
	UpgradeWaffleHeight               abi.ChainEpoch `json:"upgradeWaffleHeight"`
}

func IsNearUpgrade(epoch, upgradeEpoch abi.ChainEpoch) bool {
	return epoch > upgradeEpoch-constants.Finality && epoch < upgradeEpoch+constants.Finality
}

var DefaultForkUpgradeParam = &ForkUpgradeConfig{
	UpgradeBreezeHeight:      41280,
	BreezeGasTampingDuration: 120,
	UpgradeSmokeHeight:       51000,
	UpgradeIgnitionHeight:    94000,
	UpgradeRefuelHeight:      130800,
	UpgradeTapeHeight:        140760,
	UpgradeLiftoffHeight:     148888,
	UpgradeKumquatHeight:     170000,
	UpgradeCalicoHeight:      265200,
	UpgradePersianHeight:     265200 + 120*60,
	UpgradeAssemblyHeight:    138720,
	UpgradeOrangeHeight:      336458,
	UpgradeClausHeight:       343200,
	UpgradeTrustHeight:       550321,
	UpgradeNorwegianHeight:   665280,
	UpgradeTurboHeight:       712320,
	UpgradeHyperdriveHeight:  892800,
	UpgradeChocolateHeight:   1231620,
	UpgradeOhSnapHeight:      1594680,
	UpgradeSkyrHeight:        1960320,
	UpgradeSharkHeight:       2383680,
	UpgradeHyggeHeight:       2683348,
	UpgradeLightningHeight:   2809800,
	UpgradeThunderHeight:     2809800 + 2880*21,
	UpgradeWatermelonHeight:  3431940,
	// This fix upgrade only ran on calibrationnet
	UpgradeWatermelonFixHeight: -1,
	// This fix upgrade only ran on calibrationnet
	UpgradeWatermelonFix2Height: -2,
	UpgradeDragonHeight:         3855360,
	UpgradePhoenixHeight:        3855360 + 120,
	// This fix upgrade only ran on calibrationnet
	UpgradeCalibrationDragonFixHeight: -3,
	UpgradeWaffleHeight:               999999999999,
}

func newDefaultNetworkParamsConfig() *NetworkParamsConfig {
	defaultParams := *DefaultForkUpgradeParam
	return &NetworkParamsConfig{
		DevNet:                 true,
		ConsensusMinerMinPower: 0, // 0 means don't override the value
		ReplaceProofTypes: []abi.RegisteredSealProof{
			abi.RegisteredSealProof_StackedDrg2KiBV1,
			abi.RegisteredSealProof_StackedDrg512MiBV1,
			abi.RegisteredSealProof_StackedDrg32GiBV1,
			abi.RegisteredSealProof_StackedDrg64GiBV1,
		},
		DrandSchedule:           map[abi.ChainEpoch]DrandEnum{0: 5, -1: 1},
		ForkUpgradeParam:        &defaultParams,
		PropagationDelaySecs:    10,
		AllowableClockDriftSecs: 1,
		Eip155ChainID:           314,
	}
}

type MySQLConfig struct {
	ConnectionString string        `json:"connectionString"`
	MaxOpenConn      int           `json:"maxOpenConn"`     // 100
	MaxIdleConn      int           `json:"maxIdleConn"`     // 10
	ConnMaxLifeTime  time.Duration `json:"connMaxLifeTime"` // minuter: 60
	Debug            bool          `json:"debug"`
}

type SlashFilterDsConfig struct {
	Type  string      `json:"type"`
	MySQL MySQLConfig `json:"mysql"`
}

func newDefaultSlashFilterDsConfig() *SlashFilterDsConfig {
	return &SlashFilterDsConfig{
		Type:  "local",
		MySQL: MySQLConfig{},
	}
}

func newRateLimitConfig() *RateLimitCfg {
	return &RateLimitCfg{
		Enable: false,
	}
}

type EventConfig struct {
	// DisableRealTimeFilterAPI will disable the RealTimeFilterAPI that can create and query filters for actor events as they are emitted.
	// The API is enabled when EnableEthRPC or Events.EnableActorEventsAPI is true, but can be disabled selectively with this flag.
	DisableRealTimeFilterAPI bool `json:"disableRealTimeFilterAPI"`

	// DisableHistoricFilterAPI will disable the HistoricFilterAPI that can create and query filters for actor events
	// that occurred in the past. HistoricFilterAPI maintains a queryable index of events.
	// The API is enabled when EnableEthRPC or Events.EnableActorEventsAPI is true, but can be disabled selectively with this flag.
	DisableHistoricFilterAPI bool `json:"disableHistoricFilterAPI"`

	// FilterTTL specifies the time to live for actor event filters. Filters that haven't been accessed longer than
	// this time become eligible for automatic deletion.
	FilterTTL Duration `json:"filterTTL"`

	// MaxFilters specifies the maximum number of filters that may exist at any one time.
	MaxFilters int `json:"maxFilters"`

	// MaxFilterResults specifies the maximum number of results that can be accumulated by an actor event filter.
	MaxFilterResults int `json:"maxFilterResults"`

	// MaxFilterHeightRange specifies the maximum range of heights that can be used in a filter (to avoid querying
	// the entire chain)
	MaxFilterHeightRange uint64 `json:"maxFilterHeightRange"`

	// DatabasePath is the full path to a sqlite database that will be used to index actor events to
	// support the historic filter APIs. If the database does not exist it will be created. The directory containing
	// the database must already exist and be writeable. If a relative path is provided here, sqlite treats it as
	// relative to the CWD (current working directory).
	DatabasePath string `json:"databasePath"`

	// Others, not implemented yet:
	// Set a limit on the number of active websocket subscriptions (may be zero)
	// Set a timeout for subscription clients
	// Set upper bound on index size
}

type FevmConfig struct {
	//EnableEthRPC enables eth_rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
	EnableEthRPC bool `json:"enableEthRPC"`
	// EthTxHashMappingLifetimeDays the transaction hash lookup database will delete mappings that have been stored for more than x days
	// Set to 0 to keep all mappings
	EthTxHashMappingLifetimeDays int `json:"ethTxHashMappingLifetimeDays"`

	Event EventConfig `json:"event"`
}

type EventsConfig struct {
	// EnableActorEventsAPI enables the Actor events API that enables clients to consume events
	// emitted by (smart contracts + built-in Actors).
	// This will also enable the RealTimeFilterAPI and HistoricFilterAPI by default, but they can be
	// disabled by setting their respective Disable* options in Fevm.Event.
	EnableActorEventsAPI bool `json:"enableActorEventsAPI"`
}

func newFevmConfig() *FevmConfig {
	return &FevmConfig{
		EnableEthRPC:                 false,
		EthTxHashMappingLifetimeDays: 0,
		Event: EventConfig{
			DisableRealTimeFilterAPI: false,
			DisableHistoricFilterAPI: false,
			FilterTTL:                Duration(time.Hour * 24),
			MaxFilters:               100,
			MaxFilterResults:         10000,
			MaxFilterHeightRange:     2880, // conservative limit of one day
		},
	}
}

func newEventsConfig() *EventsConfig {
	return &EventsConfig{
		EnableActorEventsAPI: false,
	}
}

type PubsubConfig struct {
	// Run the node in bootstrap-node mode
	Bootstrapper bool `json:"bootstrapper"`
}

func newPubsubConfig() *PubsubConfig {
	return &PubsubConfig{Bootstrapper: false}
}

type FaultReporterConfig struct {
	// EnableConsensusFaultReporter controls whether the node will monitor and
	// report consensus faults. When enabled, the node will watch for malicious
	// behaviors like double-mining and parent grinding, and submit reports to the
	// network. This can earn reporter rewards, but is not guaranteed. Nodes should
	// enable fault reporting with care, as it may increase resource usage, and may
	// generate gas fees without earning rewards.
	EnableConsensusFaultReporter bool `json:"enableConsensusFaultReporter"`

	// ConsensusFaultReporterDataDir is the path where fault reporter state will be
	// persisted. This directory should have adequate space and permissions for the
	// node process.
	ConsensusFaultReporterDataDir string `json:"consensusFaultReporterDataDir"`

	// ConsensusFaultReporterAddress is the wallet address used for submitting
	// ReportConsensusFault messages. It will pay for gas fees, and receive any
	// rewards. This address should have adequate funds to cover gas fees.
	ConsensusFaultReporterAddress string `json:"consensusFaultReporterAddress"`
}

func newFaultReporterConfig() *FaultReporterConfig {
	return &FaultReporterConfig{}
}

// NewDefaultConfig returns a config object with all the fields filled out to
// their default values
func NewDefaultConfig() *Config {
	return &Config{
		API:           newDefaultAPIConfig(),
		Bootstrap:     newDefaultBootstrapConfig(),
		Datastore:     newDefaultDatastoreConfig(),
		Mpool:         newDefaultMessagePoolConfig(),
		NetworkParams: newDefaultNetworkParamsConfig(),
		Observability: newDefaultObservabilityConfig(),
		Swarm:         newDefaultSwarmConfig(),
		Wallet:        newDefaultWalletConfig(),
		SlashFilterDs: newDefaultSlashFilterDsConfig(),
		RateLimitCfg:  newRateLimitConfig(),
		FevmConfig:    newFevmConfig(),
		EventsConfig:  newEventsConfig(),
		PubsubConfig:  newPubsubConfig(),
		FaultReporter: newFaultReporterConfig(),
	}
}

// WriteFile writes the config to the given filepath.
func (cfg *Config) WriteFile(file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
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
	rawConfig, err := io.ReadAll(f)
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

// Set sets the config sub-struct referenced by `key`, e.g. 'api.address'
// or 'datastore' to the json key value pair encoded in jsonVal.
func (cfg *Config) Set(dottedKey string, jsonString string) error {
	if !json.Valid([]byte(jsonString)) {
		jsonBytes, _ := json.Marshal(jsonString)
		jsonString = string(jsonBytes)
	}

	if err := validate(dottedKey, jsonString); err != nil {
		return err
	}

	keys := strings.Split(dottedKey, ".")
	for i := len(keys) - 1; i >= 0; i-- {
		jsonString = fmt.Sprintf(`{ "%s": %s }`, keys[i], jsonString)
	}

	decoder := json.NewDecoder(strings.NewReader(jsonString))
	decoder.DisallowUnknownFields()

	return decoder.Decode(&cfg)
}

// Get gets the config sub-struct referenced by `key`, e.g. 'api.address'
func (cfg *Config) Get(key string) (interface{}, error) {
	v := reflect.Indirect(reflect.ValueOf(cfg))
	keyTags := strings.Split(key, ".")
OUTER:
	for j, keyTag := range keyTags {
		if v.Type().Kind() == reflect.Struct {
			for i := 0; i < v.NumField(); i++ {
				jsonTag := strings.Split(
					v.Type().Field(i).Tag.Get("json"),
					",")[0]
				if jsonTag == keyTag {
					v = v.Field(i)
					if j == len(keyTags)-1 {
						return v.Interface(), nil
					}
					v = reflect.Indirect(v) // only attempt one dereference
					continue OUTER
				}
			}
		}

		return nil, fmt.Errorf("key: %s invalid for config", key)
	}
	// Cannot get here as len(strings.Split(s, sep)) >= 1 with non-empty sep
	return nil, fmt.Errorf("empty key is invalid")
}

// validate runs validations on a given key and json string. validate uses the
// validators map defined at the top of this file to determine which validations
// to use for each key.
func validate(dottedKey string, jsonString string) error {
	var obj interface{}
	if err := json.Unmarshal([]byte(jsonString), &obj); err != nil {
		return err
	}
	// recursively validate sub-keys by partially unmarshalling
	if reflect.ValueOf(obj).Kind() == reflect.Map {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(jsonString), &obj); err != nil {
			return err
		}
		for key := range obj {
			if err := validate(dottedKey+"."+key, string(obj[key])); err != nil {
				return err
			}
		}
		return nil
	}

	if validationFunc, present := Validators[dottedKey]; present {
		return validationFunc(dottedKey, jsonString)
	}

	return nil
}

// validateLettersOnly validates that a given value contains only letters. If it
// does not, an error is returned using the given key for the message.
func validateLettersOnly(key string, value string) error {
	if match, _ := regexp.MatchString("^\"[a-zA-Z]+\"$", value); !match {
		return errors.Errorf(`"%s" must only contain letters`, key)
	}
	return nil
}

var (
	_ json.Marshaler   = (*Duration)(nil)
	_ json.Unmarshaler = (*Duration)(nil)
)

// Duration is a wrapper type for time.Duration
// for decoding and encoding from/to JSON
type Duration time.Duration

// UnmarshalJSON implements interface for json decoding
func (dur *Duration) UnmarshalJSON(data []byte) error {
	d, err := time.ParseDuration(strings.Trim(string(data), "\""))
	if err != nil {
		return err
	}
	*dur = Duration(d)
	return err
}

func (dur Duration) MarshalJSON() ([]byte, error) {
	d := time.Duration(dur)
	return []byte(fmt.Sprintf("\"%s\"", d.String())), nil
}
