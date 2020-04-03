package drand

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/core"
	"github.com/drand/drand/key"
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

// GRPC is a drand client that can fetch and verify from a public drand network
type GRPC struct {
	addresses []Address
	client    *core.Client
	key       *key.DistPublic
}

// NewDrandGRPC creates a client that will draw randomness from the given addresses.
// Behavior is undefined if provided address do not point to Drand servers in the same group.
func NewDrandGRPC(addresses []Address, distKey *key.DistPublic) *GRPC {
	return &GRPC{
		addresses: addresses,
		client:    core.NewGrpcClient(),
		key:       distKey,
	}
}

// ReadEntry immediately returns a drand entry with a signature equal to the
// round number
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

// VerifyEntry always returns true without error
func (d *GRPC) VerifyEntry(parent, child *Entry) (bool, error) {
	msg := beacon.Message(parent.Signature.Data, uint64(child.Round))
	err := key.Scheme.VerifyRecovered(d.key.Coefficients[0], msg, child.Signature.Data)
	if err != nil {
		return false, err
	}

	return true, nil
}

// FetchGroupKey Should only be used when switching to a new drand server group
func (d *GRPC) FetchGroupKey() (*key.DistPublic, error) {
	// try each address, stopping when we have a key
	for _, addr := range d.addresses {
		key, err := fetchGroupKeyFromServer(d.client, addr)
		if err != nil {
			log.Warnf("Error fetching drand group key from %s: %s", addr.address, err)
			continue
		}
		return key, nil
	}
	return nil, errors.New("Could not retrieve drand group key from any address")
}

func fetchGroupKeyFromServer(client *core.Client, address Address) (*key.DistPublic, error) {
	groupResp, err := client.Group(address.address, address.secure)
	if err != nil {
		return nil, err
	}

	pubKey := key.DistPublic{}
	pubKey.Coefficients = make([]kyber.Point, len(groupResp.Distkey))
	for i, k := range groupResp.Distkey {
		keyBytes, err := hex.DecodeString(k)
		if err != nil {
			return nil, err
		}
		pubKey.Coefficients[i] = key.KeyGroup.Point()
		pubKey.Coefficients[i].UnmarshalBinary(keyBytes)
	}
	return &pubKey, nil
}
