package porcelain

import (
	"context"
	"time"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/types"
)

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	AutoSealInterval uint
	BlockTime        time.Duration
	SectorSizes      []uint64
}

type protocolParamsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
	BlockTime() time.Duration
}

// ProtocolParameters TODO(rosa)
func ProtocolParameters(ctx context.Context, plumbing protocolParamsPlumbing) (*ProtocolParams, error) {
	autoSealIntervalInterface, err := plumbing.ConfigGet("mining.autoSealIntervalSeconds")
	if err != nil {
		return nil, err
	}

	autoSealInterval, ok := autoSealIntervalInterface.(uint)
	if !ok {
		return nil, errors.New("Failed to read autoSealInterval from config")
	}

	sectorSize, err := getSectorSize(ctx, plumbing)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrive sector size")
	}

	return &ProtocolParams{
		AutoSealInterval: autoSealInterval,
		BlockTime:        plumbing.BlockTime(),
		SectorSizes:      []uint64{sectorSize},
	}, nil
}

func getSectorSize(ctx context.Context, plumbing protocolParamsPlumbing) (uint64, error) {
	var proofsMode types.ProofsMode
	values, err := plumbing.MessageQuery(ctx, address.Address{}, address.StorageMarketAddress, "getProofsMode")
	if err != nil {
		return 0, errors.Wrap(err, "'getProofsMode' query message failed")
	}

	if err := cbor.DecodeInto(values[0], &proofsMode); err != nil {
		return 0, errors.Wrap(err, "could not convert query message result to Mode")
	}

	sectorSizeEnum := types.OneKiBSectorSize
	if proofsMode == types.LiveProofsMode {
		sectorSizeEnum = types.TwoHundredFiftySixMiBSectorSize
	}
	return proofs.GetMaxUserBytesPerStagedSector(sectorSizeEnum)
}
