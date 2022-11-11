// stm: #unit
package messagepool

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/statemanger"

	_ "github.com/filecoin-project/venus/pkg/crypto/secp"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/assert"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/messagepool/gasguess"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/filecoin-project/venus/venus-shared/types"
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
	msg := &types.Message{
		To:         to,
		From:       from,
		Value:      tbig.NewInt(1),
		Nonce:      nonce,
		GasLimit:   1000000,
		GasFeeCap:  tbig.NewInt(100),
		GasPremium: tbig.NewInt(1),
	}

	c := msg.Cid()
	sig, err := w.WalletSign(context.Background(), from, c.Bytes(), types.MsgMeta{})
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
		Ticket: &types.Ticket{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Parents:               tsKey.Cids(),
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
	ts, err := types.NewTipSet(blks)
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
	tma.balance[addr] = types.FromFil(v)
}

func (tma *testMpoolAPI) setBalanceRaw(addr address.Address, v tbig.Int) {
	tma.balance[addr] = v
}

func (tma *testMpoolAPI) setBlockMessages(h *types.BlockHeader, msgs ...*types.SignedMessage) {
	tma.bmsgs[h.Cid()] = msgs
}

func (tma *testMpoolAPI) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return &types.TipSet{}, nil
}

func (tma *testMpoolAPI) ChainTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	return &types.TipSet{}, nil
}

func (tma *testMpoolAPI) SubscribeHeadChanges(ctx context.Context, cb func(rev, app []*types.TipSet) error) *types.TipSet {
	tma.cb = cb
	return tma.tipsets[0]
}

func (tma *testMpoolAPI) PutMessage(ctx context.Context, m types.ChainMsg) (cid.Cid, error) {
	return cid.Undef, nil
}

func (tma *testMpoolAPI) IsLite() bool {
	return false
}

func (tma *testMpoolAPI) PubSubPublish(context.Context, string, []byte) error {
	tma.published++
	return nil
}

