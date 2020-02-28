package mining_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	dag "github.com/ipfs/go-merkledag"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestLookbackElection(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	builder := chain.NewBuilder(t, address.Undef)
	lookback := miner.ElectionLookback
	head := builder.NewGenesis()
	for i := 1; i < int(lookback); i++ {
		head = builder.AppendOn(head, 1)
	}

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}

	rnd := &consensus.FakeChainRandomness{Seed: 0}
	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	messages := chain.NewMessageStore(bs)

	t.Run("Election sees ticket lookback ancestors back", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			Election:       consensus.NewElectionMachine(rnd),
			TicketGen:      consensus.NewTicketMachine(rnd),

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, head, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
		// TODO: make an assertion about the epost/ticket produced
	})

	t.Run("Ticket gensees ticket 1 ancestor back", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			Election:       consensus.NewElectionMachine(rnd),
			TicketGen:      consensus.NewTicketMachine(rnd),

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, head, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
		// TODO: make an assertion about the epost/ticket produced

	})

}

func Test_Mine(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: e.NewCid(stateRoot), Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := th.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}

	rnd := &consensus.FakeChainRandomness{Seed: 0}
	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup
	messages := chain.NewMessageStore(bs)

	// TODO #3311: this case isn't testing much.  Testing w.Mine further needs a lot more attention.
	t.Run("Trivial success case", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			Election:       consensus.NewElectionMachine(rnd),
			TicketGen:      consensus.NewTicketMachine(rnd),

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
		// TODO: make an assertion about the ticket/epost
	})

	t.Run("Block generation fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{shouldError: true},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			Election:       consensus.NewElectionMachine(rnd),
			TicketGen:      consensus.NewTicketMachine(rnd),

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})
		outCh := make(chan mining.Output)

		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		require.Error(t, r.Err)
		assert.Contains(t, r.Err.Error(), "test error retrieving state root")
	})

	t.Run("Sent empty tipset", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			Election:       consensus.NewElectionMachine(rnd),
			TicketGen:      consensus.NewTicketMachine(rnd),

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})
		input := block.TipSet{}
		outCh := make(chan mining.Output)
		go worker.Mine(ctx, input, 0, outCh)
		r := <-outCh
		assert.EqualError(t, r.Err, "bad input tipset with no blocks sent to Mine()")
	})
}

func sharedSetupInitial() (cbor.IpldStore, *message.Pool, cid.Cid) {
	r := repo.NewInMemoryRepo()
	bs := blockstore.NewBlockstore(r.Datastore())
	cst := cborutil.NewIpldStore(bs)
	pool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := builtin.AccountActorCodeID
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T, mockSigner types.MockSigner) (
	state.Tree, *message.Pool, []address.Address, blockstore.Blockstore) {

	cst, pool, _ := sharedSetupInitial()
	ctx := context.TODO()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	vms := vm.NewStorage(bs)

	addr1, addr2, addr3, addr5 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[4]
	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward failures.
		builtin.RewardActorAddr: actor.NewActor(builtin.RewardActorCodeID, abi.NewTokenAmount(1000000)),
	})
	th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(100))
	th.RequireInitAccountActor(ctx, t, st, vms, addr2, types.NewAttoFILFromFIL(100))
	th.RequireInitAccountActor(ctx, t, st, vms, addr5, types.ZeroAttoFIL)
	_, addr4 := th.RequireNewMinerActor(ctx, t, st, vms, addr5, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
	return st, pool, []address.Address{addr1, addr2, addr3, addr4, addr5}, bs
}

