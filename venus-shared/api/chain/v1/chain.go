package v1

import (
	"context"
	"encoding/json"
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
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type IChain interface {
	IAccount
	IActor
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

type IChainInfo interface {
	BlockTime(ctx context.Context) time.Duration                                                                                                                                          //perm:read
	ChainList(ctx context.Context, tsKey types.TipSetKey, count int) ([]types.TipSetKey, error)                                                                                           //perm:read
	ChainHead(ctx context.Context) (*types.TipSet, error)                                                                                                                                 //perm:read
	ChainSetHead(ctx context.Context, key types.TipSetKey) error                                                                                                                          //perm:admin
	ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)                                                                                                       //perm:read
	ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error)                                                                        //perm:read
	ChainGetTipSetAfterHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error)                                                                     //perm:read
	StateGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) //perm:read
	StateGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error)  //perm:read
	// StateGetRandomnessDigestFromTickets is used to sample the chain for randomness.
	StateGetRandomnessDigestFromTickets(ctx context.Context, randEpoch abi.ChainEpoch, tsk types.TipSetKey) (abi.Randomness, error) //perm:read
	// StateGetRandomnessDigestFromBeacon is used to sample the beacon for randomness.
	StateGetRandomnessDigestFromBeacon(ctx context.Context, randEpoch abi.ChainEpoch, tsk types.TipSetKey) (abi.Randomness, error) //perm:read
	// StateGetBeaconEntry returns the beacon entry for the given filecoin epoch
	// by using the recorded entries on the chain. If the entry for the requested
	// epoch has not yet been produced, the call will block until the entry
	// becomes available.
	StateGetBeaconEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error)                     //perm:read
	ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error)                                     //perm:read
	ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.Message, error)                                    //perm:read
	ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*types.BlockMessages, error)                          //perm:read
	ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]types.MessageCID, error)                 //perm:read
	ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error)                              //perm:read
	ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]types.MessageCID, error)                          //perm:read
	ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error)                     //perm:read
	StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error)                //perm:read
	StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) //perm:read
	ChainNotify(ctx context.Context) (<-chan []*types.HeadChange, error)                                           //perm:read
	GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error)                                        //perm:read
	GetActor(ctx context.Context, addr address.Address) (*types.Actor, error)                                      //perm:read
	GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error)     //perm:read
	GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error)                 //perm:read
	ProtocolParameters(ctx context.Context) (*types.ProtocolParams, error)                                         //perm:read
	ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)         //perm:read
	StateNetworkName(ctx context.Context) (types.NetworkName, error)                                               //perm:read
	// StateSearchMsg looks back up to limit epochs in the chain for a message, and returns its receipt and the tipset where it was executed
	//
	// NOTE: If a replacing message is found on chain, this method will return
	// a MsgLookup for the replacing message - the MsgLookup.Message will be a different
	// CID than the one provided in the 'cid' param, MsgLookup.Receipt will contain the
	// result of the execution of the replacing message.
	//
	// If the caller wants to ensure that exactly the requested message was executed,
	// they must check that MsgLookup.Message is equal to the provided 'cid', or set the
	// `allowReplaced` parameter to false. Without this check, and with `allowReplaced`
	// set to true, both the requested and original message may appear as
	// successfully executed on-chain, which may look like a double-spend.
	//
	// A replacing message is a message with a different CID, any of Gas values, and
	// different signature, but with all other parameters matching (source/destination,
	// nonce, params, etc.)
	StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) //perm:read
	// StateWaitMsg looks back up to limit epochs in the chain for a message.
	// If not found, it blocks until the message arrives on chain, and gets to the
	// indicated confidence depth.
	//
	// NOTE: If a replacing message is found on chain, this method will return
	// a MsgLookup for the replacing message - the MsgLookup.Message will be a different
	// CID than the one provided in the 'cid' param, MsgLookup.Receipt will contain the
	// result of the execution of the replacing message.
	//
	// If the caller wants to ensure that exactly the requested message was executed,
	// they must check that MsgLookup.Message is equal to the provided 'cid', or set the
	// `allowReplaced` parameter to false. Without this check, and with `allowReplaced`
	// set to true, both the requested and original message may appear as
	// successfully executed on-chain, which may look like a double-spend.
	//
	// A replacing message is a message with a different CID, any of Gas values, and
	// different signature, but with all other parameters matching (source/destination,
	// nonce, params, etc.)
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) //perm:read
	StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error)                                                //perm:read
	VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool                                                             //perm:read
	ChainExport(context.Context, abi.ChainEpoch, bool, types.TipSetKey) (<-chan []byte, error)                                            //perm:read
	ChainGetPath(ctx context.Context, from types.TipSetKey, to types.TipSetKey) ([]*types.HeadChange, error)                              //perm:read
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
	// ChainGetEvents returns the events under an event AMT root CID.
	ChainGetEvents(context.Context, cid.Cid) ([]types.Event, error) //perm:read
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
	// StateMarketProposalPending returns whether a given proposal CID is marked as pending in the market actor
	StateMarketProposalPending(ctx context.Context, proposalCid cid.Cid, tsk types.TipSetKey) (bool, error) //perm:read
}

