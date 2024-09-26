package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestUpdateAddress(t *testing.T) {
	t.SkipNow()
	url := ""
	token := ""

	wg, err := NewWalletGateway(context.Background(), url, token)
	assert.NoError(t, err)
	var a address.Address
	for addr, accounts := range wg.addressAccount {
		fmt.Println(addr, accounts)
		a = addr
	}
	data := []byte("data to be signed")
	sig, err := wg.WalletSign(context.Background(), a, data, types.MsgMeta{
		Type: types.MTF3,
	})
	assert.NoError(t, err)
	fmt.Println(a, sig)

	assert.NoError(t, crypto.Verify(sig, a, data))
}