// TODO this test belongs in core, it calls ApplyMessages #3311
func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("new processor")

	// vms := th.VMStorage()

	// mockSigner, _ := setupSigner()
	// cst, _, _ := sharedSetupInitial()
	// ctx := context.TODO()

	// // Stick two fake actors in the state tree so they can talk.
	// addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	// _, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
	// 	address.LegacyNetworkAddress: th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(1000000)),
	// })
	// th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.ZeroAttoFIL)

	// // NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// // If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// // exercise the categorization.
	// // addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	// msg0 := types.NewMeteredMessage(addr2, addr1, 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.GasUnits(0))

	// // This is actually okay and should result in a receipt
	// msg1 := types.NewMeteredMessage(addr1, addr2, 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.GasUnits(0))

	// // The following two are sending to self -- errSelfSend, a permanent error.
	// msg2 := types.NewMeteredMessage(addr1, addr1, 1, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.GasUnits(0))
	// msg3 := types.NewMeteredMessage(addr2, addr2, 1, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.GasUnits(0))

	// messages := []*types.UnsignedMessage{msg0, msg1, msg2, msg3}

	// res, err := consensus.NewDefaultProcessor().ApplyMessagesAndPayRewards(ctx, st, vms, messages, addr1, abi.ChainEpoch(0), nil)
	// assert.NoError(t, err)
	// require.NotNil(t, res)

	// assert.Error(t, res[0].Failure)
	// assert.False(t, res[0].FailureIsPermanent)

	// assert.Nil(t, res[1].Failure)

	// assert.Error(t, res[2].Failure)
	// assert.True(t, res[2].FailureIsPermanent)
	// assert.Error(t, res[3].Failure)
	// assert.True(t, res[3].FailureIsPermanent)
}

func TestApplyBLSMessages(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ki := types.MustGenerateMixedKeyInfo(5, 5)
	mockSignerVal := types.NewMockSigner(ki)
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: e.NewCid(stateRoot), Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := th.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}

	rnd := &consensus.FakeChainRandomness{Seed: 0}
	msgStore := chain.NewMessageStore(bs)

	// assert that first two addresses have different protocols
	blsAddress := addrs[0]
	assert.Equal(t, address.BLS, blsAddress.Protocol())
	secpAddress := addrs[1]
	assert.Equal(t, address.SECP256K1, secpAddress.Protocol())

	// create secp and bls signed messages interleaved
	for i := 0; i < 10; i++ {
		var addr address.Address
		if i%2 == 0 {
			addr = blsAddress
		} else {
			addr = secpAddress
		}
		smsg := requireSignedMessage(t, mockSigner, addr, addrs[3], uint64(i/2), types.NewAttoFILFromFIL(1))
		_, err := pool.Add(ctx, smsg, abi.ChainEpoch(0))
		require.NoError(t, err)
	}

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(mockSigner.Addresses[5], rnd),

		MinerAddr:      addrs[3],
		MinerOwnerAddr: addrs[4],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  msgStore,
		Clock:         clock.NewSystemClock(),
	})

	outCh := make(chan mining.Output)
	go worker.Mine(ctx, tipSet, 0, outCh)
	r := <-outCh
	require.NoError(t, r.Err)
	block := r.NewBlock

	t.Run("messages are divided into bls and secp messages", func(t *testing.T) {
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages.Cid)
		require.NoError(t, err)

		assert.Len(t, secpMessages, 5)
		assert.Len(t, blsMessages, 5)

		for _, msg := range secpMessages {
			assert.Equal(t, address.SECP256K1, msg.Message.From.Protocol())
		}

		for _, msg := range blsMessages {
			assert.Equal(t, address.BLS, msg.From.Protocol())
		}
	})

	t.Run("all 10 messages are stored", func(t *testing.T) {
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages.Cid)
		require.NoError(t, err)

		assert.Len(t, secpMessages, 5)
		assert.Len(t, blsMessages, 5)
	})

	t.Run("block bls signature can be used to validate messages", func(t *testing.T) {
		digests := []bls.Digest{}
		keys := []bls.PublicKey{}

		_, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages.Cid)
		require.NoError(t, err)
		for _, msg := range blsMessages {
			msgBytes, err := msg.Marshal()
			require.NoError(t, err)
			digests = append(digests, bls.Hash(msgBytes))

			pubKey := bls.PublicKey{}
			copy(pubKey[:], msg.From.Payload())
			keys = append(keys, pubKey)
		}

		blsSig := bls.Signature{}
		copy(blsSig[:], block.BLSAggregateSig.Data)
		valid := bls.Verify(&blsSig, digests, keys)

		assert.True(t, valid)
	})
}

