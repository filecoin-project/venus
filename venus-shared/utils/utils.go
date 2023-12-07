package utils

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"

	acrypto "github.com/filecoin-project/go-state-types/crypto"
)

var NetworkNameWithNetworkType = map[types.NetworkName]types.NetworkType{
	types.NetworkNameMain:        types.NetworkMainnet,
	types.NetworkNameCalibration: types.NetworkCalibnet,
	types.NetworkNameButterfly:   types.NetworkButterfly,
	types.NetworkNameInterop:     types.NetworkInterop,
	types.NetworkNameIntegration: types.Integrationnet,
	types.NetworkNameForce:       types.NetworkForce,
}

var NetworkTypeWithNetworkName = func() map[types.NetworkType]types.NetworkName {
	typeName := make(map[types.NetworkType]types.NetworkName, len(NetworkNameWithNetworkType))
	for nt, nn := range NetworkNameWithNetworkType {
		typeName[nn] = nt
	}

	return typeName
}()

func NetworkNameToNetworkType(networkName types.NetworkName) (types.NetworkType, error) {
	if len(networkName) == 0 {
		return types.NetworkDefault, fmt.Errorf("network name is empty")
	}
	nt, ok := NetworkNameWithNetworkType[networkName]
	if ok {
		return nt, nil
	}
	// 2k network do not have exact network names
	return types.Network2k, nil
}

func NetworkTypeToNetworkName(networkType types.NetworkType) types.NetworkName {
	nn, ok := NetworkTypeWithNetworkName[networkType]
	if ok {
		return nn
	}

	// 2k network do not have exact network names
	return ""
}

type networkNameGetter interface {
	StateNetworkName(ctx context.Context) (types.NetworkName, error)
}

func LoadBuiltinActors(ctx context.Context, getter networkNameGetter) error {
	networkName, err := getter.StateNetworkName(ctx)
	if err != nil {
		return err
	}
	nt, err := NetworkNameToNetworkType(networkName)
	if err != nil {
		return err
	}
	if err := actors.SetNetworkBundle(int(nt)); err != nil {
		return err
	}
	ReloadMethodsMap()

	return nil
}

// Name returns the name of a variable whose type does not have a default implementation of the String() method.
func Name(a interface{}) string {
	switch v := a.(type) {
	case acrypto.DomainSeparationTag:
		switch v {
		case acrypto.DomainSeparationTag_TicketProduction:
			return "TicketProduction"
		case acrypto.DomainSeparationTag_ElectionProofProduction:
			return "ElectionProofProduction"
		case acrypto.DomainSeparationTag_WinningPoStChallengeSeed:
			return "WinningPoStChallengeSeed"
		case acrypto.DomainSeparationTag_WindowedPoStChallengeSeed:
			return "WindowedPoStChallengeSeed"
		case acrypto.DomainSeparationTag_SealRandomness:
			return "SealRandomness"
		case acrypto.DomainSeparationTag_InteractiveSealChallengeSeed:
			return "InteractiveSealChallengeSeed"
		case acrypto.DomainSeparationTag_WindowedPoStDeadlineAssignment:
			return "WindowedPoStDeadlineAssignment"
		case acrypto.DomainSeparationTag_MarketDealCronSeed:
			return "MarketDealCronSeed"
		case acrypto.DomainSeparationTag_PoStChainCommit:
			return "PoStChainCommit"
		default:
			return ""
		}

	default:
		return ""
	}
}
