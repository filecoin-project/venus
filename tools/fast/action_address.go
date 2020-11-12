package fast

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/libp2p/go-libp2p-core/peer"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
)

// AddressNew runs the address new command against the filecoin process.
func (f *Filecoin) AddressNew(ctx context.Context) (address.Address, error) {
	var newAddress address.Address
	if err := f.RunCmdJSONWithStdin(ctx, nil, &newAddress, "venus", "address", "new"); err != nil {
		return address.Undef, err
	}
	return newAddress, nil
}

// AddressLs runs the address ls command against the filecoin process.
func (f *Filecoin) AddressLs(ctx context.Context) ([]address.Address, error) {
	// the command returns an AddressListResult
	var alr commands.AddressLsResult
	if err := f.RunCmdJSONWithStdin(ctx, nil, &alr, "venus", "address", "ls"); err != nil {
		return nil, err
	}
	return alr.Addresses, nil
}

// AddressLookup runs the address lookup command against the filecoin process.
func (f *Filecoin) AddressLookup(ctx context.Context, addr address.Address) (peer.ID, error) {
	var ownerPeer peer.ID
	if err := f.RunCmdJSONWithStdin(ctx, nil, &ownerPeer, "venus", "address", "lookup", addr.String()); err != nil {
		return "", err
	}
	return ownerPeer, nil
}
