package msg

import (
	"context"

	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/wallet"
	"github.com/pkg/errors"
)

// Queryer knows how to send read-only messages for querying actor state.
type Queryer struct {
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

// NewQueryer constructs a Queryer.
func NewQueryer(repo repo.Repo, wallet *wallet.Wallet, chainReader chain.ReadStore, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Queryer {
	return &Queryer{repo, wallet, chainReader, cst, bs}
}

// Query sends a read-only message to an actor. 
func (q *Queryer) Query(ctx context.Context, to address.Address, method string, args []byte, optFrom *address.Address) (_ [][]byte, _ uint8, err error) {
	headTs := q.chainReader.Head()
	tsas, err := q.chainReader.GetTipSetAndState(ctx, headTs.String())
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve state")
	}
	st, err := state.LoadStateTree(ctx, q.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, 1, errors.Wrap(err, "failed to retrieve state")
	}
	h, err := headTs.Height()
	if err != nil {
		return nil, 1, errors.Wrap(err, "getting base tipset height")
	}

	var fromAddr address.Address
	if optFrom != nil {
		fromAddr = *optFrom
	} else {
		fromAddr, err = GetAndMaybeSetDefaultSenderAddress(q.repo, q.wallet)
		if err != nil {
			return nil, 1, errors.Wrap(err, "failed to retrieve default sender address")
		}
	}

	vms := vm.NewStorageMap(q.bs)
	return consensus.CallQueryMethod(ctx, st, vms, to, method, args, fromAddr, types.NewBlockHeight(h))
}
