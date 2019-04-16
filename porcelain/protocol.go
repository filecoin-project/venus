package porcelain

import (
	"github.com/pkg/errors"
)

type ProtocolParams struct {
	AutoSealInterval uint
}

type protocolParamsPlumbing interface {
	ConfigGet(string) (interface{}, error)
}

func ProtocolParameters(plumbing protocolParamsPlumbing) (*ProtocolParams, error) {
	autoSealIntervalInterface, err := plumbing.ConfigGet("mining.autoSealIntervalSeconds")
	if err != nil {
		return nil, err
	}

	autoSealInterval, ok := autoSealIntervalInterface.(uint)
	if !ok {
		return nil, errors.New("Failed to read autoSealInterval from config")
	}

	return &ProtocolParams{
		AutoSealInterval: autoSealInterval,
	}, nil
}
