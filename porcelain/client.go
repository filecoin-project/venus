package porcelain

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	cbor "github.com/ipfs/go-ipld-cbor"
)

// Ask is a result of querying for an ask, it may contain an error
type Ask struct {
	Miner  address.Address
	Price  *types.AttoFIL
	Expiry *types.BlockHeight
	ID     uint64

	Error error
}

type claPlubming interface {
	ActorLs(ctx context.Context) (<-chan state.GetAllActorsResult, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
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

func listAsksFromActorResult(ctx context.Context, plumbing claPlubming, actorResult state.GetAllActorsResult, out chan Ask) error {
	if actorResult.Error != nil {
		return actorResult.Error
	}

	addr, _ := address.NewFromString(actorResult.Address)
	actor := actorResult.Actor

	if !types.MinerActorCodeCid.Equals(actor.Code) && !types.BootstrapMinerActorCodeCid.Equals(actor.Code) {
		return nil
	}

	// TODO: at some point, we will need to check that the miners are actually part of the storage market
	// for now, its impossible for them not to be.
	ret, err := plumbing.MessageQuery(ctx, address.Undef, addr, "getAsks")
	if err != nil {
		return err
	}

	var asksIds []uint64
	if err := cbor.DecodeInto(ret[0], &asksIds); err != nil {
		return err
	}

	for _, id := range asksIds {
		ask, err := getAskByID(ctx, plumbing, addr, id)
		if err != nil {
			return err
		}

		out <- ask
	}

	return nil
}

func getAskByID(ctx context.Context, plumbing claPlubming, addr address.Address, id uint64) (Ask, error) {
	ret, err := plumbing.MessageQuery(ctx, address.Undef, addr, "getAsk", big.NewInt(int64(id)))
	if err != nil {
		return Ask{}, err
	}

	var ask miner.Ask
	if err := cbor.DecodeInto(ret[0], &ask); err != nil {
		return Ask{}, err
	}

	return Ask{
		Expiry: ask.Expiry,
		ID:     ask.ID.Uint64(),
		Price:  ask.Price,
		Miner:  addr,
	}, nil
}
