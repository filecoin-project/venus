package net

import (
	"context"
	"net"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	blake2b "github.com/minio/blake2b-simd"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var gossipsubLog = logging.Logger("gossipsub")

func init() {
	// configure larger overlay parameters
	pubsub.GossipSubD = 8
	pubsub.GossipSubDscore = 6
	pubsub.GossipSubDout = 3
	pubsub.GossipSubDlo = 6
	pubsub.GossipSubDhi = 12
	pubsub.GossipSubDlazy = 12
	pubsub.GossipSubDirectConnectInitialDelay = 30 * time.Second
	pubsub.GossipSubIWantFollowupTime = 5 * time.Second
	pubsub.GossipSubHistoryLength = 10
	pubsub.GossipSubGossipFactor = 0.1
}

const (
	GossipScoreThreshold             = -500
	PublishScoreThreshold            = -1000
	GraylistScoreThreshold           = -2500
	AcceptPXScoreThreshold           = 1000
	OpportunisticGraftScoreThreshold = 3.5
)

func NewGossipSub(ctx context.Context,
	h host.Host,
	sk *ScoreKeeper,
	networkName string,
	drandSchedule map[abi.ChainEpoch]config.DrandEnum,
	bootNodes []peer.AddrInfo,
) (*pubsub.PubSub, error) {
	bootstrappers := make(map[peer.ID]struct{})
	for _, info := range bootNodes {
		bootstrappers[info.ID] = struct{}{}
	}

	blockTopic := types.BlockTopic(networkName)
	msgTopic := types.MessageTopic(networkName)
	indexerIngestTopic := types.IndexerIngestTopic(networkName)

	topicParams := map[string]*pubsub.TopicScoreParams{
		blockTopic: {
			// expected 10 blocks/min
			TopicWeight: 0.1, // max cap is 50, max mesh penalty is -10, single invalid message is -100

			// 1 tick per second, maxes at 1 after 1 hour
			TimeInMeshWeight:  0.00027, // ~1/3600
			TimeInMeshQuantum: time.Second,
			TimeInMeshCap:     1,

			// deliveries decay after 1 hour, cap at 100 blocks
			FirstMessageDeliveriesWeight: 5, // max value is 500
			FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
			FirstMessageDeliveriesCap:    100, // 100 blocks in an hour

			// Mesh Delivery Failure is currently turned off for blocks
			// This is on purpose as
			// - the traffic is very low for meaningful distribution of incoming edges.
			// - the reaction time needs to be very slow -- in the order of 10 min at least
			//   so we might as well let opportunistic grafting repair the mesh on its own
			//   pace.
			// - the network is too small, so large asymmetries can be expected between mesh
			//   edges.
			// We should revisit this once the network grows.
			//
			// // tracks deliveries in the last minute
			// // penalty activates at 1 minute and expects ~0.4 blocks
			// MeshMessageDeliveriesWeight:     -576, // max penalty is -100
			// MeshMessageDeliveriesDecay:      pubsub.ScoreParameterDecay(time.Minute),
			// MeshMessageDeliveriesCap:        10,      // 10 blocks in a minute
			// MeshMessageDeliveriesThreshold:  0.41666, // 10/12/2 blocks/min
			// MeshMessageDeliveriesWindow:     10 * time.Millisecond,
			// MeshMessageDeliveriesActivation: time.Minute,
			//
			// // decays after 15 min
			// MeshFailurePenaltyWeight: -576,
			// MeshFailurePenaltyDecay:  pubsub.ScoreParameterDecay(15 * time.Minute),

			// invalid messages decay after 1 hour
			InvalidMessageDeliveriesWeight: -1000,
			InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
		},
		msgTopic: {
			// expected > 1 tx/second
			TopicWeight: 0.1, // max cap is 5, single invalid message is -100

			// 1 tick per second, maxes at 1 hour
			TimeInMeshWeight:  0.0002778, // ~1/3600
			TimeInMeshQuantum: time.Second,
			TimeInMeshCap:     1,

			// deliveries decay after 10min, cap at 100 tx
			FirstMessageDeliveriesWeight: 0.5, // max value is 50
			FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(10 * time.Minute),
			FirstMessageDeliveriesCap:    100, // 100 messages in 10 minutes

			// Mesh Delivery Failure is currently turned off for messages
			// This is on purpose as the network is still too small, which results in
			// asymmetries and potential unmeshing from negative scores.
			// // tracks deliveries in the last minute
			// // penalty activates at 1 min and expects 2.5 txs
			// MeshMessageDeliveriesWeight:     -16, // max penalty is -100
			// MeshMessageDeliveriesDecay:      pubsub.ScoreParameterDecay(time.Minute),
			// MeshMessageDeliveriesCap:        100, // 100 txs in a minute
			// MeshMessageDeliveriesThreshold:  2.5, // 60/12/2 txs/minute
			// MeshMessageDeliveriesWindow:     10 * time.Millisecond,
			// MeshMessageDeliveriesActivation: time.Minute,

			// // decays after 5min
			// MeshFailurePenaltyWeight: -16,
			// MeshFailurePenaltyDecay:  pubsub.ScoreParameterDecay(5 * time.Minute),

			// invalid messages decay after 1 hour
			InvalidMessageDeliveriesWeight: -1000,
			InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
		},
	}

	pgTopicWeights := map[string]float64{
		blockTopic: 10,
		msgTopic:   1,
	}

	ingestTopicParams := &pubsub.TopicScoreParams{
		// expected ~0.5 confirmed deals / min. sampled
		TopicWeight: 0.1,

		TimeInMeshWeight:  0.00027, // ~1/3600
		TimeInMeshQuantum: time.Second,
		TimeInMeshCap:     1,

		FirstMessageDeliveriesWeight: 0.5,
		FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
		FirstMessageDeliveriesCap:    100, // allowing for burstiness

		InvalidMessageDeliveriesWeight: -1000,
		InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
	}

	drandTopicParams := &pubsub.TopicScoreParams{
		// expected 2 beaconsn/min
		TopicWeight: 0.5, // 5x block topic; max cap is 62.5

		// 1 tick per second, maxes at 1 after 1 hour
		TimeInMeshWeight:  0.00027, // ~1/3600
		TimeInMeshQuantum: time.Second,
		TimeInMeshCap:     1,

		// deliveries decay after 1 hour, cap at 25 beacons
		FirstMessageDeliveriesWeight: 5, // max value is 125
		FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
		FirstMessageDeliveriesCap:    25, // the maximum expected in an hour is ~26, including the decay

		// Mesh Delivery Failure is currently turned off for beacons
		// This is on purpose as
		// - the traffic is very low for meaningful distribution of incoming edges.
		// - the reaction time needs to be very slow -- in the order of 10 min at least
		//   so we might as well let opportunistic grafting repair the mesh on its own
		//   pace.
		// - the network is too small, so large asymmetries can be expected between mesh
		//   edges.
		// We should revisit this once the network grows.

		// invalid messages decay after 1 hour
		InvalidMessageDeliveriesWeight: -1000,
		InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
	}

	var drandTopics []string
	drandBootstrappers := make(map[peer.ID]struct{})
	for _, drandEnum := range drandSchedule {
		df := config.DrandConfigs[drandEnum]
		addrInfo, _ := parseDrandBootstrap(df)
		for _, info := range addrInfo {
			drandBootstrappers[info.ID] = struct{}{}
		}

		topic, err := types.DrandTopic(df.ChainInfoJSON)
		if err != nil {
			return nil, err
		}
		topicParams[topic] = drandTopicParams
		pgTopicWeights[topic] = 5
		drandTopics = append(drandTopics, topic)
	}

	// Index ingestion whitelist
	topicParams[indexerIngestTopic] = ingestTopicParams

	isBootstrapNode := false

	// IP colocation whitelist
	var ipcoloWhitelist []*net.IPNet

	options := []pubsub.Option{
		// Gossipsubv1.1 configuration
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageIdFn(hashMsgId),
		pubsub.WithPeerScore(
			&pubsub.PeerScoreParams{
				AppSpecificScore: func(p peer.ID) float64 {
					// return a heavy positive score for bootstrappers so that we don't unilaterally prune
					// them and accept PX from them.
					// we don't do that in the bootstrappers themselves to avoid creating a closed mesh
					// between them (however we might want to consider doing just that)
					_, ok := bootstrappers[p]
					if ok && !isBootstrapNode {
						return 2500
					}

					// todo:
					// _, ok = drandBootstrappers[p]
					// if ok && !isBootstrapNode {
					// 	return 1500
					// }

					// TODO: we want to  plug the application specific score to the node itself in order
					//       to provide feedback to the pubsub system based on observed behaviour
					return 0
				},
				AppSpecificWeight: 1,

				// This sets the IP colocation threshold to 5 peers before we apply penalties
				IPColocationFactorThreshold: 5,
				IPColocationFactorWeight:    -100,
				IPColocationFactorWhitelist: ipcoloWhitelist,

				// P7: behavioural penalties, decay after 1hr
				BehaviourPenaltyThreshold: 6,
				BehaviourPenaltyWeight:    -10,
				BehaviourPenaltyDecay:     pubsub.ScoreParameterDecay(time.Hour),

				DecayInterval: pubsub.DefaultDecayInterval,
				DecayToZero:   pubsub.DefaultDecayToZero,

				// this retains non-positive scores for 6 hours
				RetainScore: 6 * time.Hour,

				// topic parameters
				Topics: topicParams,
			},
			&pubsub.PeerScoreThresholds{
				GossipThreshold:             GossipScoreThreshold,
				PublishThreshold:            PublishScoreThreshold,
				GraylistThreshold:           GraylistScoreThreshold,
				AcceptPXThreshold:           AcceptPXScoreThreshold,
				OpportunisticGraftThreshold: OpportunisticGraftScoreThreshold,
			},
		),
		pubsub.WithPeerScoreInspect(sk.Update, 10*time.Second),
	}

	// enable Peer eXchange on bootstrappers
	if isBootstrapNode {
		// turn off the mesh in bootstrappers -- only do gossip and PX
		pubsub.GossipSubD = 0
		pubsub.GossipSubDscore = 0
		pubsub.GossipSubDlo = 0
		pubsub.GossipSubDhi = 0
		pubsub.GossipSubDout = 0
		pubsub.GossipSubDlazy = 64
		pubsub.GossipSubGossipFactor = 0.25
		pubsub.GossipSubPruneBackoff = 5 * time.Minute
		// turn on PX
		options = append(options, pubsub.WithPeerExchange(true))
	}

	// validation queue RED
	var pgParams *pubsub.PeerGaterParams

	if isBootstrapNode {
		pgParams = pubsub.NewPeerGaterParams(
			0.33,
			pubsub.ScoreParameterDecay(2*time.Minute),
			pubsub.ScoreParameterDecay(10*time.Minute),
		).WithTopicDeliveryWeights(pgTopicWeights)
	} else {
		pgParams = pubsub.NewPeerGaterParams(
			0.33,
			pubsub.ScoreParameterDecay(2*time.Minute),
			pubsub.ScoreParameterDecay(time.Hour),
		).WithTopicDeliveryWeights(pgTopicWeights)
	}

	options = append(options, pubsub.WithPeerGater(pgParams))

	allowTopics := []string{
		blockTopic,
		msgTopic,
		indexerIngestTopic,
	}
	allowTopics = append(allowTopics, drandTopics...)
	options = append(options,
		pubsub.WithSubscriptionFilter(
			pubsub.WrapLimitSubscriptionFilter(
				pubsub.NewAllowlistSubscriptionFilter(allowTopics...),
				100)))

	return pubsub.NewGossipSub(ctx, h, options...)
}

func parseDrandBootstrap(df config.DrandConf) ([]peer.AddrInfo, error) {
	// TODO: retry resolving, don't fail if at least one resolve succeeds
	addrs, err := ParseAddresses(context.TODO(), df.Relays)
	if err != nil {
		gossipsubLog.Errorf("reoslving drand relays addresses: %+v", err)
		return nil, nil
	}

	return addrs, nil
}

func hashMsgId(m *pubsub_pb.Message) string {
	hash := blake2b.Sum256(m.Data)
	return string(hash[:])
}
