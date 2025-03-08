package types

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	gpbft "github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	exitcode "github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
)

type ComputeStateOutput struct {
	Root  cid.Cid
	Trace []*InvocResult
}

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
	Message ChainMsg
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

type ActorState struct {
	Balance BigInt
	Code    cid.Cid
	State   interface{}
}

type NetworkName string

const (
	NetworkNameMain        NetworkName = "mainnet"
	NetworkNameCalibration NetworkName = "calibrationnet"
	NetworkNameButterfly   NetworkName = "butterflynet"
	NetworkNameInterop     NetworkName = "interopnet"
	NetworkNameIntegration NetworkName = "integrationnet"
	NetworkNameForce       NetworkName = "forcenet"
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

type PubsubScore struct {
	ID    peer.ID
	Score *pubsub.PeerScoreSnapshot
}

type Partition struct {
	AllSectors        bitfield.BitField
	FaultySectors     bitfield.BitField
	RecoveringSectors bitfield.BitField
	LiveSectors       bitfield.BitField
	ActiveSectors     bitfield.BitField
}

type Fault struct {
	Miner address.Address
	Epoch abi.ChainEpoch
}

type MessageMatch struct {
	To   address.Address
	From address.Address
}

type MsigTransaction struct {
	ID     int64
	To     address.Address
	Value  abi.TokenAmount
	Method abi.MethodNum
	Params []byte

	Approved []address.Address
}

type MsigVesting struct {
	InitialBalance abi.TokenAmount
	StartEpoch     abi.ChainEpoch
	UnlockDuration abi.ChainEpoch
}

var EmptyVesting = MsigVesting{
	InitialBalance: EmptyInt,
	StartEpoch:     -1,
	UnlockDuration: -1,
}

type MsigInfo struct {
	ApprovalsThreshold uint64
	Signers            []address.Address
	InitialBalance     abi.TokenAmount
	CurrentBalance     abi.TokenAmount
	LockBalance        abi.TokenAmount
	StartEpoch         abi.ChainEpoch
	UnlockDuration     abi.ChainEpoch
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

type MarketDealState struct {
	SectorNumber     abi.SectorNumber // 0 if not yet included in proven sector (0 is also a valid sector number).
	SectorStartEpoch abi.ChainEpoch   // -1 if not yet included in proven sector
	LastUpdatedEpoch abi.ChainEpoch   // -1 if deal state never updated
	SlashEpoch       abi.ChainEpoch   // -1 if deal never slashed
}

func MakeDealState(mds market.DealState) MarketDealState {
	return MarketDealState{
		SectorNumber:     mds.SectorNumber(),
		SectorStartEpoch: mds.SectorStartEpoch(),
		LastUpdatedEpoch: mds.LastUpdatedEpoch(),
		SlashEpoch:       mds.SlashEpoch(),
	}
}

type mstate struct {
	s MarketDealState
}

func (m mstate) SectorNumber() abi.SectorNumber {
	return m.s.SectorNumber
}

func (m mstate) SectorStartEpoch() abi.ChainEpoch {
	return m.s.SectorStartEpoch
}

func (m mstate) LastUpdatedEpoch() abi.ChainEpoch {
	return m.s.LastUpdatedEpoch
}

func (m mstate) SlashEpoch() abi.ChainEpoch {
	return m.s.SlashEpoch
}

func (m mstate) Equals(o market.DealState) bool {
	return market.DealStatesEqual(m, o)
}

func (m MarketDealState) Iface() market.DealState {
	return mstate{m}
}

type MarketDeal struct {
	Proposal DealProposal
	State    MarketDealState
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
	APIVersion APIVersion
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
	// VoucherReedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt BigInt
}

type SyncState struct {
	ActiveSyncs []ActiveSync

	VMApplied uint64
}

// just compatible code lotus
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
	case StageIdle:
		return "idle"
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
	Head    *TipSet
	Sender  peer.ID
}

func (target *Target) String() string {
	return fmt.Sprintf("{sender:%s height=%d head=%s}", target.Sender, target.Head.Height(), target.Head.Key())
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
	PendingOwnerAddress        *address.Address
	Beneficiary                address.Address
	BeneficiaryTerm            *BeneficiaryTerm
	PendingBeneficiaryTerm     *PendingBeneficiaryChange
}

type NetworkParams struct {
	NetworkName             NetworkName
	BlockDelaySecs          uint64
	ConsensusMinerMinPower  abi.StoragePower
	SupportedProofTypes     []abi.RegisteredSealProof
	PreCommitChallengeDelay abi.ChainEpoch
	ForkUpgradeParams       ForkUpgradeParams
	Eip155ChainID           int
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
	UpgradeSharkHeight       abi.ChainEpoch
	UpgradeHyggeHeight       abi.ChainEpoch
	UpgradeLightningHeight   abi.ChainEpoch
	UpgradeThunderHeight     abi.ChainEpoch
	UpgradeWatermelonHeight  abi.ChainEpoch
	UpgradeDragonHeight      abi.ChainEpoch
	UpgradePhoenixHeight     abi.ChainEpoch
	UpgradeWaffleHeight      abi.ChainEpoch
	UpgradeTuktukHeight      abi.ChainEpoch
	UpgradeTeepHeight        abi.ChainEpoch
}

type NodeStatus struct {
	SyncStatus  NodeSyncStatus
	PeerStatus  NodePeerStatus
	ChainStatus NodeChainStatus
}

type NodeSyncStatus struct {
	Epoch  uint64
	Behind uint64
}

type NodePeerStatus struct {
	PeersToPublishMsgs   int
	PeersToPublishBlocks int
}

type NodeChainStatus struct {
	BlocksPerTipsetLast100      float64
	BlocksPerTipsetLastFinality float64
}

// F3ParticipationTicket represents a ticket that authorizes a miner to
// participate in the F3 consensus.
type F3ParticipationTicket []byte

// F3ParticipationLease defines the lease granted to a storage provider for
// participating in F3 consensus, detailing the session identifier, issuer,
// subject, and the expiration instance.
type F3ParticipationLease struct {
	// Network is the name of the network this lease belongs to.
	Network gpbft.NetworkName
	// Issuer is the identity of the node that issued the lease, encoded as base58.
	Issuer string
	// MinerID is the actor ID of the miner that holds the lease.
	MinerID uint64
	// FromInstance specifies the instance ID from which this lease is valid.
	FromInstance uint64
	// ValidityTerm specifies the number of instances for which the lease remains
	// valid from the FromInstance.
	ValidityTerm uint64
}

func (l *F3ParticipationLease) ToInstance() uint64 {
	return l.FromInstance + l.ValidityTerm
}

// F3Participant captures information about the miners that are currently
// participating in F3, along with the number of instances for which their lease
// is valid.
type F3Participant struct {
	// MinerID is the actor ID of the miner that is
	MinerID uint64
	// FromInstance specifies the instance ID from which this lease is valid.
	FromInstance uint64
	// ValidityTerm specifies the number of instances for which the lease remains
	// valid from the FromInstance.
	ValidityTerm uint64
}

var (
	// ErrF3Disabled signals that F3 consensus process is disabled.
	ErrF3Disabled = &errF3Disabled{}
	// ErrF3ParticipationTicketInvalid signals that F3ParticipationTicket cannot be decoded.
	ErrF3ParticipationTicketInvalid = &errF3ParticipationTicketInvalid{}
	// ErrF3ParticipationTicketExpired signals that the current GPBFT instance as surpassed the expiry of the ticket.
	ErrF3ParticipationTicketExpired = &errF3ParticipationTicketExpired{}
	// ErrF3ParticipationIssuerMismatch signals that the ticket is not issued by the current node.
	ErrF3ParticipationIssuerMismatch = &errF3ParticipationIssuerMismatch{}
	// ErrF3ParticipationTooManyInstances signals that participation ticket cannot be
	// issued because it asks for too many instances.
	ErrF3ParticipationTooManyInstances = &errF3ParticipationTooManyInstances{}
	// ErrF3ParticipationTicketStartBeforeExisting signals that participation ticket
	// is before the start instance of an existing lease held by the miner.
	ErrF3ParticipationTicketStartBeforeExisting = &errF3ParticipationTicketStartBeforeExisting{}
	// ErrF3NotReady signals that the F3 instance isn't ready for participation yet. The caller
	// should back off and try again later.
	ErrF3NotReady = &errF3NotReady{}

	_ error = (*ErrOutOfGas)(nil)
	// _ error = (*ErrActorNotFound)(nil)
	_ error = (*errF3Disabled)(nil)
	_ error = (*errF3ParticipationTicketInvalid)(nil)
	_ error = (*errF3ParticipationTicketExpired)(nil)
	_ error = (*errF3ParticipationIssuerMismatch)(nil)
	_ error = (*errF3NotReady)(nil)
	_ error = (*ErrExecutionReverted)(nil)
	_ error = (*ErrNullRound)(nil)
)

// ErrOutOfGas signals that a call failed due to insufficient gas.
type ErrOutOfGas struct{}

func (ErrOutOfGas) Error() string { return "call ran out of gas" }

// ErrActorNotFound signals that the actor is not found.
// type ErrActorNotFound struct{}

// func (ErrActorNotFound) Error() string { return "actor not found" }

type errF3Disabled struct{}

func (errF3Disabled) Error() string { return "f3 is disabled" }

type errF3ParticipationTicketInvalid struct{}

func (errF3ParticipationTicketInvalid) Error() string { return "ticket is not valid" }

type errF3ParticipationTicketExpired struct{}

func (errF3ParticipationTicketExpired) Error() string { return "ticket has expired" }

type errF3ParticipationIssuerMismatch struct{}

func (errF3ParticipationIssuerMismatch) Error() string { return "issuer does not match current node" }

type errF3ParticipationTooManyInstances struct{}

func (errF3ParticipationTooManyInstances) Error() string { return "requested instance count too high" }

type errF3ParticipationTicketStartBeforeExisting struct{}

func (errF3ParticipationTicketStartBeforeExisting) Error() string {
	return "ticket starts before existing lease"
}

type errF3NotReady struct{}

func (errF3NotReady) Error() string { return "f3 isn't yet ready to participate" }

// ErrExecutionReverted is used to return execution reverted with a reason for a revert in the `data` field.
type ErrExecutionReverted struct {
	Message string
	Data    string
}

// Error returns the error message.
func (e *ErrExecutionReverted) Error() string { return e.Message }

// NewErrExecutionReverted creates a new ErrExecutionReverted with the given reason.
func NewErrExecutionReverted(exitCode exitcode.ExitCode, error, reason string, data []byte) *ErrExecutionReverted {
	return &ErrExecutionReverted{
		Message: fmt.Sprintf("message execution failed (exit=[%s], revert reason=[%s], vm error=[%s])", exitCode, reason, error),
		Data:    fmt.Sprintf("0x%x", data),
	}
}

type ErrNullRound struct {
	Epoch   abi.ChainEpoch
	Message string
}

func NewErrNullRound(epoch abi.ChainEpoch) *ErrNullRound {
	return &ErrNullRound{
		Epoch:   epoch,
		Message: fmt.Sprintf("requested epoch was a null round (%d)", epoch),
	}
}

func (e *ErrNullRound) Error() string {
	return e.Message
}

// Is performs a non-strict type check, we only care if the target is an ErrNullRound
// and will ignore the contents (specifically there is no matching on Epoch).
func (e *ErrNullRound) Is(target error) bool {
	_, ok := target.(*ErrNullRound)
	return ok
}
