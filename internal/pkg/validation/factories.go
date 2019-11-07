package validation

import (
	vchain "github.com/filecoin-project/chain-validation/pkg/chain"
	vstate "github.com/filecoin-project/chain-validation/pkg/state"
	"github.com/filecoin-project/chain-validation/pkg/suites"
)

type factories struct {
	*Applier
}

var _ suites.Factories = &factories{}

// NewFactories returns a factory.
func NewFactories() suites.Factories {
	return &factories{NewApplier()}
}

// NewState returns a state wrapper.
func (f *factories) NewState() vstate.Wrapper {
	return NewState()
}

// NewMessageFactory returns a message factory.
func (f *factories) NewMessageFactory(wrapper vstate.Wrapper) vchain.MessageFactory {
	signer := wrapper.(*StateWrapper).Signer()
	return NewMessageFactory(signer)
}
