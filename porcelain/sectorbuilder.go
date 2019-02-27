package porcelain

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

type sbsPlumbing interface {
	ConfigGet(dottedPath string) (interface{}, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
	SectorBuilderStart(minerAddr address.Address, sectorID uint64) error
}

// SectorBuilderSetup starts the sector builder with the default miner address
// from config and the last used sector id fetched from the miner actor.
func SectorBuilderSetup(ctx context.Context, plumbing sbsPlumbing) error {
	minerAddrString, err := plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return errors.Wrap(err, "failed to get node's mining address")
	}
	minerAddr, err := address.NewFromString(minerAddrString.(string))
	if err != nil {
		return errors.Wrap(err, "failed to get node's mining address")
	}

	lastUsedSectorID, err := sectorBuilderGetLastUsedID(ctx, plumbing, minerAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to get last used sector id for miner w/address %s", minerAddr.String())
	}

	err = plumbing.SectorBuilderStart(minerAddr, lastUsedSectorID)
	if err != nil {
		return errors.Wrapf(err, "failed to initialize sector builder for miner %s", minerAddr.String())
	}

	return nil
}

// sectorBuilderGetLastUsedID gets the last sector id used by the sectorbuilder
func sectorBuilderGetLastUsedID(ctx context.Context, plumbing sbsPlumbing, minerAddr address.Address) (uint64, error) {
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
