package types

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/api"
)

type HeadChangeType string

// HeadChangeTopic is the topic used to publish new heads.
const HeadChangeTopic = "headchange"
const (
	HCRevert  HeadChangeType = "revert"
	HCApply   HeadChangeType = "apply"
	HCCurrent HeadChangeType = "current"
)

type HeadChange struct {
	Type HeadChangeType
	Val  *TipSet
}

type ObjStat struct {
	Size  uint64
	Links uint64
}

// ChainMessage is an on-chain message with its block and receipt.
type ChainMessage struct { //nolint
	TS      *TipSet
	Message *Message
	Block   *BlockHeader
	Receipt *MessageReceipt
}

// BlsMessages[x].cid = Cids[x]
// SecpkMessages[y].cid = Cids[BlsMessages.length + y]
type BlockMessages struct {
	BlsMessages   []*Message
	SecpkMessages []*SignedMessage
	Cids          []cid.Cid
}

type MessageCID struct {
	Cid     cid.Cid
	Message *Message
}

type NetworkName string

const (
	NetworkNameMain        NetworkName = "mainnet"
	NetworkNameCalibration NetworkName = "calibrationnet"
	NetworkNameButterfly   NetworkName = "butterflynet"
	NetworkNameInterop     NetworkName = "interopnet"
	NetworkNameIntegration NetworkName = "integrationnet"
)

type NetworkType int

const (
	NetworkDefault   NetworkType = 0
	NetworkMainnet   NetworkType = 0x1
	Network2k        NetworkType = 0x2
	NetworkDebug     NetworkType = 0x3
	NetworkCalibnet  NetworkType = 0x4
	NetworkNerpa     NetworkType = 0x5
	NetworkInterop   NetworkType = 0x6
	NetworkForce     NetworkType = 0x7
	NetworkButterfly NetworkType = 0x8

	Integrationnet NetworkType = 0x30
)

type Partition struct {
	AllSectors        bitfield.BitField
	FaultySectors     bitfield.BitField
	RecoveringSectors bitfield.BitField
	LiveSectors       bitfield.BitField
	ActiveSectors     bitfield.BitField
}

// SectorInfo provides information about a sector construction
type SectorInfo struct {
	Size         abi.SectorSize
	MaxPieceSize abi.UnpaddedPieceSize
}

type ProtocolParams struct {
	Network          string
	BlockTime        time.Duration
	SupportedSectors []SectorInfo
}

type Deadline struct {
	PostSubmissions      bitfield.BitField
	DisputableProofCount uint64
}

var MarketBalanceNil = MarketBalance{}

type MarketDeal struct {
	Proposal market.DealProposal
	State    market.DealState
}

type MinerPower struct {
	MinerPower  power.Claim
	TotalPower  power.Claim
	HasMinPower bool
}

type MinerSectors struct {
	// Live sectors that should be proven.
	Live uint64
	// Sectors actively contributing to power.
	Active uint64
	// Sectors with failed proofs.
	Faulty uint64
}

type MarketBalance struct {
	Escrow big.Int
	Locked big.Int
}

type DealCollateralBounds struct {
	Min abi.TokenAmount
	Max abi.TokenAmount
}

type MsgLookup struct {
	Message   cid.Cid // Can be different than requested, in case it was replaced, but only gas values changed
	Receipt   MessageReceipt
	ReturnDec interface{}
	TipSet    TipSetKey
	Height    abi.ChainEpoch
}

type MiningBaseInfo struct { //nolint
	MinerPower        abi.StoragePower
	NetworkPower      abi.StoragePower
	Sectors           []builtin.ExtendedSectorInfo
	WorkerKey         address.Address
	SectorSize        abi.SectorSize
	PrevBeaconEntry   BeaconEntry
	BeaconEntries     []BeaconEntry
	EligibleForMining bool
}

type BlockTemplate struct {
	Miner            address.Address
	Parents          TipSetKey
	Ticket           *Ticket
	Eproof           *ElectionProof
	BeaconValues     []BeaconEntry
	Messages         []*SignedMessage
	Epoch            abi.ChainEpoch
	Timestamp        uint64
	WinningPoStProof []builtin.PoStProof
}

type EstimateMessage struct {
	Msg  *Message
	Spec *MessageSendSpec
}

type EstimateResult struct {
	Msg *Message
	Err string
}

type MessageSendSpec struct {
	MaxFee            abi.TokenAmount
	GasOverEstimation float64
	GasOverPremium    float64
}

// Version provides various build-time information
type Version struct {
	Version string

	// APIVersion is a binary encoded semver version of the remote implementing
	// this api
	//
	APIVersion api.Version
}

