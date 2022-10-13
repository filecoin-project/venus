package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

type PCHDir int

const (
	PCHUndef PCHDir = iota
	PCHInbound
	PCHOutbound
)

type Status struct {
	ControlAddr address.Address
	Direction   PCHDir
}

type PaychGetOpts struct {
	OffChain bool
}

type ChannelInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
}

type PaymentInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
	Vouchers     []*SignedVoucher
}

type VoucherSpec struct {
	Amount      BigInt
	TimeLockMin abi.ChainEpoch
	TimeLockMax abi.ChainEpoch
	MinSettle   abi.ChainEpoch

	Extra *ModVerifyParams
}

// VoucherCreateResult is the response to calling PaychVoucherCreate
type VoucherCreateResult struct {
	// Voucher that was created, or nil if there was an error or if there
	// were insufficient funds in the channel
	Voucher *SignedVoucher
	// Shortfall is the additional amount that would be needed in the channel
	// in order to be able to create the voucher
	Shortfall BigInt
}
