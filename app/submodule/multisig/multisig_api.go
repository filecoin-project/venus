package multisig

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	multisig2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/multisig"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/multisig"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

var _ apiface.IMultiSig = &multiSig{}

type multiSig struct {
	*MultiSigSubmodule
}

type MsigProposeResponse int

const (
	MsigApprove MsigProposeResponse = iota
	MsigCancel
)

func newMultiSig(m *MultiSigSubmodule) apiface.IMultiSig {
	return &multiSig{
		MultiSigSubmodule: m,
	}
}

func (a *multiSig) messageBuilder(ctx context.Context, from address.Address) (multisig.MessageBuilder, error) {
	nver, err := a.state.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	return multisig.Message(specactors.VersionForNetwork(nver), from), nil
}

// MsigCreate creates a multisig wallet
// It takes the following params: <required number of senders>, <approving addresses>, <unlock duration>
//<initial balance>, <sender address of the create msg>, <gas price>
func (a *multiSig) MsigCreate(ctx context.Context, req uint64, addrs []address.Address, duration abi.ChainEpoch, val types.BigInt, src address.Address, gp types.BigInt) (cid.Cid, error) {

	mb, err := a.messageBuilder(ctx, src)
	if err != nil {
		return cid.Undef, err
	}

	msg, err := mb.Create(addrs, req, 0, duration, val)
	if err != nil {
		return cid.Undef, err
	}

	// send the message out to the network
	smsg, err := a.mpool.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return cid.Undef, err
	}

	return smsg.Cid(), nil
}

// MsigPropose proposes a multisig message
// It takes the following params: <multisig address>, <recipient address>, <value to transfer>,
// <sender address of the propose msg>, <method to call in the proposed message>, <params to include in the proposed message>
func (a *multiSig) MsigPropose(ctx context.Context, msig address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {

	mb, err := a.messageBuilder(ctx, src)
	if err != nil {
		return cid.Undef, err
	}

	msg, err := mb.Propose(msig, to, amt, abi.MethodNum(method), params)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to create proposal: %w", err)
	}

	smsg, err := a.mpool.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to push message: %w", err)
	}

	return smsg.Cid(), nil
}

// MsigAddPropose proposes adding a signer in the multisig
// It takes the following params: <multisig address>, <sender address of the propose msg>,
// <new signer>, <whether the number of required signers should be increased>
func (a *multiSig) MsigAddPropose(ctx context.Context, msig address.Address, src address.Address, newAdd address.Address, inc bool) (cid.Cid, error) {
	enc, actErr := serializeAddParams(newAdd, inc)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigPropose(ctx, msig, msig, big.Zero(), src, uint64(multisig.Methods.AddSigner), enc)
}

func (a *multiSig) MsigAddApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, newAdd address.Address, inc bool) (cid.Cid, error) {
	enc, actErr := serializeAddParams(newAdd, inc)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigApproveTxnHash(ctx, msig, txID, proposer, msig, big.Zero(), src, uint64(multisig.Methods.AddSigner), enc)
}

// MsigAddApprove approves a previously proposed AddSigner message
// It takes the following params: <multisig address>, <sender address of the approve msg>, <proposed message ID>,
// <proposer address>, <new signer>, <whether the number of required signers should be increased>
func (a *multiSig) MsigAddCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, newAdd address.Address, inc bool) (cid.Cid, error) {
	enc, actErr := serializeAddParams(newAdd, inc)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigCancel(ctx, msig, txID, msig, big.Zero(), src, uint64(multisig.Methods.AddSigner), enc)
}

