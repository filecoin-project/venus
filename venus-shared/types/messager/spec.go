package messager

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
)

type SendSpec struct {
	ExpireEpoch       abi.ChainEpoch `json:"expireEpoch"`
	GasOverEstimation float64        `json:"gasOverEstimation"`
	MaxFee            big.Int        `json:"maxFee,omitempty"`
	MaxFeeCap         big.Int        `json:"maxFeeCap"`
}

type SharedSpec struct {
	ID uint `json:"id"`

	GasOverEstimation float64 `json:"gasOverEstimation"`
	MaxFee            big.Int `json:"maxFee,omitempty"`
	MaxFeeCap         big.Int `json:"maxFeeCap"`

	SelMsgNum uint64 `json:"selMsgNum"`
}

func (ss *SharedSpec) GetSendSpec() *SendSpec {
	if ss == nil {
		return nil
	}

	return &SendSpec{
		GasOverEstimation: ss.GasOverEstimation,
		MaxFee:            ss.MaxFee,
		MaxFeeCap:         ss.MaxFeeCap,
	}
}
