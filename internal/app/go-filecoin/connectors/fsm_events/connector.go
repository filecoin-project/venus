package fsmeventsconnector

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/chainsampler"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/util/fsm"
)

var log = logging.Logger("fsm_events") // nolint: deadcode

type FiniteStateMachineEventsConnector struct {
	scheduler *chainsampler.HeightThresholdScheduler
	tsp       chain.TipSetProvider
}

var _ fsm.Events = new(FiniteStateMachineEventsConnector)

func New(scheduler *chainsampler.HeightThresholdScheduler, tsp chain.TipSetProvider) FiniteStateMachineEventsConnector {
	return FiniteStateMachineEventsConnector{
		scheduler: scheduler,
		tsp:       tsp,
	}
}

func (f FiniteStateMachineEventsConnector) ChainAt(hnd fsm.HeightHandler, rev fsm.RevertHandler, confidence int, h abi.ChainEpoch) error {
	// wait for an epoch past the target that gives us some confidence it won't reorg
	l := f.scheduler.AddListener(h + abi.ChainEpoch(confidence))

	ctx := context.Background()

	go func() {
		var handledToken fsm.TipSetToken
		for {
			select {
			case <-l.DoneCh:
				return
			case err := <-l.ErrCh:
				log.Warn(err)
				return
			case tsk := <-l.HitCh:
				ts, err := f.tsp.GetTipSet(tsk)
				if err != nil {
					log.Error(err)
					return
				}

				targetTipset, err := chain.FindTipsetAtEpoch(ctx, ts, h, f.tsp)
				if err != nil {
					log.Error(err)
					return
				}

				handledToken, err := encoding.Encode(targetTipset.Key())
				if err != nil {
					log.Error(err)
					return
				}

				sampleHeight, err := targetTipset.Height()
				if err != nil {
					log.Error(err)
					return
				}
				err = hnd(ctx, handledToken, sampleHeight)
				if err != nil {
					log.Error(err)
					return
				}
			case <-l.InvalidCh:
				err := rev(ctx, handledToken)
				if err != nil {
					log.Error(err)
					return
				}
			}
		}
	}()
	return nil
}
