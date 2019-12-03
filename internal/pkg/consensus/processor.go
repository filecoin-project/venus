package consensus

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var (
	// Tags
	msgMethodKey = tag.MustNewKey("consensus/keys/message_method")

	// Timers
	amTimer = metrics.NewTimerMs("consensus/apply_message", "Duration of message application in milliseconds", msgMethodKey)
)

// MessageValidator validates the syntax and semantics of a message before it is applied.
type MessageValidator interface {
	// Validate checks a message for validity.
	Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error
}

// BlockRewarder applies all rewards due to the miner's owner for processing a block including block reward and gas
type BlockRewarder interface {
	// BlockReward pays out the mining reward
	BlockReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address) error

	// GasReward pays gas from the sender to the miner
	GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.UnsignedMessage, cost types.AttoFIL) error
}

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *types.MessageReceipt
	ExecutionError error
}

// ApplyMessageResult is the result of applying a single message.
type ApplyMessageResult struct {
	ApplicationResult        // Application-level result, if error is nil.
	Failure            error // Failure to apply the message
	FailureIsPermanent bool  // Whether failure is permanent, has no chance of succeeding later.
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	validator     MessageValidator
	blockRewarder BlockRewarder
	actors        builtin.Actors
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		validator:     NewDefaultMessageValidator(),
		blockRewarder: NewDefaultBlockRewarder(),
		actors:        builtin.DefaultActors,
	}
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(validator MessageValidator, rewarder BlockRewarder, actors builtin.Actors) *DefaultProcessor {
	return &DefaultProcessor{
		validator:     validator,
		blockRewarder: rewarder,
		actors:        actors,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all
// blocks in a TipSet.  It is similar to ProcessBlock with a few key differences.
// Most importantly ProcessTipSet relies on the precondition that each input block
// is valid with respect to the base state st, that is, ProcessBlock is free of
// errors when applied to each block individually over the given state.
// ProcessTipSet only returns errors in the case of faults.  Other errors
// coming from calls to ApplyMessage can be traced to different blocks in the
// TipSet containing conflicting messages and are returned in the result slice.
// Blocks are applied in the sorted order of their tickets.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.StorageMap, ts block.TipSet, tsMessages [][]*types.UnsignedMessage, ancestors []block.TipSet) (results []*ApplyMessageResult, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	h, err := ts.Height()
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "processing empty tipset")
	}
	bh := types.NewBlockHeight(h)

	dedupedMessages, err := DeduppedMessages(tsMessages)

	for blkIdx := 0; blkIdx < ts.Len(); blkIdx++ {
		blk := ts.At(blkIdx)
		minerOwnerAddr, err := p.minerOwnerAddress(ctx, st, vms, blk.Miner)
		if err != nil {
			return nil, err
		}

		blkMessages := dedupedMessages[blkIdx]
		blkResults, err := p.ApplyMessagesAndPayRewards(ctx, st, vms, blkMessages, minerOwnerAddr, bh, ancestors)
		if err != nil {
			return nil, err
		}

		results = append(results, blkResults...)
	}
	return
}

// DeduppedMessages removes all messages that have the same cid
func DeduppedMessages(tsMessages [][]*types.UnsignedMessage) ([][]*types.UnsignedMessage, error) {
	allMessages := make([][]*types.UnsignedMessage, len(tsMessages))
	msgFilter := make(map[cid.Cid]struct{})

	for i, blkMessages := range tsMessages {
		for _, msg := range blkMessages {
			mCid, err := msg.Cid()
			if err != nil {
				return nil, err
			}

			_, found := msgFilter[mCid]
			if !found {
				allMessages[i] = append(allMessages[i], msg)
				msgFilter[mCid] = struct{}{}
			}
		}
	}
	return allMessages, nil
}

