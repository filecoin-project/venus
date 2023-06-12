package v0

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	lminer "github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type IChain interface {
	IAccount
	IActor
	IBeacon
	IMinerState
	IChainInfo
}

type IAccount interface {
	StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) //perm:read
}

type IActor interface {
	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) //perm:read
	ListActor(ctx context.Context) (map[address.Address]*types.Actor, error)                             //perm:read
}

type IBeacon interface {
	BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error) //perm:read
}

type IChainInfo interface {
	BlockTime(ctx context.Context) time.Duration                                                                                                                                          //perm:read
	ChainList(ctx context.Context, tsKey types.TipSetKey, count int) ([]types.TipSetKey, error)                                                                                           //perm:read
	ChainHead(ctx context.Context) (*types.TipSet, error)                                                                                                                                 //perm:read
	ChainSetHead(ctx context.Context, key types.TipSetKey) error                                                                                                                          //perm:admin
	ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)                                                                                                       //perm:read
	ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error)                                                                        //perm:read
	ChainGetRandomnessFromBeacon(ctx context.Context, key types.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)  //perm:read
	ChainGetRandomnessFromTickets(ctx context.Context, tsk types.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) //perm:read
	ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error)                                                                                                            //perm:read
	ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.Message, error)                                                                                                           //perm:read
	ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*types.BlockMessages, error)                                                                                                 //perm:read
	ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]types.MessageCID, error)                                                                                        //perm:read
	ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error)                                                                                                     //perm:read
	ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]types.MessageCID, error)                                                                                                 //perm:read
	ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error)                                                                                            //perm:read
	StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error)                                                                                       //perm:read
	StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error)                                                                        //perm:read
	ChainNotify(ctx context.Context) (<-chan []*types.HeadChange, error)                                                                                                                  //perm:read
	GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error)                                                                                                               //perm:read
	GetActor(ctx context.Context, addr address.Address) (*types.Actor, error)                                                                                                             //perm:read
	GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error)                                                                            //perm:read
	GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error)                                                                                        //perm:read
	ProtocolParameters(ctx context.Context) (*types.ProtocolParams, error)                                                                                                                //perm:read
	ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)                                                                                //perm:read
	StateNetworkName(ctx context.Context) (types.NetworkName, error)                                                                                                                      //perm:read
	StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error)                                                                                //perm:read
	StateSearchMsg(ctx context.Context, msg cid.Cid) (*types.MsgLookup, error)                                                                                                            //perm:read
	StateSearchMsgLimited(ctx context.Context, cid cid.Cid, limit abi.ChainEpoch) (*types.MsgLookup, error)                                                                               //perm:read
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64) (*types.MsgLookup, error)                                                                                           //perm:read
	StateWaitMsgLimited(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch) (*types.MsgLookup, error)                                                              //perm:read
	StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error)                                                                                                //perm:read
	VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool                                                                                                             //perm:read
	ChainExport(context.Context, abi.ChainEpoch, bool, types.TipSetKey) (<-chan []byte, error)                                                                                            //perm:read
	ChainGetPath(ctx context.Context, from types.TipSetKey, to types.TipSetKey) ([]*types.HeadChange, error)                                                                              //perm:read
	// StateGetNetworkParams return current network params
	StateGetNetworkParams(ctx context.Context) (*types.NetworkParams, error) //perm:read
	// StateActorCodeCIDs returns the CIDs of all the builtin actors for the given network version
	StateActorCodeCIDs(context.Context, network.Version) (map[string]cid.Cid, error) //perm:read
	// ChainGetGenesis returns the genesis tipset.
	ChainGetGenesis(context.Context) (*types.TipSet, error) //perm:read
	// StateActorManifestCID returns the CID of the builtin actors manifest for the given network version
	StateActorManifestCID(context.Context, network.Version) (cid.Cid, error)                            //perm:read
	StateCall(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error) //perm:read
	StateReplay(context.Context, types.TipSetKey, cid.Cid) (*types.InvocResult, error)                  //perm:read
	// StateCompute is a flexible command that applies the given messages on the given tipset.
	// The messages are run as though the VM were at the provided height.
	//
	// When called, StateCompute will:
	// - Load the provided tipset, or use the current chain head if not provided
	// - Compute the tipset state of the provided tipset on top of the parent state
	//   - (note that this step runs before vmheight is applied to the execution)
	//   - Execute state upgrade if any were scheduled at the epoch, or in null
	//     blocks preceding the tipset
	//   - Call the cron actor on null blocks preceding the tipset
	//   - For each block in the tipset
	//     - Apply messages in blocks in the specified
	//     - Award block reward by calling the reward actor
	//   - Call the cron actor for the current epoch
	// - If the specified vmheight is higher than the current epoch, apply any
	//   needed state upgrades to the state
	// - Apply the specified messages to the state
	//
	// The vmheight parameter sets VM execution epoch, and can be used to simulate
	// message execution in different network versions. If the specified vmheight
	// epoch is higher than the epoch of the specified tipset, any state upgrades
	// until the vmheight will be executed on the state before applying messages
	// specified by the user.
	//
	// Note that the initial tipset state computation is not affected by the
	// vmheight parameter - only the messages in the `apply` set are
	//
	// If the caller wants to simply compute the state, vmheight should be set to
	// the epoch of the specified tipset.
	//
	// Messages in the `apply` parameter must have the correct nonces, and gas
	// values set.
	StateCompute(context.Context, abi.ChainEpoch, []*types.Message, types.TipSetKey) (*types.ComputeStateOutput, error) //perm:read
	// StateGetRandomnessFromTickets is used to sample the chain for randomness.
	StateGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) //perm:read
	// StateGetRandomnessFromBeacon is used to sample the beacon for randomness.
	StateGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) //perm:read
}

