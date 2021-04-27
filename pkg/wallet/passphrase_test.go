package wallet

import (
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/scrypt"

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

func TestScrypt(t *testing.T) {
	for n := uint8(14); n < 24; n++ {
		b := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = scrypt.Key([]byte("password"), []byte("salt"), 1<<n, 8, 1, 32)
			}
		})
		cost := b.T / time.Duration(b.N)
		t.Logf("N = 2^%d\t%dms\n", n, cost/time.Millisecond)
	}
}