// ApplyMessage attempts to apply a message to a state tree. It is the
// sole driver of state tree transitions in the system. Both block
// validation and mining use this function and we should treat any changes
// to it with extreme care.
//
// If ApplyMessage returns no error then the message was successfully applied
// to the state tree: it did not result in any invalid transitions. As you will see
// below, this does not necessarily mean that the message "succeeded" for some
// senses of "succeeded". We choose therefore to say the message was or was not
// successfully applied.
//
// If ApplyMessage returns an error then the message would've resulted in
// an invalid state transition -- it was not successfully applied. When
// ApplyMessage returns an error one of three predicates will be true:
//   - IsFault(err): a system fault occurred (corrupt disk, violated precondition,
//     etc). This is Bad. Caller should stop doing whatever they are doing and get a doctor.
//     No guarantees are made about the state of the state tree.
//   - IsApplyErrorPermanent: the message was not applied and is unlikely
//     to *ever* be successfully applied (equivalently, it is unlikely to
//     ever result in a valid state transition). For example, the message might
//     attempt to transfer negative value. The message should probably be discarded.
//     All state tree mutations will have been reverted.
//   - IsApplyErrorTemporary: the message was not applied though it is
//     *possible* but not certain that the message may become applyable in
//     the future (eg, nonce is too high). The state was reverted.
//
// Please carefully consider the following intent with respect to messages.
// The intentions span the following concerns:
//   - whether the message was successfully applied: if not don't include it
//     in a block. If so inc sender's nonce and include it in a block.
//   - whether the message might be successfully applied at a later time
//     (IsApplyErrorTemporary) vs not (IsApplyErrorPermanent). If the caller
//     is the mining code it could remove permanently unapplyable messages from
//     the message pool but keep temporarily unapplyable messages around to try
//     applying to a future block.
//   - whether to keep or revert state: should we keep or revert state changes
//     caused by the message and its callees? We always revert state changes
//     from unapplyable messages. We might or might not revert changes from
//     applyable messages.
//
// Specific intentions include:
//   - fault errors: immediately return to the caller no matter what
//   - nonce too low: permanently unapplyable (don't include, revert changes, discard)
//   	-- if we have a re-order of the chain the message with nonce too low could
//       become applyable, but having two of the same message with the same nonce is
//       nonce-sensical
//   - nonce too high: temporarily unapplyable (don't include, revert, keep in pool)
//   - sender account exists but insufficient funds: successfully applied
//       (include it in the block but revert its changes). This an explicit choice
//       to make failing transfers not replayable (just like a bank transfer is not
//       replayable).
//   - sender account does not exist: temporarily unapplyable (don't include, revert,
//       keep in pool). There could be an account-creating message forthcoming.
//   - send to self: permanently unapplyable (don't include in a block, revert changes,
//       discard)
//   - transfer negative value: permanently unapplyable (as above)
//   - all other vmerrors: successfully applied! Include in the block and
//       revert changes. Necessarily all vm errors that are not faults are
//       revert errors.
//   - everything else: successfully applied (include, keep changes)
//
func (p *DefaultProcessor) ApplyMessage(ctx context.Context, st state.Tree, vms vm.StorageMap, msg *types.UnsignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight, gasTracker *vm.GasTracker, ancestors []block.TipSet) (result *ApplicationResult, err error) {
	msgCid, err := msg.Cid()
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "could not get message cid")
	}

	tagMethod := fmt.Sprintf("%s", msg.Method)
	ctx, err = tag.New(ctx, tag.Insert(msgMethodKey, tagMethod))
	if err != nil {
		log.Debugf("failed to insert tag for message method: %s", err.Error())
	}

	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ApplyMessage")
	span.AddAttributes(trace.StringAttribute("message", msgCid.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	// used for log timer call below
	amsw := amTimer.Start(ctx)
	defer amsw.Stop(ctx)

	cachedStateTree := state.NewCachedTree(st)

	r, err := p.attemptApplyMessage(ctx, cachedStateTree, vms, msg, bh, gasTracker, ancestors)
	if err == nil {
		err = cachedStateTree.Commit(ctx)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not commit state tree")
		}
	} else if errors.IsFault(err) {
		return nil, err
	} else if !errors.ShouldRevert(err) {
		return nil, errors.NewFaultError("someone is a bad programmer: only return revert and fault errors")
	}

	if r.GasAttoFIL.IsPositive() {
		gasError := p.blockRewarder.GasReward(ctx, st, minerOwnerAddr, msg, r.GasAttoFIL)
		if gasError != nil {
			return nil, errors.NewFaultError("failed to transfer gas reward to owner of miner")
		}
	}

	// Reject invalid state transitions.
	var executionError error
	if isTemporaryError(err) {
		return nil, errors.ApplyErrorTemporaryWrapf(err, "apply message failed")
	} else if isPermanentError(err) {
		return nil, errors.ApplyErrorPermanentWrapf(err, "apply message failed")
	} else if err != nil { // nolint: staticcheck
		// Return the executionError to caller for informational purposes, but otherwise
		// do nothing. All other vm errors are ok: the state was rolled back
		// above but we applied the message successfully. This intentionally
		// includes errInsufficientFunds because we don't want the message
		// to be replayable.
		executionError = err
		log.Infof("ApplyMessage failed: %s %s", executionError, msg.String())
	}

	// At this point we consider the message successfully applied so inc
	// the nonce.
	fromActor, err := st.GetActor(ctx, msg.From)
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "couldn't load from actor")
	}
	fromActor.IncNonce()
	if err := st.SetActor(ctx, msg.From, fromActor); err != nil {
		return nil, errors.FaultErrorWrap(err, "could not set from actor after inc nonce")
	}

	return &ApplicationResult{Receipt: r, ExecutionError: executionError}, nil
}

