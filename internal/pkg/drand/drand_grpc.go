package drand

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"time"

	dchain "github.com/drand/drand/chain"
	dclient "github.com/drand/drand/client"
	hclient "github.com/drand/drand/client/http"
	"github.com/drand/kyber"
	logging "github.com/ipfs/go-log/v2"
)

type DrandConfig struct {
	Servers       []string
	Relays        []string
	ChainInfoJSON string
}

var log = logging.Logger("drand")

// Address points to a drand server
type Address struct {
	address string
	secure  bool
}

// NewAddress creates a new address
func NewAddress(a string, secure bool) Address {
	return Address{a, secure}
}

// GRPC is a drand client that can fetch and verify from a public drand network
type GRPC struct {
	client dclient.Client

	pubkey kyber.Point

	// seconds
	interval time.Duration

	drandGenTime uint64
	filGenTime   uint64
	filRoundTime uint64

	// internal state
	latestEntry *Entry
	cache       map[Round]*Entry
}

var _ IFace = &GRPC{}

// NewGRPC creates a client that will draw randomness from the given addresses.
// distKeyCoeff are hex encoded strings representing a distributed public key
// Behavior is undefined if provided address do not point to Drand servers in the same group.
func NewGRPC(filecoinGenTime time.Time, rd time.Duration, config DrandConfig) (*GRPC, error) {
	drandChain, err := dchain.InfoFromJSON(bytes.NewReader([]byte(config.ChainInfoJSON)))
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal drand chain info: %w", err)
	}

	var clients []dclient.Client
	for _, url := range config.Servers {
		hc, err := hclient.NewWithInfo(url, drandChain, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create http drand client: %w", err)
		}
		clients = append(clients, hc)
	}

	opts := []dclient.Option{
		dclient.WithChainInfo(drandChain),
		dclient.WithCacheSize(1024),
		dclient.WithAutoWatch(),
	}

	client, err := dclient.Wrap(clients, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating drand client")
	}

	return &GRPC{
		client:       client,
		cache:        make(map[Round]*Entry),
		pubkey:       drandChain.PublicKey,
		interval:     drandChain.Period,
		drandGenTime: uint64(drandChain.GenesisTime),
		filRoundTime: uint64(rd),
		filGenTime:   uint64(filecoinGenTime.UnixNano()),
	}, nil
}

// ReadEntry fetches an entry from one of the drand servers (trying them sequentially) and returns the result.
func (d *GRPC) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	if entry, ok := d.cache[drandRound]; ok {
		return entry, nil
	}

	result, err := d.client.Get(ctx, uint64(drandRound))
	if err != nil {
		return nil, fmt.Errorf("get drand failed %v", err)
	}

	entry := &Entry{
		Round: drandRound,
		Data:  result.Signature(),
	}
	d.updateLocalState(entry)
	return entry, nil
}

func (d *GRPC) updateLocalState(entry *Entry) {
	if d.latestEntry == nil {
		d.latestEntry = entry
	}
	if entry.Round > d.latestEntry.Round {
		d.latestEntry = entry
	}
	d.cache[entry.Round] = entry
}

// VerifyEntry verifies that the child's signature is a valid signature of the previous entry.
func (d *GRPC) VerifyEntry(prev, curr *Entry) (bool, error) {
	b := &dchain.Beacon{
		PreviousSig: prev.Data,
		Round:       uint64(curr.Round),
		Signature:   curr.Data,
	}
	err := dchain.VerifyBeacon(d.pubkey, b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *GRPC) MaxBeaconRoundForEpoch(filEpoch abi.ChainEpoch) Round {
	// TODO: sometimes the genesis time for filecoin is zero and this goes negative
	latestTs := ((uint64(filEpoch) * d.filRoundTime) + d.filGenTime) - d.filRoundTime
	dround := (latestTs - d.drandGenTime) / uint64(d.interval.Seconds())
	return Round(dround)
}

func roundsInInterval(startTime, endTime time.Time, startTimeOfRound func(Round) time.Time, roundDuration time.Duration) []Round {
	// Find first round after startTime
	genesisTime := startTimeOfRound(Round(0))
	truncatedStartRound := Round(startTime.Sub(genesisTime) / roundDuration)
	var round Round
	if startTimeOfRound(truncatedStartRound).Equal(startTime) {
		round = truncatedStartRound
	} else {
		round = truncatedStartRound + 1
	}
	roundTime := startTimeOfRound(round)
	var rounds []Round
	// Advance a round time until we hit endTime, adding rounds
	for roundTime.Before(endTime) {
		rounds = append(rounds, round)
		round++
		roundTime = startTimeOfRound(round)
	}
	return rounds
}
