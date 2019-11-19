package porcelain

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type chainHeadPlumbing interface {
	ChainHeadKey() block.TipSetKey
	ChainTipSet(key block.TipSetKey) (block.TipSet, error)
}

// ChainHead gets the current head tipset from plumbing.
func ChainHead(plumbing chainHeadPlumbing) (block.TipSet, error) {
	return plumbing.ChainTipSet(plumbing.ChainHeadKey())
}

type fullBlockPlumbing interface {
	ChainGetBlock(context.Context, cid.Cid) (*block.Block, error)
	ChainGetMessages(context.Context, types.TxMeta) ([]*types.SignedMessage, error)
}

// GetFullBlock returns a full block: header, messages, receipts.
func GetFullBlock(ctx context.Context, plumbing fullBlockPlumbing, id cid.Cid) (*block.FullBlock, error) {
	var out block.FullBlock
	var err error

	out.Header, err = plumbing.ChainGetBlock(ctx, id)
	if err != nil {
		return nil, err
	}

	out.Messages, err = plumbing.ChainGetMessages(ctx, out.Header.Messages)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

type getStableActorPlumbing interface {
	ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error)
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	ActorGetSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (_ *vm.FunctionSignature, err error)
}

// GetStableActor looks up an actor by address. If the address is an actor address it will first convert it to an id address.
func GetStableActor(ctx context.Context, plumbing getStableActorPlumbing, addr address.Address) (*actor.Actor, error) {
	stateAddr, err := retrieveActorIDForActorAddress(ctx, plumbing, addr)
	if err != nil {
		return nil, err
	}

	// get actor
	return plumbing.ActorGet(ctx, stateAddr)
}

// GetStableActorSignature looks up and actor method signature by address. If the addresss is an actor address it will first convert it to an id address.
func GetStableActorSignature(ctx context.Context, plumbing getStableActorPlumbing, actorAddr address.Address, method types.MethodID) (_ *vm.FunctionSignature, err error) {
	stateAddr, err := retrieveActorIDForActorAddress(ctx, plumbing, actorAddr)
	if err != nil {
		return nil, err
	}

	// get actor
	return plumbing.ActorGetSignature(ctx, stateAddr, method)
}

func retrieveActorIDForActorAddress(ctx context.Context, plumbing getStableActorPlumbing, addr address.Address) (address.Address, error) {
	if addr.Protocol() != address.Actor {
		return addr, nil
	}

	head := plumbing.ChainHeadKey()
	ret, err := plumbing.MessageQuery(ctx, address.Undef, address.InitAddress, initactor.GetActorIDForAddress, head, addr)
	if err != nil {
		return address.Undef, err
	}

	idVal, err := abi.Deserialize(ret[0], abi.Integer)
	if err != nil {
		return address.Undef, err
	}

	return address.NewIDAddress(idVal.Val.(*big.Int).Uint64())

}
