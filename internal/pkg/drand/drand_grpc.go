package drand

import (
	"context"
	"errors"
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

	// First filecoin round is the first drand round before filecoinGenesisTime

	// check that there is a drand round between drandGenTime and filecoinGenTime and error otherwise

	return &GRPC{
		addresses:     addresses,
		client:        core.NewGrpcClient(),
		key:           distKey,
		genesisTime:   gt,
		roundTime:     rd,
		firstFilecoin: ffr,
		cache:         make(map[Round]*Entry),
	}, nil
}

// ReadEntry fetches an entry from one of the drand servers (trying them sequentially) and returns the result.
// **NOTE** this will block if called on a skipped round.
func (d *GRPC) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	if entry, ok := d.cache[drandRound]; ok {
		return entry, nil
	}

	// try each address, stopping when we have a key
	for _, addr := range d.addresses {
		pub, err := d.client.Public(addr.address, d.key, addr.secure, int(drandRound))
		if err != nil {
			log.Warnf("Error fetching drand randomness from %s: %s", addr.address, err)
			continue
		}

		entry := &Entry{
			Round:       drandRound,
			Signature:   pub.GetSignature(),
			parentRound: Round(pub.PreviousRound),
		}
		d.updateLocalState(entry)
		return entry, nil
	}
	return nil, errors.New("Could not retrieve drand randomess from any address")
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
	msg := beacon.Message(parent.Signature, uint64(parent.Round), uint64(child.Round))
	err := key.Scheme.VerifyRecovered(d.key.Coefficients[0], msg, child.Signature)
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
// **IMPORTANT** This function will block if it cannot determine the exact
// rounds in this interval due to potential gaps.  This happens in two cases:
// 1. RoundsInInterval is called on an interval extending into the future
// 2. There is an ongoing drand outage causing a gap in drand rounds
// 3. There is a network partition blocking this GPRC from the latest rounds
func (d *GRPC) RoundsInInterval(ctx context.Context, startTime, endTime time.Time) ([]Round, error) {
	idealRounds := roundsInIntervalWhenNoGaps(startTime, endTime, d.StartTimeOfRound, d.roundTime)
	if len(idealRounds) == 0 { // exit early if no rounds possible
		return []Round{}, nil
	}
	minRound := idealRounds[0]
	maxRound := idealRounds[len(idealRounds)-1]

	// if our latest round is greater set traverse to start at our latest round
	var next *Entry
	var err error
	if d.latestEntry != nil && d.latestEntry.Round > idealRounds[len(idealRounds)-1] {
		next = d.latestEntry
	} else {
		// wait on network to determine which rounds exist.  If maxRound is
		// skipped this will error or block indefinitely.
		next, err = d.ReadEntry(ctx, maxRound)
		if err != nil {
			return nil, err
		}
	}

	var rounds []Round
	for next.Round >= minRound {
		if next.Round <= maxRound {
			rounds = append(rounds, next.Round)
		}
		// handle possible first-DRAND-round edge case
		if next.Round == 0 {
			break
		}
		next, err = d.ReadEntry(ctx, next.parentRound)
		if err != nil {
			return nil, err
		}
	}
	return reverse(rounds), nil
}

func reverse(rounds []Round) []Round {
	revRounds := make([]Round, len(rounds))
	for i := 0; i < len(rounds); i++ {
		revRounds[i] = rounds[len(rounds)-1-i]
	}
	return revRounds
}
