package consensus

import (
	"context"
	"math/big"

	"github.com/ipfs/go-cid"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

var (
	// Tags
	msgMethodKey = tag.MustNewKey("consensus/keys/message_method")

	// Timers
	amTimer = metrics.NewTimerMs("consensus/apply_message", "Duration of message application in milliseconds", msgMethodKey)
	pbTimer = metrics.NewTimerMs("consensus/process_block", "Duration of block processing in milliseconds")
)

// BlockRewarder applies all rewards due to the miner's owner for processing a block including block reward and gas
type BlockRewarder interface {
	// BlockReward pays out the mining reward
	BlockReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address) error

	// GasReward pays gas from the sender to the miner
	GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.SignedMessage, cost types.AttoFIL) error
}

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *types.MessageReceipt
	ExecutionError error
}

// ProcessTipSetResponse records the results of successfully applied messages,
// and the sets of successful and failed message cids.  Information of successes
// and failures is key for helping match user messages with receipts in the case
// of message conflicts.
type ProcessTipSetResponse struct {
	Results   []*ApplicationResult
	Successes map[cid.Cid]struct{}
	Failures  map[cid.Cid]struct{}
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	signedMessageValidator SignedMessageValidator
	blockRewarder          BlockRewarder
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		signedMessageValidator: NewDefaultMessageValidator(),
		blockRewarder:          NewDefaultBlockRewarder(),
	}
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(validator SignedMessageValidator, rewarder BlockRewarder) *DefaultProcessor {
	return &DefaultProcessor{
		signedMessageValidator: validator,
		blockRewarder:          rewarder,
	}
}

