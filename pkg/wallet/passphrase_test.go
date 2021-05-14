package wallet

import (
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/config"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestEncrypKeyAndDecryptKey(t *testing.T) {
	tf.UnitTest(t)

	ds := datastore.NewMapDatastore()
	defer func() {
		require.NoError(t, ds.Close())
	}()

	fs, err := NewDSBackend(ds, config.DefaultPassphraseConfig(), "")
	assert.NoError(t, err)

	w := New(fs)
	err = w.SetPassword(TestPassword)
	assert.NoError(t, err)

	ki, err := w.NewKeyInfo()
	assert.NoError(t, err)

	addr, err := ki.Address()
	assert.NoError(t, err)

	key := &Key{
		ID:      uuid.NewRandom(),
		Address: addr,
		KeyInfo: ki,
	}

	b, err := encryptKey(key, []byte(TestPassword), config.DefaultPassphraseConfig().ScryptN, config.DefaultPassphraseConfig().ScryptP)
	assert.NoError(t, err)

	key2, err := decryptKey(b, []byte(TestPassword))
	assert.NoError(t, err)

	assert.Equal(t, key, key2)
}
