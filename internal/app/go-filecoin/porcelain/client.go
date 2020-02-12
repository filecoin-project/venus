package porcelain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Ask is a result of querying for an ask, it may contain an error
type Ask struct {
	Miner  address.Address
	Price  types.AttoFIL
	Expiry *types.BlockHeight
	ID     uint64

	Error error
}

type claPlubming interface {
	ActorLs(ctx context.Context) (<-chan state.GetAllActorsResult, error)
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
}

// ClientListAsks returns a channel with asks from the latest chain state
func ClientListAsks(ctx context.Context, plumbing claPlubming) <-chan Ask {
	out := make(chan Ask)

	go func() {
		defer close(out)
		actorCh, err := plumbing.ActorLs(ctx)
		if err != nil {
			out <- Ask{
				Error: err,
			}
			return
		}

		for actorResult := range actorCh {
			err := listAsksFromActorResult(ctx, plumbing, actorResult, out)
			if err != nil {
				out <- Ask{
					Error: err,
				}
				return
			}
		}
	}()

	return out
}

func listAsksFromActorResult(ctx context.Context, plumbing claPlubming, res state.GetAllActorsResult, out chan Ask) error {
	if res.Error != nil {
		return res.Error
	}

	if !types.MinerActorCodeCid.Equals(res.Actor.Code.Cid) && !types.BootstrapMinerActorCodeCid.Equals(res.Actor.Code.Cid) {
		return nil
	}

	// TODO: at some point, we will need to check that the miners are actually part of the storage market
	// for now, its impossible for them not to be.
	ret, err := plumbing.MessageQuery(ctx, address.Undef, res.Address, miner.GetAsks, plumbing.ChainHeadKey())
	if err != nil {
		return err
	}

	var asksIds []types.Uint64
	if err := encoding.Decode(ret[0], &asksIds); err != nil {
		return err
	}
	fmt.Printf("asksIds: %v\n", asksIds)

	for _, id := range asksIds {
		ask, err := getAskByID(ctx, plumbing, res.Address, uint64(id))
		if err != nil {
			return err
		}
		fmt.Printf("ask: %v\n", ask)

		out <- ask
	}

	return nil
}

// The subset of plumbing used by ClientVerifyStorageDeal
type cvsdPlumbing interface {
	ChainHeadKey() block.TipSetKey
	DealGet(ctx context.Context, proposalCid cid.Cid) (*storagedeal.Deal, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
}

// ClientVerifyStorageDeal check to see that a storage deal is in the `Complete` state, and that its PIP is valid
// returns nil if successful
func ClientVerifyStorageDeal(ctx context.Context, plumbing cvsdPlumbing, proposalCid cid.Cid, proofInfo *storagedeal.ProofInfo) error {
	// Get the deal out of local storage.  This Deal was stored when we made the
	// proposal, and has never been updated
	deal, err := plumbing.DealGet(ctx, proposalCid)
	if err != nil {
		return errors.Wrap(err, "failed to get deal")
	}

	params := []interface{}{
		deal.CommP[:],
		deal.Proposal.Size,
		proofInfo.SectorID,
		proofInfo.PieceInclusionProof,
	}

	_, err = plumbing.MessageQuery(ctx, address.Undef, deal.Miner, miner.VerifyPieceInclusion, plumbing.ChainHeadKey(), params...)
	if err != nil {
		return err
	}

	return nil
}

func getAskByID(ctx context.Context, plumbing claPlubming, addr address.Address, id uint64) (Ask, error) {
	ret, err := plumbing.MessageQuery(ctx, address.Undef, addr, miner.GetAsk, plumbing.ChainHeadKey(), big.NewInt(int64(id)))
	if err != nil {
		return Ask{}, err
	}

	var ask miner.Ask
	if err := encoding.Decode(ret[0], &ask); err != nil {
		return Ask{}, err
	}
	fmt.Printf("pre processed ask: %v\n", ask)

	return Ask{
		Expiry: ask.Expiry,
		ID:     ask.ID.Uint64(),
		Price:  ask.Price,
		Miner:  addr,
	}, nil
}
