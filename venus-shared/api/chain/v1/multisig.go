package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IMultiSig interface {
	MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val types.BigInt, src address.Address, gp types.BigInt) (*types.MessagePrototype, error)   //perm:sign
	MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (*types.MessagePrototype, error)         //perm:sign
	MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (*types.MessagePrototype, error)                                        //perm:sign
	MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (*types.MessagePrototype, error) //perm:sign
	MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (*types.MessagePrototype, error)                            //perm:sign
	// MsigCancel cancels a previously-proposed multisig message
	// It takes the following params: <multisig address>, <proposed transaction ID>, <recipient address>, <value to transfer>,
	// <sender address of the cancel msg>, <method to call in the proposed message>, <params to include in the proposed message>
	MsigCancelTxnHash(context.Context, address.Address, uint64, address.Address, types.BigInt, address.Address, uint64, []byte) (*types.MessagePrototype, error)                                                                   //perm:sign
	MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (*types.MessagePrototype, error)                                                               //perm:sign
	MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (*types.MessagePrototype, error)                        //perm:sign
	MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (*types.MessagePrototype, error)                                                   //perm:sign
	MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (*types.MessagePrototype, error)                                                                                                      //perm:sign
	MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (*types.MessagePrototype, error) //perm:sign
	MsigCancel(ctx context.Context, msig address.Address, txID uint64, src address.Address) (*types.MessagePrototype, error)                                                                                                       //perm:sign
	MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (*types.MessagePrototype, error)                                                                //perm:sign

	// MsigGetVested returns the amount of FIL that vested in a multisig in a certain period.
	// It takes the following params: <multisig address>, <start epoch>, <end epoch>
	MsigGetVested(context.Context, address.Address, types.TipSetKey, types.TipSetKey) (types.BigInt, error) //perm:read
	// MsigGetAvailableBalance returns the portion of a multisig's balance that can be withdrawn or spent
	MsigGetAvailableBalance(context.Context, address.Address, types.TipSetKey) (types.BigInt, error) //perm:read
	// MsigGetVestingSchedule returns the vesting details of a given multisig.
	MsigGetVestingSchedule(context.Context, address.Address, types.TipSetKey) (types.MsigVesting, error) //perm:read

	//MsigGetPending returns pending transactions for the given multisig
	//wallet. Once pending transactions are fully approved, they will no longer
	//appear here.
	MsigGetPending(context.Context, address.Address, types.TipSetKey) ([]*types.MsigTransaction, error) //perm:read
}
