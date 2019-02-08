package series

import (
	"context"
	"math/big"

	"gx/ipfs/QmXWZCd8jfaHmt4UDSnjKmGcrQMw95bDGWqEeVLVJjoANX/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fat"
)

// SetupGenesisNode will initialize, start, configure, and issue the "start mining" command to the filecoin process `node`.
// Process `node` will use the genesisfile at `gcURI`, be configured with miner `minerAddress`, and import the address of the
// miner `minerOwner`. Lastly the process `node` will start mining.
func SetupGenesisNode(ctx context.Context, node *fat.Filecoin, gcURI string, minerAddress address.Address, minerOwner files.File) error {

	if _, err := node.InitDaemon(ctx, "--genesisfile", gcURI); err != nil {
		return err
	}

	if _, err := node.StartDaemon(ctx, true); err != nil {
		return err
	}

	if err := node.ConfigSet(ctx, "mining.minerAddress", minerAddress.String()); err != nil {
		return err
	}

	wallet, err := node.WalletImport(ctx, minerOwner)
	if err != nil {
		return err
	}

	if err := node.ConfigSet(ctx, "wallet.defaultAddress", wallet[0].String()); err != nil {
		return err
	}

	_, err = node.MinerUpdatePeerid(ctx, minerAddress, node.PeerID, fat.AOFromAddr(wallet[0]), fat.AOPrice(big.NewFloat(300)), fat.AOLimit(300))
	if err != nil {
		return err
	}

	return node.MiningStart(ctx)
}
