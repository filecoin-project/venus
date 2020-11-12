package fast

import (
	"context"
	"strings"

	"github.com/filecoin-project/go-address"
	files "github.com/ipfs/go-ipfs-files"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// WalletBalance run the wallet balance command against the filecoin process.
func (f *Filecoin) WalletBalance(ctx context.Context, addr address.Address) (types.AttoFIL, error) {
	var balance types.AttoFIL
	if err := f.RunCmdJSONWithStdin(ctx, nil, &balance, "venus", "wallet", "balance", addr.String()); err != nil {
		return types.ZeroAttoFIL, err
	}
	return balance, nil
}

// WalletImport run the wallet import command against the filecoin process.
func (f *Filecoin) WalletImport(ctx context.Context, file files.File) ([]address.Address, error) {
	// the command returns an AddressListResult
	var alr commands.AddressLsResult
	if err := f.RunCmdJSONWithStdin(ctx, file, &alr, "venus", "wallet", "import"); err != nil {
		return nil, err
	}
	return alr.Addresses, nil
}

// WalletExport run the wallet export command against the filecoin process.
func (f *Filecoin) WalletExport(ctx context.Context, addrs []address.Address) ([]*crypto.KeyInfo, error) {
	// the command returns an KeyInfoListResult
	var klr commands.WalletSerializeResult
	// we expect to interact with an array of KeyInfo(s)
	var out []*crypto.KeyInfo
	var sAddrs []string
	for _, a := range addrs {
		sAddrs = append(sAddrs, a.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &klr, "venus", "wallet", "export", strings.Join(sAddrs, " ")); err != nil {
		return nil, err
	}

	// transform the KeyInfoListResult to an array of KeyInfo(s)
	return append(out, klr.KeyInfo...), nil
}
