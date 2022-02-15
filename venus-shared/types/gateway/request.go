package gateway

import (
	"github.com/google/uuid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	sharedTypes "github.com/filecoin-project/venus/venus-shared/types"
)

type ComputeProofRequest struct {
	SectorInfos []builtin.ExtendedSectorInfo
	Rand        abi.PoStRandomness
	Height      abi.ChainEpoch
	NWVersion   network.Version
}

type ConnectedCompleted struct {
	ChannelId uuid.UUID // nolint
}

type WalletSignRequest struct {
	Signer address.Address
	ToSign []byte
	Meta   sharedTypes.MsgMeta
}
