package messagepool

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/repo"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/pkg/messagepool/gasguess"
	"github.com/filecoin-project/venus/pkg/wallet"
)

func TestRepubMessages(t *testing.T) {
	oldRepublishBatchDelay := RepublishBatchDelay
	RepublishBatchDelay = time.Microsecond
	defer func() {
		RepublishBatchDelay = oldRepublishBatchDelay
	}()

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, "mptest", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	r1 := repo.NewInMemoryRepo()
	backend1, err := wallet.NewDSBackend(r1.WalletDatastore())
	if err != nil {
		t.Fatal(err)
	}
	w1 := wallet.New(backend1)

	a1, err := wallet.NewAddress(w1, address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	r2 := repo.NewInMemoryRepo()
	backend2, err := wallet.NewDSBackend(r2.WalletDatastore())
	if err != nil {
		t.Fatal(err)
	}
	w2 := wallet.New(backend2)

	a2, err := wallet.NewAddress(w2, address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]

	tma.setBalance(a1, 1) // in FIL

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(m)
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
