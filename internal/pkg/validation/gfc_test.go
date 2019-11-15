package validation

import (
	"testing"

	"github.com/filecoin-project/chain-validation/pkg/suites"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestValueTransfer(t *testing.T) {
	tf.UnitTest(t)

	factory := NewFactories()
	expectedGasUsed := uint64(0)
	suites.AccountValueTransferSuccess(t, factory, expectedGasUsed)
	suites.AccountValueTransferZeroFunds(t, factory, expectedGasUsed)
	suites.AccountValueTransferOverBalanceNonZero(t, factory, expectedGasUsed)
	suites.AccountValueTransferOverBalanceZero(t, factory, expectedGasUsed)
	suites.AccountValueTransferToSelf(t, factory, expectedGasUsed)
	suites.AccountValueTransferFromKnownToUnknownAccount(t, factory, expectedGasUsed)
	suites.AccountValueTransferFromUnknownToKnownAccount(t, factory, expectedGasUsed)
	suites.AccountValueTransferFromUnknownToUnknownAccount(t, factory, expectedGasUsed)
}