// MsigSwapPropose proposes swapping 2 signers in the multisig
// It takes the following params: <multisig address>, <sender address of the propose msg>,
// <old signer>, <new signer>
func (a *multiSig) MsigSwapPropose(ctx context.Context, msig address.Address, src address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {
	enc, actErr := serializeSwapParams(oldAdd, newAdd)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigPropose(ctx, msig, msig, big.Zero(), src, uint64(multisig.Methods.SwapSigner), enc)
}

// MsigSwapApprove approves a previously proposed SwapSigner
// It takes the following params: <multisig address>, <sender address of the approve msg>, <proposed message ID>,
// <proposer address>, <old signer>, <new signer>
func (a *multiSig) MsigSwapApprove(ctx context.Context, msig address.Address, src address.Address, txID uint64, proposer address.Address, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {
	enc, actErr := serializeSwapParams(oldAdd, newAdd)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigApproveTxnHash(ctx, msig, txID, proposer, msig, big.Zero(), src, uint64(multisig.Methods.SwapSigner), enc)
}

func (a *multiSig) MsigSwapCancel(ctx context.Context, msig address.Address, src address.Address, txID uint64, oldAdd address.Address, newAdd address.Address) (cid.Cid, error) {
	enc, actErr := serializeSwapParams(oldAdd, newAdd)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigCancel(ctx, msig, txID, msig, big.Zero(), src, uint64(multisig.Methods.SwapSigner), enc)
}

// MsigSwapCancel cancels a previously proposed SwapSigner message
// It takes the following params: <multisig address>, <sender address of the cancel msg>, <proposed message ID>,
// <old signer>, <new signer>
func (a *multiSig) MsigApprove(ctx context.Context, msig address.Address, txID uint64, src address.Address) (cid.Cid, error) {
	return a.msigApproveOrCancelSimple(ctx, MsigApprove, msig, txID, src)
}

// MsigApproveTxnHash approves a previously-proposed multisig message, specified
// using both transaction ID and a hash of the parameters used in the
// proposal. This method of approval can be used to ensure you only approve
// exactly the transaction you think you are.
// It takes the following params: <multisig address>, <proposed message ID>, <proposer address>, <recipient address>, <value to transfer>,
// <sender address of the approve msg>, <method to call in the proposed message>, <params to include in the proposed message>
func (a *multiSig) MsigApproveTxnHash(ctx context.Context, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {
	return a.msigApproveOrCancelTxnHash(ctx, MsigApprove, msig, txID, proposer, to, amt, src, method, params)
}

// MsigCancel cancels a previously-proposed multisig message
// It takes the following params: <multisig address>, <proposed transaction ID>, <recipient address>, <value to transfer>,
// <sender address of the cancel msg>, <method to call in the proposed message>, <params to include in the proposed message>
func (a *multiSig) MsigCancel(ctx context.Context, msig address.Address, txID uint64, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {
	return a.msigApproveOrCancelTxnHash(ctx, MsigCancel, msig, txID, src, to, amt, src, method, params)
}

// MsigRemoveSigner proposes the removal of a signer from the multisig.
// It accepts the multisig to make the change on, the proposer address to
// send the message from, the address to be removed, and a boolean
// indicating whether or not the signing threshold should be lowered by one
// along with the address removal.
func (a *multiSig) MsigRemoveSigner(ctx context.Context, msig address.Address, proposer address.Address, toRemove address.Address, decrease bool) (cid.Cid, error) {
	enc, actErr := serializeRemoveParams(toRemove, decrease)
	if actErr != nil {
		return cid.Undef, actErr
	}

	return a.MsigPropose(ctx, msig, msig, types.NewInt(0), proposer, uint64(multisig.Methods.RemoveSigner), enc)
}

// MsigGetVested returns the amount of FIL that vested in a multisig in a certain period.
// It takes the following params: <multisig address>, <start epoch>, <end epoch>
func (a *multiSig) MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error) {
	startTS, err := a.state.ChainGetTipSet(ctx, start)
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("loading start tipset %s: %w", start, err)
	}

	endTS, err := a.state.ChainGetTipSet(ctx, end)
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("loading end tipset %s: %w", end, err)
	}

	if startTS.Height() > endTS.Height() {
		return types.EmptyInt, xerrors.Errorf("start tipset %d is after end tipset %d", startTS.Height(), endTS.Height())
	} else if startTS.Height() == endTS.Height() {
		return big.Zero(), nil
	}

	//LoadActor(ctx, addr, endTs)
	act, err := a.state.GetParentStateRootActor(ctx, endTS, addr)
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("failed to load multisig actor at end epoch: %w", err)
	}

	msas, err := multisig.Load(a.store.Store(ctx), act)
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("failed to load multisig actor state: %w", err)
	}

	startLk, err := msas.LockedBalance(startTS.Height())
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("failed to compute locked balance at start height: %w", err)
	}

	endLk, err := msas.LockedBalance(endTS.Height())
	if err != nil {
		return types.EmptyInt, xerrors.Errorf("failed to compute locked balance at end height: %w", err)
	}

	return types.BigSub(startLk, endLk), nil
}