var (
	// These errors are only to be used by ApplyMessage; they shouldn't be
	// used in any other context as they are an implementation detail.
	errFromAccountNotFound       = errors.NewRevertError("from (sender) account not found")
	errGasAboveBlockLimit        = errors.NewRevertError("message gas limit above block gas limit")
	errGasPriceZero              = errors.NewRevertError("message gas price is zero")
	errGasTooHighForCurrentBlock = errors.NewRevertError("message gas limit too high for current block")
	errNonceTooHigh              = errors.NewRevertError("nonce too high")
	errNonceTooLow               = errors.NewRevertError("nonce too low")
	errNonAccountActor           = errors.NewRevertError("message from non-account actor")
	errNegativeValue             = errors.NewRevertError("negative value")
	errInsufficientGas           = errors.NewRevertError("balance insufficient to cover transfer+gas")
	errInvalidSignature          = errors.NewRevertError("invalid signature by sender over message data")
	// TODO we'll eventually handle sending to self.
	errSelfSend = errors.NewRevertError("cannot send to self")
)

// CallQueryMethod calls a method on an actor in the given state tree. It does
// not make any changes to the state/blockchain and is useful for interrogating
// actor state. Block height bh is optional; some methods will ignore it.
func (p *DefaultProcessor) CallQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method types.MethodID, params []byte, from address.Address, optBh *types.BlockHeight) ([][]byte, uint8, error) {
	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedTree(st)

	msg := &types.UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: 0,
		Value:      types.ZeroAttoFIL,
		Method:     method,
		Params:     params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	// translate address before retrieving from actor
	toAddr, err := p.resolveAddress(ctx, msg, cachedSt, vms, gasTracker)
	if err != nil {
		return nil, 1, errors.FaultErrorWrapf(err, "Could not resolve actor address")
	}

	toActor, err := st.GetActor(ctx, toAddr)
	if err != nil {
		return nil, 1, errors.ApplyErrorPermanentWrapf(err, "failed to get To actor")
	}

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
		Actors:      p.actors,
	}

	vmCtx := vm.NewVMContext(vmCtxParams)
	ret, retCode, err := vm.Send(ctx, vmCtx)
	return ret, retCode, err
}

// PreviewQueryMethod estimates the amount of gas that will be used by a method
// call. It accepts all the same arguments as CallQueryMethod.
func (p *DefaultProcessor) PreviewQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method types.MethodID, params []byte, from address.Address, optBh *types.BlockHeight) (types.GasUnits, error) {
	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedTree(st)

	msg := &types.UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: 0,
		Value:      types.ZeroAttoFIL,
		Method:     method,
		Params:     params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	// translate address before retrieving from actor
	toAddr, err := p.resolveAddress(ctx, msg, cachedSt, vms, gasTracker)
	if err != nil {
		return types.NewGasUnits(0), errors.FaultErrorWrapf(err, "Could not resolve actor address")
	}

	toActor, err := st.GetActor(ctx, toAddr)
	if err != nil {
		return types.NewGasUnits(0), errors.ApplyErrorPermanentWrapf(err, "failed to get To actor")
	}

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
		Actors:      p.actors,
	}
	vmCtx := vm.NewVMContext(vmCtxParams)
	_, _, err = vm.Send(ctx, vmCtx)

	return vmCtx.GasUnits(), err
}

