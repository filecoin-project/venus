package porcelain

import (
	"context"
	"time"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	AutoSealInterval     uint
	BlockTime            time.Duration
	ProofsMode           types.ProofsMode
	SupportedSectorSizes []*types.BytesAmount
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

	proofsMode, err := getProofsMode(ctx, plumbing)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve proofs mode")
	}

	supportedSectorSizes := []*types.BytesAmount{types.OneKiBSectorSize}
	if proofsMode == types.LiveProofsMode {
		supportedSectorSizes[0] = types.TwoHundredFiftySixMiBSectorSize
	}

	return &ProtocolParams{
		AutoSealInterval:     autoSealInterval,
		BlockTime:            plumbing.BlockTime(),
		ProofsMode:           proofsMode,
		SupportedSectorSizes: supportedSectorSizes,
	}, nil
}

// IsSupportedSectorSize returns true if the given sector size is supported by
// the network.
func (pp *ProtocolParams) IsSupportedSectorSize(sectorSize *types.BytesAmount) bool {
	for _, size := range pp.SupportedSectorSizes {
		if size.Equal(sectorSize) {
			return true
		}
	}

	return false
}

func getProofsMode(ctx context.Context, plumbing protocolParamsPlumbing) (types.ProofsMode, error) {
	var proofsMode types.ProofsMode
	values, err := plumbing.MessageQuery(ctx, address.Address{}, address.StorageMarketAddress, "getProofsMode")
	if err != nil {
		return 0, errors.Wrap(err, "'getProofsMode' query message failed")
	}

	if err := cbor.DecodeInto(values[0], &proofsMode); err != nil {
		return 0, errors.Wrap(err, "could not convert query message result to ProofsMode")
	}

	return proofsMode, nil
}
