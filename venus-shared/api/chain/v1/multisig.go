package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/messagepool"
)

type IMultiSig interface {
	// Rule[perm:read]
	MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val chain.BigInt, src address.Address, gp chain.BigInt) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt chain.BigInt, src address.Address, method uint64, params []byte) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt chain.BigInt, src address.Address, method uint64, params []byte) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigCancel(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt chain.BigInt, src address.Address, method uint64, params []byte) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (*messagepool.MessagePrototype, error)
	// Rule[perm:read]
	MsigGetVested(ctx context.Context, addr address.Address, start chain.TipSetKey, end chain.TipSetKey) (chain.BigInt, error)
}
