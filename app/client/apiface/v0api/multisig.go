package v0api

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
)

type IMultiSig interface {
	// MsigCreate creates a multisig wallet
	// It takes the following params: <required number of senders>, <approving addresses>, <unlock duration>
	//<initial balance>, <sender address of the create msg>, <gas price>
	// Rule[perm:read]
	MsigCreate(context.Context, uint64, []address.Address, abi.ChainEpoch, types.BigInt, address.Address, types.BigInt) (cid.Cid, error)
	// Rule[perm:read]
	MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// Rule[perm:read]
	MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:read]
	MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:read]
	MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:read]
	MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:read]
	MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:read]
	MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:read]
	MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error)
	// Rule[perm:read]
	MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// Rule[perm:read]
	MsigCancel(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// Rule[perm:read]
	MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (cid.Cid, error)
	// Rule[perm:read]
	MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error)
}
