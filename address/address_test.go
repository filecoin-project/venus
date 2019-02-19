package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/bls-signatures"
)

func TestEmptyAddress(t *testing.T) {
	assert := assert.New(t)

	var emptyAddr Address
	assert.True(emptyAddr.Empty())
	assert.Equal(Undef, emptyAddr)

	stuffAddr := Address{"stuff"}
	assert.False(stuffAddr.Empty())
	assert.NotEqual(Undef, stuffAddr)
}

func TestNewAddress(t *testing.T) {
	assert := assert.New(t)

	t.Run("New ID Address", func(t *testing.T) {
		idAddress, err := NewFromActorID(Testnet, uint64(1))
		assert.NoError(err)
		assert.Equal(Testnet, idAddress.Network())
		assert.Equal(ID, idAddress.Protocol())

	})

	t.Run("New Actor Address", func(t *testing.T) {
		actorAddress, err := NewFromActor(Testnet, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
		assert.NoError(err)
		assert.Equal(Testnet, actorAddress.Network())
		assert.Equal(Actor, actorAddress.Protocol())
	})

	t.Run("New BLS Address", func(t *testing.T) {
		blsAddress, err := NewFromBLS(Testnet, bls.PrivateKeyPublicKey((bls.PrivateKeyGenerate())))
		assert.NoError(err)
		assert.Equal(Testnet, blsAddress.Network())
		assert.Equal(BLS, blsAddress.Protocol())
	})

}

func TestAddressDecodeEncode(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	t.Run("Encode Decode ID Address", func(t *testing.T) {
		idAddress, err := NewFromActorID(Testnet, uint64(1))
		require.NoError(err)

		addrString := idAddress.String()

		addrFromString, err := NewFromString(addrString)
		assert.NoError(err)
		assert.Equal(idAddress.Bytes(), addrFromString.Bytes())
	})

	t.Run("Encode Decode Actor Address", func(t *testing.T) {
		actorAddress, err := NewFromActor(Testnet, Hash([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}))
		assert.NoError(err)

		addrString := actorAddress.String()

		addrFromString, err := NewFromString(addrString)
		assert.NoError(err)
		assert.Equal(actorAddress.Bytes(), addrFromString.Bytes())
	})

	t.Run("Encode Decode BLS Address", func(t *testing.T) {
		blsPk := bls.PrivateKeyPublicKey((bls.PrivateKeyGenerate()))
		blsAddress, err := NewFromBLS(Testnet, blsPk)
		assert.NoError(err)

		addrString := blsAddress.String()

		addrFromString, err := NewFromString(addrString)
		assert.NoError(err)
		assert.Equal(blsAddress.Bytes(), addrFromString.Bytes())

		// is the pubkey there valid?
		assert.Equal(blsAddress.Data(), blsPk[:])
	})
}
