package v1

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/paych"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/stmgr"
)

type ObjStat struct {
	Size  uint64
	Links uint64
}

type HeadChange struct {
	Type string
	Val  *chain.TipSet
}

// ChainMessage is an on-chain message with its block and receipt.
type ChainMessage struct { //nolint
	TS      *chain.TipSet
	Message *chain.Message
	Block   *chain.BlockHeader
	Receipt *chain.MessageReceipt
}

// BlsMessages[x].cid = Cids[x]
// SecpkMessages[y].cid = Cids[BlsMessages.length + y]
type BlockMessages struct {
	BlsMessages   []*chain.Message
	SecpkMessages []*chain.SignedMessage
	Cids          []cid.Cid
}

type Message struct {
	Cid     cid.Cid
	Message *chain.Message
}

type NetworkName string

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
	Receipt   chain.MessageReceipt
	ReturnDec interface{}
	TipSet    chain.TipSetKey
	Height    abi.ChainEpoch
}

type MiningBaseInfo struct { //nolint
	MinerPower        abi.StoragePower
	NetworkPower      abi.StoragePower
	Sectors           []builtin.SectorInfo
	WorkerKey         address.Address
	SectorSize        abi.SectorSize
	PrevBeaconEntry   chain.BeaconEntry
	BeaconEntries     []chain.BeaconEntry
	EligibleForMining bool
}

type BlockTemplate struct {
	Miner            address.Address
	Parents          chain.TipSetKey
	Ticket           chain.Ticket
	Eproof           *chain.ElectionProof
	BeaconValues     []*chain.BeaconEntry
	Messages         []*chain.SignedMessage
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
}

// Version provides various build-time information
type Version struct {
	Version string

	// APIVersion is a binary encoded semver version of the remote implementing
	// this api
	//
	APIVersion api.Version
}

type PCHDir int

const (
	PCHUndef PCHDir = iota
	PCHInbound
	PCHOutbound
)

type PaychStatus struct {
	ControlAddr address.Address
	Direction   PCHDir
}

type ChannelInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
}

type ChannelAvailableFunds struct {
	// Channel is the address of the channel
	Channel *address.Address
	// From is the from address of the channel (channel creator)
	From address.Address
	// To is the to address of the channel
	To address.Address
	// ConfirmedAmt is the amount of funds that have been confirmed on-chain
	// for the channel
	ConfirmedAmt chain.BigInt
	// PendingAmt is the amount of funds that are pending confirmation on-chain
	PendingAmt chain.BigInt
	// PendingWaitSentinel can be used with PaychGetWaitReady to wait for
	// confirmation of pending funds
	PendingWaitSentinel *cid.Cid
	// QueuedAmt is the amount that is queued up behind a pending request
	QueuedAmt chain.BigInt
	// VoucherRedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt chain.BigInt
}

type PaymentInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
	Vouchers     []*paych.SignedVoucher
}

type VoucherSpec struct {
	Amount      chain.BigInt
	TimeLockMin abi.ChainEpoch
	TimeLockMax abi.ChainEpoch
	MinSettle   abi.ChainEpoch

	Extra *paych.ModVerifyParams
}

// VoucherCreateResult is the response to calling PaychVoucherCreate
type VoucherCreateResult struct {
	// Voucher that was created, or nil if there was an error or if there
	// were insufficient funds in the channel
	Voucher *paych.SignedVoucher
	// Shortfall is the additional amount that would be needed in the channel
	// in order to be able to create the voucher
	Shortfall chain.BigInt
}

// ChainInfo is used to track metadata about a peer and its chain.
type ChainInfo struct {
	// The originator of the TipSetKey propagation wave.
	Source peer.ID
	// The peer that sent us the TipSetKey message.
	Sender peer.ID
	Head   *chain.TipSet
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
	Base     *chain.TipSet
	Target   *chain.TipSet

	Stage  SyncStateStage
	Height abi.ChainEpoch

	Start   time.Time
	End     time.Time
	Message string
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
	Msg            *chain.Message
	MsgRct         *chain.MessageReceipt
	GasCost        MsgGasCost
	ExecutionTrace stmgr.ExecutionTrace
	Error          string
	Duration       time.Duration
}
