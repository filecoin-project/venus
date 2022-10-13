package types

import (
	paychtypes "github.com/filecoin-project/go-state-types/builtin/v8/paych"
	markettypes "github.com/filecoin-project/go-state-types/builtin/v9/market"
	minertypes "github.com/filecoin-project/go-state-types/builtin/v9/miner"
	powertypes "github.com/filecoin-project/go-state-types/builtin/v9/power"
	verifregtypes "github.com/filecoin-project/go-state-types/builtin/v9/verifreg"
)

type (
	ComputeDataCommitmentParams = markettypes.ComputeDataCommitmentParams
	ComputeDataCommitmentReturn = markettypes.ComputeDataCommitmentReturn
	SectorDataSpec              = markettypes.SectorDataSpec
	DealProposal                = markettypes.DealProposal
	WithdrawBalanceParams       = markettypes.WithdrawBalanceParams
	PublishStorageDealsParams   = markettypes.PublishStorageDealsParams
	ClientDealProposal          = markettypes.ClientDealProposal
	DealState                   = markettypes.DealState

	ChangeWorkerAddressParams    = minertypes.ChangeWorkerAddressParams
	CompactSectorNumbersParams   = minertypes.CompactSectorNumbersParams
	ExpirationExtension          = minertypes.ExpirationExtension
	ExtendSectorExpirationParams = minertypes.ExtendSectorExpirationParams
	PreCommitSectorBatchParams   = minertypes.PreCommitSectorBatchParams
	TerminationDeclaration       = minertypes.TerminationDeclaration
	TerminateSectorsParams       = minertypes.TerminateSectorsParams
	SectorPreCommitOnChainInfo   = minertypes.SectorPreCommitOnChainInfo
	SectorOnChainInfo            = minertypes.SectorOnChainInfo
	SectorPreCommitInfo          = minertypes.SectorPreCommitInfo
	BeneficiaryTerm              = minertypes.BeneficiaryTerm
	PendingBeneficiaryChange     = minertypes.PendingBeneficiaryChange

	CreateMinerParams = powertypes.CreateMinerParams
	CreateMinerReturn = powertypes.CreateMinerReturn

	SignedVoucher   = paychtypes.SignedVoucher
	ModVerifyParams = paychtypes.ModVerifyParams
	LaneState       = paychtypes.LaneState

	ClaimId      = verifregtypes.ClaimId // nolint
	Claim        = verifregtypes.Claim
	AllocationId = verifregtypes.AllocationId // nolint
	Allocation   = verifregtypes.Allocation
)

var (
	NewLabelFromString = markettypes.NewLabelFromString
	DealWeight         = markettypes.DealWeight

	QAPowerMax               = minertypes.QAPowerMax
	PreCommitDepositForPower = minertypes.PreCommitDepositForPower
	InitialPledgeForPower    = minertypes.InitialPledgeForPower

	NoAllocationID = verifregtypes.NoAllocationID
)
