package vmcontext

import (
	"testing"

	vsuites "github.com/filecoin-project/chain-validation/suites"
)

func TestChainValidationSuite(t *testing.T) {
	f := NewFactories()

	vsuites.TestValueTransferSimple(t, f)
	vsuites.TestValueTransferAdvance(t, f)
	vsuites.TestAccountActorCreation(t, f)

	vsuites.TestInitActorSequentialIDAddressCreate(t, f)

	// Skipping since multisig address resolution breaks tests
	// https://github.com/filecoin-project/specs-actors/issues/184
	// vsuites.TestMultiSigActor(t, f)

	// Skipping since payment channel because runtime sys calls are not implemented in runtime adapter
	// vsuites.TestPaych(t, f)
}

func TestMessageApplication(t *testing.T) {
	f := NewFactories()

	vsuites.TestMessageApplicationEdgecases(t, f)
}

func TestTipSetApplication(t *testing.T) {
	f := NewFactories()

	vsuites.TestBlockMessageInfoApplication(t, f)
}
