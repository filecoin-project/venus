package chain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/go-address"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
)

var mockSigner, _ = testhelpers.NewMockSignersAndKeyInfo(10)

var newSignedMessage = testhelpers.NewSignedMessageForTestGetter(mockSigner)

func setupTest(t *testing.T) (cbor.IpldStore, *Store, *MessageStore, *Waiter) {
	builder := NewBuilder(t, address.Undef)
	waiter := NewWaiter(builder.store, builder.mstore, builder.bs, builder.cstore)
	waiter.Stmgr = builder.IStmgr()

	return builder.cstore, builder.store, builder.mstore, waiter
}

func TestWaitRespectsContextCancel(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	_, _, _, waiter := setupTest(t)

	var err error
	var chainMessage *types.ChainMessage
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		chainMessage, err = waiter.Wait(ctx, newSignedMessage(0), constants.DefaultConfidence, constants.DefaultMessageWaitLookback, true)
	}()

	cancel()

	select {
	case <-doneCh:
		fmt.Println(err)
		// assert.Error(t, err)
	case <-time.After(2 * time.Second):
		assert.Fail(t, "Wait should have returned when context was canceled")
	}
	assert.Nil(t, chainMessage)
}
