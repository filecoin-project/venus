package mining_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	dag "github.com/ipfs/go-merkledag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestLookbackElection(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	builder := chain.NewBuilder(t, address.Undef)
	head := builder.NewGenesis()
	for i := 1; i < int(miner.ElectionLookback); i++ {
		head = builder.AppendOn(head, 1)
	}

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}

	rnd := &consensus.FakeChainRandomness{Seed: 0}
	samp := &consensus.FakeSampler{Seed: 0}
	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	messages := chain.NewMessageStore(bs)

	t.Run("Election sees ticket lookback ancestors back", func(t *testing.T) {
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
			TicketGen:      consensus.NewTicketMachine(samp),

			MessageSource:    pool,
			MessageQualifier: &mining.NoMessageQualifier{},
			Blockstore:       bs,
			MessageStore:     messages,
			Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
		})

		blk, err := worker.Mine(ctx, head, 0)
		assert.NoError(t, err)

		expectedTicket := makeExpectedTicket(ctx, t, rnd, mockSigner, head, miner.ElectionLookback, minerAddr, minerOwnerAddr)
		assert.Equal(t, expectedTicket, blk.Header.Ticket)
	})
}

func Test_Mine(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: stateRoot, Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := block.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, tsKey block.TipSetKey) (state.Tree, error) {
		return st, nil
	}

	rnd := &consensus.FakeChainRandomness{Seed: 0}
	samp := &consensus.FakeSampler{Seed: 0}
	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup
	messages := chain.NewMessageStore(bs)

	// TODO #3311: this case isn't testing much.  Testing w.Mine further needs a lot more attention.
	t.Run("Trivial success case", func(t *testing.T) {
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
			TicketGen:      consensus.NewTicketMachine(samp),

			MessageSource:    pool,
			MessageQualifier: &mining.NoMessageQualifier{},
			Blockstore:       bs,
			MessageStore:     messages,
			Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
		})

		blk, err := worker.Mine(ctx, tipSet, 0)
		assert.NoError(t, err)

		expectedTicket := makeExpectedTicket(ctx, t, rnd, mockSigner, tipSet, miner.ElectionLookback, minerAddr, minerOwnerAddr)
		assert.Equal(t, expectedTicket, blk.Header.Ticket)
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
			TicketGen:      consensus.NewTicketMachine(samp),

			MessageSource:    pool,
			MessageQualifier: &mining.NoMessageQualifier{},
			Blockstore:       bs,
			MessageStore:     messages,
			Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
		})

		_, err := worker.Mine(ctx, tipSet, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error retrieving state root")
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
			TicketGen:      consensus.NewTicketMachine(samp),

			MessageSource:    pool,
			MessageQualifier: &mining.NoMessageQualifier{},
			Blockstore:       bs,
			MessageStore:     messages,
			Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
		})
		input := block.TipSet{}
		_, err := worker.Mine(ctx, input, 0)
		assert.EqualError(t, err, "bad input tipset with no blocks sent to Mine()")
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
		builtin.RewardActorAddr: actor.NewActor(builtin.RewardActorCodeID, abi.NewTokenAmount(1000000), cid.Undef),
	})

	_, addr4 := th.RequireNewMinerActor(ctx, t, st, vms, addr5, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
	return st, pool, []address.Address{addr1, addr2, addr3, addr4, addr5}, bs
}

func makeExpectedTicket(ctx context.Context, t *testing.T, rnd *consensus.FakeChainRandomness, mockSigner *types.MockSigner,
	head block.TipSet, lookback abi.ChainEpoch, minerAddr address.Address, minerOwnerAddr address.Address) block.Ticket {
	height, err := head.Height()
	require.NoError(t, err)
	entropy, err := encoding.Encode(minerAddr)
	require.NoError(t, err)
	seed, err := rnd.SampleChainRandomness(ctx, head.Key(), acrypto.DomainSeparationTag_TicketProduction, height-lookback, entropy)
	require.NoError(t, err)
	expectedVrfProof, err := mockSigner.SignBytes(ctx, seed, minerOwnerAddr)
	require.NoError(t, err)
	return block.Ticket{VRFProof: expectedVrfProof.Data}
}

func TestApplyBLSMessages(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("using legacy vmcontext")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ki := types.MustGenerateMixedKeyInfo(5, 5)
	mockSigner := types.NewMockSigner(ki)

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: stateRoot, Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := block.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSigner)
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
		smsg := requireSignedMessage(t, &mockSigner, addr, addrs[3], uint64(i/2), types.NewAttoFILFromFIL(1))
		_, err := pool.Add(ctx, smsg, abi.ChainEpoch(0))
		require.NoError(t, err)
	}

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI((&mockSigner).Addresses[5], rnd),

		MinerAddr:      addrs[3],
		MinerOwnerAddr: addrs[4],
		WorkerSigner:   &mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     msgStore,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	block, err := worker.Mine(ctx, tipSet, 0)
	require.NoError(t, err)

	t.Run("messages are divided into bls and secp messages", func(t *testing.T) {
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Header.Messages.Cid)
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
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Header.Messages.Cid)
		require.NoError(t, err)

		assert.Len(t, secpMessages, 5)
		assert.Len(t, blsMessages, 5)
	})

	t.Run("block bls signature can be used to validate messages", func(t *testing.T) {
		digests := []bls.Digest{}
		keys := []bls.PublicKey{}

		_, blsMessages, err := msgStore.LoadMessages(ctx, block.Header.Messages.Cid)
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
		copy(blsSig[:], block.Header.BLSAggregateSig.Data)
		valid := bls.Verify(&blsSig, digests, keys)

		assert.True(t, valid)
	})
}

