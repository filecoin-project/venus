package messagepool

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/assert"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/sigs/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/sigs/secp"
	"github.com/filecoin-project/venus/pkg/messagepool/gasguess"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

func init() {
	_ = logging.SetLogLevel("*", "INFO")
}

type testMpoolAPI struct {
	cb func(rev, app []*types.TipSet) error

	bmsgs      map[cid.Cid][]*types.SignedMessage
	statenonce map[address.Address]uint64
	balance    map[address.Address]tbig.Int

	tipsets []*types.TipSet

	published int

	baseFee tbig.Int
}

func mkAddress(i uint64) address.Address {
	a, err := address.NewIDAddress(i)
	if err != nil {
		panic(err)
	}
	return a
}

func mkMessage(from, to address.Address, nonce uint64, w *wallet.Wallet) *types.SignedMessage {
	msg := &types.UnsignedMessage{
		To:         to,
		From:       from,
		Value:      tbig.NewInt(1),
		Nonce:      nonce,
		GasLimit:   1000000,
		GasFeeCap:  tbig.NewInt(100),
		GasPremium: tbig.NewInt(1),
	}

	c := msg.Cid()
	sig, err := w.WalletSign(from, c.Bytes(), wallet.MsgMeta{})
	if err != nil {
		panic(err)
	}
	return &types.SignedMessage{
		Message:   *msg,
		Signature: *sig,
	}
}

