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

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

type IChain interface {
	IAccount
	IActor
	IBeacon
	IMinerState
	IChainInfo
}

type IAccount interface {
	StateAccountKey(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (address.Address, error) //perm:read
}

type IActor interface {
	StateGetActor(ctx context.Context, actor address.Address, tsk chain.TipSetKey) (*chain.Actor, error) //perm:read
	ListActor(ctx context.Context) (map[address.Address]*chain.Actor, error)                             //perm:read
}

type IBeacon interface {
	BeaconGetEntry(ctx context.Context, epoch abi.ChainEpoch) (*chain.BeaconEntry, error) //perm:read
}

type IChainInfo interface {
	BlockTime(ctx context.Context) time.Duration                                                                                                                                          //perm:read
	ChainList(ctx context.Context, tsKey chain.TipSetKey, count int) ([]chain.TipSetKey, error)                                                                                           //perm:read
	ChainHead(ctx context.Context) (*chain.TipSet, error)                                                                                                                                 //perm:read
	ChainSetHead(ctx context.Context, key chain.TipSetKey) error                                                                                                                          //perm:admin
	ChainGetTipSet(ctx context.Context, key chain.TipSetKey) (*chain.TipSet, error)                                                                                                       //perm:read
	ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk chain.TipSetKey) (*chain.TipSet, error)                                                                        //perm:read
	ChainGetRandomnessFromBeacon(ctx context.Context, key chain.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)  //perm:read
	ChainGetRandomnessFromTickets(ctx context.Context, tsk chain.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) //perm:read
	ChainGetBlock(ctx context.Context, id cid.Cid) (*chain.BlockHeader, error)                                                                                                            //perm:read
	ChainGetMessage(ctx context.Context, msgID cid.Cid) (*chain.Message, error)                                                                                                           //perm:read
	ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*chain2.BlockMessages, error)                                                                                                //perm:read
	ChainGetMessagesInTipset(ctx context.Context, key chain.TipSetKey) ([]chain2.Message, error)                                                                                          //perm:read
	ChainGetReceipts(ctx context.Context, id cid.Cid) ([]chain.MessageReceipt, error)                                                                                                     //perm:read
	ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]chain2.Message, error)                                                                                                   //perm:read
	ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*chain.MessageReceipt, error)                                                                                            //perm:read
	StateVerifiedRegistryRootKey(ctx context.Context, tsk chain.TipSetKey) (address.Address, error)                                                                                       //perm:read
	StateVerifierStatus(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (*abi.StoragePower, error)                                                                        //perm:read
	ChainNotify(ctx context.Context) (<-chan []*chain2.HeadChange, error)                                                                                                                 //perm:read
	GetFullBlock(ctx context.Context, id cid.Cid) (*chain.FullBlock, error)                                                                                                               //perm:read
	GetActor(ctx context.Context, addr address.Address) (*chain.Actor, error)                                                                                                             //perm:read
	GetParentStateRootActor(ctx context.Context, ts *chain.TipSet, addr address.Address) (*chain.Actor, error)                                                                            //perm:read
	GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*chain.BeaconEntry, error)                                                                                        //perm:read
	MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*chain2.ChainMessage, error)                                                                   //perm:read
	ProtocolParameters(ctx context.Context) (*chain2.ProtocolParams, error)                                                                                                               //perm:read
	ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *chain.TipSet) (address.Address, error)                                                                                //perm:read
	StateNetworkName(ctx context.Context) (chain2.NetworkName, error)                                                                                                                     //perm:read
	StateGetReceipt(ctx context.Context, msg cid.Cid, from chain.TipSetKey) (*chain.MessageReceipt, error)                                                                                //perm:read
	StateSearchMsg(ctx context.Context, msg cid.Cid) (*chain2.MsgLookup, error)                                                                                                           //perm:read
	StateSearchMsgLimited(ctx context.Context, cid cid.Cid, limit abi.ChainEpoch) (*chain2.MsgLookup, error)                                                                              //perm:read
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64) (*chain2.MsgLookup, error)                                                                                          //perm:read
	StateWaitMsgLimited(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch) (*chain2.MsgLookup, error)                                                             //perm:read
	StateNetworkVersion(ctx context.Context, tsk chain.TipSetKey) (network.Version, error)                                                                                                //perm:read
	VerifyEntry(parent, child *chain.BeaconEntry, height abi.ChainEpoch) bool                                                                                                             //perm:read
	ChainExport(context.Context, abi.ChainEpoch, bool, chain.TipSetKey) (<-chan []byte, error)                                                                                            //perm:read
	ChainGetPath(ctx context.Context, from chain.TipSetKey, to chain.TipSetKey) ([]*chain2.HeadChange, error)                                                                             //perm:read
}

