package porcelain

import (
	"context"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
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
		maxUserBytes := types.NewBytesAmount(ffi.GetMaxUserBytesPerStagedSector(sectorSize.Uint64()))
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
	values, err := plumbing.MessageQuery(ctx, address.Address{}, address.StorageMarketAddress, storagemarket.GetProofsMode, plumbing.ChainHeadKey())
	if err != nil {
		return 0, errors.Wrap(err, "'GetProofsMode' query message failed")
	}

	if err := encoding.Decode(values[0], &proofsMode); err != nil {
		return 0, errors.Wrap(err, "could not convert query message result to ProofsMode")
	}

	return proofsMode, nil
}

func getNetworkName(ctx context.Context, plumbing protocolParamsPlumbing) (string, error) {
	nameBytes, err := plumbing.MessageQuery(ctx, address.Address{}, address.InitAddress, initactor.GetNetworkMethodID, plumbing.ChainHeadKey())
	if err != nil {
		return "", errors.Wrap(err, "'getNetwork' query message failed")
	}

	return string(nameBytes[0]), nil
}