func mkBlock(parents *types.TipSet, weightInc int64, ticketNonce uint64) *types.BlockHeader {
	addr := mkAddress(123561)

	c, err := cid.Decode("bafyreicmaj5hhoy5mgqvamfhgexxyergw7hdeshizghodwkjg6qmpoco7i")
	if err != nil {
		panic(err)
	}

	pstateRoot := c
	if parents != nil {
		pstateRoot = parents.Blocks()[0].ParentStateRoot
	}

	var height abi.ChainEpoch
	var tsKey types.TipSetKey
	weight := tbig.NewInt(weightInc)
	var timestamp uint64
	if parents != nil {
		height = parents.Height()
		height = height + 1
		timestamp = parents.MinTimestamp() + constants.MainNetBlockDelaySecs
		weight = tbig.Add(parents.Blocks()[0].ParentWeight, weight)
		tsKey = parents.Key()
	}

	return &types.BlockHeader{
		Miner: addr,
		ElectionProof: &types.ElectionProof{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Ticket: types.Ticket{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Parents:               tsKey,
		ParentMessageReceipts: c,
		BLSAggregate:          &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentWeight:          weight,
		Messages:              c,
		Height:                height,
		Timestamp:             timestamp,
		ParentStateRoot:       pstateRoot,
		BlockSig:              &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentBaseFee:         tbig.NewInt(int64(constants.MinimumBaseFee)),
	}
}

func mkTipSet(blks ...*types.BlockHeader) *types.TipSet {
	ts, err := types.NewTipSet(blks...)
	if err != nil {
		panic(err)
	}
	return ts
}

func newTestMpoolAPI() *testMpoolAPI {
	tma := &testMpoolAPI{
		bmsgs:      make(map[cid.Cid][]*types.SignedMessage),
		statenonce: make(map[address.Address]uint64),
		balance:    make(map[address.Address]tbig.Int),
		baseFee:    tbig.NewInt(100),
	}
	genesis := mkBlock(nil, 1, 1)
	tma.tipsets = append(tma.tipsets, mkTipSet(genesis))
	return tma
}

func (tma *testMpoolAPI) nextBlock() *types.BlockHeader {
	newBlk := mkBlock(tma.tipsets[len(tma.tipsets)-1], 1, 1)
	tma.tipsets = append(tma.tipsets, mkTipSet(newBlk))
	return newBlk
}

func (tma *testMpoolAPI) nextBlockWithHeight(height uint64) *types.BlockHeader {
	newBlk := mkBlock(tma.tipsets[len(tma.tipsets)-1], 1, 1)
	newBlk.Height = abi.ChainEpoch(height)
	tma.tipsets = append(tma.tipsets, mkTipSet(newBlk))
	return newBlk
}

func (tma *testMpoolAPI) applyBlock(t *testing.T, b *types.BlockHeader) {
	t.Helper()
	if err := tma.cb(nil, []*types.TipSet{mkTipSet(b)}); err != nil {
		t.Fatal(err)
	}
}

func (tma *testMpoolAPI) revertBlock(t *testing.T, b *types.BlockHeader) {
	t.Helper()
	if err := tma.cb([]*types.TipSet{mkTipSet(b)}, nil); err != nil {
		t.Fatal(err)
	}
}

func (tma *testMpoolAPI) setStateNonce(addr address.Address, v uint64) {
	tma.statenonce[addr] = v
}

func (tma *testMpoolAPI) setBalance(addr address.Address, v uint64) {
	tma.balance[addr] = types.NewAttoFILFromFIL(v)
}

func (tma *testMpoolAPI) setBalanceRaw(addr address.Address, v tbig.Int) {
	tma.balance[addr] = v
}

func (tma *testMpoolAPI) setBlockMessages(h *types.BlockHeader, msgs ...*types.SignedMessage) {
	tma.bmsgs[h.Cid()] = msgs
}

func (tma *testMpoolAPI) ChainHead() (*types.TipSet, error) {
	return &types.TipSet{}, nil
}

func (tma *testMpoolAPI) ChainTipSet(key types.TipSetKey) (*types.TipSet, error) {
	return &types.TipSet{}, nil
}

func (tma *testMpoolAPI) SubscribeHeadChanges(cb func(rev, app []*types.TipSet) error) *types.TipSet {
	tma.cb = cb
	return tma.tipsets[0]
}

func (tma *testMpoolAPI) PutMessage(m types.ChainMsg) (cid.Cid, error) {
	return cid.Undef, nil
}

func (tma *testMpoolAPI) PubSubPublish(string, []byte) error {
	tma.published++
	return nil
}

func (tma *testMpoolAPI) GetActorAfter(addr address.Address, ts *types.TipSet) (*types.Actor, error) {
	// regression check for load bug
	if ts == nil {
		panic("GetActorAfter called with nil tipset")
	}

	balance, ok := tma.balance[addr]
	if !ok {
		balance = tbig.NewInt(1000e6)
		tma.balance[addr] = balance
	}

	msgs := make([]*types.SignedMessage, 0)
	for _, b := range ts.Blocks() {
		for _, m := range tma.bmsgs[b.Cid()] {
			if m.Message.From == addr {
				msgs = append(msgs, m)
			}
		}
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Message.Nonce < msgs[j].Message.Nonce
	})

	nonce := tma.statenonce[addr]

	for _, m := range msgs {
		if m.Message.Nonce != nonce {
			break
		}
		nonce++
	}

	return &types.Actor{
		Code:    builtin2.StorageMarketActorCodeID,
		Nonce:   nonce,
		Balance: balance,
	}, nil
}

func (tma *testMpoolAPI) StateAccountKey(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	if addr.Protocol() != address.BLS && addr.Protocol() != address.SECP256K1 {
		return address.Undef, fmt.Errorf("given address was not a key addr")
	}
	return addr, nil
}

func (tma *testMpoolAPI) MessagesForBlock(h *types.BlockHeader) ([]*types.UnsignedMessage, []*types.SignedMessage, error) {
	return nil, tma.bmsgs[h.Cid()], nil
}

func (tma *testMpoolAPI) MessagesForTipset(ts *types.TipSet) ([]types.ChainMsg, error) {
	if len(ts.Blocks()) != 1 {
		panic("cant deal with multiblock tipsets in this test")
	}

	bm, sm, err := tma.MessagesForBlock(ts.Blocks()[0])
	if err != nil {
		return nil, err
	}

	var out []types.ChainMsg
	for _, m := range bm {
		out = append(out, m)
	}

	for _, m := range sm {
		out = append(out, m)
	}

	return out, nil
}

func (tma *testMpoolAPI) LoadTipSet(tsk types.TipSetKey) (*types.TipSet, error) {
	for _, ts := range tma.tipsets {
		if tsk.Equals(ts.Key()) {
			return ts, nil
		}
	}

	return nil, fmt.Errorf("tipset not found")
}

func (tma *testMpoolAPI) ChainComputeBaseFee(ctx context.Context, ts *types.TipSet) (tbig.Int, error) {
	return tma.baseFee, nil
}

func assertNonce(t *testing.T, mp *MessagePool, addr address.Address, val uint64) {
	tf.UnitTest(t)

	t.Helper()
	n, err := mp.GetNonce(addr)
	if err != nil {
		t.Fatal(err)
	}

	if n != val {
		t.Fatalf("expected nonce of %d, got %d", val, n)
	}
}

func mustAdd(t *testing.T, mp *MessagePool, msg *types.SignedMessage) {
	tf.UnitTest(t)

	t.Helper()
	if err := mp.Add(msg); err != nil {
		t.Fatal(err)
	}
}

func newWalletAndMpool(t *testing.T, tma *testMpoolAPI) (*wallet.Wallet, *MessagePool) {
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	return newWallet(t), mp
}

func newWallet(t *testing.T) *wallet.Wallet {
	r := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(r.WalletDatastore(), r.Config().Wallet.PassphraseConfig, "")
	assert.NoError(t, err)

	w := wallet.New(backend)
	err = w.SetPassword(wallet.TestPassword)
	assert.NoError(t, err)

	return w
}

func TestMessagePool(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)

	a := tma.nextBlock()

	sender, err := w.NewAddress(address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}
	target := mkAddress(1001)

	var msgs []*types.SignedMessage
	for i := 0; i < 5; i++ {
		msgs = append(msgs, mkMessage(sender, target, uint64(i), w))
	}

	tma.setStateNonce(sender, 0)
	assertNonce(t, mp, sender, 0)
	mustAdd(t, mp, msgs[0])
	assertNonce(t, mp, sender, 1)
	mustAdd(t, mp, msgs[1])
	assertNonce(t, mp, sender, 2)

	tma.setBlockMessages(a, msgs[0], msgs[1])
	tma.applyBlock(t, a)

	assertNonce(t, mp, sender, 2)
}

