package validation

import (
	"testing"

	"github.com/filecoin-project/chain-validation/pkg/suites"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestValueTransfer(t *testing.T) {
	tf.UnitTest(t)

	factory := NewFactories()
	suites.AccountValueTransferSuccess(t, factory, 0)
	suites.AccountValueTransferZeroFunds(t, factory, 0)
	suites.AccountValueTransferOverBalanceNonZero(t, factory, 0)
	suites.AccountValueTransferOverBalanceZero(t, factory, 0)
	suites.AccountValueTransferToSelf(t, factory, 0)
	suites.AccountValueTransferFromKnownToUnknownAccount(t, factory, 0)
	suites.AccountValueTransferFromUnknownToKnownAccount(t, factory, 0)
	suites.AccountValueTransferFromUnknownToUnknownAccount(t, factory, 0)
}
