package fast

import (
	"context"
	"strings"

	"github.com/ipfs/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/types"
)

// WalletBalance run the wallet balance command against the filecoin process.
func (f *Filecoin) WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error) {
	var balance types.AttoFIL
	if err := f.RunCmdJSONWithStdin(ctx, nil, &balance, "go-filecoin", "wallet", "balance", addr.String()); err != nil {
		return types.ZeroAttoFIL, err
	}
	return balance, nil
}

// WalletImport run the wallet import command against the filecoin process.
func (f *Filecoin) WalletImport(ctx context.Context, file files.File) ([]address.Address, error) {
	// the command returns an AddressListResult
	var alr commands.AddressLsResult
	// we expect to interact with an array of address
	var out []address.Address

	if err := f.RunCmdJSONWithStdin(ctx, file, &alr, "go-filecoin", "wallet", "import"); err != nil {
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

// WalletExport run the wallet export command against the filecoin process.
func (f *Filecoin) WalletExport(ctx context.Context, addrs []address.Address) ([]*types.KeyInfo, error) {
	// the command returns an KeyInfoListResult
	var klr commands.WalletSerializeResult
	// we expect to interact with an array of KeyInfo(s)
	var out []*types.KeyInfo
	var sAddrs []string
	for _, a := range addrs {
		sAddrs = append(sAddrs, a.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &klr, "go-filecoin", "wallet", "export", strings.Join(sAddrs, " ")); err != nil {
		return nil, err
	}

	// transform the KeyInfoListResult to an array of KeyInfo(s)
	return append(out, klr.KeyInfo...), nil
}