type ChannelAvailableFunds struct {
	// Channel is the address of the channel
	Channel *address.Address
	// From is the from address of the channel (channel creator)
	From address.Address
	// To is the to address of the channel
	To address.Address
	// ConfirmedAmt is the total amount of funds that have been confirmed on-chain for the channel
	ConfirmedAmt BigInt
	// PendingAmt is the amount of funds that are pending confirmation on-chain
	PendingAmt BigInt
	// NonReservedAmt is part of ConfirmedAmt that is available for use (e.g. when the payment channel was pre-funded)
	NonReservedAmt BigInt
	// PendingAvailableAmt is the amount of funds that are pending confirmation on-chain that will become available once confirmed
	PendingAvailableAmt BigInt
	// PendingWaitSentinel can be used with PaychGetWaitReady to wait for
	// confirmation of pending funds
	PendingWaitSentinel *cid.Cid
	// QueuedAmt is the amount that is queued up behind a pending request
	QueuedAmt BigInt
	// VoucherRedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt BigInt
}

type SyncState struct {
	ActiveSyncs []ActiveSync

	VMApplied uint64
}

//just compatible code lotus
type SyncStateStage int

const (
	StageIdle = SyncStateStage(iota)
	StageHeaders
	StagePersistHeaders
	StageMessages
	StageSyncComplete
	StageSyncErrored
	StageFetchingMessages
)

func (v SyncStateStage) String() string {
	switch v {
	case StageHeaders:
		return "header sync"
	case StagePersistHeaders:
		return "persisting headers"
	case StageMessages:
		return "message sync"
	case StageSyncComplete:
		return "complete"
	case StageSyncErrored:
		return "error"
	case StageFetchingMessages:
		return "fetching messages"
	default:
		return fmt.Sprintf("<unknown: %d>", v)
	}
}

type ActiveSync struct {
	WorkerID uint64
	Base     *TipSet
	Target   *TipSet

	Stage  SyncStateStage
	Height abi.ChainEpoch

	Start   time.Time
	End     time.Time
	Message string
}

type Target struct {
	State   SyncStateStage
	Base    *TipSet
	Current *TipSet
	Start   time.Time
	End     time.Time
	Err     error
	ChainInfo
}

type TargetTracker struct {
	History []*Target
	Buckets []*Target
}

type MsgGasCost struct {
	Message            cid.Cid // Can be different than requested, in case it was replaced, but only gas values changed
	GasUsed            abi.TokenAmount
	BaseFeeBurn        abi.TokenAmount
	OverEstimationBurn abi.TokenAmount
	MinerPenalty       abi.TokenAmount
	MinerTip           abi.TokenAmount
	Refund             abi.TokenAmount
	TotalCost          abi.TokenAmount
}

type InvocResult struct {
	MsgCid         cid.Cid
	Msg            *Message
	MsgRct         *MessageReceipt
	GasCost        MsgGasCost
	ExecutionTrace ExecutionTrace
	Error          string
	Duration       time.Duration
}

type MinerInfo struct {
	Owner                      address.Address   // Must be an ID-address.
	Worker                     address.Address   // Must be an ID-address.
	NewWorker                  address.Address   // Must be an ID-address.
	ControlAddresses           []address.Address // Must be an ID-addresses.
	WorkerChangeEpoch          abi.ChainEpoch
	PeerId                     *peer.ID // nolint
	Multiaddrs                 []abi.Multiaddrs
	WindowPoStProofType        abi.RegisteredPoStProof
	SectorSize                 abi.SectorSize
	WindowPoStPartitionSectors uint64
	ConsensusFaultElapsed      abi.ChainEpoch
}

type NetworkParams struct {
	NetworkName             NetworkName
	BlockDelaySecs          uint64
	ConsensusMinerMinPower  abi.StoragePower
	SupportedProofTypes     []abi.RegisteredSealProof
	PreCommitChallengeDelay abi.ChainEpoch
	ForkUpgradeParams       ForkUpgradeParams
}

type ForkUpgradeParams struct {
	UpgradeSmokeHeight       abi.ChainEpoch
	UpgradeBreezeHeight      abi.ChainEpoch
	UpgradeIgnitionHeight    abi.ChainEpoch
	UpgradeLiftoffHeight     abi.ChainEpoch
	UpgradeAssemblyHeight    abi.ChainEpoch
	UpgradeRefuelHeight      abi.ChainEpoch
	UpgradeTapeHeight        abi.ChainEpoch
	UpgradeKumquatHeight     abi.ChainEpoch
	BreezeGasTampingDuration abi.ChainEpoch
	UpgradeCalicoHeight      abi.ChainEpoch
	UpgradePersianHeight     abi.ChainEpoch
	UpgradeOrangeHeight      abi.ChainEpoch
	UpgradeClausHeight       abi.ChainEpoch
	UpgradeTrustHeight       abi.ChainEpoch
	UpgradeNorwegianHeight   abi.ChainEpoch
	UpgradeTurboHeight       abi.ChainEpoch
	UpgradeHyperdriveHeight  abi.ChainEpoch
	UpgradeChocolateHeight   abi.ChainEpoch
	UpgradeOhSnapHeight      abi.ChainEpoch
	UpgradeSkyrHeight        abi.ChainEpoch
}
