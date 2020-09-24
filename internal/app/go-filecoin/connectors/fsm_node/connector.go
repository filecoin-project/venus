package fsmnodeconnector

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	fsm "github.com/filecoin-project/go-filecoin/vendors/storage-sealing"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

type FiniteStateMachineNodeConnector struct {
	minerAddr   address.Address
	waiter      *msg.Waiter
	chain       *chain.Store
	chainState  *cst.ChainStateReadWriter
	stateViewer *appstate.TipSetStateViewer
	outbox      *message.Outbox
}

var _ fsm.SealingAPI = new(FiniteStateMachineNodeConnector)

func New(minerAddr address.Address, waiter *msg.Waiter, chain *chain.Store, viewer *appstate.TipSetStateViewer, outbox *message.Outbox, chainState *cst.ChainStateReadWriter) *FiniteStateMachineNodeConnector {
	return &FiniteStateMachineNodeConnector{
		minerAddr:   minerAddr,
		chain:       chain,
		chainState:  chainState,
		outbox:      outbox,
		stateViewer: viewer,
		waiter:      waiter,
	}
}

func (f *FiniteStateMachineNodeConnector) StateWaitMsg(ctx context.Context, mcid cid.Cid) (fsm.MsgLookup, error) {
	var lookup fsm.MsgLookup
	err := f.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(blk *block.Block, message *types.SignedMessage, r *vm.MessageReceipt) error {
		lookup.Height = blk.Height
		receipt := fsm.MessageReceipt{
			ExitCode: r.ExitCode,
			Return:   r.ReturnValue,
			GasUsed:  int64(r.GasUsed),
		}
		lookup.Receipt = receipt

		// find tip set key at block height
		tsHead, err := f.chain.GetTipSet(f.chain.GetHead())
		if err != nil {
			return err
		}
		tsAtHeight, err := chain.FindTipsetAtEpoch(ctx, tsHead, blk.Height, f.chain)
		if err != nil {
			return err
		}

		tsk := tsAtHeight.Key()
		token, err := encoding.Encode(tsk)
		if err != nil {
			return err
		}

		lookup.TipSetTok = token
		return nil
	})
	if err != nil {
		return fsm.MsgLookup{}, err
	}

	return lookup, err
}

func (f *FiniteStateMachineNodeConnector) StateSearchMsg(ctx context.Context, mcid cid.Cid) (*fsm.MsgLookup, error) {
	var lookup fsm.MsgLookup
	cm, res, err := f.waiter.Find(ctx, msg.DefaultMessageWaitLookback, func(msg *types.SignedMessage, msgCid cid.Cid) bool {
		if mcid.Equals(msgCid){
			return true
		}
		return false
	})

	if err != nil {
		return &fsm.MsgLookup{}, err
	}

	if !res {
		return &fsm.MsgLookup{}, fmt.Errorf("Could not find message [%s]", mcid.String())
	}

	lookup.Height = cm.Block.Height
	lookup.Receipt = fsm.MessageReceipt{
		ExitCode: cm.Receipt.ExitCode,
		Return:   cm.Receipt.ReturnValue,
		GasUsed:  int64(cm.Receipt.GasUsed),
	}

	// find tip set key at block height
	tsHead, err := f.chain.GetTipSet(f.chain.GetHead())
	if err != nil {
		return &fsm.MsgLookup{}, err
	}
	tsAtHeight, err := chain.FindTipsetAtEpoch(ctx, tsHead, cm.Block.Height, f.chain)
	if err != nil {
		return &fsm.MsgLookup{}, err
	}

	tsk := tsAtHeight.Key()
	token, err := encoding.Encode(tsk)
	if err != nil {
		return &fsm.MsgLookup{}, err
	}

	lookup.TipSetTok = token

	return &lookup, err
}

func (f *FiniteStateMachineNodeConnector) StateComputeDataCommitment(ctx context.Context, _ address.Address, sectorType abi.RegisteredSealProof, deals []abi.DealID, tok fsm.TipSetToken) (cid.Cid, error) {
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return cid.Undef, err
	}

	return view.MarketComputeDataCommitment(ctx, sectorType, deals)
}

func (f *FiniteStateMachineNodeConnector) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tok fsm.TipSetToken) (*miner.SectorPreCommitOnChainInfo, error) {
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return nil, err
	}

	info, found, err := view.MinerGetPrecommittedSector(ctx, maddr, sectorNumber)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("Could not find pre-committed sector for miner %s", maddr.String())
	}

	return info, nil
}

// ToDo There may be a problem
func (f *FiniteStateMachineNodeConnector) StateSectorGetInfo(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tok fsm.TipSetToken) (*miner.SectorOnChainInfo, error){
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return nil, err
	}

	info, found, err := view.MinerGetSector(ctx, maddr, sectorNumber)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("Could not find sector [%v] for miner %s", sectorNumber, maddr.String())
	}

	return info, nil
}

// ToDo Not fully realized
func (f *FiniteStateMachineNodeConnector) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tok fsm.TipSetToken) (*fsm.SectorLocation, error) {
	_, err := f.stateViewForToken(tok)
	if err != nil {
		return nil, err
	}

	panic("implement me")
}

func (f *FiniteStateMachineNodeConnector) StateMinerSectorSize(ctx context.Context, maddr address.Address, tok fsm.TipSetToken) (abi.SectorSize, error) {
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return 0, err
	}

	conf, err := view.MinerSectorConfiguration(ctx, maddr)
	if err != nil {
		return 0, err
	}
	return conf.SectorSize, err
}

