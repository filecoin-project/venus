package wallet

import (
	"context"
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

	fs, err := NewDSBackend(context.Background(), ds, config.TestPassphraseConfig(), TestPassword)
	assert.NoError(t, err)

	w := New(fs)
	ctx := context.Background()
	ki, err := w.NewKeyInfo(ctx)
	assert.NoError(t, err)

	addr, err := ki.Address()
	assert.NoError(t, err)

	key := &Key{
		ID:      uuid.NewRandom(),
		Address: addr,
		KeyInfo: ki,
	}

	b, err := encryptKey(key, TestPassword, config.TestPassphraseConfig().ScryptN, config.TestPassphraseConfig().ScryptP)
	assert.NoError(t, err)

	key2, err := decryptKey(b, TestPassword)
	assert.NoError(t, err)

	assert.Equal(t, key.ID, key2.ID)
	assert.Equal(t, key.Address, key2.Address)
	assert.Equal(t, key.KeyInfo.Key(), key2.KeyInfo.Key())
}

func TestScrypt(t *testing.T) {
	t.Skipf("had test this too much, ignore this time!")
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
