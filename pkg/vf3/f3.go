package vf3

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/filecoin-project/go-f3"
	"github.com/filecoin-project/go-f3/blssig"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/wallet"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log"
)

type F3 struct {
	inner *f3.F3
	ec    *ecWrapper

	signer gpbft.Signer
	leaser *leaser

	stopFunc func()
}

func init() {
	// Set up otel to prometheus reporting so that F3 metrics are reported via lotus
	// prometheus metrics. This bridge eventually gets picked up by opencensus
	// exporter as HTTP handler. This by default registers an otel collector against
	// the global prometheus registry. In the future, we should clean up metrics in
	// Lotus and move it all to use otel. For now, bridge away.
	if bridge, err := prometheus.New(); err != nil {
		log.Errorf("could not create the otel prometheus exporter: %v", err)
	} else {
		provider := metric.NewMeterProvider(metric.WithReader(bridge))
		otel.SetMeterProvider(provider)
	}
}

type F3Params struct {
	PubSub       *pubsub.PubSub
	Host         host.Host
	ChainStore   *chain.Store
	StateManager *statemanger.Stmgr
	Datastore    datastore.Batching
	WalletSign   wallet.WalletSignFunc
	SyncerAPI    v1api.ISyncer
	Config       *Config
	RepoPath     string

	Net v1api.INetwork
}

var log = logging.Logger("f3")

func New(mctx context.Context, params F3Params) (*F3, error) {
	if params.Config.StaticManifest == nil {
		return nil, fmt.Errorf("configuration invalid, nil StaticManifest in the Config")
	}
	ds := namespace.Wrap(params.Datastore, datastore.NewKey("/f3"))
	ec := &ecWrapper{
		ChainStore:   params.ChainStore,
		StateManager: params.StateManager,
		SyncerAPI:    params.SyncerAPI,
	}
	verif := blssig.VerifierWithKeyOnG1()

	f3FsPath := filepath.Join(params.RepoPath, "f3")
	module, err := f3.New(mctx, *params.Config.StaticManifest, ds,
		params.Host, params.PubSub, verif, ec, f3FsPath)

	if err != nil {
		return nil, fmt.Errorf("creating F3: %w", err)
	}

	nodeID, err := params.Net.ID(mctx)
	if err != nil {
		return nil, fmt.Errorf("getting node ID: %w", err)
	}

	// maxLeasableInstances is the maximum number of leased F3 instances this node
	// would give out.
	const maxLeasableInstances = 5
	status := func() (manifest.Manifest, gpbft.InstanceProgress) {
		return module.Manifest(), module.Progress()
	}

	fff := &F3{
		inner:  module,
		ec:     ec,
		signer: &signer{sign: params.WalletSign},
		leaser: newParticipationLeaser(nodeID, status, maxLeasableInstances),
	}

	go func() {
		err := fff.inner.Start(mctx)
		if err != nil {
			log.Fatalf("running f3: %+v", err)
			return
		}
	}()

	// Start signing F3 messages.
	lCtx, cancel := context.WithCancel(mctx)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		fff.runSigningLoop(lCtx)
	}()

	fff.stopFunc = func() {
		cancel()
		<-doneCh
	}

	return fff, nil
}

func (fff *F3) Stop(ctx context.Context) error {
	if err := fff.inner.Stop(ctx); err != nil {
		return err
	}

	fff.stopFunc()

	return nil
}

func (fff *F3) runSigningLoop(ctx context.Context) {
	participateOnce := func(ctx context.Context, mb *gpbft.MessageBuilder, minerID uint64) error {
		signatureBuilder, err := mb.PrepareSigningInputs(gpbft.ActorID(minerID))
		if errors.Is(err, gpbft.ErrNoPower) {
			// we don't have any power in F3, continue
			log.Debugf("no power to participate in F3: %+v", err)
			return nil
		}
		if err != nil {
			return fmt.Errorf("preparing signing inputs: %+v", err)
		}
		// if worker keys were stored not in the node, the signatureBuilder can be send there
		// the sign can be called where the keys are stored and then
		// {signatureBuilder, payloadSig, vrfSig} can be sent back to lotus for broadcast
		payloadSig, vrfSig, err := signatureBuilder.Sign(ctx, fff.signer)
		if err != nil {
			return fmt.Errorf("signing message: %+v", err)
		}
		log.Debugf("miner with id %d is sending message in F3", minerID)
		fff.inner.Broadcast(ctx, signatureBuilder, payloadSig, vrfSig)
		return nil
	}

	msgCh := fff.inner.MessagesToSign()

	var mb *gpbft.MessageBuilder
	alreadyParticipated := make(map[uint64]struct{})
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-fff.leaser.notifyParticipation:
			if mb == nil {
				continue
			}
		case mb = <-msgCh: // never closed
			clear(alreadyParticipated)
		}

		participants := fff.leaser.getParticipantsByInstance(mb.NetworkName, mb.Payload.Instance)
		for _, id := range participants {
			if _, ok := alreadyParticipated[id]; ok {
				continue
			} else if err := participateOnce(ctx, mb, id); err != nil {
				log.Errorf("while participating for miner f0%d: %+v", id, err)
			} else {
				alreadyParticipated[id] = struct{}{}
			}
		}
	}
}

func (fff *F3) GetOrRenewParticipationTicket(_ context.Context, minerID uint64, previous types.F3ParticipationTicket, instances uint64) (types.F3ParticipationTicket, error) {
	return fff.leaser.getOrRenewParticipationTicket(minerID, previous, instances)
}

func (fff *F3) Participate(_ context.Context, ticket types.F3ParticipationTicket) (types.F3ParticipationLease, error) {
	return fff.leaser.participate(ticket)
}

func (fff *F3) GetCert(ctx context.Context, instance uint64) (*certs.FinalityCertificate, error) {
	return fff.inner.GetCert(ctx, instance)
}

func (fff *F3) GetLatestCert(ctx context.Context) (*certs.FinalityCertificate, error) {
	return fff.inner.GetLatestCert(ctx)
}

func (fff *F3) GetManifest(ctx context.Context) (*manifest.Manifest, error) {
	m := fff.inner.Manifest()
	if m.InitialPowerTable.Defined() {
		return &m, nil
	}
	cert0, err := fff.inner.GetCert(ctx, 0)
	if err != nil {
		return &m, nil // return manifest without power table
	}

	m.InitialPowerTable = cert0.ECChain.Base().PowerTable
	return &m, nil
}

func (fff *F3) GetPowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) {
	return fff.ec.getPowerTableTSK(ctx, tsk)
}

func (fff *F3) GetF3PowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) {
	return fff.inner.GetPowerTable(ctx, tsk.Bytes())
}

func (fff *F3) IsRunning() bool {
	return fff.inner.IsRunning()
}

func (fff *F3) Progress() gpbft.InstanceProgress {
	return fff.inner.Progress()
}

func (fff *F3) ListParticipants() []types.F3Participant {
	leases := fff.leaser.getValidLeases()
	participants := make([]types.F3Participant, len(leases))
	for i, lease := range leases {
		participants[i] = types.F3Participant{
			MinerID:      lease.MinerID,
			FromInstance: lease.FromInstance,
			ValidityTerm: lease.ValidityTerm,
		}
	}
	return participants
}
