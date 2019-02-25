package porcelain

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

type sbgluiPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// SectorBuilderGetLastUsedID gets the last sector id used by the sectorbuilder
func SectorBuilderGetLastUsedID(ctx context.Context, plumbing sbgluiPlumbing, minerAddr address.Address) (uint64, error) {
	rets, methodSignature, err := plumbing.MessageQuery(
		ctx,
		address.Address{},
		minerAddr,
		"getLastUsedSectorID",
	)
	if err != nil {
		return 0, errors.Wrap(err, "failed to call query method getLastUsedSectorID")
	}

	lastUsedSectorIDVal, err := abi.Deserialize(rets[0], methodSignature.Return[0])
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert returned ABI value")
	}
	lastUsedSectorID, ok := lastUsedSectorIDVal.Val.(uint64)
	if !ok {
		return 0, errors.New("failed to convert returned ABI value to uint64")
	}

	return lastUsedSectorID, nil
}
