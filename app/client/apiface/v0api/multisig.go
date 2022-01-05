package v0api

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	types "github.com/filecoin-project/venus/venus-shared/chain"
)

type IMultiSig interface {
	// MsigCreate creates a multisig wallet
	// It takes the following params: <required number of senders>, <approving addresses>, <unlock duration>
	//<initial balance>, <sender address of the create msg>, <gas price>
	// Rule[perm:sign]
	MsigCreate(context.Context, uint64, []address.Address, abi.ChainEpoch, types.BigInt, address.Address, types.BigInt) (cid.Cid, error)
	// Rule[perm:sign]
	MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// Rule[perm:sign]
	MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:sign]
	MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:sign]
	MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (cid.Cid, error)
	// Rule[perm:sign]
	MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:sign]
	MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:sign]
	MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// Rule[perm:sign]
	MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error)
	// MsigCancel cancels a previously-proposed multisig message
	// It takes the following params: <multisig address>, <proposed transaction ID>, <recipient address>, <value to transfer>,
	// <sender address of the cancel msg>, <method to call in the proposed message>, <params to include in the proposed message>
	// Rule[perm:sign]
	MsigCancelTxnHash(context.Context, address.Address, uint64, address.Address, types.BigInt, address.Address, uint64, []byte) (cid.Cid, error)
	// Rule[perm:sign]
	MsigCancel(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error)
	// Rule[perm:sign]
	MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (cid.Cid, error)
	// Rule[perm:read]
	MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error)
}
