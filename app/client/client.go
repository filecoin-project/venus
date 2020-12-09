package client

import (
	"context"
	"io"
	"math/big"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	chainApiTypes "github.com/filecoin-project/venus/app/submodule/chain"
	messageApiTypes "github.com/filecoin-project/venus/app/submodule/messaging"
	"github.com/filecoin-project/venus/app/submodule/messaging/msg"
	mineApiTypes "github.com/filecoin-project/venus/app/submodule/minging"
	syncApiTypes "github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/status"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/message"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/wallet"
	cid "github.com/ipfs/go-cid/_rsrch/cidiface"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type FullNode struct {
	ChainReadObj func(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj  func(context.Context, cid.Cid) (bool, error)
	ChainExport  func(context.Context, block.TipSetKey, io.Writer) error

	StateAccountKey func(context.Context, address.Address, block.TipSetKey) (address.Address, error)

	StateGetActor     func(context.Context, address.Address, block.TipSetKey) (*types.Actor, error)
	ActorGetSignature func(context.Context, address.Address, abi.MethodNum) (vm.ActorMethodSignature, error)
	ListActor         func(context.Context) (map[address.Address]*types.Actor, error)

	BeaconGetEntry func(context.Context, abi.ChainEpoch) (*block.BeaconEntry, error)

	ConfigSet func(string, string) error
	ConfigGet func(string) (interface{}, error)

	NetworkGetBandwidthStats  func() metrics.Stats
	NetworkGetPeerAddresses   func() []ma.Multiaddr
	NetworkGetPeerID          func() peer.ID
	NetworkFindProvidersAsync func(context.Context, cid.Cid, int) chan peer.AddrInfo
	NetworkGetClosestPeers    func(context.Context, string) (chan peer.ID, error)
	NetworkFindPeer           func(context.Context, peer.ID) (peer.AddrInfo, error)
	NetworkConnect            func(context.Context, []string) (chan net.ConnectionResult, error)
	NetworkPeers              func(context.Context, bool) (*net.SwarmConnInfos, error)

	MessagePoolWait    func(context.Context, uint) ([]*types.SignedMessage, error)
	OutboxQueues       func() []address.Address
	OutboxQueueLs      func(address.Address) []*message.Queued
	OutboxQueueClear   func(context.Context, address.Address) error
	MessagePoolPending func() []*types.SignedMessage
	MessagePoolGet     func(cid.Cid) (*types.SignedMessage, error)
	MessagePoolRemove  func(cid.Cid)
	MessageSend        func(context.Context, address.Address, types.AttoFIL, types.AttoFIL, types.AttoFIL, types.Unit, abi.MethodNum, []byte) (cid.Cid, error)
	SignedMessageSend  func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MessageWait        func(context.Context, cid.Cid, abi.ChainEpoch) (*msg.ChainMessage, error)
	StateSearchMsg     func(context.Context, cid.Cid) (*messageApiTypes.MsgLookup, error)
	StateWaitMsg       func(context.Context, cid.Cid, abi.ChainEpoch) (*messageApiTypes.MsgLookup, error)
	StateGetReceipt    func(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)

	BlockTime                     func() time.Duration
	ChainList                     func(context.Context, block.TipSetKey, int) ([]block.TipSetKey, error)
	ProtocolParameters            func(context.Context) (*chainApiTypes.ProtocolParams, error)
	ChainHead                     func(context.Context) (*block.TipSet, error)
	ChainSetHead                  func(context.Context, block.TipSetKey) error
	ChainGetTipSet                func(block.TipSetKey) (*block.TipSet, error)
	ChainGetTipSetByHeight        func(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	ChainGetBlock                 func(context.Context, cid.Cid) (*block.Block, error)
	ChainGetMessages              func(context.Context, cid.Cid) (*chainApiTypes.BlockMessage, error)
	ChainGetBlockMessages         func(context.Context, cid.Cid) (*chainApiTypes.BlockMessages, error)
	ChainGetReceipts              func(context.Context, cid.Cid) ([]types.MessageReceipt, error)
	GetFullBlock                  func(context.Context, cid.Cid) (*block.FullBlock, error)
	ResolveToKeyAddr              func(context.Context, address.Address, *block.TipSet) (address.Address, error)
	ChainNotify                   func(context.Context) chan []*chain.HeadChange
	GetEntry                      func(context.Context, abi.ChainEpoch, uint64) (*block.BeaconEntry, error)
	VerifyEntry                   func(*block.BeaconEntry, abi.ChainEpoch) bool
	ChainGetRandomnessFromBeacon  func(context.Context, block.TipSetKey, acrypto.DomainSeparationTag, abi.ChainEpoch, []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets func(context.Context, block.TipSetKey, acrypto.DomainSeparationTag, abi.ChainEpoch, []byte) (abi.Randomness, error)
	StateNetworkVersion           func(context.Context, block.TipSetKey) (network.Version, error)

	DAGGetNode     func(context.Context, string) (interface{}, error)
	DAGGetFileSize func(context.Context, cid.Cid) (uint64, error)
	DAGCat         func(context.Context, cid.Cid) (io.Reader, error)
	DAGImportData  func(context.Context, io.Reader) (ipld.Node, error)

	StateMinerSectorAllocated          func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (bool, error)
	StateSectorPreCommitInfo           func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (miner.SectorPreCommitOnChainInfo, error)
	StateSectorGetInfo                 func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (*miner.SectorOnChainInfo, error)
	StateSectorPartition               func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (*miner.SectorLocation, error)
	StateMinerSectorSize               func(context.Context, address.Address, block.TipSetKey) (abi.SectorSize, error)
	StateMinerInfo                     func(context.Context, address.Address, block.TipSetKey) (miner.MinerInfo, error)
	StateMinerWorkerAddress            func(context.Context, address.Address, block.TipSetKey) (address.Address, error)
	StateMinerRecoveries               func(context.Context, address.Address, block.TipSetKey) (bitfield.BitField, error)
	StateMinerFaults                   func(context.Context, address.Address, block.TipSetKey) (bitfield.BitField, error)
	StateMinerProvingDeadline          func(context.Context, address.Address, block.TipSetKey) (*dline.Info, error)
	StateMinerPartitions               func(context.Context, address.Address, uint64, block.TipSetKey) ([]chainApiTypes.Partition, error)
	StateMinerDeadlines                func(context.Context, address.Address, block.TipSetKey) ([]chainApiTypes.Deadline, error)
	StateMinerSectors                  func(context.Context, address.Address, *bitfield.BitField, block.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateMarketStorageDeal             func(context.Context, abi.DealID, block.TipSetKey) (*chainApiTypes.MarketDeal, error)
	StateMinerPreCommitDepositForPower func(context.Context, address.Address, miner.SectorPreCommitInfo, block.TipSetKey) (big.Int, error)
	StateMinerInitialPledgeCollateral  func(context.Context, address.Address, miner.SectorPreCommitInfo, block.TipSetKey) (big.Int, error)
	StateVMCirculatingSupplyInternal   func(context.Context, block.TipSetKey) (chain.CirculatingSupply, error)
	StateCirculatingSupply             func(context.Context, block.TipSetKey) (abi.TokenAmount, error)

	SyncerStatus             func() status.Status
	ChainTipSetWeight        func(context.Context, block.TipSetKey) (fbig.Int, error)
	ChainSyncHandleNewTipSet func(*block.ChainInfo) error
	SyncSubmitBlock          func(context.Context, *block.BlockMsg) error
	StateCall                func(context.Context, *types.UnsignedMessage, *block.TipSet) (*syncApiTypes.InvocResult, error)

	MinerGetBaseInfo func(context.Context, beacon.Schedule, block.TipSetKey, abi.ChainEpoch, address.Address) (*block.MiningBaseInfo, error)
	MinerCreateBlock func(context.Context, *mineApiTypes.BlockTemplate) (*block.BlockMsg, error)

	WalletBalance           func(context.Context, address.Address) (abi.TokenAmount, error)
	WalletDefaultAddress    func() (address.Address, error)
	WalletAddresses         func() []address.Address
	SetWalletDefaultAddress func(address.Address) error
	WalletNewAddress        func(address.Protocol) (address.Address, error)
	WalletImport            func([]*crypto.KeyInfo) ([]address.Address, error)
	WalletExport            func([]address.Address) ([]*crypto.KeyInfo, error)
	WalletSign              func(context.Context, address.Address, []byte, wallet.MsgMeta) (*crypto.Signature, error)
}

type MiningAPI struct {
	MinerGetBaseInfo func(context.Context, beacon.Schedule, block.TipSetKey, abi.ChainEpoch, address.Address) (*block.MiningBaseInfo, error)
	MinerCreateBlock func(context.Context, *mineApiTypes.BlockTemplate) (*block.BlockMsg, error)
}

type WalletAPI struct {
	WalletBalance           func(context.Context, address.Address) (abi.TokenAmount, error)
	WalletDefaultAddress    func() (address.Address, error)
	WalletAddresses         func() []address.Address
	SetWalletDefaultAddress func(address.Address) error
	WalletNewAddress        func(address.Protocol) (address.Address, error)
	WalletImport            func([]*crypto.KeyInfo) ([]address.Address, error)
	WalletExport            func([]address.Address) ([]*crypto.KeyInfo, error)
	WalletSign              func(context.Context, address.Address, []byte, wallet.MsgMeta) (*crypto.Signature, error)
}

type MinerStateAPI struct {
	StateMinerSectorAllocated          func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (bool, error)
	StateSectorPreCommitInfo           func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (miner.SectorPreCommitOnChainInfo, error)
	StateSectorGetInfo                 func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (*miner.SectorOnChainInfo, error)
	StateSectorPartition               func(context.Context, address.Address, abi.SectorNumber, block.TipSetKey) (*miner.SectorLocation, error)
	StateMinerSectorSize               func(context.Context, address.Address, block.TipSetKey) (abi.SectorSize, error)
	StateMinerInfo                     func(context.Context, address.Address, block.TipSetKey) (miner.MinerInfo, error)
	StateMinerWorkerAddress            func(context.Context, address.Address, block.TipSetKey) (address.Address, error)
	StateMinerRecoveries               func(context.Context, address.Address, block.TipSetKey) (bitfield.BitField, error)
	StateMinerFaults                   func(context.Context, address.Address, block.TipSetKey) (bitfield.BitField, error)
	StateMinerProvingDeadline          func(context.Context, address.Address, block.TipSetKey) (*dline.Info, error)
	StateMinerPartitions               func(context.Context, address.Address, uint64, block.TipSetKey) ([]chainApiTypes.Partition, error)
	StateMinerDeadlines                func(context.Context, address.Address, block.TipSetKey) ([]chainApiTypes.Deadline, error)
	StateMinerSectors                  func(context.Context, address.Address, *bitfield.BitField, block.TipSetKey) ([]*miner.SectorOnChainInfo, error)
	StateMarketStorageDeal             func(context.Context, abi.DealID, block.TipSetKey) (*chainApiTypes.MarketDeal, error)
	StateMinerPreCommitDepositForPower func(context.Context, address.Address, miner.SectorPreCommitInfo, block.TipSetKey) (big.Int, error)
	StateMinerInitialPledgeCollateral  func(context.Context, address.Address, miner.SectorPreCommitInfo, block.TipSetKey) (big.Int, error)
	StateVMCirculatingSupplyInternal   func(context.Context, block.TipSetKey) (chain.CirculatingSupply, error)
	StateCirculatingSupply             func(context.Context, block.TipSetKey) (abi.TokenAmount, error)
}

type SyncerAPI struct {
	SyncerStatus             func() status.Status
	ChainTipSetWeight        func(context.Context, block.TipSetKey) (fbig.Int, error)
	ChainSyncHandleNewTipSet func(*block.ChainInfo) error
	SyncSubmitBlock          func(context.Context, *block.BlockMsg) error
	StateCall                func(context.Context, *types.UnsignedMessage, *block.TipSet) (*syncApiTypes.InvocResult, error)
}

type ConfigAPI struct {
	ConfigSet func(string, string) error
	ConfigGet func(string) (interface{}, error)
}

type NetworkAPI struct {
	NetworkGetBandwidthStats  func() metrics.Stats
	NetworkGetPeerAddresses   func() []ma.Multiaddr
	NetworkGetPeerID          func() peer.ID
	NetworkFindProvidersAsync func(context.Context, cid.Cid, int) chan peer.AddrInfo
	NetworkGetClosestPeers    func(context.Context, string) (chan peer.ID, error)
	NetworkFindPeer           func(context.Context, peer.ID) (peer.AddrInfo, error)
	NetworkConnect            func(context.Context, []string) (chan net.ConnectionResult, error)
	NetworkPeers              func(context.Context, bool) (*net.SwarmConnInfos, error)
}

type MessagingAPI struct {
	MessagePoolWait    func(context.Context, uint) ([]*types.SignedMessage, error)
	OutboxQueues       func() []address.Address
	OutboxQueueLs      func(address.Address) []*message.Queued
	OutboxQueueClear   func(context.Context, address.Address) error
	MessagePoolPending func() []*types.SignedMessage
	MessagePoolGet     func(cid.Cid) (*types.SignedMessage, error)
	MessagePoolRemove  func(cid.Cid)
	MessageSend        func(context.Context, address.Address, types.AttoFIL, types.AttoFIL, types.AttoFIL, types.Unit, abi.MethodNum, []byte) (cid.Cid, error)
	SignedMessageSend  func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MessageWait        func(context.Context, cid.Cid, abi.ChainEpoch) (*msg.ChainMessage, error)
	StateSearchMsg     func(context.Context, cid.Cid) (*messageApiTypes.MsgLookup, error)
	StateWaitMsg       func(context.Context, cid.Cid, abi.ChainEpoch) (*messageApiTypes.MsgLookup, error)
	StateGetReceipt    func(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)
}

type ChainInfoAPI struct {
	BlockTime                     func() time.Duration
	ChainList                     func(context.Context, block.TipSetKey, int) ([]block.TipSetKey, error)
	ProtocolParameters            func(context.Context) (*chainApiTypes.ProtocolParams, error)
	ChainHead                     func(context.Context) (*block.TipSet, error)
	ChainSetHead                  func(context.Context, block.TipSetKey) error
	ChainGetTipSet                func(block.TipSetKey) (*block.TipSet, error)
	ChainGetTipSetByHeight        func(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	ChainGetBlock                 func(context.Context, cid.Cid) (*block.Block, error)
	ChainGetMessages              func(context.Context, cid.Cid) (*chainApiTypes.BlockMessage, error)
	ChainGetBlockMessages         func(context.Context, cid.Cid) (*chainApiTypes.BlockMessages, error)
	ChainGetReceipts              func(context.Context, cid.Cid) ([]types.MessageReceipt, error)
	GetFullBlock                  func(context.Context, cid.Cid) (*block.FullBlock, error)
	ResolveToKeyAddr              func(context.Context, address.Address, *block.TipSet) (address.Address, error)
	ChainNotify                   func(context.Context) chan []*chain.HeadChange
	GetEntry                      func(context.Context, abi.ChainEpoch, uint64) (*block.BeaconEntry, error)
	VerifyEntry                   func(*block.BeaconEntry, abi.ChainEpoch) bool
	ChainGetRandomnessFromBeacon  func(context.Context, block.TipSetKey, acrypto.DomainSeparationTag, abi.ChainEpoch, []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets func(context.Context, block.TipSetKey, acrypto.DomainSeparationTag, abi.ChainEpoch, []byte) (abi.Randomness, error)
	StateNetworkVersion           func(context.Context, block.TipSetKey) (network.Version, error)
}

type DbAPI struct {
	ChainReadObj func(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj  func(context.Context, cid.Cid) (bool, error)
	ChainExport  func(context.Context, block.TipSetKey, io.Writer) error
}

type AccountAPI struct {
	StateAccountKey func(context.Context, address.Address, block.TipSetKey) (address.Address, error)
}

type ActorAPI struct {
	StateGetActor     func(context.Context, address.Address, block.TipSetKey) (*types.Actor, error)
	ActorGetSignature func(context.Context, address.Address, abi.MethodNum) (vm.ActorMethodSignature, error)
	ListActor         func(context.Context) (map[address.Address]*types.Actor, error)
}

type BeaconAPI struct {
	BeaconGetEntry func(context.Context, abi.ChainEpoch) (*block.BeaconEntry, error)
}

type BlockServiceAPI struct {
	DAGGetNode     func(context.Context, string) (interface{}, error)
	DAGGetFileSize func(context.Context, cid.Cid) (uint64, error)
	DAGCat         func(context.Context, cid.Cid) (io.Reader, error)
	DAGImportData  func(context.Context, io.Reader) (ipld.Node, error)
}
