// stm: #unit
package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
)

type mockAccountView struct {
}

func (mac *mockAccountView) ResolveToKeyAddr(_ context.Context, addr address.Address) (address.Address, error) {
	return addr, nil
}

func TestSigner(t *testing.T) {
	ctx := context.Background()
	ds := datastore.NewMapDatastore()
	fs, err := wallet.NewDSBackend(ctx, ds, config.TestPassphraseConfig(), wallet.TestPassword)
	assert.NoError(t, err)
	wallet := wallet.New(fs)
	walletAddr, err := wallet.NewAddress(ctx, address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	signer := NewSigner(&mockAccountView{}, wallet)
	// stm: @STATE_VIEW_SIGN_BYTES_001
	_, err = signer.SignBytes(ctx, []byte("to sign data"), walletAddr)
	assert.NoError(t, err)

	// stm: @STATE_VIEW_HAS_ADDRESS_001
	has, err := signer.HasAddress(ctx, walletAddr)
	assert.NoError(t, err)
	assert.True(t, has)

}
