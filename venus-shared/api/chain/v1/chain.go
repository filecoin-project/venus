package v1

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

	"github.com/filecoin-project/go-state-types/builtin/v8/miner"
	lminer "github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
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
	// StateGetBeaconEntry returns the beacon entry for the given filecoin epoch. If
	// the entry has not yet been produced, the call will block until the entry
	// becomes available
	StateGetBeaconEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error)                         //perm:read
	ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error)                                         //perm:read
	ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.Message, error)                                        //perm:read
	ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*types.BlockMessages, error)                              //perm:read
	ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]types.MessageCID, error)                     //perm:read
	ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error)                                  //perm:read
	ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]types.MessageCID, error)                              //perm:read
	ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error)                         //perm:read
	StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error)                    //perm:read
	StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error)     //perm:read
	ChainNotify(ctx context.Context) (<-chan []*types.HeadChange, error)                                               //perm:read
	GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error)                                            //perm:read
	GetActor(ctx context.Context, addr address.Address) (*types.Actor, error)                                          //perm:read
	GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error)         //perm:read
	GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error)                     //perm:read
	MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*types.ChainMessage, error) //perm:read
	ProtocolParameters(ctx context.Context) (*types.ProtocolParams, error)                                             //perm:read
	ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)             //perm:read
	StateNetworkName(ctx context.Context) (types.NetworkName, error)                                                   //perm:read
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
}

type IMinerState interface {
	StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk types.TipSetKey) (bool, error)                            //perm:read
	StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (miner.SectorPreCommitOnChainInfo, error) //perm:read
	StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error)               //perm:read
	StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorLocation, error)    //perm:read
	StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (abi.SectorSize, error)                                           //perm:read
	StateMinerInfo(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (types.MinerInfo, error)                                                //perm:read
	StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (address.Address, error)                                       //perm:read
	StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                        //perm:read
	StateMinerFaults(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (bitfield.BitField, error)                                            //perm:read
	StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (*dline.Info, error)                                         //perm:read
	StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk types.TipSetKey) ([]types.Partition, error)                          //perm:read
	StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]types.Deadline, error)                                          //perm:read
	StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error)    //perm:read
	StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk types.TipSetKey) (*types.MarketDeal, error)                                          //perm:read
	StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)     //perm:read
	StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk types.TipSetKey) (big.Int, error)      //perm:read
	StateVMCirculatingSupplyInternal(ctx context.Context, tsk types.TipSetKey) (types.CirculatingSupply, error)                                             //perm:read
	StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error)                                                               //perm:read
	StateMarketDeals(ctx context.Context, tsk types.TipSetKey) (map[string]*types.MarketDeal, error)                                                        //perm:read
	StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk types.TipSetKey) ([]*miner.SectorOnChainInfo, error)                            //perm:read
	StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)                                                  //perm:read
	// StateLookupRobustAddress returns the public key address of the given ID address for non-account addresses (multisig, miners etc)
	StateLookupRobustAddress(context.Context, address.Address, types.TipSetKey) (address.Address, error)                                                     //perm:read
	StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)                                                                     //perm:read
	StateListActors(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error)                                                                     //perm:read
	StateMinerPower(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*types.MinerPower, error)                                               //perm:read
	StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk types.TipSetKey) (big.Int, error)                                             //perm:read
	StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk types.TipSetKey) (*lminer.SectorExpiration, error)  //perm:read
	StateMinerSectorCount(ctx context.Context, addr address.Address, tsk types.TipSetKey) (types.MinerSectors, error)                                        //perm:read
	StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (types.MarketBalance, error)                                          //perm:read
	StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk types.TipSetKey) (types.DealCollateralBounds, error) //perm:read
	StateVerifiedClientStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error)                                     //perm:read
}
