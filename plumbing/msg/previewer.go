package msg

import (
	"context"

	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// Previewer calculates the amount of Gas needed for a command
type Previewer struct {
	// For getting the default address. Lame.
	repo   repo.Repo
	wallet *wallet.Wallet
	// To get the head tipset state root.
	chainReader chain.ReadStore
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewPreviewer constructs a Previewer.
func NewPreviewer(repo repo.Repo, wallet *wallet.Wallet, chainReader chain.ReadStore, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Previewer {
	return &Previewer{repo, wallet, chainReader, cst, bs}
}

// Preview sends a read-only message to an actor.
func (p *Previewer) Preview(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldnt encode message params")
	}

	headTs := p.chainReader.Head()
	tsas, err := p.chainReader.GetTipSetAndState(ctx, headTs.String())
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldnt get latest state root")
	}
	st, err := state.LoadStateTree(ctx, p.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "could load tree for latest state root")
	}
	h, err := headTs.Height()
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldnt get base tipset height")
	}

	vms := vm.NewStorageMap(p.bs)
	usedGas, err := consensus.PreviewQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "query method returned an error")
	}
	return usedGas, nil
}