func (tma *testMpoolAPI) GetActorAfter(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, error) {
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

func (tma *testMpoolAPI) StateAccountKeyAtFinality(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	if addr.Protocol() != address.BLS && addr.Protocol() != address.SECP256K1 {
		return address.Undef, fmt.Errorf("given address was not a key addr")
	}
	return addr, nil
}

func (tma *testMpoolAPI) StateNetworkVersion(ctx context.Context, h abi.ChainEpoch) network.Version {
	return constants.TestNetworkVersion
}

func (tma *testMpoolAPI) StateAccountKey(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	if addr.Protocol() != address.BLS && addr.Protocol() != address.SECP256K1 {
		return address.Undef, fmt.Errorf("given address was not a key addr")
	}
	return addr, nil
}

func (tma *testMpoolAPI) MessagesForBlock(ctx context.Context, h *types.BlockHeader) ([]*types.Message, []*types.SignedMessage, error) {
	return nil, tma.bmsgs[h.Cid()], nil
}

func (tma *testMpoolAPI) MessagesForTipset(ctx context.Context, ts *types.TipSet) ([]types.ChainMsg, error) {
	if len(ts.Blocks()) != 1 {
		panic("cant deal with multiblock tipsets in this test")
	}

	bm, sm, err := tma.MessagesForBlock(ctx, ts.Blocks()[0])
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

func (tma *testMpoolAPI) LoadTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
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
	n, err := mp.GetNonce(context.Background(), addr, types.EmptyTSK)
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
	if err := mp.Add(context.TODO(), msg); err != nil {
		t.Fatal(err)
	}
}

func newWalletAndMpool(t *testing.T, tma *testMpoolAPI) (*wallet.Wallet, *MessagePool) {
	ds := datastore.NewMapDatastore()

	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	stmgr := statemanger.NewStateManger(builder.Store(), eval, nil, fork.NewMockFork(), nil, nil)

	mp, err := New(context.Background(), tma, stmgr, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	return newWallet(t), mp
}

func newWallet(t *testing.T) *wallet.Wallet {
	r := repo.NewInMemoryRepo()
	backend, err := wallet.NewDSBackend(context.Background(), r.WalletDatastore(), r.Config().Wallet.PassphraseConfig, wallet.TestPassword)
	assert.NoError(t, err)

	return wallet.New(backend)
}

func TestMessagePool(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)
	// stm: @MESSAGEPOOL_POOL_CLOSE_001
	defer mp.Close() // nolint

	a := tma.nextBlock()

	sender, err := w.NewAddress(context.Background(), address.SECP256K1)
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
	// stm: @MESSAGEPOOL_POOL_ADD_001
	mustAdd(t, mp, msgs[0])
	assertNonce(t, mp, sender, 1)
	mustAdd(t, mp, msgs[1])
	assertNonce(t, mp, sender, 2)

	tma.setBlockMessages(a, msgs[0], msgs[1])

	// stm: @MESSAGEPOOL_POOL_GET_MESSAGES_FOR_BLOCKS_001
	blockMsgs, err := mp.MessagesForBlocks(ctx, []*types.BlockHeader{a})
	assert.NoError(t, err)
	assert.Equal(t, len(blockMsgs), 2)
	tma.applyBlock(t, a)

	assertNonce(t, mp, sender, 2)
	{ // test verify message signature
		// stm: @MESSAGEPOOL_POOL_VERIFY_MSG_SIG_001
		assert.NoError(t, mp.VerifyMsgSig(msgs[2]))
	}
	{ // test publish message
		mustAdd(t, mp, msgs[2])
		assertNonce(t, mp, sender, msgs[2].Message.Nonce+1)
		pendingMsgs, _ := mp.PendingFor(ctx, sender)
		assert.Equal(t, len(pendingMsgs), 1)
		// stm: @MESSAGEPOOL_POOL_PUBLISH_FOR_WALLET
		assert.NoError(t, mp.PublishMsgForWallet(ctx, sender))
		//// stm: @MESSAGEPOOL_POOL_PUBLISH_001
		assert.NoError(t, mp.PublishMsg(ctx, msgs[2]))
		assertNonce(t, mp, sender, msgs[2].Message.Nonce+1)
	}
	{ // test delete by address
		// delete pending message with sender is message.From
		// stm: @MESSAGEPOOL_POOL_DELETE_BY_ADDRESS_001
		assert.NoError(t, mp.DeleteByAdress(sender))
		// since message.From is deleted, the pending messages should be 0
		pendingMsgs, _ := mp.PendingFor(ctx, sender)
		assert.Equal(t, len(pendingMsgs), 0)
	}
	{ // test remove message
		mustAdd(t, mp, msgs[2])
		pendingMsgs, _ := mp.PendingFor(ctx, sender)
		assert.Equal(t, len(pendingMsgs), 1)
		// stm: @MESSAGEPOOL_POOL_REMOVE_001
		mp.Remove(ctx, sender, msgs[2].Message.Nonce, false)
		pendingMsgs, _ = mp.PendingFor(ctx, sender)
		assert.Equal(t, len(pendingMsgs), 0)
	}

	{ // test push untrusted.
		// stm: @MESSAGEPOOL_POOL_PUSH_UNTRUSTED
		msgCID, err := mp.PushUntrusted(ctx, msgs[2])
		assert.NoError(t, err)
		assert.Equal(t, msgCID, msgs[2].Cid())

		assertNonce(t, mp, sender, msgs[2].Message.Nonce+1)

		// stm: @MESSAGEPOOL_POOL_GET_PENDING_001
		pendingMsgs01, _ := mp.Pending(ctx)
		assert.Equal(t, len(pendingMsgs01), 1)

		// stm: @MESSAGEPOOL_POOL_GET_PENDING_FOR_ADDRESS_001
		pendingMsgs02, _ := mp.PendingFor(ctx, sender)
		assert.Equal(t, len(pendingMsgs02), 1)

		assert.Equal(t, pendingMsgs01, pendingMsgs02)
	}
	{ // test check messages
		mustAdd(t, mp, msgs[3])
		// stm: @MESSAGEPOOL_POOL_CHECK_PENDING_MESSAGES_001
		_, err := mp.CheckPendingMessages(ctx, sender)
		assert.NoError(t, err)
		// stm: @MESSAGEPOOL_POOL_CHECK_MESSAGES_001
		_, err = mp.CheckMessages(ctx, []*types.MessagePrototype{
			{ValidNonce: true, Message: msgs[3].Message},
		})
		assert.NoError(t, err)
		// stm:@MESSAGEPOOL_POOL_RECOVER_SIG_001
		assert.Nil(t, mp.RecoverSig(&msgs[4].Message))
	}

}

func TestCheckMessageBig(t *testing.T) {
	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)
	from, err := w.NewAddress(context.Background(), address.SECP256K1)
	assert.NoError(t, err)

	tma.setBalance(from, 1000e9)

	to := mkAddress(1001)

	{
		msg := &types.Message{
			To:         to,
			From:       from,
			Value:      types.NewInt(1),
			Nonce:      0,
			GasLimit:   50000000,
			GasFeeCap:  types.NewInt(100),
			GasPremium: types.NewInt(1),
			Params:     make([]byte, 41<<10), // 41KiB payload
		}

		sig, err := w.WalletSign(context.Background(), from, msg.Cid().Bytes(), types.MsgMeta{})
		if err != nil {
			panic(err)
		}
		sm := &types.SignedMessage{
			Message:   *msg,
			Signature: *sig,
		}
		mustAdd(t, mp, sm)
	}

	{
		msg := &types.Message{
			To:         to,
			From:       from,
			Value:      types.NewInt(1),
			Nonce:      0,
			GasLimit:   50000000,
			GasFeeCap:  types.NewInt(100),
			GasPremium: types.NewInt(1),
			Params:     make([]byte, 64<<10), // 64KiB payload
		}

		sig, err := w.WalletSign(context.Background(), from, msg.Cid().Bytes(), types.MsgMeta{})
		if err != nil {
			panic(err)
		}
		sm := &types.SignedMessage{
			Message:   *msg,
			Signature: *sig,
		}
		err = mp.Add(context.TODO(), sm)
		assert.ErrorIs(t, err, ErrMessageTooBig)
	}
}