func TestMessagePoolMessagesInEachBlock(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)

	a := tma.nextBlock()

	sender, err := w.NewAddress(address.BLS)
	if err != nil {
		t.Fatal(err)
	}
	target := mkAddress(1001)

	var msgs []*types.SignedMessage
	for i := 0; i < 5; i++ {
		m := mkMessage(sender, target, uint64(i), w)
		msgs = append(msgs, m)
		mustAdd(t, mp, m)
	}

	tma.setStateNonce(sender, 0)

	tma.setBlockMessages(a, msgs[0], msgs[1])
	tma.applyBlock(t, a)
	tsa := mkTipSet(a)

	_, _ = mp.Pending()

	selm, _ := mp.SelectMessages(tsa, 1)
	if len(selm) == 0 {
		t.Fatal("should have returned the rest of the messages")
	}
}

func TestRevertMessages(t *testing.T) {
	tf.UnitTest(t)

	futureDebug = true
	defer func() {
		futureDebug = false
	}()

	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)

	a := tma.nextBlock()
	b := tma.nextBlock()

	sender, err := w.NewAddress(address.BLS)
	if err != nil {
		t.Fatal(err)
	}
	target := mkAddress(1001)

	var msgs []*types.SignedMessage
	for i := 0; i < 5; i++ {
		msgs = append(msgs, mkMessage(sender, target, uint64(i), w))
	}

	tma.setBlockMessages(a, msgs[0])
	tma.setBlockMessages(b, msgs[1], msgs[2], msgs[3])

	mustAdd(t, mp, msgs[0])
	mustAdd(t, mp, msgs[1])
	mustAdd(t, mp, msgs[2])
	mustAdd(t, mp, msgs[3])

	tma.setStateNonce(sender, 0)
	tma.applyBlock(t, a)
	assertNonce(t, mp, sender, 4)

	tma.setStateNonce(sender, 1)
	tma.applyBlock(t, b)
	assertNonce(t, mp, sender, 4)
	tma.setStateNonce(sender, 0)
	tma.revertBlock(t, b)

	assertNonce(t, mp, sender, 4)

	p, _ := mp.Pending()
	fmt.Printf("%+v\n", p)
	if len(p) != 3 {
		t.Fatal("expected three messages in mempool")
	}

}

