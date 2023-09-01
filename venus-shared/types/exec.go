package types

import (
	"encoding/json"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
)

type GasTrace struct {
	Name       string
	TotalGas   int64         `json:"tg"`
	ComputeGas int64         `json:"cg"`
	StorageGas int64         `json:"sg"`
	TimeTaken  time.Duration `json:"tt"`
}

type MessageTrace struct {
	From        address.Address
	To          address.Address
	Value       abi.TokenAmount
	Method      abi.MethodNum
	Params      []byte
	ParamsCodec uint64
	GasLimit    uint64
	ReadOnly    bool
	CodeCid     cid.Cid
}

type ReturnTrace struct {
	ExitCode    exitcode.ExitCode
	Return      []byte
	ReturnCodec uint64
}

type ExecutionTrace struct {
	Msg        MessageTrace
	MsgRct     ReturnTrace
	GasCharges []*GasTrace      `cborgen:"maxlen=1000000000"`
	Subcalls   []ExecutionTrace `cborgen:"maxlen=1000000000"`
}

func (gt *GasTrace) MarshalJSON() ([]byte, error) {
	type GasTraceCopy GasTrace
	cpy := (*GasTraceCopy)(gt)
	return json.Marshal(cpy)
}