func TestMessagePoolMessagesInEachBlock(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()

	w, mp := newWalletAndMpool(t, tma)

	a := tma.nextBlock()

	sender, err := w.NewAddress(context.Background(), address.SECP256K1)
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

	_, _ = mp.Pending(context.TODO())

	// stm: @MESSAGEPOOL_POOL_SELECT_MESSAGES_001
	selm, _ := mp.SelectMessages(context.Background(), tsa, 1)
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

	sender, err := w.NewAddress(context.Background(), address.SECP256K1)
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

	p, _ := mp.Pending(context.TODO())
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

	sender, err := w.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}
	tma.setBalance(sender, 1) // in FIL
	target := mkAddress(1001)

	for i := 0; i < 5; i++ {
		smsg := mkMessage(sender, target, uint64(i), w)
		if err := mp.Add(context.TODO(), smsg); err != nil {
			t.Fatal(err)
		}
	}

	for i := 10; i < 50; i++ {
		smsg := mkMessage(sender, target, uint64(i), w)
		if err := mp.Add(context.TODO(), smsg); err != nil {
			t.Fatal(err)
		}
	}

	mp.cfg.SizeLimitHigh = 40
	mp.cfg.SizeLimitLow = 10

	// stm: @MESSAGEPOOL_POOL_PRUNE_001
	mp.Prune()

	msgs, _ := mp.Pending(context.TODO())
	if len(msgs) != 5 {
		t.Fatal("expected only 5 messages in pool, got: ", len(msgs))
	}
}

