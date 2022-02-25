package gateway

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IProofEvent interface {
	IProofClient
	IProofServiceProvider
}

type IProofClient interface {
	ListConnectedMiners(ctx context.Context) ([]address.Address, error)                                                                                                                                        //perm:admin
	ListMinerConnection(ctx context.Context, addr address.Address) (*gtypes.MinerState, error)                                                                                                                 //perm:admin
	ComputeProof(ctx context.Context, miner address.Address, sectorInfos []builtin.ExtendedSectorInfo, rand abi.PoStRandomness, height abi.ChainEpoch, nwVersion network.Version) ([]builtin.PoStProof, error) //perm:admin

}
type IProofServiceProvider interface {
	ResponseProofEvent(ctx context.Context, resp *gtypes.ResponseEvent) error                                      //perm:read
	ListenProofEvent(ctx context.Context, policy *gtypes.ProofRegisterPolicy) (<-chan *gtypes.RequestEvent, error) //perm:read
}
