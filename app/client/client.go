package client

import (
	"context"
	"io"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-jsonrpc/auth"
	chainApiTypes "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	mineApiTypes "github.com/filecoin-project/venus/app/submodule/mining"
	venusnetwork "github.com/filecoin-project/venus/app/submodule/network"
	syncApiTypes "github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/status"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	state2 "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type FullNode struct {
	SyncerStatus             func() status.Status
	ChainTipSetWeight        func(context.Context, block.TipSetKey) (big.Int, error)
	ChainSyncHandleNewTipSet func(*block.ChainInfo) error
	SyncSubmitBlock          func(context.Context, *block.BlockMsg) error
	StateCall                func(context.Context, *types.UnsignedMessage, block.TipSetKey) (*syncApiTypes.InvocResult, error)

	MpoolPush               func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MpoolGetConfig          func(context.Context) (*messagepool.MpoolConfig, error)
	MpoolSetConfig          func(context.Context, *messagepool.MpoolConfig) error
	MpoolSelect             func(context.Context, block.TipSetKey, float64) ([]*types.SignedMessage, error)
	MpoolPending            func(context.Context, block.TipSetKey) ([]*types.SignedMessage, error)
	MpoolClear              func(context.Context, bool) error
	MpoolPushUntrusted      func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MpoolPushMessage        func(context.Context, *types.UnsignedMessage, *types.MessageSendSpec) (*types.SignedMessage, error)
	MpoolBatchPush          func(context.Context, []*types.SignedMessage) ([]cid.Cid, error)
	MpoolBatchPushUntrusted func(context.Context, []*types.SignedMessage) ([]cid.Cid, error)
	MpoolBatchPushMessage   func(context.Context, []*types.UnsignedMessage, *types.MessageSendSpec) ([]*types.SignedMessage, error)
	MpoolGetNonce           func(context.Context, address.Address) (uint64, error)
	MpoolSub                func(context.Context) (chan messagepool.MpoolUpdate, error)
	SendMsg                 func(context.Context, address.Address, address.Address, abi.MethodNum, abi.TokenAmount, abi.TokenAmount, []byte) (cid.Cid, error)
	GasEstimateMessageGas   func(context.Context, *types.UnsignedMessage, *types.MessageSendSpec, block.TipSetKey) (*types.UnsignedMessage, error)
	GasEstimateFeeCap       func(context.Context, *types.UnsignedMessage, int64, block.TipSetKey) (big.Int, error)
	GasEstimateGasPremium   func(context.Context, uint64, address.Address, int64, block.TipSetKey) (big.Int, error)

	ChainReadObj func(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj  func(context.Context, cid.Cid) (bool, error)
	ChainExport  func(context.Context, block.TipSetKey, io.Writer) error

	BeaconGetEntry func(context.Context, abi.ChainEpoch) (*block.BeaconEntry, error)

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
	StateMarketDeals                   func(ctx context.Context, tsk block.TipSetKey) (map[string]state2.MarketDeal, error)
	StateMinerActiveSectors            func(ctx context.Context, maddr address.Address, tsk block.TipSetKey) ([]*miner.SectorOnChainInfo, error)

	StateAccountKey func(context.Context, address.Address, block.TipSetKey) (address.Address, error)

	StateGetActor     func(context.Context, address.Address, block.TipSetKey) (*types.Actor, error)
	ActorGetSignature func(context.Context, address.Address, abi.MethodNum) (vm.ActorMethodSignature, error)
	ListActor         func(context.Context) (map[address.Address]*types.Actor, error)

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
	NetAddrsListen            func(context.Context) (peer.AddrInfo, error)

	MinerGetBaseInfo func(context.Context, address.Address, abi.ChainEpoch, block.TipSetKey) (*block.MiningBaseInfo, error)
	MinerCreateBlock func(context.Context, *mineApiTypes.BlockTemplate) (*block.BlockMsg, error)

	DAGGetNode     func(context.Context, string) (interface{}, error)
	DAGGetFileSize func(context.Context, cid.Cid) (uint64, error)
	DAGCat         func(context.Context, cid.Cid) (io.Reader, error)
	DAGImportData  func(context.Context, io.Reader) (ipld.Node, error)

	BlockTime                     func() time.Duration
	ChainList                     func(context.Context, block.TipSetKey, int) ([]block.TipSetKey, error)
	ProtocolParameters            func(context.Context) (*chainApiTypes.ProtocolParams, error)
	ChainHead                     func(context.Context) (*block.TipSet, error)
	ChainSetHead                  func(context.Context, block.TipSetKey) error
	ChainGetTipSet                func(block.TipSetKey) (*block.TipSet, error)
	ChainGetTipSetByHeight        func(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	GetActor                      func(context.Context, address.Address) (*types.Actor, error)
	ChainGetBlock                 func(context.Context, cid.Cid) (*block.Block, error)
	ChainGetMessage               func(context.Context, cid.Cid) (*types.UnsignedMessage, error)
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
	MessageWait                   func(context.Context, cid.Cid, abi.ChainEpoch) (*cst.ChainMessage, error)
	StateSearchMsg                func(context.Context, cid.Cid) (*cst.MsgLookup, error)
	StateWaitMsg                  func(context.Context, cid.Cid, abi.ChainEpoch) (*cst.MsgLookup, error)
	StateGetReceipt               func(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)
	StateNetworkName              func(ctx context.Context) (chainApiTypes.NetworkName, error)

	WalletBalance        func(context.Context, address.Address) (abi.TokenAmount, error)
	WalletHas            func(context.Context, address.Address) (bool, error)
	WalletDefaultAddress func() (address.Address, error)
	WalletAddresses      func() []address.Address
	WalletSetDefault     func(context.Context, address.Address) error
	WalletNewAddress     func(address.Protocol) (address.Address, error)
	WalletImport         func(*crypto.KeyInfo) (address.Address, error)
	WalletExport         func([]address.Address) ([]*crypto.KeyInfo, error)
	WalletSign           func(context.Context, address.Address, []byte, wallet.MsgMeta) (*crypto.Signature, error)
	WalletSignMessage    func(context.Context, address.Address, *types.UnsignedMessage) (*types.SignedMessage, error)
}

type DbAPI struct {
	ChainReadObj func(context.Context, cid.Cid) ([]byte, error)
	ChainHasObj  func(context.Context, cid.Cid) (bool, error)
	ChainExport  func(context.Context, block.TipSetKey, io.Writer) error
}

type BeaconAPI struct {
	BeaconGetEntry func(context.Context, abi.ChainEpoch) (*block.BeaconEntry, error)
}

type SyncerAPI struct {
	SyncerStatus             func() status.Status
	ChainTipSetWeight        func(context.Context, block.TipSetKey) (big.Int, error)
	ChainSyncHandleNewTipSet func(*block.ChainInfo) error
	SyncSubmitBlock          func(context.Context, *block.BlockMsg) error
	StateCall                func(context.Context, *types.UnsignedMessage, block.TipSetKey) (*syncApiTypes.InvocResult, error)
}

type MessagePoolAPI struct {
	MpoolPush               func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MpoolGetConfig          func(context.Context) (*messagepool.MpoolConfig, error)
	MpoolSetConfig          func(context.Context, *messagepool.MpoolConfig) error
	MpoolSelect             func(context.Context, block.TipSetKey, float64) ([]*types.SignedMessage, error)
	MpoolPending            func(context.Context, block.TipSetKey) ([]*types.SignedMessage, error)
	MpoolClear              func(context.Context, bool) error
	MpoolPushUntrusted      func(context.Context, *types.SignedMessage) (cid.Cid, error)
	MpoolPushMessage        func(context.Context, *types.UnsignedMessage, *types.MessageSendSpec) (*types.SignedMessage, error)
	MpoolBatchPush          func(context.Context, []*types.SignedMessage) ([]cid.Cid, error)
	MpoolBatchPushUntrusted func(context.Context, []*types.SignedMessage) ([]cid.Cid, error)
	MpoolBatchPushMessage   func(context.Context, []*types.UnsignedMessage, *types.MessageSendSpec) ([]*types.SignedMessage, error)
	MpoolGetNonce           func(context.Context, address.Address) (uint64, error)
	MpoolSub                func(context.Context) (chan messagepool.MpoolUpdate, error)
	SendMsg                 func(context.Context, address.Address, address.Address, abi.MethodNum, abi.TokenAmount, abi.TokenAmount, []byte) (cid.Cid, error)
	GasEstimateMessageGas   func(context.Context, *types.UnsignedMessage, *types.MessageSendSpec, block.TipSetKey) (*types.UnsignedMessage, error)
	GasEstimateFeeCap       func(context.Context, *types.UnsignedMessage, int64, block.TipSetKey) (big.Int, error)
	GasEstimateGasPremium   func(context.Context, uint64, address.Address, int64, block.TipSetKey) (big.Int, error)
}

type BlockServiceAPI struct {
	DAGGetNode     func(context.Context, string) (interface{}, error)
	DAGGetFileSize func(context.Context, cid.Cid) (uint64, error)
	DAGCat         func(context.Context, cid.Cid) (io.Reader, error)
	DAGImportData  func(context.Context, io.Reader) (ipld.Node, error)
}

type ChainInfoAPI struct {
	BlockTime                     func() time.Duration
	ChainList                     func(context.Context, block.TipSetKey, int) ([]block.TipSetKey, error)
	ProtocolParameters            func(context.Context) (*chainApiTypes.ProtocolParams, error)
	ChainHead                     func(context.Context) (*block.TipSet, error)
	ChainSetHead                  func(context.Context, block.TipSetKey) error
	ChainGetTipSet                func(block.TipSetKey) (*block.TipSet, error)
	ChainGetTipSetByHeight        func(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	GetActor                      func(context.Context, address.Address) (*types.Actor, error)
	ChainGetBlock                 func(context.Context, cid.Cid) (*block.Block, error)
	ChainGetMessage               func(context.Context, cid.Cid) (*types.UnsignedMessage, error)
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
	MessageWait                   func(context.Context, cid.Cid, abi.ChainEpoch) (*cst.ChainMessage, error)
	StateSearchMsg                func(context.Context, cid.Cid) (*cst.MsgLookup, error)
	StateWaitMsg                  func(context.Context, cid.Cid, abi.ChainEpoch) (*cst.MsgLookup, error)
	StateGetReceipt               func(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)
	StateNetworkName              func(ctx context.Context) (chainApiTypes.NetworkName, error)
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
	StateMarketDeals                   func(ctx context.Context, tsk block.TipSetKey) (map[string]state2.MarketDeal, error)
	StateMinerActiveSectors            func(ctx context.Context, maddr address.Address, tsk block.TipSetKey) ([]*miner.SectorOnChainInfo, error)
}

type AccountAPI struct {
	StateAccountKey func(context.Context, address.Address, block.TipSetKey) (address.Address, error)
}

type ActorAPI struct {
	StateGetActor     func(context.Context, address.Address, block.TipSetKey) (*types.Actor, error)
	ActorGetSignature func(context.Context, address.Address, abi.MethodNum) (vm.ActorMethodSignature, error)
	ListActor         func(context.Context) (map[address.Address]*types.Actor, error)
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
	Version                   func(context.Context) (venusnetwork.Version, error)
	NetAddrsListen            func(context.Context) (peer.AddrInfo, error)
}

type MiningAPI struct {
	MinerGetBaseInfo func(context.Context, address.Address, abi.ChainEpoch, block.TipSetKey) (*block.MiningBaseInfo, error)
	MinerCreateBlock func(context.Context, *mineApiTypes.BlockTemplate) (*block.BlockMsg, error)
}

type WalletAPI struct {
	WalletBalance        func(context.Context, address.Address) (abi.TokenAmount, error)
	WalletHas            func(context.Context, address.Address) (bool, error)
	WalletDefaultAddress func() (address.Address, error)
	WalletAddresses      func() []address.Address
	WalletSetDefault     func(context.Context, address.Address) error
	WalletNewAddress     func(address.Protocol) (address.Address, error)
	WalletImport         func(*crypto.KeyInfo) (address.Address, error)
	WalletExport         func([]address.Address) ([]*crypto.KeyInfo, error)
	WalletSign           func(context.Context, address.Address, []byte, wallet.MsgMeta) (*crypto.Signature, error)
	WalletSignMessage    func(context.Context, address.Address, *types.UnsignedMessage) (*types.SignedMessage, error)
}

type JwtAuthAPI struct {
	AuthVerify func(ctx context.Context, token string) ([]auth.Permission, error)
	AuthNew    func(ctx context.Context, perms []auth.Permission) ([]byte, error)
}
