package validation

import (
	"testing"

	"github.com/filecoin-project/chain-validation/suites"
)

func TestValueTransfer(t *testing.T) {
	factory := NewFactories()
	suites.TestValueTransferSimple(t, factory)
	suites.TestValueTransferAdvance(t, factory)

}

func TestMultiSig(t *testing.T) {
	factory := NewFactories()
	suites.TestMultiSigActor(t, factory)
}

func TestPaych(t *testing.T) {
	factory := NewFactories()
	suites.TestPaych(t, factory)
}

func TestActorCreation(t *testing.T) {
	factory := NewFactories()
	suites.TestAccountActorCreation(t, factory)
}

func TestInitActor(t *testing.T) {
	factory := NewFactories()
	suites.TestInitActorSequentialIDAddressCreate(t, factory)
}
