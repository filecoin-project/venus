package messagepool

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/ipfs/go-datastore"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/messagepool/gasguess"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestRepubMessages(t *testing.T) {
	tf.UnitTest(t)

	oldRepublishBatchDelay := RepublishBatchDelay
	RepublishBatchDelay = time.Microsecond
	defer func() {
		RepublishBatchDelay = oldRepublishBatchDelay
	}()

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, config.DefaultMessagePoolParam, "mptest", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	w1 := newWallet(t)
	a1, err := w1.NewAddress(address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	w2 := newWallet(t)
	a2, err := w2.NewAddress(address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]

	tma.setBalance(a1, 1) // in FIL

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(context.TODO(), m)
		if err != nil {
			t.Fatal(err)
		}
	}

	if tma.published != 10 {
		t.Fatalf("expected to have published 10 messages, but got %d instead", tma.published)
	}

	mp.repubTrigger <- struct{}{}
	time.Sleep(100 * time.Millisecond)

	if tma.published != 20 {
		t.Fatalf("expected to have published 20 messages, but got %d instead", tma.published)
	}
}
