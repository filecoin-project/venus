package drand

import (
	"context"
	"errors"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/core"
	"github.com/drand/drand/key"
	"github.com/drand/drand/net"
	"github.com/drand/kyber"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
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
}

// NewGRPC creates a client that will draw randomness from the given addresses.
// distKeyCoeff are hex encoded strings representing a distributed public key
// Behavior is undefined if provided address do not point to Drand servers in the same group.
func NewGRPC(addresses []Address, distKeyCoeff [][]byte) (*GRPC, error) {
	distKey, err := groupKeycoefficientsToDistPublic(distKeyCoeff)
	if err != nil {
		return nil, err
	}

	return &GRPC{
		addresses: addresses,
		client:    core.NewGrpcClient(),
		key:       distKey,
	}, nil
}

// ReadEntry fetches an entry from one of the drand servers (trying them sequentially) and returns the result.
func (d *GRPC) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	// try each address, stopping when we have a key
	for _, addr := range d.addresses {
		pub, err := d.client.Public(addr.address, d.key, addr.secure, int(drandRound))
		if err != nil {
			log.Warnf("Error fetching drand randomness from %s: %s", addr.address, err)
			continue
		}

		return &Entry{
			Round: drandRound,
			Signature: crypto.Signature{
				Type: crypto.SigTypeBLS,
				Data: pub.GetSignature(),
			},
		}, nil
	}
	return nil, errors.New("Could not retrieve drand randomess from any address")
}

// VerifyEntry verifies that the child's signature is a valid signature of the previous entry.
func (d *GRPC) VerifyEntry(parent, child *Entry) (bool, error) {
	msg := beacon.Message(parent.Signature.Data, uint64(parent.Round), uint64(child.Round))
	err := key.Scheme.VerifyRecovered(d.key.Coefficients[0], msg, child.Signature.Data)
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
func (d *GRPC) FetchGroupConfig(addresses []string, secure bool, overrideGroupAddrs bool) ([]string, [][]byte, error) {
	defaultManager := net.NewCertManager()
	client := core.NewGrpcClientFromCert(defaultManager)

	// try each address, stopping when we have a key
	for _, addr := range addresses {
		groupAddrs, keyCoeffs, err := fetchGroupServer(client, Address{addr, secure})
		if err != nil {
			log.Warnf("Error fetching drand group key from %s: %s", addr, err)
			continue
		}

		distKey, err := groupKeycoefficientsToDistPublic(keyCoeffs)
		if err != nil {
			return nil, nil, err
		}
		d.key = distKey

		if overrideGroupAddrs {
			d.addresses = drandAddresses(addresses, secure)
		} else {
			d.addresses = drandAddresses(groupAddrs, secure)
		}

		return groupAddrs, keyCoeffs, nil
	}
	return nil, nil, errors.New("Could not retrieve drand group key from any address")
}

func drandAddresses(addresses []string, secure bool) []Address {
	addrs := make([]Address, len(addresses))
	for i, a := range addresses {
		addrs[i] = NewAddress(a, secure)
	}
	return addrs
}

func fetchGroupServer(client *core.Client, address Address) ([]string, [][]byte, error) {
	groupResp, err := client.Group(address.address, address.secure)
	if err != nil {
		return nil, nil, err
	}

	nodes := groupResp.GetNodes()
	addrs := make([]string, len(nodes))
	for i, nd := range nodes {
		addrs[i] = nd.GetAddress()
	}

	return addrs, groupResp.DistKey, nil
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