// attemptApplyMessage encapsulates the work of trying to apply the message in order
// to make ApplyMessage more readable. The distinction is that attemptApplyMessage
// should deal with trying to apply the message to the state tree whereas
// ApplyMessage should deal with any side effects and how it should be presented
// to the caller. attemptApplyMessage should only be called from ApplyMessage.
func (p *DefaultProcessor) attemptApplyMessage(ctx context.Context, st *state.CachedTree, store vm.StorageMap, msg *types.UnsignedMessage, bh *types.BlockHeight, gasTracker *vm.GasTracker, ancestors []block.TipSet) (*types.MessageReceipt, error) {
	gasTracker.ResetForNewMessage(msg)
	if err := blockGasLimitError(gasTracker); err != nil {
		return &types.MessageReceipt{
			ExitCode:   errors.CodeError(err),
			GasAttoFIL: types.ZeroAttoFIL,
		}, err
	}

	fromActor, err := st.GetActor(ctx, msg.From)
	if state.IsActorNotFoundError(err) {
		return &types.MessageReceipt{
			ExitCode:   errors.CodeError(err),
			GasAttoFIL: types.ZeroAttoFIL,
		}, errFromAccountNotFound
	} else if err != nil {
		return nil, errors.FaultErrorWrapf(err, "failed to get From actor %s", msg.From)
	}

	err = p.validator.Validate(ctx, msg, fromActor)
	if err != nil {
		return &types.MessageReceipt{
			ExitCode:   errors.CodeError(err),
			GasAttoFIL: types.ZeroAttoFIL,
		}, err
	}

	// Processing an external message from an empty actor upgrades it to an account actor.
	if fromActor.Empty() {
		err := account.UpgradeActor(fromActor)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "failed to upgrade empty actor")
		}
	}

	// translate address before retrieving from actor
	toAddr, err := p.resolveAddress(ctx, msg, st, store, gasTracker)
	if err != nil {
		return nil, errors.FaultErrorWrapf(err, "Could not resolve actor address")
	}

	toActor, err := st.GetOrCreateActor(ctx, toAddr, func() (*actor.Actor, error) {
		// Addresses are deterministic so sending a message to a non-existent address must not install an actor,
		// else actors could be installed ahead of address activation. So here we create the empty, upgradable
		// actor to collect any balance that may be transferred.
		return &actor.Actor{}, nil
	})
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "failed to get To actor")
	}

	vmCtxParams := vm.NewContextParams{
		From:        fromActor,
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   msg,
		State:       st,
		StorageMap:  store,
		GasTracker:  gasTracker,
		BlockHeight: bh,
		Ancestors:   ancestors,
		Actors:      p.actors,
	}
	vmCtx := vm.NewVMContext(vmCtxParams)

	ret, exitCode, vmErr := vm.Send(ctx, vmCtx)
	if errors.IsFault(vmErr) {
		return nil, vmErr
	}

	// compute gas charge
	gasCharge := msg.GasPrice.MulBigInt(big.NewInt(int64(vmCtx.GasUnits())))

	receipt := &types.MessageReceipt{
		ExitCode:   exitCode,
		GasAttoFIL: gasCharge,
	}

	receipt.Return = append(receipt.Return, ret...)

	return receipt, vmErr
}

// resolveAddress looks up associated id address if actor address. Otherwise it returns the same address.
func (p *DefaultProcessor) resolveAddress(ctx context.Context, msg *types.UnsignedMessage, st *state.CachedTree, vms vm.StorageMap, gt *vm.GasTracker) (address.Address, error) {
	if msg.To.Protocol() != address.Actor {
		return msg.To, nil
	}

	to, err := st.GetActor(ctx, address.InitAddress)
	if err != nil {
		return address.Undef, err
	}

	vmCtxParams := vm.NewContextParams{
		Message:    msg,
		State:      st,
		StorageMap: vms,
		GasTracker: gt,
		Actors:     p.actors,
		To:         to,
	}
	vmCtx := vm.NewVMContext(vmCtxParams)
	ret, _, err := vmCtx.Send(address.InitAddress, initactor.GetActorIDForAddress, types.ZeroAttoFIL, []interface{}{msg.To})
	if err != nil {
		return address.Undef, err
	}

	id, err := abi.Deserialize(ret[0], abi.Integer)
	if err != nil {
		return address.Undef, err
	}

	idAddr, err := address.NewIDAddress(id.Val.(*big.Int).Uint64())
	if err != nil {
		return address.Undef, err
	}

	return idAddr, nil
}

// ApplyMessagesAndPayRewards pays the block mining reward to the miner's owner and then applies
// messages, in order, to a state tree.
// Returns a message application result for each message.
func (p *DefaultProcessor) ApplyMessagesAndPayRewards(ctx context.Context, st state.Tree, vms vm.StorageMap,
	messages []*types.UnsignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight,
	ancestors []block.TipSet) ([]*ApplyMessageResult, error) {
	var results []*ApplyMessageResult

	// Pay block reward.
	if err := p.blockRewarder.BlockReward(ctx, st, minerOwnerAddr); err != nil {
		return nil, err
	}

	// Process all messages.
	gasTracker := vm.NewGasTracker()
	for _, msg := range messages {
		r, err := p.ApplyMessage(ctx, st, vms, msg, minerOwnerAddr, bh, gasTracker, ancestors)
		switch {
		case errors.IsFault(err):
			return nil, err
		case errors.IsApplyErrorPermanent(err):
			results = append(results, &ApplyMessageResult{ApplicationResult{}, err, true})
		case errors.IsApplyErrorTemporary(err):
			results = append(results, &ApplyMessageResult{ApplicationResult{}, err, false})
		case err != nil:
			panic("someone is a bad programmer: error is neither fault, perm or temp")
		default:
			results = append(results, &ApplyMessageResult{*r, nil, false})
		}
	}
	return results, nil
}

