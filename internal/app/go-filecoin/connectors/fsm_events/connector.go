package fsmeventsconnector

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
)

type FiniteStateMachineEventsConnector struct {
	scheduler chainsampler.HeightThresholdScheduler
}

func (f FiniteStateMachineEventsConnector) ChainAt(hnd fsm.HeightHandler, rev fsm.RevertHandler, confidence int, h abi.ChainEpoch) error {
	l := f.scheduler.AddListener(h)
}

var _ fsm.Events = new(FiniteStateMachineEventsConnector)

func New() FiniteStateMachineEventsConnector {
	return FiniteStateMachineEventsConnector{}
}