// ProcessBlock is the entrypoint for validating the state transitions
// of the messages in a block. When we receive a new block from the
// network ProcessBlock applies the block's messages to the beginning
// state tree ensuring that all transitions are valid, accumulating
// changes in the state tree, and returning the message receipts.
//
// ProcessBlock returns an error if the block contains a message, the
// application of which would result in an invalid state transition (eg, a
// message to transfer value from an unknown account). ProcessBlock
// can return one of three kinds of errors (see ApplyMessage: fault
// error, permanent error, temporary error). For the purposes of
// block validation the caller probably doesn't care if the error
// was temporary or permanent; either way the block has a bad
// message and should be thrown out. Caller should always differentiate
// a fault error as it signals something Very Bad has happened
// (eg, disk corruption).
//
// To be clear about intent: if ProcessBlock returns an ApplyError
// it is signaling that the message should not have been included
// in the block. If no error is returned this means that the
// message was applied, BUT SUCCESSFUL APPLICATION DOES NOT
// NECESSARILY MEAN THE IMPLIED CALL SUCCEEDED OR SENDER INTENT
// WAS REALIZED. It just means that the transition if any was
// valid. For example, a message that errors out in the VM
// will in many cases be successfully applied even though an
// error was thrown causing any state changes to be rolled back.
// See comments on ApplyMessage for specific intent.
func (p *DefaultProcessor) ProcessBlock(ctx context.Context, st state.Tree, vms vm.StorageMap, blk *types.Block, blkMessages []*types.SignedMessage, ancestors []types.TipSet) (results []*ApplicationResult, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessBlock")
	span.AddAttributes(trace.StringAttribute("block", blk.Cid().String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	pbsw := pbTimer.Start(ctx)
	defer pbsw.Stop(ctx)

	var emptyResults []*ApplicationResult

	// find miner's owner address
	minerOwnerAddr, err := minerOwnerAddress(ctx, st, vms, blk.Miner)
	if err != nil {
		return nil, err
	}

	bh := types.NewBlockHeight(uint64(blk.Height))
	res, faultErr := p.ApplyMessagesAndPayRewards(ctx, st, vms, blkMessages, minerOwnerAddr, bh, ancestors)
	if faultErr != nil {
		return emptyResults, faultErr
	}
	if len(res.PermanentErrors) > 0 {
		return emptyResults, res.PermanentErrors[0]
	}
	if len(res.TemporaryErrors) > 0 {
		return emptyResults, res.TemporaryErrors[0]
	}
	return res.Results, nil
}

// ProcessTipSet computes the state transition specified by the messages in all
// blocks in a TipSet.  It is similar to ProcessBlock with a few key differences.
// Most importantly ProcessTipSet relies on the precondition that each input block
// is valid with respect to the base state st, that is, ProcessBlock is free of
// errors when applied to each block individually over the given state.
// ProcessTipSet only returns errors in the case of faults.  Other errors
// coming from calls to ApplyMessage can be traced to different blocks in the
// TipSet containing conflicting messages and are ignored.  Blocks are applied
// in the sorted order of their tickets.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.StorageMap, ts types.TipSet, tsMessages [][]*types.SignedMessage, ancestors []types.TipSet) (response *ProcessTipSetResponse, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	h, err := ts.Height()
	if err != nil {
		return &ProcessTipSetResponse{}, errors.FaultErrorWrap(err, "processing empty tipset")
	}
	bh := types.NewBlockHeight(h)
	msgFilter := make(map[string]struct{})

	var res ProcessTipSetResponse
	res.Failures = make(map[cid.Cid]struct{})
	res.Successes = make(map[cid.Cid]struct{})

	// TODO: this can be made slightly more efficient by reusing the validation
	// transition of the first validated block (change would reach here and
	// consensus functions).
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		// find miner's owner address
		minerOwnerAddr, err := minerOwnerAddress(ctx, st, vms, blk.Miner)
		if err != nil {
			return &ProcessTipSetResponse{}, err
		}

		// filter out duplicates within TipSet
		var msgs []*types.SignedMessage
		for _, msg := range tsMessages[i] {
			mCid, err := msg.Cid()
			if err != nil {
				return &ProcessTipSetResponse{}, errors.FaultErrorWrap(err, "error getting message cid")
			}
			if _, ok := msgFilter[mCid.String()]; ok {
				continue
			}
			msgs = append(msgs, msg)
			// filter all messages that we attempted to apply
			// TODO is there ever a reason to try a duplicate failed message again within the same tipset?
			msgFilter[mCid.String()] = struct{}{}
		}
		amRes, err := p.ApplyMessagesAndPayRewards(ctx, st, vms, msgs, minerOwnerAddr, bh, ancestors)
		if err != nil {
			return &ProcessTipSetResponse{}, err
		}
		res.Results = append(res.Results, amRes.Results...)
		for _, msg := range amRes.SuccessfulMessages {
			mCid, err := msg.Cid()
			if err != nil {
				return &ProcessTipSetResponse{}, errors.FaultErrorWrap(err, "error getting message cid")
			}
			res.Successes[mCid] = struct{}{}
		}
		for _, msg := range amRes.PermanentFailures {
			mCid, err := msg.Cid()
			if err != nil {
				return &ProcessTipSetResponse{}, errors.FaultErrorWrap(err, "error getting message cid")
			}
			res.Failures[mCid] = struct{}{}
		}
		for _, msg := range amRes.TemporaryFailures {
			mCid, err := msg.Cid()
			if err != nil {
				return &ProcessTipSetResponse{}, errors.FaultErrorWrap(err, "error getting message cid")
			}
			res.Failures[mCid] = struct{}{}
		}
	}

	return &res, nil
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
func (p *DefaultProcessor) ApplyMessage(ctx context.Context, st state.Tree, vms vm.StorageMap, msg *types.SignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight, gasTracker *vm.GasTracker, ancestors []types.TipSet) (result *ApplicationResult, err error) {
	msgCid, err := msg.Cid()
	if err != nil {
		return nil, errors.FaultErrorWrap(err, "could not get message cid")
	}

	tagMethod := msg.Method
	if tagMethod == "" {
		tagMethod = "sendFIL"
	}
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

	cachedStateTree := state.NewCachedStateTree(st)

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
func CallQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method string, params []byte, from address.Address, optBh *types.BlockHeight) ([][]byte, uint8, error) {
	toActor, err := st.GetActor(ctx, to)
	if err != nil {
		return nil, 1, errors.ApplyErrorPermanentWrapf(err, "failed to get To actor")
	}

	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedStateTree(st)

	msg := &types.Message{
		From:   from,
		To:     to,
		Nonce:  0,
		Value:  types.ZeroAttoFIL,
		Method: method,
		Params: params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		Message:     msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
	}

	vmCtx := vm.NewVMContext(vmCtxParams)
	ret, retCode, err := vm.Send(ctx, vmCtx)
	return ret, retCode, err
}

// PreviewQueryMethod estimates the amount of gas that will be used by a method
// call. It accepts all the same arguments as CallQueryMethod.
func PreviewQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method string, params []byte, from address.Address, optBh *types.BlockHeight) (types.GasUnits, error) {
	toActor, err := st.GetActor(ctx, to)
	if err != nil {
		return types.NewGasUnits(0), errors.ApplyErrorPermanentWrapf(err, "failed to get To actor")
	}

	// not committing or flushing storage structures guarantees changes won't make it to stored state tree or datastore
	cachedSt := state.NewCachedStateTree(st)

	msg := &types.Message{
		From:   from,
		To:     to,
		Nonce:  0,
		Value:  types.ZeroAttoFIL,
		Method: method,
		Params: params,
	}

	// Set the gas limit to the max because this message send should always succeed; it doesn't cost gas.
	gasTracker := vm.NewGasTracker()
	gasTracker.MsgGasLimit = types.BlockGasLimit

	vmCtxParams := vm.NewContextParams{
		To:          toActor,
		Message:     msg,
		State:       cachedSt,
		StorageMap:  vms,
		GasTracker:  gasTracker,
		BlockHeight: optBh,
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
func (p *DefaultProcessor) attemptApplyMessage(ctx context.Context, st *state.CachedTree, store vm.StorageMap, msg *types.SignedMessage, bh *types.BlockHeight, gasTracker *vm.GasTracker, ancestors []types.TipSet) (*types.MessageReceipt, error) {
	gasTracker.ResetForNewMessage(msg.MeteredMessage)
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

	err = p.signedMessageValidator.Validate(ctx, msg, fromActor)
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

	toActor, err := st.GetOrCreateActor(ctx, msg.To, func() (*actor.Actor, error) {
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
		Message:     &msg.Message,
		State:       st,
		StorageMap:  store,
		GasTracker:  gasTracker,
		BlockHeight: bh,
		Ancestors:   ancestors,
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

// ApplyMessagesResponse is the output struct of ApplyMessages.  It exists to
// prevent callers from mistakenly mixing up outputs of the same type.
type ApplyMessagesResponse struct {
	Results            []*ApplicationResult
	PermanentFailures  []*types.SignedMessage
	TemporaryFailures  []*types.SignedMessage
	SuccessfulMessages []*types.SignedMessage

	// Application Errors
	PermanentErrors []error
	TemporaryErrors []error
}

// ApplyMessagesAndPayRewards begins by paying the block mining reward to the miner's owner. It then applies messages to a state tree.
// It returns an ApplyMessagesResponse which wraps the results of message application,
// groupings of messages with permanent failures, temporary failures, and
// successes, and the permanent and temporary errors raised during application.
// ApplyMessages will return an error iff a fault message occurs.
// Precondition: signatures of messages are checked by the caller.
func (p *DefaultProcessor) ApplyMessagesAndPayRewards(ctx context.Context, st state.Tree, vms vm.StorageMap, messages []*types.SignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight, ancestors []types.TipSet) (ApplyMessagesResponse, error) {
	var emptyRet ApplyMessagesResponse
	var ret ApplyMessagesResponse

	// transfer block reward to miner's owner from network address.
	if err := p.blockRewarder.BlockReward(ctx, st, minerOwnerAddr); err != nil {
		return ApplyMessagesResponse{}, err
	}

	gasTracker := vm.NewGasTracker()

	// process all messages
	for _, smsg := range messages {
		r, err := p.ApplyMessage(ctx, st, vms, smsg, minerOwnerAddr, bh, gasTracker, ancestors)
		// If the message should not have been in the block, bail somehow.
		switch {
		case errors.IsFault(err):
			return emptyRet, err
		case errors.IsApplyErrorPermanent(err):
			ret.PermanentFailures = append(ret.PermanentFailures, smsg)
			ret.PermanentErrors = append(ret.PermanentErrors, err)
		case errors.IsApplyErrorTemporary(err):
			ret.TemporaryFailures = append(ret.TemporaryFailures, smsg)
			ret.TemporaryErrors = append(ret.TemporaryErrors, err)
		case err != nil:
			panic("someone is a bad programmer: error is neither fault, perm or temp")
		default:
			ret.SuccessfulMessages = append(ret.SuccessfulMessages, smsg)
			ret.Results = append(ret.Results, r)
		}
	}
	return ret, nil
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
	cachedTree := state.NewCachedStateTree(st)
	if err := rewardTransfer(ctx, address.NetworkAddress, minerOwnerAddr, br.BlockRewardAmount(), cachedTree); err != nil {
		return errors.FaultErrorWrap(err, "Error attempting to pay block reward")
	}
	return cachedTree.Commit(ctx)
}

// GasReward transfers the gas cost reward from the sender actor to the minerOwnerAddr
func (br *DefaultBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.SignedMessage, gas types.AttoFIL) error {
	cachedTree := state.NewCachedStateTree(st)
	if err := rewardTransfer(ctx, msg.From, minerOwnerAddr, gas, cachedTree); err != nil {
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
func minerOwnerAddress(ctx context.Context, st state.Tree, vms vm.StorageMap, minerAddr address.Address) (address.Address, error) {
	ret, code, err := CallQueryMethod(ctx, st, vms, minerAddr, "getOwner", []byte{}, address.Undef, types.NewBlockHeight(0))
	if err != nil {
		return address.Undef, errors.FaultErrorWrap(err, "could not get miner owner")
	}
	if code != 0 {
		return address.Undef, errors.NewFaultErrorf("could not get miner owner. error code %d", code)
	}
	return address.NewFromBytes(ret[0])
}
