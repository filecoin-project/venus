package porcelain

import (
	"context"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// ProtocolParams TODO(rosa)
type ProtocolParams struct {
	AutoSealInterval uint
	ProofsMode       types.ProofsMode
}

type protocolParamsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
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

	return &ProtocolParams{
		AutoSealInterval: autoSealInterval,
		ProofsMode:       proofsMode,
	}, nil
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
