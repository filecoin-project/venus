package porcelain

import (
	"context"
	"time"

	"github.com/filecoin-project/go-sectorbuilder"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// SectorInfo provides information about a sector construction
type SectorInfo struct {
	Size         *types.BytesAmount
	MaxPieceSize *types.BytesAmount
}

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	Network          string
	AutoSealInterval uint
	BlockTime        time.Duration
	ProofsMode       types.ProofsMode
	SupportedSectors []SectorInfo
}

type protocolParamsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	ChainHeadKey() types.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, baseKey types.TipSetKey, params ...interface{}) ([][]byte, error)
	BlockTime() time.Duration
}

// ProtocolParameters returns protocol parameter information about the node
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

	networkName, err := getNetworkName(ctx, plumbing)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve network name")
	}

	sectorSizes := []*types.BytesAmount{types.OneKiBSectorSize}
	if proofsMode == types.LiveProofsMode {
		sectorSizes[0] = types.TwoHundredFiftySixMiBSectorSize
	}

	supportedSectors := []SectorInfo{}
	for _, sectorSize := range sectorSizes {
		maxUserBytes := types.NewBytesAmount(go_sectorbuilder.GetMaxUserBytesPerStagedSector(sectorSize.Uint64()))
		supportedSectors = append(supportedSectors, SectorInfo{sectorSize, maxUserBytes})
	}

	return &ProtocolParams{
		Network:          networkName,
		AutoSealInterval: autoSealInterval,
		BlockTime:        plumbing.BlockTime(),
		ProofsMode:       proofsMode,
		SupportedSectors: supportedSectors,
	}, nil
}

// IsSupportedSectorSize returns true if the given sector size is supported by
// the network.
func (pp *ProtocolParams) IsSupportedSectorSize(sectorSize *types.BytesAmount) bool {
	for _, sector := range pp.SupportedSectors {
		if sector.Size.Equal(sectorSize) {
			return true
		}
	}

	return false
}

func getProofsMode(ctx context.Context, plumbing protocolParamsPlumbing) (types.ProofsMode, error) {
	var proofsMode types.ProofsMode
	values, err := plumbing.MessageQuery(ctx, address.Address{}, address.StorageMarketAddress, "getProofsMode", plumbing.ChainHeadKey())
	if err != nil {
		return 0, errors.Wrap(err, "'getProofsMode' query message failed")
	}

	if err := cbor.DecodeInto(values[0], &proofsMode); err != nil {
		return 0, errors.Wrap(err, "could not convert query message result to ProofsMode")
	}

	return proofsMode, nil
}

func getNetworkName(ctx context.Context, plumbing protocolParamsPlumbing) (string, error) {
	nameBytes, err := plumbing.MessageQuery(ctx, address.Address{}, address.InitAddress, "getNetwork", plumbing.ChainHeadKey())
	if err != nil {
		return "", errors.Wrap(err, "'getNetwork' query message failed")
	}

	return string(nameBytes[0]), nil
}
