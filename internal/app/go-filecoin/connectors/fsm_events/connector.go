package fsmeventsconnector

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
)

type FiniteStateMachineEventsConnector struct {
	scheduler chainsampler.HeightThresholdScheduler
	tsp       chain.TipSetProvider
}

func (f FiniteStateMachineEventsConnector) ChainAt(hnd fsm.HeightHandler, rev fsm.RevertHandler, confidence int, h abi.ChainEpoch) error {
	l := f.scheduler.AddListener(h + abi.ChainEpoch(confidence))

	ctx := context.Background()

	var handledToken fsm.TipSetToken

	for {
		select {
		case <-l.DoneCh:
			return nil
		case err := <-l.ErrCh:
			return err
		case tsk := <-l.HitCh:
			ts, err := f.tsp.GetTipSet(tsk)
			if err != nil {
				return err
			}

			confidentTipSet, err := chain.FindTipsetAtEpoch(ctx, f.tsp, ts, h-abi.ChainEpoch(confidence))
			if err != nil {
				return err
			}

			handledToken, err := encoding.Encode(confidentTipSet.Key())
			if err != nil {
				return err
			}

			actualHeight, err := ts.Height()
			if err != nil {
				return err
			}
			hnd(ctx, handledToken, actualHeight)

		case <-l.InvalidCh:
			rev(ctx, handledToken)
		}
	}
}

func (f FiniteStateMachineEventsConnector) findTipsetAtEpoch(ctx context.Context, start block.TipSet, epoch abi.ChainEpoch) (ts block.TipSet, err error) {
	iterator := chain.IterAncestors(ctx, f.tsp, start)
	var h abi.ChainEpoch
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return
		}
		ts = iterator.Value()
		h, err = ts.Height()
		if err != nil {
			return
		}
		if h <= epoch {
			break
		}
	}
	// If the iterator completed, ts is the genesis tipset.
	return
}

var _ fsm.Events = new(FiniteStateMachineEventsConnector)

func New() FiniteStateMachineEventsConnector {
	return FiniteStateMachineEventsConnector{}
}