func TestPruningSimple(t *testing.T) {
	tf.UnitTest(t)

	oldMaxNonceGap := MaxNonceGap
	MaxNonceGap = 1000
	defer func() {
		MaxNonceGap = oldMaxNonceGap
	}()

	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)

	a := tma.nextBlock()
	tma.applyBlock(t, a)

	sender, err := w.NewAddress(address.BLS)
	if err != nil {
		t.Fatal(err)
	}
	tma.setBalance(sender, 1) // in FIL
	target := mkAddress(1001)

	for i := 0; i < 5; i++ {
		smsg := mkMessage(sender, target, uint64(i), w)
		if err := mp.Add(smsg); err != nil {
			t.Fatal(err)
		}
	}

	for i := 10; i < 50; i++ {
		smsg := mkMessage(sender, target, uint64(i), w)
		if err := mp.Add(smsg); err != nil {
			t.Fatal(err)
		}
	}

	mp.cfg.SizeLimitHigh = 40
	mp.cfg.SizeLimitLow = 10

	mp.Prune()

	msgs, _ := mp.Pending()
	if len(msgs) != 5 {
		t.Fatal("expected only 5 messages in pool, got: ", len(msgs))
	}
}

func TestLoadLocal(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
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

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL
	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	msgs := make(map[cid.Cid]struct{})
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		c, err := mp.Push(m)
		if err != nil {
			t.Fatal(err)
		}
		msgs[c] = struct{}{}
	}
	err = mp.Close()
	if err != nil {
		t.Fatal(err)
	}

	mp, err = New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	pmsgs, _ := mp.Pending()
	if len(msgs) != len(pmsgs) {
		t.Fatalf("expected %d messages, but got %d", len(msgs), len(pmsgs))
	}

	for _, m := range pmsgs {
		c := m.Cid()
		_, ok := msgs[c]
		if !ok {
			t.Fatal("unknown message")
		}

		delete(msgs, c)
	}

	if len(msgs) > 0 {
		t.Fatalf("not all messages were laoded; missing %d messages", len(msgs))
	}
}

func TestClearAll(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
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

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL
	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(m)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w2, a2, a1, uint64(i), gasLimit, uint64(i+1))
		mustAdd(t, mp, m)
	}

	mp.Clear(true)

	pending, _ := mp.Pending()
	if len(pending) > 0 {
		t.Fatalf("cleared the mpool, but got %d pending messages", len(pending))
	}
}

func TestClearNonLocal(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
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

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL

	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(m)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w2, a2, a1, uint64(i), gasLimit, uint64(i+1))
		mustAdd(t, mp, m)
	}

	mp.Clear(false)

	pending, _ := mp.Pending()
	if len(pending) != 10 {
		t.Fatalf("expected 10 pending messages, but got %d instead", len(pending))
	}

	for _, m := range pending {
		if m.Message.From != a1 {
			t.Fatalf("expected message from %s but got one from %s instead", a1, m.Message.From)
		}
	}
}

func TestUpdates(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(tma, ds, config.DefaultForkUpgradeParam, "mptest", nil, nil, nil)
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

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	ch, err := mp.Updates(ctx)
	if err != nil {
		t.Fatal(err)
	}

	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(m)
		if err != nil {
			t.Fatal(err)
		}

		_, ok := <-ch
		if !ok {
			t.Fatal("expected update, but got a closed channel instead")
		}
	}

	err = mp.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, ok := <-ch
	if ok {
		t.Fatal("expected closed channel, but got an update instead")
	}
}