type IMinerState interface {
	StateMinerSectorAllocated(ctx context.Context, maddr address.Address, s abi.SectorNumber, tsk chain.TipSetKey) (bool, error)                              //perm:read
	StateSectorPreCommitInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk chain.TipSetKey) (miner.SectorPreCommitOnChainInfo, error)   //perm:read
	StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk chain.TipSetKey) (*miner.SectorOnChainInfo, error)                 //perm:read
	StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk chain.TipSetKey) (*miner.SectorLocation, error)       //perm:read
	StateMinerSectorSize(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (abi.SectorSize, error)                                             //perm:read
	StateMinerInfo(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (miner.MinerInfo, error)                                                  //perm:read
	StateMinerWorkerAddress(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (address.Address, error)                                         //perm:read
	StateMinerRecoveries(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (bitfield.BitField, error)                                          //perm:read
	StateMinerFaults(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (bitfield.BitField, error)                                              //perm:read
	StateMinerProvingDeadline(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (*dline.Info, error)                                           //perm:read
	StateMinerPartitions(ctx context.Context, maddr address.Address, dlIdx uint64, tsk chain.TipSetKey) ([]chain2.Partition, error)                           //perm:read
	StateMinerDeadlines(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) ([]chain2.Deadline, error)                                           //perm:read
	StateMinerSectors(ctx context.Context, maddr address.Address, sectorNos *bitfield.BitField, tsk chain.TipSetKey) ([]*miner.SectorOnChainInfo, error)      //perm:read
	StateMarketStorageDeal(ctx context.Context, dealID abi.DealID, tsk chain.TipSetKey) (*chain2.MarketDeal, error)                                           //perm:read
	StateMinerPreCommitDepositForPower(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk chain.TipSetKey) (big.Int, error)       //perm:read
	StateMinerInitialPledgeCollateral(ctx context.Context, maddr address.Address, pci miner.SectorPreCommitInfo, tsk chain.TipSetKey) (big.Int, error)        //perm:read
	StateVMCirculatingSupplyInternal(ctx context.Context, tsk chain.TipSetKey) (chain.CirculatingSupply, error)                                               //perm:read
	StateCirculatingSupply(ctx context.Context, tsk chain.TipSetKey) (abi.TokenAmount, error)                                                                 //perm:read
	StateMarketDeals(ctx context.Context, tsk chain.TipSetKey) (map[string]chain2.MarketDeal, error)                                                          //perm:read
	StateMinerActiveSectors(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) ([]*miner.SectorOnChainInfo, error)                              //perm:read
	StateLookupID(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (address.Address, error)                                                    //perm:read
	StateListMiners(ctx context.Context, tsk chain.TipSetKey) ([]address.Address, error)                                                                      //perm:read
	StateListActors(ctx context.Context, tsk chain.TipSetKey) ([]address.Address, error)                                                                      //perm:read
	StateMinerPower(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (*chain2.MinerPower, error)                                               //perm:read
	StateMinerAvailableBalance(ctx context.Context, maddr address.Address, tsk chain.TipSetKey) (big.Int, error)                                              //perm:read
	StateSectorExpiration(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tsk chain.TipSetKey) (*miner.SectorExpiration, error)    //perm:read
	StateMinerSectorCount(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (chain2.MinerSectors, error)                                        //perm:read
	StateMarketBalance(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (chain2.MarketBalance, error)                                          //perm:read
	StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk chain.TipSetKey) (chain2.DealCollateralBounds, error) //perm:read
	StateVerifiedClientStatus(ctx context.Context, addr address.Address, tsk chain.TipSetKey) (*abi.StoragePower, error)                                      //perm:read
}
