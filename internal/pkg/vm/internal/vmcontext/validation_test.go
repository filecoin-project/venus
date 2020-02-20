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

	// Skipping as this tests uses the payment channel actor code ID and it DNE in go-filecoin
	// vsuites.TestInitActorSequentialIDAddressCreate(t, f)

	// Skipping since multisig actor DNE in go-filecoin
	// vsuites.TestMultiSigActor(t, f)

	// Skipping since payment channel actor DNE in go-filecoin
	// vsuites.TestPaych(t, f)
}
