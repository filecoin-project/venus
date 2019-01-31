package series

import (
	"context"
	"math/big"

	"gx/ipfs/QmXWZCd8jfaHmt4UDSnjKmGcrQMw95bDGWqEeVLVJjoANX/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fat"
)

// SetupGenesisNode will initalize, start, configure, and issue the "start mining" command to the filecoin process `node`.
// Process `node` will use the genesisfile at `gcUri`, be configured with miner `minerAddress`, and import the address of the
// miner `minerOwner`. Lastly the process `node` will start mining.
func SetupGenesisNode(ctx context.Context, node *fat.Filecoin, gcUri string, minerAddress address.Address, minerOwner files.File) error {

	if _, err := node.InitDaemon(ctx, "--genesisfile", gcUri); err != nil {
		return err
	}

	if _, err := node.StartDaemon(ctx, false); err != nil {
		return err
	}

	if err := node.ConfigSet(ctx, `"mining.minerAddress"`, minerAddress.String()); err != nil {
		return err
	}

	if _, err := node.WalletImport(ctx, minerOwner); err != nil {
		return err
	}

	if _, err := node.MinerUpdatePeerid(ctx, minerAddress, node.PeerID, fat.AOPrice(big.NewFloat(300)), fat.AOLimit(300)); err != nil {
		return err
	}

	return node.MiningStart(ctx)
}
