package apiface

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type IMultiSig interface {
	// MsigCreate creates a multisig wallet
	// @req: required number of senders
	// @addrs: approving addresses
	// @duration: unlock duration
	// @val: initial balance
	// @src: sender address of the create msg
	// @gp: gas price
	// Rule[perm:read]
	MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val types.BigInt, src address.Address, gp types.BigInt) (cid.Cid, error)
	// MsigPropose proposes a multisig message
	// @msig: multisig address
	// @to: recipient address
	// @amt: value to transfer
	// @src: sender address of the propose msg
	// @method: method to call in the proposed message
	// @params: params to include in the proposed message>
	// Rule[perm:read]
	MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// MsigAddPropose proposes adding a signer in the multisig
	// @msig: multisig address
	// @src: sender address of the propose msg,
	// @newAdd: new signer
	// @inc: whether the number of required signers should be increased
	// Rule[perm:read]
	MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// MsigAddApprove approves a previously proposed AddSigner message
	// @msig: multisig address
	// @src: sender address of the approve msg
	// @txID: proposed message ID
	// @proposer: proposer address
	// @newAdd: new signer
	// @inc: whether the number of required signers should be increased
	// Rule[perm:read]
	MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (cid.Cid, error)
	// MsigAddCancel cancels a previously proposed AddSigner message
	// @msig: multisig address
	// @src: sender address of the cancel msg
	// @txID: proposed message ID
	// @newAdd: new signer
	// @inc: whether the number of required signers should be increased
	// Rule[perm:read]
	MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (cid.Cid, error)
	// MsigSwapPropose proposes swapping 2 signers in the multisig
	// @msig: multisig address
	// @src: sender address of the propose msg
	// @oldAdd: old signer
	// @newAdd: new signer
	// Rule[perm:read]
	MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// MsigSwapApprove approves a previously proposed SwapSigner
	// @msig: multisig address
	// @src: sender address of the approve msg
	// @txID: proposed message ID
	// @proposer: proposer address
	// @oldAdd: old signer
	// @newAdd: new signer
	// Rule[perm:read]
	MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// MsigSwapCancel cancels a previously proposed SwapSigner message
	// @msig: multisig address
	// @src: sender address of the cancel msg
	// @txID: proposed message ID
	// @oldAdd: old signer
	// @newAdd: new signer
	// Rule[perm:read]
	MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (cid.Cid, error)
	// MsigApprove approves a previously-proposed multisig message by transaction ID
	// @msig: multisig address
	// @txID: proposed transaction ID
	// @src: igner address
	// Rule[perm:read]
	MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error)
	// MsigApproveTxnHash approves a previously-proposed multisig message, specified
	// using both transaction ID and a hash of the parameters used in the
	// proposal. This method of approval can be used to ensure you only approve
	// exactly the transaction you think you are.
	// @msig: multisig address
	// @txID: proposed message ID
	// @proposer: proposer address
	// @to: recipient address
	// @amt: value to transfer
	// @src: sender address of the approve msg
	// @method: method to call in the proposed message
	// @params: params to include in the proposed message
	// Rule[perm:read]
	MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// MsigCancel cancels a previously-proposed multisig message
	// @msig: multisig address
	// @txID: roposed transaction ID
	// @to: recipient address
	// @amt: value to transfer
	// @src: sender address of the cancel msg
	// @method: method to call in the proposed message
	// @params: params to include in the proposed message
	// Rule[perm:read]
	MsigCancel(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error)
	// MsigRemoveSigner proposes the removal of a signer from the multisig.
	// It accepts the multisig to make the change on, the proposer address to
	// send the message from, the address to be removed, and a boolean
	// indicating whether or not the signing threshold should be lowered by one
	// along with the address removal.
	// @msig: multisig address
	// @proposer: proposer address
	// @toRemove: the member of multisig address
	// @decrease: whether the number of required signers should be decreased
	// Rule[perm:read]
	MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (cid.Cid, error)
	// MsigGetVested returns the amount of FIL that vested in a multisig in a certain period.
	// @msig: multisig address
	// @start: start epoch
	// @end: end epoch
	// Rule[perm:read]
	MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error)
}
