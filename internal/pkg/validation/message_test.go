package validation

import (
	"math/big"
	"testing"

	"github.com/filecoin-project/chain-validation/pkg/chain"
	"github.com/filecoin-project/chain-validation/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestMessageFactory(t *testing.T) {
	gasPrice := big.NewInt(1)
	gasLimit := state.GasUnit(1000)

	signer, keys := types.NewMockSignersAndKeyInfo(1)
	factory := NewMessageFactory(signer)
	p := chain.NewMessageProducer(factory, gasLimit, gasPrice)

	sender, err := keys[0].Address()
	require.NoError(t, err)
	bfAddr := factory.FromSingletonAddress(state.BurntFundsAddress)
	m, err := p.Transfer(state.Address(sender.Bytes()), bfAddr, 0, 1)
	require.NoError(t, err)

	messages := p.Messages()
	assert.Equal(t, 1, len(messages))
	msg := m.(*types.SignedMessage)
	assert.Equal(t, m, msg)
	assert.Equal(t, sender, msg.Message.From)
	assert.Equal(t, address.BurntFundsAddress, msg.Message.To)
	assert.Equal(t, types.NewAttoFIL(big.NewInt(1)), msg.Message.Value)
}
