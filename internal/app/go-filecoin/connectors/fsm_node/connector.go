package fsmnodeconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	fsm "github.com/filecoin-project/storage-fsm"
	"github.com/ipfs/go-cid"
)

type FiniteStateMachineNodeConnector struct{}

var _ fsm.SealingAPI = new(FiniteStateMachineNodeConnector)

func New() FiniteStateMachineNodeConnector {
	return FiniteStateMachineNodeConnector{}
}

func (f FiniteStateMachineNodeConnector) StateWaitMsg(context.Context, cid.Cid) (fsm.MsgLookup, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) StateComputeDataCommitment(ctx context.Context, maddr address.Address, sectorType abi.RegisteredProof, deals []abi.DealID, tok fsm.TipSetToken) (cid.Cid, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tok fsm.TipSetToken) (*miner.SectorPreCommitOnChainInfo, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) StateMinerSectorSize(context.Context, address.Address, fsm.TipSetToken) (abi.SectorSize, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tok fsm.TipSetToken) (address.Address, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) StateMarketStorageDeal(context.Context, abi.DealID, fsm.TipSetToken) (market.DealProposal, market.DealState, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) SendMsg(ctx context.Context, from, to address.Address, method abi.MethodNum, value, gasPrice big.Int, gasLimit int64, params []byte) (cid.Cid, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) ChainHead(ctx context.Context) (fsm.TipSetToken, abi.ChainEpoch, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) ChainGetRandomness(ctx context.Context, tok fsm.TipSetToken, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	panic("implement me")
}

func (f FiniteStateMachineNodeConnector) ChainGetTicket(ctx context.Context, tok fsm.TipSetToken) (abi.SealRandomness, abi.ChainEpoch, error) {
	panic("implement me")
}
