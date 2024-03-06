package actorevent

import (
	"context"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/eth"
	"github.com/filecoin-project/venus/pkg/config"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type ActorEventSubModule struct {
	chainModule       *chain.ChainSubmodule
	ethModule         *eth.EthSubModule
	cfg               *config.Config
	actorEventHandler v1api.IActorEvent
}

func NewActorEventSubModule(ctx context.Context,
	cfg *config.Config,
	chainModule *chain.ChainSubmodule,
	ethModule *eth.EthSubModule,
) (*ActorEventSubModule, error) {
	aem := &ActorEventSubModule{
		cfg:               cfg,
		ethModule:         ethModule,
		chainModule:       chainModule,
		actorEventHandler: &ActorEventDummy{},
	}

	if !cfg.EventsConfig.EnableActorEventsAPI {
		return aem, nil
	}

	fm := ethModule.GetEventFilterManager()
	if cfg.FevmConfig.Event.DisableRealTimeFilterAPI {
		fm = nil
	}

	netParams, err := chainModule.API().StateGetNetworkParams(ctx)
	if err != nil {
		return nil, err
	}

	aem.actorEventHandler = NewActorEventHandler(
		chainModule.ChainReader,
		fm,
		time.Duration(netParams.BlockDelaySecs)*time.Second,
		abi.ChainEpoch(cfg.FevmConfig.Event.MaxFilterHeightRange),
	)

	return aem, nil
}

func (aem *ActorEventSubModule) API() v1api.IActorEvent {
	return aem.actorEventHandler
}
