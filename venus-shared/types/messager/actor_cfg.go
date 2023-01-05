package messager

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"

	"time"
)

type ActorCfg struct {
	ID           types.UUID     `json:"id"`
	ActorVersion actors.Version `json:"version"`
	// max for current, use nonce and +1
	MethodType
	FeeSpec

	CreatedAt time.Time `json:"createAt"` // 创建时间
	UpdatedAt time.Time `json:"updateAt"` // 更新时间
}

type MethodType struct {
	Code   cid.Cid       `json:"code"`
	Method abi.MethodNum `json:"method"`
}

type FeeSpec struct {
	GasOverEstimation float64 `json:"gasOverEstimation"`
	MaxFee            big.Int `json:"maxFee,omitempty"`
	GasFeeCap         big.Int `json:"gasFeeCap"`
	GasOverPremium    float64 `json:"gasOverPremium"`
	BaseFee           big.Int `json:"baseFee"`
}

type ChangeGasSpecParams struct {
	GasOverEstimation *float64 `json:"gasOverEstimation"`
	MaxFee            big.Int  `json:"maxFee,omitempty"`
	GasFeeCap         big.Int  `json:"gasFeeCap"`
	GasOverPremium    *float64 `json:"gasOverPremium"`
	BaseFee           big.Int  `json:"baseFee"`
}
