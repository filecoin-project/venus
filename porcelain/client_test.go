package porcelain_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type claPlumbing struct {
	actorFail   bool
	actorChFail bool
	messageFail bool

	MinerAddress address.Address
}

func (cla *claPlumbing) ActorLs(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	out := make(chan state.GetAllActorsResult)

	if cla.actorFail {
		return nil, errors.New("ACTOR FAILURE")
	}

	go func() {
		defer close(out)
		for i := 0; i < 42; i++ {
			if cla.actorChFail {
				out <- state.GetAllActorsResult{
					Error: errors.New("ACTOR CHANNEL FAILURE"),
				}
			} else {
				cla.MinerAddress = address.NewForTestGetter()()
				actor := actor.Actor{Code: types.MinerActorCodeCid}
				out <- state.GetAllActorsResult{
					Address: cla.MinerAddress.String(),
					Actor:   &actor,
				}
			}
		}
	}()

	return out, nil
}

func (cla *claPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	if cla.messageFail {
		return nil, errors.New("MESSAGE FAILURE")
	}

	if method == "getAsks" {
		askIDs, _ := cbor.DumpObject([]uint64{0})
		return [][]byte{askIDs}, nil
	}

	ask := miner.Ask{
		Expiry: types.NewBlockHeight(1),
		ID:     big.NewInt(2),
		Price:  types.NewAttoFILFromFIL(3),
	}
	askBytes, _ := cbor.DumpObject(ask)
	return [][]byte{askBytes}, nil
}

func TestClientListAsks(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		plumbing := &claPlumbing{}

		results := porcelain.ClientListAsks(ctx, plumbing)
		result := <-results

		expectedResult := porcelain.Ask{
			Expiry: types.NewBlockHeight(1),
			ID:     uint64(2),
			Miner:  plumbing.MinerAddress,
			Price:  types.NewAttoFILFromFIL(3),
		}

		assert.Equal(expectedResult, result)
	})

	t.Run("failed actor ls", func(t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		plumbing := &claPlumbing{
			actorFail: true,
		}

		results := porcelain.ClientListAsks(ctx, plumbing)
		result := <-results

		assert.Error(result.Error, "ACTOR FAILURE")
	})

	t.Run("failed actor ls via channel", func(t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		plumbing := &claPlumbing{
			actorChFail: true,
		}

		results := porcelain.ClientListAsks(ctx, plumbing)
		result := <-results

		assert.Error(result.Error, "ACTOR CHANNEL FAILURE")
	})

	t.Run("failed message query", func(t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		plumbing := &claPlumbing{
			messageFail: true,
		}

		results := porcelain.ClientListAsks(ctx, plumbing)
		result := <-results

		assert.Error(result.Error, "MESSAGE FAILURE")
	})
}
