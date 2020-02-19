package validation

import (
	vstate "github.com/filecoin-project/chain-validation/state"
)

type factories struct {
	*Applier
}

var _ vstate.Factories = &factories{}

func NewFactories() *factories {
	applier := NewApplier()
	return &factories{applier}
}

func (f *factories) NewState() vstate.Wrapper {
	return NewState()
}