func (a *multiSig) msigApproveOrCancelSimple(ctx context.Context, operation MsigProposeResponse, msig address.Address, txID uint64, src address.Address) (cid.Cid, error) {
	if msig == address.Undef {
		return cid.Undef, xerrors.Errorf("must provide multisig address")
	}

	if src == address.Undef {
		return cid.Undef, xerrors.Errorf("must provide source address")
	}

	mb, err := a.messageBuilder(ctx, src)
	if err != nil {
		return cid.Undef, err
	}

	var msg *types.Message
	switch operation {
	case MsigApprove:
		msg, err = mb.Approve(msig, txID, nil)
	case MsigCancel:
		msg, err = mb.Cancel(msig, txID, nil)
	default:
		return cid.Undef, xerrors.Errorf("Invalid operation for msigApproveOrCancel")
	}
	if err != nil {
		return cid.Undef, err
	}

	smsg, err := a.mpool.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return cid.Undef, err
	}

	return smsg.Cid(), nil

}

func (a *multiSig) msigApproveOrCancelTxnHash(ctx context.Context, operation MsigProposeResponse, msig address.Address, txID uint64, proposer address.Address, to address.Address, amt types.BigInt, src address.Address, method uint64, params []byte) (cid.Cid, error) {
	if msig == address.Undef {
		return cid.Undef, xerrors.Errorf("must provide multisig address")
	}

	if src == address.Undef {
		return cid.Undef, xerrors.Errorf("must provide source address")
	}

	if proposer.Protocol() != address.ID {
		proposerID, err := a.state.StateLookupID(ctx, proposer, types.EmptyTSK)
		if err != nil {
			return cid.Undef, err
		}
		proposer = proposerID
	}

	p := multisig.ProposalHashData{
		Requester: proposer,
		To:        to,
		Value:     amt,
		Method:    abi.MethodNum(method),
		Params:    params,
	}

	mb, err := a.messageBuilder(ctx, src)
	if err != nil {
		return cid.Undef, err
	}

	var msg *types.Message
	switch operation {
	case MsigApprove:
		msg, err = mb.Approve(msig, txID, &p)
	case MsigCancel:
		msg, err = mb.Cancel(msig, txID, &p)
	default:
		return cid.Undef, xerrors.Errorf("Invalid operation for msigApproveOrCancel")
	}
	if err != nil {
		return cid.Undef, err
	}

	smsg, err := a.mpool.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return cid.Undef, err
	}

	return smsg.Cid(), nil
}

func serializeAddParams(new address.Address, inc bool) ([]byte, error) {
	enc, actErr := specactors.SerializeParams(&multisig2.AddSignerParams{
		Signer:   new,
		Increase: inc,
	})
	if actErr != nil {
		return nil, actErr
	}

	return enc, nil
}

func serializeSwapParams(old address.Address, new address.Address) ([]byte, error) {
	enc, actErr := specactors.SerializeParams(&multisig2.SwapSignerParams{
		From: old,
		To:   new,
	})
	if actErr != nil {
		return nil, actErr
	}

	return enc, nil
}

func serializeRemoveParams(rem address.Address, dec bool) ([]byte, error) {
	enc, actErr := specactors.SerializeParams(&multisig2.RemoveSignerParams{
		Signer:   rem,
		Decrease: dec,
	})
	if actErr != nil {
		return nil, actErr
	}

	return enc, nil
}
