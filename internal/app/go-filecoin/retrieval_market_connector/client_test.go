package retrievalmarketconnector_test

import (
	"testing"
)

func TestNewRetrievalClientNodeConnector(t *testing.T) {}

func TestRetrievalClientNodeConnector_GetOrCreatePaymentChannel(t *testing.T) {
	t.Run("Errors if clientWallet addr is invalid", func(t *testing.T) {})

	t.Run("if the payment channel must be created", func(t *testing.T) {
		t.Run("creates a new payment channel registry entry", func(t *testing.T) {})
		t.Run("new payment channel message is posted on chain", func(t *testing.T) {})
		t.Run("returns address.Undef and nil", func(t *testing.T) {})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {})
		t.Run("Errors if minerWallet addr is invalid", func(t *testing.T) {})
		t.Run("Errors if cant get head tipset", func(t *testing.T) {})
		t.Run("Errors if cant get chain height", func(t *testing.T) {})
	})

	t.Run("if payment channel exists", func(t *testing.T) {
		t.Run("Retrieves existing payment channel address", func(t *testing.T) {})
		t.Run("Returns a payment channel type of address and nil", func(t *testing.T){})
	})

	t.Run("When message posts to chain", func(t *testing.T){
		t.Run("results added to payment channel registry", func(t *testing.T) {})
		t.Run("sets payment channel id in registry", func(t *testing.T) {})
		t.Run("sets error in registry if receipt can't be deserialized", func(t *testing.T) {})
		t.Run("sets error in registry if address in message is not in the registry", func(t *testing.T) {})
	})
}


func TestRetrievalClientNodeConnector_AllocateLane(t *testing.T) {
	t.Run("Errors if payment channel does not exist", func(t *testing.T){})
	t.Run("Increments and returns lastLane val", func(t *testing.T){})
}

func TestRetrievalClientNodeConnector_CreatePaymentVoucher(t *testing.T) {
	t.Run("Errors if payment channel does not exist", func(t *testing.T){})

}