func requireSignedMessage(t *testing.T, signer types.Signer, from, to address.Address, nonce uint64, value types.AttoFIL) *types.SignedMessage {
	msg := types.NewMeteredMessage(from, to, nonce, value, builtin.MethodSend, []byte{}, types.NewAttoFILFromFIL(1), 300)
	smsg, err := types.NewSignedMessage(*msg, signer)
	require.NoError(t, err)
	return smsg
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx := context.Background()

	mockSigner, blockSignerAddr := setupSigner()
	st, pool, addrs, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	rnd := &consensus.FakeChainRandomness{Seed: 0}
	minerAddr := addrs[4]
	minerOwnerAddr := addrs[3]
	messages := chain.NewMessageStore(bs)

	meta := fakeTSMetadata{}
	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: meta,
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     th.NewFakeProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()

	parentTipset := builder.AppendManyOn(99, genesis)
	baseTipset := builder.AppendOn(parentTipset, 2)
	assert.Equal(t, 2, baseTipset.Len())

	fakePoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStForTest(), consensus.MakeFakeVRFProofForTest(), consensus.MakeFakeWinnersForTest()...)

	blk, err := worker.Generate(ctx, baseTipset, block.Ticket{VRFProof: []byte{2}}, 0, fakePoStInfo)
	assert.NoError(t, err)

	txMeta, err := messages.LoadTxMeta(ctx, blk.Messages.Cid)
	require.NoError(t, err)
	assert.Equal(t, types.EmptyMessagesCID, txMeta.SecpRoot.Cid)

	expectedStateRoot, err := meta.GetTipSetStateRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedStateRoot, blk.StateRoot.Cid)

	expectedReceipts, err := meta.GetTipSetReceiptsRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedReceipts, blk.MessageReceipts.Cid)

	assert.Equal(t, uint64(101), blk.Height)
	assert.Equal(t, fbig.NewInt(120), blk.ParentWeight)
	assert.Equal(t, block.Ticket{VRFProof: []byte{2}}, blk.Ticket)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	rnd := &consensus.FakeChainRandomness{Seed: 0}
	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(rnd),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMeteredMessage(addrs[2], addrs[0], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), types.GasUnits(0))
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(t, err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), types.GasUnits(0))
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(t, err)

	// add the following and then increment the actor nonce at addrs[1], nonceTooLow, a permanent error.
	msg3 := types.NewMeteredMessage(addrs[1], addrs[0], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), types.GasUnits(0))
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner)
	require.NoError(t, err)

	msg4 := types.NewMeteredMessage(addrs[1], addrs[2], 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), types.GasUnits(0))
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner)
	require.NoError(t, err)

	_, err = pool.Add(ctx, smsg1, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg2, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg3, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg4, 0)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 4)

	// Set actor nonce past nonce of message in pool.
	// Have to do this here to get a permanent error in the pool.
	act, actID := th.RequireLookupActor(ctx, t, st, vm.NewStorage(bs), addrs[1])
	require.NoError(t, err)

	act.CallSeqNum = 2
	err = st.SetActor(ctx, actID, act)
	require.NoError(t, err)

	stateRoot, err := st.Commit(ctx)
	require.NoError(t, err)

	baseBlock := block.Block{
		Parents:   block.NewTipSetKey(newCid()),
		Height:    100,
		StateRoot: e.NewCid(stateRoot),
	}
	fakePoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStForTest(), consensus.MakeFakeVRFProofForTest(), consensus.MakeFakeWinnersForTest()...)

	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, 0, fakePoStInfo)
	assert.NoError(t, err)

	// This is the temporary failure + the good message,
	// which will be removed by the node if this block is accepted.
	assert.Len(t, pool.Pending(), 2)
	assert.Contains(t, pool.Pending(), smsg1)
	assert.Contains(t, pool.Pending(), smsg2)

	// message and receipts can be loaded from message store and have
	// length 1.
	msgs, _, err := messages.LoadMessages(ctx, blk.Messages.Cid)
	require.NoError(t, err)
	assert.Len(t, msgs, 1) // This is the good message
}

