package messager

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
)

type SendSpec struct {
	ExpireEpoch       abi.ChainEpoch `json:"expireEpoch"`
	GasOverEstimation float64        `json:"gasOverEstimation"`
	MaxFee            big.Int        `json:"maxFee,omitempty"`
	GasOverPremium    float64        `json:"gasOverPremium"`
}

type SharedSpec struct {
	ID uint `json:"id"`

	GasOverEstimation float64 `json:"gasOverEstimation"`
	MaxFee            big.Int `json:"maxFee,omitempty"`
	GasFeeCap         big.Int `json:"gasFeeCap"`
	GasOverPremium    float64 `json:"gasOverPremium"`
	BaseFee           big.Int `json:"baseFee"`

	SelMsgNum uint64 `json:"selMsgNum"`
}

type AddressSpec struct {
	Address           address.Address `json:"address"`
	GasOverEstimation float64         `json:"gasOverEstimation"`
	GasOverPremium    float64         `json:"gasOverPremium"`
	MaxFeeStr         string          `json:"maxFeeStr"`
	GasFeeCapStr      string          `json:"gasFeeCapStr"`
	BaseFeeStr        string          `json:"baseFeeStr"`
}

func (ss *SharedSpec) GetSendSpec() *SendSpec {
	if ss == nil {
		return nil
	}

	return &SendSpec{
		GasOverEstimation: ss.GasOverEstimation,
		MaxFee:            ss.MaxFee,
		GasOverPremium:    ss.GasOverPremium,
	}
}