func requireSignedMessage(t *testing.T, signer types.Signer, from, to address.Address, nonce uint64, value types.AttoFIL) *types.SignedMessage {
	msg := types.NewMeteredMessage(from, to, nonce, value, builtin.MethodSend, []byte{}, types.NewAttoFILFromFIL(1), 300)
	smsg, err := types.NewSignedMessage(context.TODO(), *msg, signer)
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

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     messages,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()

	parentTipset := builder.AppendManyOn(99, genesis)
	baseTipset := builder.AppendOn(parentTipset, 2)
	assert.Equal(t, 2, baseTipset.Len())

	blk, err := worker.Generate(ctx, baseTipset, block.Ticket{VRFProof: []byte{2}}, consensus.MakeFakeVRFProofForTest(), 0, consensus.MakeFakePoStsForTest(), nil)
	assert.NoError(t, err)

	txMeta, err := messages.LoadTxMeta(ctx, blk.Header.Messages.Cid)
	require.NoError(t, err)
	assert.Equal(t, types.EmptyMessagesCID, txMeta.SecpRoot.Cid)

	expectedStateRoot, err := meta.GetTipSetStateRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedStateRoot, blk.Header.StateRoot.Cid)

	expectedReceipts, err := meta.GetTipSetReceiptsRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedReceipts, blk.Header.MessageReceipts.Cid)

	assert.Equal(t, uint64(101), blk.Header.Height)
	assert.Equal(t, fbig.NewInt(120), blk.Header.ParentWeight)
	assert.Equal(t, block.Ticket{VRFProof: []byte{2}}, blk.Header.Ticket)
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

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     messages,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMeteredMessage(addrs[2], addrs[0], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), gas.NewGas(0))
	smsg1, err := types.NewSignedMessage(ctx, *msg1, &mockSigner)
	require.NoError(t, err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), gas.NewGas(0))
	smsg2, err := types.NewSignedMessage(ctx, *msg2, &mockSigner)
	require.NoError(t, err)

	// add the following and then increment the actor nonce at addrs[1], nonceTooLow, a permanent error.
	msg3 := types.NewMeteredMessage(addrs[1], addrs[0], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), gas.NewGas(0))
	smsg3, err := types.NewSignedMessage(ctx, *msg3, &mockSigner)
	require.NoError(t, err)

	msg4 := types.NewMeteredMessage(addrs[1], addrs[2], 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(1), gas.NewGas(0))
	smsg4, err := types.NewSignedMessage(ctx, *msg4, &mockSigner)
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
		StateRoot: stateRoot,
	}

	blk, err := worker.Generate(ctx, block.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeVRFProofForTest(), 0, consensus.MakeFakePoStsForTest(), nil)
	assert.NoError(t, err)

	// This is the temporary failure + the good message,
	// which will be removed by the node if this block is accepted.
	assert.Len(t, pool.Pending(), 2)
	assert.Contains(t, pool.Pending(), smsg1)
	assert.Contains(t, pool.Pending(), smsg2)

	// message and receipts can be loaded from message store and have
	// length 1.
	msgs, _, err := messages.LoadMessages(ctx, blk.Header.Messages.Cid)
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

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     messages,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	h := abi.ChainEpoch(100)
	w := fbig.NewInt(1000)
	baseBlock := block.Block{
		Height:       h,
		ParentWeight: w,
		StateRoot:    newCid(),
	}
	baseTipSet := block.RequireNewTipSet(t, &baseBlock)
	ticket := mining.NthTicket(7)
	blk, err := worker.Generate(ctx, baseTipSet, ticket, consensus.MakeFakeVRFProofForTest(), 0, consensus.MakeFakePoStsForTest(), nil)
	assert.NoError(t, err)

	assert.Equal(t, h+1, blk.Header.Height)
	assert.Equal(t, minerAddr, blk.Header.Miner)
	assert.Equal(t, ticket, blk.Header.Ticket)

	blk, err = worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeVRFProofForTest(), 1, consensus.MakeFakePoStsForTest(), nil)
	assert.NoError(t, err)

	assert.Equal(t, h+2, blk.Header.Height)
	assert.Equal(t, fbig.Add(w, fbig.NewInt(10.0)), blk.Header.ParentWeight)
	assert.Equal(t, minerAddr, blk.Header.Miner)
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

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     messages,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	assert.Len(t, pool.Pending(), 0)
	baseBlock := block.Block{
		Parents:   block.NewTipSetKey(newCid()),
		Height:    100,
		StateRoot: newCid(),
	}
	blk, err := worker.Generate(ctx, block.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeVRFProofForTest(), 0, consensus.MakeFakePoStsForTest(), nil)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0) // This is the temporary failure.
	txMeta, err := messages.LoadTxMeta(ctx, blk.Header.Messages.Cid)
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

		MessageSource:    pool,
		MessageQualifier: &mining.NoMessageQualifier{},
		Blockstore:       bs,
		MessageStore:     messages,
		Clock:            clock.NewChainClock(100000000, 30*time.Second, 6*time.Second),
	})

	// This is actually okay and should result in a receipt
	msg := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), gas.Unit(0))
	smsg, err := types.NewSignedMessage(ctx, *msg, &mockSigner)
	require.NoError(t, err)
	_, err = pool.Add(ctx, smsg, 0)
	require.NoError(t, err)

	assert.Len(t, pool.Pending(), 1)
	baseBlock := block.Block{
		Parents:   block.NewTipSetKey(newCid()),
		Height:    100,
		StateRoot: newCid(),
	}
	baseTipSet := block.RequireNewTipSet(t, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeVRFProofForTest(), 0, consensus.MakeFakePoStsForTest(), nil)
	assert.Error(t, err, "boom")
	assert.Nil(t, blk.Header)

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
