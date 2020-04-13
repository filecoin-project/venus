package fsmeventsconnector

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
)

type FiniteStateMachineEventsConnector struct{}

func (f FiniteStateMachineEventsConnector) ChainAt(hnd fsm.HeightHandler, rev fsm.RevertHandler, confidence int, h abi.ChainEpoch) error {
	panic("implement me")
}

var _ fsm.Events = new(FiniteStateMachineEventsConnector)

func New() FiniteStateMachineEventsConnector {
	return FiniteStateMachineEventsConnector{}
}