func TestLoadLocal(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(context.Background(), tma, nil, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	w1 := newWallet(t)
	a1, err := w1.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	w2 := newWallet(t)
	a2, err := w2.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL
	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	msgs := make(map[cid.Cid]struct{})
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		// stm: @MESSAGEPOOL_POOL_PUSH_001
		c, err := mp.Push(context.TODO(), m)
		if err != nil {
			t.Fatal(err)
		}
		msgs[c] = struct{}{}
	}
	err = mp.Close()
	if err != nil {
		t.Fatal(err)
	}

	mp, err = New(context.Background(), tma, nil, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	pmsgs, _ := mp.Pending(context.TODO())
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

	mp, err := New(context.Background(), tma, nil, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	w1 := newWallet(t)
	a1, err := w1.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	w2 := newWallet(t)
	a2, err := w2.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL
	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(context.TODO(), m)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w2, a2, a1, uint64(i), gasLimit, uint64(i+1))
		mustAdd(t, mp, m)
	}

	// stm: @MESSAGEPOOL_POOL_CLEAR_001
	mp.Clear(context.Background(), true)

	pending, _ := mp.Pending(context.TODO())
	if len(pending) > 0 {
		t.Fatalf("cleared the mpool, but got %d pending messages", len(pending))
	}
}

func TestClearNonLocal(t *testing.T) {
	tf.UnitTest(t)

	tma := newTestMpoolAPI()
	ds := datastore.NewMapDatastore()

	mp, err := New(context.Background(), tma, nil, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	w1 := newWallet(t)
	a1, err := w1.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	w2 := newWallet(t)
	a2, err := w2.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	tma.setBalance(a1, 1) // in FIL
	tma.setBalance(a2, 1) // in FIL

	gasLimit := gasguess.Costs[gasguess.CostKey{Code: builtin2.StorageMarketActorCodeID, M: 2}]
	for i := 0; i < 10; i++ {
		m := makeTestMessage(w1, a1, a2, uint64(i), gasLimit, uint64(i+1))
		_, err := mp.Push(context.TODO(), m)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		m := makeTestMessage(w2, a2, a1, uint64(i), gasLimit, uint64(i+1))
		mustAdd(t, mp, m)
	}

	mp.Clear(context.Background(), false)

	pending, _ := mp.Pending(context.TODO())
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

	mp, err := New(context.Background(), tma, nil, ds, config.NewDefaultConfig().NetworkParams, config.DefaultMessagePoolParam, "mptest", nil)
	if err != nil {
		t.Fatal(err)
	}

	// the actors
	w1 := newWallet(t)
	a1, err := w1.NewAddress(context.Background(), address.SECP256K1)
	if err != nil {
		t.Fatal(err)
	}

	w2 := newWallet(t)
	a2, err := w2.NewAddress(context.Background(), address.SECP256K1)
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
		_, err := mp.Push(context.TODO(), m)
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

func TestCapGasFee(t *testing.T) {
	t.Run("use default maxfee", func(t *testing.T) {
		msg := &types.Message{
			GasLimit:   100_000_000,
			GasFeeCap:  abi.NewTokenAmount(100_000_000),
			GasPremium: abi.NewTokenAmount(100_000),
		}
		CapGasFee(func() (abi.TokenAmount, error) {
			return abi.NewTokenAmount(100_000_000_000), nil
		}, msg, nil)
		assert.Equal(t, msg.GasFeeCap.Int64(), int64(1000))
		assert.Equal(t, msg.GasPremium.Int.Int64(), int64(1000))
	})

	t.Run("use spec maxfee", func(t *testing.T) {
		msg := &types.Message{
			GasLimit:   100_000_000,
			GasFeeCap:  abi.NewTokenAmount(100_000_000),
			GasPremium: abi.NewTokenAmount(100_000),
		}
		CapGasFee(nil, msg, &types.MessageSendSpec{MaxFee: abi.NewTokenAmount(100_000_000_000)})
		assert.Equal(t, msg.GasFeeCap.Int64(), int64(1000))
		assert.Equal(t, msg.GasPremium.Int.Int64(), int64(1000))
	})

	t.Run("use smaller feecap value when fee is enough", func(t *testing.T) {
		msg := &types.Message{
			GasLimit:   100_000_000,
			GasFeeCap:  abi.NewTokenAmount(100_000),
			GasPremium: abi.NewTokenAmount(100_000_000),
		}
		CapGasFee(nil, msg, &types.MessageSendSpec{MaxFee: abi.NewTokenAmount(100_000_000_000_000)})
		assert.Equal(t, msg.GasFeeCap.Int64(), int64(100_000))
		assert.Equal(t, msg.GasPremium.Int.Int64(), int64(100_000))
	})
}