func TestGenerateSetsBasicFields(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	rnd := &consensus.FakeChainRandomness{Seed: 0}
	minerAddr := addrs[3]
	th.RequireInitAccountActor(ctx, t, st, vm.NewStorage(bs), addrs[4], types.ZeroAttoFIL)
	minerOwnerAddr := addrs[4]

	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(rnd),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	h := abi.ChainEpoch(100)
	w := fbig.NewInt(1000)
	baseBlock := block.Block{
		Height:       h,
		ParentWeight: w,
		StateRoot:    e.NewCid(newCid()),
	}
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	ticket := mining.NthTicket(7)
	fakePoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStForTest(), consensus.MakeFakeVRFProofForTest(), consensus.MakeFakeWinnersForTest()...)
	blk, err := worker.Generate(ctx, baseTipSet, ticket, 0, fakePoStInfo)
	assert.NoError(t, err)

	assert.Equal(t, h+1, blk.Height)
	assert.Equal(t, minerAddr, blk.Miner)
	assert.Equal(t, ticket, blk.Ticket)

	blk, err = worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, 1, fakePoStInfo)
	assert.NoError(t, err)

	assert.Equal(t, h+2, blk.Height)
	assert.Equal(t, fbig.Add(w, fbig.NewInt(10.0)), blk.ParentWeight)
	assert.Equal(t, minerAddr, blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	rnd := &consensus.FakeChainRandomness{Seed: 0}
	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(rnd),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	assert.Len(t, pool.Pending(), 0)
	baseBlock := block.Block{
		Parents:   block.NewTipSetKey(newCid()),
		Height:    100,
		StateRoot: e.NewCid(newCid()),
	}
	fakePoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStForTest(), consensus.MakeFakeVRFProofForTest(), consensus.MakeFakeWinnersForTest()...)
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, 0, fakePoStInfo)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0) // This is the temporary failure.
	txMeta, err := messages.LoadTxMeta(ctx, blk.Messages.Cid)
	require.NoError(t, err)
	assert.Equal(t, types.EmptyMessagesCID, txMeta.SecpRoot.Cid)
	assert.Equal(t, types.EmptyMessagesCID, txMeta.BLSRoot.Cid)
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}
	rnd := &consensus.FakeChainRandomness{Seed: 0}
	messages := chain.NewMessageStore(bs)
	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr, rnd),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{shouldError: true},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(rnd),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	// This is actually okay and should result in a receipt
	msg := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	smsg, err := types.NewSignedMessage(*msg, &mockSigner)
	require.NoError(t, err)
	_, err = pool.Add(ctx, smsg, 0)
	require.NoError(t, err)

	assert.Len(t, pool.Pending(), 1)
	baseBlock := block.Block{
		Parents:   block.NewTipSetKey(newCid()),
		Height:    100,
		StateRoot: e.NewCid(newCid()),
	}
	fakePoStInfo := block.NewEPoStInfo(consensus.MakeFakePoStForTest(), consensus.MakeFakeVRFProofForTest(), consensus.MakeFakeWinnersForTest()...)
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, 0, fakePoStInfo)
	assert.Error(t, err, "boom")
	assert.Nil(t, blk)

	assert.Len(t, pool.Pending(), 1) // No messages are removed from the pool.
}

func getWeightTest(_ context.Context, ts block.TipSet) (fbig.Int, error) {
	w, err := ts.ParentWeight()
	if err != nil {
		return fbig.Zero(), err
	}
	// consensus.ecV = 10
	return fbig.Add(w, fbig.NewInt(int64(ts.Len()*10))), nil
}

func setupSigner() (types.MockSigner, address.Address) {
	mockSigner, _ := types.NewMockSignersAndKeyInfo(10)

	signerAddr := mockSigner.Addresses[len(mockSigner.Addresses)-1]
	return mockSigner, signerAddr
}

type fakeTSMetadata struct {
	shouldError bool
}

func (tm fakeTSMetadata) GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error) {
	if tm.shouldError {
		return cid.Undef, errors.New("test error retrieving state root")
	}
	return dag.NewRawNode([]byte("state root")).Cid(), nil
}

func (tm fakeTSMetadata) GetTipSetReceiptsRoot(key block.TipSetKey) (cid.Cid, error) {
	return dag.NewRawNode([]byte("receipt root")).Cid(), nil
}
