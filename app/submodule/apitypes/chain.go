package apitypes

import (
	"time"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/types"
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

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	Network          string
	BlockTime        time.Duration
	SupportedSectors []SectorInfo
}

type Deadline struct {
	PostSubmissions      bitfield.BitField
	DisputableProofCount uint64
}

// BlsMessages[x].cid = Cids[x]
// SecpkMessages[y].cid = Cids[BlsMessages.length + y]
type BlockMessages struct {
	BlsMessages   []*types.UnsignedMessage
	SecpkMessages []*types.SignedMessage
	Cids          []cid.Cid
}

type MarketDeal struct {
	Proposal market.DealProposal
	State    market.DealState
}

type NetworkName string

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

var MarketBalanceNil = MarketBalance{}

type Message struct {
	Cid     cid.Cid
	Message *types.UnsignedMessage
}

type MinerPower struct {
	MinerPower  power.Claim
	TotalPower  power.Claim
	HasMinPower bool
}

type MsgLookup = chain.MsgLookup