type IMinerState interface {
	StateReadState(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.ActorState, error)                                    //perm:read
	StateListMessages(ctx context.Context, match *types.MessageMatch, tsk types.TipSetKey, toht abi.ChainEpoch) ([]cid.Cid, error)                //perm:read
	StateDecodeParams(ctx context.Context, toAddr address.Address, method abi.MethodNum, params []byte, tsk types.TipSetKey) (interface{}, error) //perm:read
	StateEncodeParams(ctx context.Context, toActCode cid.Cid, method abi.MethodNum, params json.RawMessage) ([]byte, error)                       //perm:read
	StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error)                  //perm:read
	// StateSectorPreCommitInfo returns the PreCommit info for the specified miner's sector.
	// Returns nil and no error if the sector isn't precommitted.
	//
	// Note that the sector number may be allocated while PreCommitInfo is nil. This means that either allocated sector
	// numbers were compacted, and the sector number was marked as allocated in order to reduce size of the allocated
	// sectors bitfield, or that the sector was precommitted, but the precommit has expired.
	StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*types.SectorPreCommitOnChainInfo, error) //perm:read
	StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorOnChainInfo, error)               //perm:read
	StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorLocation, error)     //perm:read
	StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error)                                            //perm:read
	StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (types.MinerInfo, error)                                                 //perm:read
	StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error)                                        //perm:read
	StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                             //perm:read
	StateAllMinerFaults(ctx context.Context, lookback abi.ChainEpoch, ts types.TipSetKey) ([]*types.Fault, error)                                            //perm:read
	StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                         //perm:read
	StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error)                                          //perm:read
	StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]types.Partition, error)                           //perm:read
	StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]types.Deadline, error)                                           //perm:read
	StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*lminer.SectorOnChainInfo, error)    //perm:read
	StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*types.MarketDeal, error)                                           //perm:read
	// StateGetAllocationForPendingDeal returns the allocation for a given deal ID of a pending deal. Returns nil if
	// pending allocation is not found.
	StateGetAllocationForPendingDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*verifreg.Allocation, error) //perm:read
	// StateGetAllocationIdForPendingDeal is like StateGetAllocationForPendingDeal except it returns the allocation ID
	StateGetAllocationIdForPendingDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (verifreg.AllocationId, error) //perm:read
	// StateGetAllocation returns the allocation for a given address and allocation ID.
	StateGetAllocation(ctx context.Context, clientAddr address.Address, allocationID verifreg.AllocationId, tsk types.TipSetKey) (*verifreg.Allocation, error) //perm:read
	// StateGetAllAllocations returns the all the allocations available in verified registry actor.
	StateGetAllAllocations(ctx context.Context, tsk types.TipSetKey) (map[verifreg.AllocationId]verifreg.Allocation, error) //perm:read
	// StateGetAllocations returns the all the allocations for a given client.
	StateGetAllocations(ctx context.Context, clientAddr address.Address, tsk types.TipSetKey) (map[verifreg.AllocationId]verifreg.Allocation, error) //perm:read
	// StateGetClaim returns the claim for a given address and claim ID.
	StateGetClaim(ctx context.Context, providerAddr address.Address, claimID verifreg.ClaimId, tsk types.TipSetKey) (*verifreg.Claim, error) //perm:read
	// StateGetClaims returns the all the claims for a given provider.
	StateGetClaims(ctx context.Context, providerAddr address.Address, tsk types.TipSetKey) (map[verifreg.ClaimId]verifreg.Claim, error) //perm:read
	// StateGetAllClaims returns the all the claims available in verified registry actor.
	StateGetAllClaims(ctx context.Context, tsk types.TipSetKey) (map[verifreg.ClaimId]verifreg.Claim, error) //perm:read
	// StateComputeDataCID computes DataCID from a set of on-chain deals
	StateComputeDataCID(ctx context.Context, maddr address.Address, sectorType abi.RegisteredSealProof, deals []abi.DealID, tsk types.TipSetKey) (cid.Cid, error) //perm:read
	StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci types.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)           //perm:read
	// StateMinerInitialPledgeCollateral attempts to calculate the initial pledge collateral based on a SectorPreCommitInfo.
	// This method uses the DealIDs field in SectorPreCommitInfo to determine the amount of verified
	// deal space in the sector in order to perform a QAP calculation. Since network version 22 and
	// the introduction of DDO, the DealIDs field can no longer be used to reliably determine verified
	// deal space; therefore, this method is deprecated. Use StateMinerInitialPledgeForSector instead
	// and pass in the verified deal space directly.
	//
	// Deprecated: Use StateMinerInitialPledgeForSector instead.
	StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci types.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error) //perm:read
	// StateMinerInitialPledgeForSector returns the initial pledge collateral for a given sector
	// duration, size, and combined size of any verified pieces within the sector. This calculation
	// depends on current network conditions (total power, total pledge and current rewards) at the
	// given tipset.
	StateMinerInitialPledgeForSector(ctx context.Context, sectorDuration abi.ChainEpoch, sectorSize abi.SectorSize, verifiedSize uint64, tsk types.TipSetKey) (types.BigInt, error) //perm:read
	// StateMinerCreationDeposit calculates the deposit required for creating a new miner
	// according to FIP-0077 specification. This deposit is based on the network's current
	// economic parameters including circulating supply, network power, and pledge collateral.
	//
	// See: node/impl/full/state.go StateMinerCreationDeposit implementation.
	StateMinerCreationDeposit(ctx context.Context, tsk types.TipSetKey) (types.BigInt, error)                                     //perm:read
	StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (types.CirculatingSupply, error)                   //perm:read
	StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error)                                     //perm:read
	StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]*types.MarketDeal, error)                              //perm:read
	StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*lminer.SectorOnChainInfo, error) //perm:read
	StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)                        //perm:read
	// StateLookupRobustAddress returns the public key address of the given ID address for non-account addresses (multisig, miners etc)
	StateLookupRobustAddress(context.Context, address.Address, types.TipSetKey) (address.Address, error)                                                     //perm:read
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
	// StateMinerAllocated returns a bitfield containing all sector numbers marked as allocated in miner state
	StateMinerAllocated(context.Context, address.Address, types.TipSetKey) (*bitfield.BitField, error) //perm:read
}