// DefaultBlockRewarder pays the block reward from the network actor to the miner's owner.
type DefaultBlockRewarder struct{}

// NewDefaultBlockRewarder creates a new rewarder that actually pays the appropriate rewards.
func NewDefaultBlockRewarder() *DefaultBlockRewarder {
	return &DefaultBlockRewarder{}
}

var _ BlockRewarder = (*DefaultBlockRewarder)(nil)

// BlockReward transfers the block reward from the network actor to the miner's owner.
func (br *DefaultBlockRewarder) BlockReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address) error {
	cachedTree := state.NewCachedTree(st)
	if err := rewardTransfer(ctx, address.NetworkAddress, minerOwnerAddr, br.BlockRewardAmount(), cachedTree); err != nil {
		return errors.FaultErrorWrap(err, "Error attempting to pay block reward")
	}
	return cachedTree.Commit(ctx)
}

// GasReward transfers the gas cost reward from the sender actor to the minerOwnerAddr
func (br *DefaultBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.UnsignedMessage, cost types.AttoFIL) error {
	cachedTree := state.NewCachedTree(st)
	if err := rewardTransfer(ctx, msg.From, minerOwnerAddr, cost, cachedTree); err != nil {
		return errors.FaultErrorWrap(err, "Error attempting to pay gas reward")
	}
	return cachedTree.Commit(ctx)
}

// BlockRewardAmount returns the max FIL value miners can claim as the block reward.
// TODO this is one of the system parameters that should be configured as part of
// https://github.com/filecoin-project/go-filecoin/issues/884.
func (br *DefaultBlockRewarder) BlockRewardAmount() types.AttoFIL {
	return types.NewAttoFILFromFIL(1000)
}

// rewardTransfer retrieves two actors from the given addresses and attempts to transfer the given value from the balance of the first's to the second.
func rewardTransfer(ctx context.Context, fromAddr, toAddr address.Address, value types.AttoFIL, st *state.CachedTree) error {
	fromActor, err := st.GetActor(ctx, fromAddr)
	if err != nil {
		return errors.FaultErrorWrap(err, "could not retrieve from actor for reward transfer.")
	}

	toActor, err := st.GetOrCreateActor(ctx, toAddr, func() (*actor.Actor, error) {
		return &actor.Actor{}, nil
	})
	if err != nil {
		return errors.FaultErrorWrap(err, "failed to get To actor")
	}

	return vm.Transfer(fromActor, toActor, value)
}

func blockGasLimitError(gasTracker *vm.GasTracker) error {
	if gasTracker.GasAboveBlockLimit() {
		return errGasAboveBlockLimit
	} else if gasTracker.GasTooHighForCurrentBlock() {
		return errGasTooHighForCurrentBlock
	}
	return nil
}

func isTemporaryError(err error) bool {
	return err == errFromAccountNotFound ||
		err == errNonceTooHigh ||
		err == errGasTooHighForCurrentBlock
}

func isPermanentError(err error) bool {
	return err == errInsufficientGas ||
		err == errSelfSend ||
		err == errInvalidSignature ||
		err == errNonceTooLow ||
		err == errNonAccountActor ||
		err == errNegativeValue ||
		err == errors.Errors[errors.ErrCannotTransferNegativeValue] ||
		err == errGasAboveBlockLimit
}

// minerOwnerAddress finds the address of the owner of the given miner
func (p *DefaultProcessor) minerOwnerAddress(ctx context.Context, st state.Tree, vms vm.StorageMap, minerAddr address.Address) (address.Address, error) {
	ret, code, err := p.CallQueryMethod(ctx, st, vms, minerAddr, miner.GetOwner, []byte{}, address.Undef, types.NewBlockHeight(0))
	if err != nil {
		return address.Undef, errors.FaultErrorWrap(err, "could not get miner owner")
	}
	if code != 0 {
		return address.Undef, errors.NewFaultErrorf("could not get miner owner. error code %d", code)
	}
	return address.NewFromBytes(ret[0])
}