func (f *FiniteStateMachineNodeConnector) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tok fsm.TipSetToken) (address.Address, error) {
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return address.Undef, err
	}

	_, worker, err := view.MinerControlAddresses(ctx, maddr)
	return worker, err
}

func (f *FiniteStateMachineNodeConnector) StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tok fsm.TipSetToken) (market.DealProposal, error) {
	view, err := f.stateViewForToken(tok)
	if err != nil {
		return market.DealProposal{}, err
	}

	deal, err := view.MarketDealProposal(ctx, dealID)
	if err != nil {
		return market.DealProposal{}, err
	}

	return deal, err
}

// ToDo Not fully realized
func (f *FiniteStateMachineNodeConnector) StateMinerDeadlines(ctx context.Context, maddr address.Address, tok fsm.TipSetToken) ([]*miner.Deadline, error) {
	var tsk block.TipSetKey
	err := encoding.Decode(tok, &tsk)
	if err != nil {
		return nil, err
	}

	_, err = f.stateViewer.StateView(tsk)
	if err != nil {
		return nil, err
	}

	panic("implement me")
}

// ToDo Not fully realized
func (f *FiniteStateMachineNodeConnector) StateMinerPreCommitDepositForPower(context.Context, address.Address, miner.SectorPreCommitInfo, fsm.TipSetToken) (big.Int, error) {
	panic("implement me")
}

func (f *FiniteStateMachineNodeConnector) StateMinerInitialPledgeCollateral(context.Context, address.Address, miner.SectorPreCommitInfo, fsm.TipSetToken) (big.Int, error) {
	// The FSM uses this result to attach value equal to the collateral to the ProveCommit message sent from the
	// worker account. This isn't absolutely necessary if the miner actor already has sufficient unlocked balance.
	// The initial pledge requirement calculations are currently very difficult to access, so I'm returning
	// zero here pending a proper implementation after cleaning up the actors.
	// TODO https://github.com/filecoin-project/go-filecoin/issues/4035
	return big.Zero(), nil
}

func (f *FiniteStateMachineNodeConnector) SendMsg(ctx context.Context, from, to address.Address, method abi.MethodNum, value, maxFee abi.TokenAmount, params []byte) (cid.Cid, error) {
	mcid, cerr, err := f.outbox.SendEncoded(
		ctx,
		from,
		to,
		value,
		maxFee,
		types.NewGasPremium(1),
		gas.Unit(0),
		true,
		method,
		params,
	)
	if err != nil {
		return cid.Undef, err
	}
	err = <-cerr
	if err != nil {
		return cid.Undef, err
	}
	return mcid, nil
}

func (f *FiniteStateMachineNodeConnector) ChainHead(_ context.Context) (fsm.TipSetToken, abi.ChainEpoch, error) {
	ts, err := f.chain.GetTipSet(f.chain.GetHead())
	if err != nil {
		return fsm.TipSetToken{}, 0, err
	}

	epoch, err := ts.Height()
	if err != nil {
		return fsm.TipSetToken{}, 0, err
	}

	tok, err := encoding.Encode(ts.Key())
	if err != nil {
		return fsm.TipSetToken{}, 0, err
	}

	return tok, epoch, nil
}

// ToDo Not fully realized
func (f *FiniteStateMachineNodeConnector) ChainGetRandomnessFromBeacon(ctx context.Context, tok fsm.TipSetToken, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	panic("implement me")
}

// ToDo Not fully realized
func (f *FiniteStateMachineNodeConnector) ChainGetRandomnessFromTickets(ctx context.Context, tok fsm.TipSetToken, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	panic("implement me")
}

func (f *FiniteStateMachineNodeConnector) ChainGetRandomness(ctx context.Context, tok fsm.TipSetToken, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return abi.Randomness{}, err
	}
	return f.chainState.SampleChainRandomness(ctx, tsk, personalization, randEpoch, entropy)
}

func (f *FiniteStateMachineNodeConnector) ChainGetTicket(ctx context.Context, tok fsm.TipSetToken) (abi.SealRandomness, abi.ChainEpoch, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return abi.SealRandomness{}, 0, err
	}

	ts, err := f.chain.GetTipSet(tsk)
	if err != nil {
		return abi.SealRandomness{}, 0, err
	}

	epoch, err := ts.Height()
	if err != nil {
		return abi.SealRandomness{}, 0, err
	}

	randomEpoch := epoch - miner.ChainFinality

	buf := new(bytes.Buffer)
	err = f.minerAddr.MarshalCBOR(buf)
	if err != nil {
		return abi.SealRandomness{}, 0, err
	}

	randomness, err := f.ChainGetRandomness(ctx, tok, crypto.DomainSeparationTag_SealRandomness, randomEpoch, buf.Bytes())
	return abi.SealRandomness(randomness), randomEpoch, err
}

func (f *FiniteStateMachineNodeConnector) ChainReadObj(ctx context.Context, obj cid.Cid) ([]byte, error) {
	return f.chainState.ReadObj(ctx, obj)
}

func (f *FiniteStateMachineNodeConnector) stateViewForToken(tok fsm.TipSetToken) (*appstate.View, error) {
	var tsk block.TipSetKey
	err := encoding.Decode(tok, &tsk)
	if err != nil {
		return nil, err
	}

	return f.stateViewer.StateView(tsk)
}
