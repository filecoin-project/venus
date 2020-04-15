package fsmeventsconnector

import (
	"context"

	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
)

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

				confidentTipSet, err := chain.FindTipsetAtEpoch(ctx, f.tsp, ts, h-abi.ChainEpoch(confidence))
				if err != nil {
					log.Error(err)
					return
				}

				handledToken, err := encoding.Encode(confidentTipSet.Key())
				if err != nil {
					log.Error(err)
					return
				}

				actualHeight, err := ts.Height()
				if err != nil {
					log.Error(err)
					return
				}
				hnd(ctx, handledToken, actualHeight)
			case <-l.InvalidCh:
				rev(ctx, handledToken)
			}
		}
	}()
	return nil
}
