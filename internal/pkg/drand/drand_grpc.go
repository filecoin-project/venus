package drand

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/core"
	"github.com/drand/drand/key"
	"github.com/drand/drand/net"
	"github.com/drand/kyber"
	logging "github.com/ipfs/go-log"
)

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
	addresses []Address
	client    *core.Client
	key       *key.DistPublic

	// The time of the 0th round of the DRAND chain
	genesisTime time.Time
	// the time of genesis block of the Filecoin chain
	filecoinGenesisTime time.Time
	// The DRAND round first included in the filecoin blockchain
	firstFilecoin Round
	// Duration of a round in this DRAND network
	roundTime time.Duration

	// internal state
	latestEntry *Entry
	cache       map[Round]*Entry
}

var _ IFace = &GRPC{}

// NewGRPC creates a client that will draw randomness from the given addresses.
// distKeyCoeff are hex encoded strings representing a distributed public key
// Behavior is undefined if provided address do not point to Drand servers in the same group.
func NewGRPC(addresses []Address, distKeyCoeff [][]byte, drandGenTime time.Time, filecoinGenTime time.Time, rd time.Duration) (*GRPC, error) {
	distKey, err := groupKeycoefficientsToDistPublic(distKeyCoeff)
	if err != nil {
		return nil, err
	}

	grpc := &GRPC{
		addresses:           addresses,
		client:              core.NewGrpcClient(),
		key:                 distKey,
		genesisTime:         drandGenTime,
		filecoinGenesisTime: filecoinGenTime,
		// firstFilecoin set in updateFirsFilecoinRound below
		roundTime: rd,
		cache:     make(map[Round]*Entry),
	}
	err = grpc.updateFirstFilecoinRound()
	if err != nil {
		return nil, err
	}
	return grpc, nil
}

func (d *GRPC) updateFirstFilecoinRound() error {
	// First filecoin round is the first drand round before filecoinGenesisTime
	searchStart := d.filecoinGenesisTime.Add(-1 * d.roundTime)
	results := d.RoundsInInterval(searchStart, d.filecoinGenesisTime)
	if len(results) != 1 {
		return fmt.Errorf("found %d drand rounds between filecoinGenTime and filecoinGenTime - drandRountDuration, expected 1", len(results))
	}
	d.firstFilecoin = results[0]
	return nil
}

// ReadEntry fetches an entry from one of the drand servers (trying them sequentially) and returns the result.
func (d *GRPC) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	if entry, ok := d.cache[drandRound]; ok {
		return entry, nil
	}

	// try each address, stopping when we have a key
	for _, addr := range d.addresses {
		if ctx.Err() != nil { // Don't try any more peers after cancellation.
			return nil, ctx.Err()
		}
		// The drand client doesn't accept a context, so is un-cancellable :-(
		pub, err := d.client.Public(addr.address, d.key, addr.secure, int(drandRound))
		if err != nil {
			log.Warnf("Error fetching drand randomness from %s: %s", addr.address, err)
			continue
		}

		// Because the client.Public() call can't be cancelled by this context, it can return at any time,
		// potentially leading to concurrent state updates below racing a new call to into ReadEntry() (because
		// the caller thought it was cancelled already).
		// This check will mostly, but not completely securely, avoid this. A robust fix requires avoiding
		// concurrent calls to here completely, which ultimately arise from the goroutine in the mining scheduler.
		// https://github.com/filecoin-project/go-filecoin/issues/4065
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		entry := &Entry{
			Round: drandRound,
			Data:  pub.GetSignature(),
		}
		d.updateLocalState(entry)
		return entry, nil
	}
	return nil, errors.New("could not retrieve drand randomess from any address")
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
func (d *GRPC) VerifyEntry(parent, child *Entry) (bool, error) {
	msg := beacon.Message(uint64(child.Round), parent.Data)
	err := key.Scheme.VerifyRecovered(d.key.Coefficients[0], msg, child.Data)
	if err != nil {
		return false, err
	}

	return true, nil
}

// FetchGroupConfig Should only be used when switching to a new drand server group.
// Returns hex encoded group key coefficients that can be used to construct a public key.
// If overrideGroupAddrs is true, the given set of addresses will be set as the drand nodes.
// Otherwise drand address config will be set from the retrieved group info. The
// override is useful when the the drand server is behind NAT.
func (d *GRPC) FetchGroupConfig(addresses []string, secure bool, overrideGroupAddrs bool) ([]string, [][]byte, uint64, int, error) {
	defaultManager := net.NewCertManager()
	client := core.NewGrpcClientFromCert(defaultManager)

	// try each address, stopping when we have a key
	for _, addr := range addresses {
		groupAddrs, keyCoeffs, genesisTime, roundSeconds, err := fetchGroupServer(client, Address{addr, secure})
		if err != nil {
			log.Warnf("Error fetching drand group key from %s: %s", addr, err)
			continue
		}
		d.genesisTime = time.Unix(int64(genesisTime), 0)
		d.roundTime = time.Duration(roundSeconds) * time.Second

		distKey, err := groupKeycoefficientsToDistPublic(keyCoeffs)
		if err != nil {
			return nil, nil, 0, 0, err
		}
		d.key = distKey

		if overrideGroupAddrs {
			d.addresses = drandAddresses(addresses, secure)
		} else {
			d.addresses = drandAddresses(groupAddrs, secure)
		}

		err = d.updateFirstFilecoinRound() // this depends on genesis and round time so recalculate
		if err != nil {
			return nil, nil, 0, 0, err
		}

		return groupAddrs, keyCoeffs, genesisTime, roundSeconds, nil
	}
	return nil, nil, 0, 0, errors.New("Could not retrieve drand group key from any address")
}

func drandAddresses(addresses []string, secure bool) []Address {
	addrs := make([]Address, len(addresses))
	for i, a := range addresses {
		addrs[i] = NewAddress(a, secure)
	}
	return addrs
}

func fetchGroupServer(client *core.Client, address Address) ([]string, [][]byte, uint64, int, error) {
	groupResp, err := client.Group(address.address, address.secure)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	nodes := groupResp.GetNodes()
	addrs := make([]string, len(nodes))
	for i, nd := range nodes {
		addrs[i] = nd.GetAddress()
	}

	return addrs, groupResp.DistKey, groupResp.GenesisTime, int(groupResp.Period), nil
}

func groupKeycoefficientsToDistPublic(coefficients [][]byte) (*key.DistPublic, error) {
	pubKey := key.DistPublic{}
	pubKey.Coefficients = make([]kyber.Point, len(coefficients))
	for i, k := range coefficients {
		pubKey.Coefficients[i] = key.KeyGroup.Point()
		err := pubKey.Coefficients[i].UnmarshalBinary(k)
		if err != nil {
			return nil, err
		}
	}
	return &pubKey, nil
}

// FirstFilecoinRound returns the configured first drand round included in the filecoin blockchain
func (d *GRPC) FirstFilecoinRound() Round {
	return d.firstFilecoin
}

// StartTimeOfRound returns the time the given DRAND round will start if it is unskipped
func (d *GRPC) StartTimeOfRound(round Round) time.Time {
	return d.genesisTime.Add(d.roundTime * time.Duration(round))
}

// RoundsInInterval returns all rounds in the given interval.

func (d *GRPC) RoundsInInterval(startTime, endTime time.Time) []Round {
	return roundsInInterval(startTime, endTime, d.StartTimeOfRound, d.roundTime)
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
