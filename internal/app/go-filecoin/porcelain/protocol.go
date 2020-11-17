package porcelain

import (
	"context"
	"time"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/constants"
)

// SectorInfo provides information about a sector construction
type SectorInfo struct {
	Size         abi.SectorSize
	MaxPieceSize abi.UnpaddedPieceSize
}

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	Network          string
	BlockTime        time.Duration
	SupportedSectors []SectorInfo
}

type protocolParamsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	ChainHeadKey() block.TipSetKey
	ProtocolStateView(baseKey block.TipSetKey) (ProtocolStateView, error)
	BlockTime() time.Duration
}

type ProtocolStateView interface {
	InitNetworkName(ctx context.Context) (string, error)
}

// ProtocolParameters returns protocol parameter information about the node
func ProtocolParameters(ctx context.Context, plumbing protocolParamsPlumbing) (*ProtocolParams, error) {
	networkName, err := getNetworkName(ctx, plumbing)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve network name")
	}

	sectorSizes := []abi.SectorSize{constants.DevSectorSize, constants.FiveHundredTwelveMiBSectorSize}

	var supportedSectors []SectorInfo
	for _, sectorSize := range sectorSizes {
		maxUserBytes := abi.PaddedPieceSize(sectorSize).Unpadded()
		supportedSectors = append(supportedSectors, SectorInfo{sectorSize, maxUserBytes})
	}

	return &ProtocolParams{
		Network:          networkName,
		BlockTime:        plumbing.BlockTime(),
		SupportedSectors: supportedSectors,
	}, nil
}

func getNetworkName(ctx context.Context, plumbing protocolParamsPlumbing) (string, error) {
	view, err := plumbing.ProtocolStateView(plumbing.ChainHeadKey())
	if err != nil {
		return "", errors.Wrap(err, "failed to query state")
	}
	return view.InitNetworkName(ctx)
}
