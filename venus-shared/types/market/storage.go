package market

import (
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	market8 "github.com/filecoin-project/specs-actors/v8/actors/builtin/market"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/builtin/v8/market"
)

//todo  move to sealer

// PendingDealInfo has info about pending deals and when they are due to be
// published
type PendingDealInfo struct {
	Deals              []market.ClientDealProposal
	PublishPeriodStart time.Time
	PublishPeriod      time.Duration
}

type SectorOffset struct {
	Sector abi.SectorNumber
	Offset abi.PaddedPieceSize
}

// DealInfo is a tuple of deal identity and its schedule
type PieceDealInfo struct {
	PublishCid   *cid.Cid
	DealID       abi.DealID
	DealProposal *market8.DealProposal
	DealSchedule DealSchedule
	KeepUnsealed bool
}

// DealSchedule communicates the time interval of a piecestorage deal. The deal must
// appear in a sealed (proven) sector no later than StartEpoch, otherwise it
// is invalid.
type DealSchedule struct {
	StartEpoch abi.ChainEpoch
	EndEpoch   abi.ChainEpoch
}

type SectorState string

type SealTicket struct {
	Value abi.SealRandomness
	Epoch abi.ChainEpoch
}

type SealSeed struct {
	Value abi.InteractiveSealRandomness
	Epoch abi.ChainEpoch
}

type SectorLog struct {
	Kind      string
	Timestamp uint64

	Trace string

	Message string
}

type SectorInfo struct {
	SectorID     abi.SectorNumber
	State        SectorState
	CommD        *cid.Cid
	CommR        *cid.Cid
	Proof        []byte
	Deals        []abi.DealID
	Ticket       SealTicket
	Seed         SealSeed
	PreCommitMsg *cid.Cid
	CommitMsg    *cid.Cid
	Retries      uint64
	ToUpgrade    bool

	LastErr string

	Log []SectorLog

	// On Chain Info
	SealProof          abi.RegisteredSealProof // The seal proof type implies the PoSt proof/s
	Activation         abi.ChainEpoch          // Epoch during which the sector proof was accepted
	Expiration         abi.ChainEpoch          // Epoch during which the sector expires
	DealWeight         abi.DealWeight          // Integral of active deals over sector lifetime
	VerifiedDealWeight abi.DealWeight          // Integral of active verified deals over sector lifetime
	InitialPledge      abi.TokenAmount         // Pledge collected to commit this sector
	// Expiration Info
	OnTime abi.ChainEpoch
	// non-zero if sector is faulty, epoch at which it will be permanently
	// removed if it doesn't recover
	Early abi.ChainEpoch
}

type SealedRef struct {
	SectorID abi.SectorNumber
	Offset   abi.PaddedPieceSize
	Size     abi.UnpaddedPieceSize
}

type SealedRefs struct {
	Refs []SealedRef
}

type AddrUse int

const (
	PreCommitAddr AddrUse = iota
	CommitAddr
	DealPublishAddr
	PoStAddr

	TerminateSectorsAddr
)
