package fast

import (
	"context"
	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

	"github.com/libp2p/go-libp2p-core/peer"
)

// AddressNew runs the address new command against the filecoin process.
func (f *Filecoin) AddressNew(ctx context.Context) (address.Address, error) {
	var newAddress address.Address
	if err := f.RunCmdJSONWithStdin(ctx, nil, &newAddress, "go-filecoin", "address", "new"); err != nil {
		return address.Undef, err
	}
	return newAddress, nil
}

// AddressLs runs the address ls command against the filecoin process.
func (f *Filecoin) AddressLs(ctx context.Context) ([]address.Address, error) {
	// the command returns an AddressListResult
	var alr commands.AddressLsResult
	// we expect to interact with an array of address
	var out []address.Address

	if err := f.RunCmdJSONWithStdin(ctx, nil, &alr, "go-filecoin", "address", "ls"); err != nil {
		return nil, err
	}

	// transform the AddressListResult to an array of addresses
	for _, addr := range alr.Addresses {
		a, err := address.NewFromString(addr)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// AddressLookup runs the address lookup command against the filecoin process.
func (f *Filecoin) AddressLookup(ctx context.Context, addr address.Address) (peer.ID, error) {
	var ownerPeer peer.ID
	if err := f.RunCmdJSONWithStdin(ctx, nil, &ownerPeer, "go-filecoin", "address", "lookup", addr.String()); err != nil {
		return "", err
	}
	return ownerPeer, nil
}
