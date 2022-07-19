package messager

import (
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

	SelMsgNum uint64 `json:"selMsgNum"`
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