type IMinerState interface {
	// StateReadState returns the indicated actor's state.
	StateReadState(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.ActorState, error) //perm:read
	// StateDecodeParams attempts to decode the provided params, based on the recipient actor address and method number.
	StateDecodeParams(ctx context.Context, toAddr address.Address, method abi.MethodNum, params []byte, tsk types.TipSetKey) (interface{}, error) //perm:read
	// StateListMessages looks back and returns all messages with a matching to or from address, stopping at the given height.
	StateListMessages(ctx context.Context, match *types.MessageMatch, tsk types.TipSetKey, toht abi.ChainEpoch) ([]cid.Cid, error)                          //perm:read
	StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error)                            //perm:read
	StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (types.SectorPreCommitOnChainInfo, error) //perm:read
	StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*types.SectorOnChainInfo, error)               //perm:read
	StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorLocation, error)    //perm:read
	StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error)                                           //perm:read
	StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (types.MinerInfo, error)                                                //perm:read
	StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error)                                       //perm:read
	StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                        //perm:read
	StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                            //perm:read
	// StateAllMinerFaults returns all non-expired Faults that occur within lookback epochs of the given tipset
	StateAllMinerFaults(ctx context.Context, lookback abi.ChainEpoch, ts types.TipSetKey) ([]*types.Fault, error)                                        //perm:read
	StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error)                                      //perm:read
	StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]types.Partition, error)                       //perm:read
	StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]types.Deadline, error)                                       //perm:read
	StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*types.SectorOnChainInfo, error) //perm:read
	StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*types.MarketDeal, error)                                       //perm:read
	// StateGetAllocationForPendingDeal returns the allocation for a given deal ID of a pending deal. Returns nil if
	// pending allocation is not found.
	StateGetAllocationForPendingDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*types.Allocation, error) //perm:read
	// StateGetAllocation returns the allocation for a given address and allocation ID.
	StateGetAllocation(ctx context.Context, clientAddr address.Address, allocationID types.AllocationId, tsk types.TipSetKey) (*types.Allocation, error) //perm:read
	// StateGetAllocations returns the all the allocations for a given client.
	StateGetAllocations(ctx context.Context, clientAddr address.Address, tsk types.TipSetKey) (map[types.AllocationId]types.Allocation, error) //perm:read
	// StateGetClaim returns the claim for a given address and claim ID.
	StateGetClaim(ctx context.Context, providerAddr address.Address, claimID types.ClaimId, tsk types.TipSetKey) (*types.Claim, error) //perm:read
	// StateGetClaims returns the all the claims for a given provider.
	StateGetClaims(ctx context.Context, providerAddr address.Address, tsk types.TipSetKey) (map[types.ClaimId]types.Claim, error)                            //perm:read
	StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci types.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)      //perm:read
	StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci types.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)       //perm:read
	StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (types.CirculatingSupply, error)                                              //perm:read
	StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error)                                                                //perm:read
	StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]*types.MarketDeal, error)                                                         //perm:read
	StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*types.SectorOnChainInfo, error)                             //perm:read
	StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)                                                   //perm:read
	StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)                                                                     //perm:read
	StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)                                                                     //perm:read
	StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*types.MinerPower, error)                                               //perm:read
	StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error)                                             //perm:read
	StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorExpiration, error)  //perm:read
	StateChangedActors(context.Context, cid.Cid, cid.Cid) (map[string]types.Actor, error)                                                                    //perm:read
	StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (types.MinerSectors, error)                                        //perm:read
	StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (types.MarketBalance, error)                                          //perm:read
	StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk types.TipSetKey) (types.DealCollateralBounds, error) //perm:read
	StateVerifiedClientStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error)                                     //perm:read
}
