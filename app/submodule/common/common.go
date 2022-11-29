package common

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	apiwrapper "github.com/filecoin-project/venus/app/submodule/common/v0api"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/filecoin-project/venus/venus-shared/api/chain"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.ICommon = (*CommonModule)(nil)

type CommonModule struct { // nolint
	chainModule    *chain2.ChainSubmodule
	netModule      *network.NetworkSubmodule
	blockDelaySecs uint64
}

func NewCommonModule(chainModule *chain2.ChainSubmodule, netModule *network.NetworkSubmodule, blockDelaySecs uint64) *CommonModule {
	return &CommonModule{
		chainModule:    chainModule,
		netModule:      netModule,
		blockDelaySecs: blockDelaySecs,
	}
}

func (cm *CommonModule) Version(ctx context.Context) (types.Version, error) {
	return types.Version{
		Version:    constants.UserVersion(),
		APIVersion: chain.FullAPIVersion1,
	}, nil
}

func (cm *CommonModule) NodeStatus(ctx context.Context, inclChainStatus bool) (status types.NodeStatus, err error) {
	curTS, err := cm.chainModule.API().ChainHead(ctx)
	if err != nil {
		return status, err
	}

	status.SyncStatus.Epoch = uint64(curTS.Height())
	timestamp := time.Unix(int64(curTS.MinTimestamp()), 0)
	delta := time.Since(timestamp).Seconds()
	status.SyncStatus.Behind = uint64(delta / float64(cm.blockDelaySecs))

	// get peers in the messages and blocks topics
	peersMsgs := make(map[peer.ID]struct{})
	peersBlocks := make(map[peer.ID]struct{})

	for _, p := range cm.netModule.Pubsub.ListPeers(types.MessageTopic(cm.netModule.NetworkName)) {
		peersMsgs[p] = struct{}{}
	}

	for _, p := range cm.netModule.Pubsub.ListPeers(types.BlockTopic(cm.netModule.NetworkName)) {
		peersBlocks[p] = struct{}{}
	}

	// get scores for all connected and recent peers
	scores, err := cm.netModule.API().NetPubsubScores(ctx)
	if err != nil {
		return status, err
	}

	for _, score := range scores {
		if score.Score.Score > net.PublishScoreThreshold {
			_, inMsgs := peersMsgs[score.ID]
			if inMsgs {
				status.PeerStatus.PeersToPublishMsgs++
			}

			_, inBlocks := peersBlocks[score.ID]
			if inBlocks {
				status.PeerStatus.PeersToPublishBlocks++
			}
		}
	}

	if inclChainStatus && status.SyncStatus.Epoch > uint64(constants.Finality) {
		blockCnt := 0
		ts := curTS

		for i := 0; i < 100; i++ {
			blockCnt += len(ts.Blocks())
			tsk := ts.Parents()
			ts, err = cm.chainModule.API().ChainGetTipSet(ctx, tsk)
			if err != nil {
				return status, err
			}
		}

		status.ChainStatus.BlocksPerTipsetLast100 = float64(blockCnt) / 100

		for i := 100; i < int(constants.Finality); i++ {
			blockCnt += len(ts.Blocks())
			tsk := ts.Parents()
			ts, err = cm.chainModule.API().ChainGetTipSet(ctx, tsk)
			if err != nil {
				return status, err
			}
		}

		status.ChainStatus.BlocksPerTipsetLastFinality = float64(blockCnt) / float64(constants.Finality)
	}

	return status, nil
}

func (cm *CommonModule) API() v1api.ICommon {
	return cm
}

func (cm *CommonModule) V0API() v0api.ICommon {
	return &apiwrapper.WrapperV1ICommon{ICommon: cm}
}
