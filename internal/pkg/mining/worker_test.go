package mining_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dag "github.com/ipfs/go-merkledag"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestLookbackElection(t *testing.T) {
	tf.UnitTest(t)

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	builder := chain.NewBuilder(t, address.Undef)
	lookback := consensus.ElectionLookback
	head := builder.AppendManyOn(lookback-1, builder.NewGenesis())
	ancestors := builder.RequireTipSets(head.Key(), lookback)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return ancestors, nil
	}

	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	messages := chain.NewMessageStore(bs)

	t.Run("Election sees ticket lookback ancestors back", func(t *testing.T) {
		electionTicket, err := ancestors[lookback-1].MinTicket()
		require.NoError(t, err)
		mem := consensus.NewMockElectionMachine(func(ticket block.Ticket) {
			assert.Equal(t, electionTicket, ticket)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			GetAncestors:   getAncestors,
			Election:       mem,
			TicketGen:      &consensus.FakeTicketMachine{},

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, head, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
	})

	t.Run("Ticket gensees ticket 1 ancestor back", func(t *testing.T) {
		genTicket, err := ancestors[0].MinTicket()
		require.NoError(t, err)
		mtm := consensus.NewMockTicketMachine(func(ticket block.Ticket) {
			assert.Equal(t, genTicket, ticket)
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			GetAncestors:   getAncestors,
			Election:       &consensus.FakeElectionMachine{},
			TicketGen:      mtm,

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, head, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
	})

}

func Test_Mine(t *testing.T) {
	tf.UnitTest(t)

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: stateRoot, Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := th.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return []block.TipSet{tipSet}, nil
	}

	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	messages := chain.NewMessageStore(bs)

	// TODO #3311: this case isn't testing much.  Testing w.Mine further needs a lot more attention.
	t.Run("Trivial success case", func(t *testing.T) {
		ticketGen := false
		sawTicket := func(_ block.Ticket) {
			ticketGen = true
		}
		testTicketGen := consensus.NewMockTicketMachine(sawTicket)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			GetAncestors:   getAncestors,
			Election:       &consensus.FakeElectionMachine{},
			TicketGen:      testTicketGen,

			MessageSource: pool,
			Processor:     th.NewFakeProcessor(),
			Blockstore:    bs,
			MessageStore:  messages,
			Clock:         clock.NewSystemClock(),
		})

		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
		assert.True(t, ticketGen)
	})

	t.Run("Block generation fails", func(t *testing.T) {
		ticketGen := false
		sawTicket := func(_ block.Ticket) {
			ticketGen = true
		}
		testTicketGen := consensus.NewMockTicketMachine(sawTicket)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{shouldError: true},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			GetAncestors:   getAncestors,
			Election:       &consensus.FakeElectionMachine{},
			TicketGen:      testTicketGen,

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
		assert.True(t, ticketGen)
	})

	t.Run("Sent empty tipset", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ticketGen := false
		sawTicket := func(_ block.Ticket) {
			ticketGen = true
		}
		testTicketGen := consensus.NewMockTicketMachine(sawTicket)
		worker := mining.NewDefaultWorker(mining.WorkerParameters{
			API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			WorkerSigner:   mockSigner,

			TipSetMetadata: fakeTSMetadata{},
			GetStateTree:   getStateTree,
			GetWeight:      getWeightTest,
			GetAncestors:   getAncestors,
			Election:       &consensus.FakeElectionMachine{},
			TicketGen:      testTicketGen,

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
		assert.False(t, ticketGen)
	})
}

func sharedSetupInitial() (*hamt.CborIpldStore, *message.Pool, cid.Cid) {
	cst := hamt.NewCborStore()
	pool := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T, mockSigner types.MockSigner) (
	state.Tree, *message.Pool, []address.Address, blockstore.Blockstore) {

	cst, pool, fakeActorCodeCid := sharedSetupInitial()
	vms := th.VMStorage()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	// Now tracking in #3311
	addr1, addr2, addr3, addr4, addr5 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[3], mockSigner.Addresses[4]
	act1 := th.RequireNewFakeActor(t, vms, addr1, fakeActorCodeCid)
	act2 := th.RequireNewFakeActor(t, vms, addr2, fakeActorCodeCid)
	fakeNetAct := th.RequireNewFakeActorWithTokens(t, vms, addr3, fakeActorCodeCid, types.NewAttoFILFromFIL(1000000))
	minerAct := th.RequireNewMinerActor(t, vms, addr4, addr5, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
	minerOwner := th.RequireNewFakeActor(t, vms, addr5, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward failures.
		address.NetworkAddress: fakeNetAct,

		addr1: act1,
		addr2: act2,
		addr4: minerAct,
		addr5: minerOwner,
	})
	return st, pool, []address.Address{addr1, addr2, addr3, addr4, addr5}, bs
}

// TODO this test belongs in core, it calls ApplyMessages #3311
func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	tf.UnitTest(t)

	vms := th.VMStorage()

	mockSigner, _ := setupSigner()
	cst, _, fakeActorCodeCid := sharedSetupInitial()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := th.RequireNewFakeActor(t, vms, addr1, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(1000000)),
		addr1:                  act1,
	})

	ctx := context.Background()

	// NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// exercise the categorization.
	// addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg0 := types.NewMeteredMessage(addr2, addr1, 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))

	// This is actually okay and should result in a receipt
	msg1 := types.NewMeteredMessage(addr1, addr2, 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg2 := types.NewMeteredMessage(addr1, addr1, 1, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))
	msg3 := types.NewMeteredMessage(addr2, addr2, 1, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))

	messages := []*types.UnsignedMessage{msg0, msg1, msg2, msg3}

	res, err := consensus.NewDefaultProcessor().ApplyMessagesAndPayRewards(ctx, st, vms, messages, addr1, types.NewBlockHeight(0), nil)
	assert.NoError(t, err)
	require.NotNil(t, res)

	assert.Error(t, res[0].Failure)
	assert.False(t, res[0].FailureIsPermanent)

	assert.Nil(t, res[1].Failure)

	assert.Error(t, res[2].Failure)
	assert.True(t, res[2].FailureIsPermanent)
	assert.Error(t, res[3].Failure)
	assert.True(t, res[3].FailureIsPermanent)
}

func TestApplyBLSMessages(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ki := types.MustGenerateMixedKeyInfo(5, 5)
	mockSignerVal := types.NewMockSigner(ki)
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &block.Block{Height: 0, StateRoot: stateRoot, Ticket: block.Ticket{VRFProof: []byte{0}}}
	tipSet := th.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return []block.TipSet{tipSet}, nil
	}

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
		_, err := pool.Add(ctx, smsg, uint64(0))
		require.NoError(t, err)
	}

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(mockSigner.Addresses[5]),

		MinerAddr:      addrs[3],
		MinerOwnerAddr: addrs[4],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
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
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages)
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
		secpMessages, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages)
		require.NoError(t, err)

		assert.Len(t, secpMessages, 5)
		assert.Len(t, blsMessages, 5)
	})

	t.Run("block bls signature can be used to validate messages", func(t *testing.T) {
		digests := []bls.Digest{}
		keys := []bls.PublicKey{}

		_, blsMessages, err := msgStore.LoadMessages(ctx, block.Messages)
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
		copy(blsSig[:], block.BLSAggregateSig)
		valid := bls.Verify(&blsSig, digests, keys)

		assert.True(t, valid)
	})
}

func requireSignedMessage(t *testing.T, signer types.Signer, from, to address.Address, nonce uint64, value types.AttoFIL) *types.SignedMessage {
	msg := types.NewMeteredMessage(from, to, nonce, value, types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(1), 300)
	smsg, err := types.NewSignedMessage(*msg, signer)
	require.NoError(t, err)
	return smsg
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	mockSigner, blockSignerAddr := setupSigner()
	st, pool, addrs, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}

	minerAddr := addrs[4]
	minerOwnerAddr := addrs[3]

	messages := chain.NewMessageStore(bs)

	meta := fakeTSMetadata{}
	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: meta,
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
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

	blk, err := worker.Generate(ctx, baseTipset, block.Ticket{VRFProof: []byte{2}}, consensus.MakeFakeElectionProofForTest(), 0)

	assert.NoError(t, err)

	assert.Equal(t, types.EmptyMessagesCID, blk.Messages.SecpRoot)

	expectedStateRoot, err := meta.GetTipSetStateRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedStateRoot, blk.StateRoot)

	expectedReceipts, err := meta.GetTipSetReceiptsRoot(parentTipset.Key())
	require.NoError(t, err)
	assert.Equal(t, expectedReceipts, blk.MessageReceipts)

	assert.Equal(t, types.Uint64(101), blk.Height)
	assert.Equal(t, types.Uint64(120), blk.ParentWeight)
	assert.Equal(t, block.Ticket{VRFProof: []byte{2}}, blk.Ticket)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMeteredMessage(addrs[2], addrs[0], 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(t, err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(t, err)

	// add the following and then increment the actor nonce at addrs[1], nonceTooLow, a permanent error.
	msg3 := types.NewMeteredMessage(addrs[1], addrs[0], 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner)
	require.NoError(t, err)

	msg4 := types.NewMeteredMessage(addrs[1], addrs[2], 1, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))
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
	act, err := st.GetActor(ctx, addrs[1])
	require.NoError(t, err)

	act.Nonce = types.Uint64(2)
	err = st.SetActor(ctx, addrs[1], act)
	require.NoError(t, err)

	stateRoot, err := st.Flush(ctx)
	require.NoError(t, err)

	baseBlock := block.Block{
		Parents:       block.NewTipSetKey(newCid()),
		Height:        types.Uint64(100),
		StateRoot:     stateRoot,
		ElectionProof: consensus.MakeFakeElectionProofForTest(),
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeElectionProofForTest(), 0)
	assert.NoError(t, err)

	// This is the temporary failure + the good message,
	// which will be removed by the node if this block is accepted.
	assert.Len(t, pool.Pending(), 2)
	assert.Contains(t, pool.Pending(), smsg1)
	assert.Contains(t, pool.Pending(), smsg2)

	// message and receipts can be loaded from message store and have
	// length 1.
	msgs, _, err := messages.LoadMessages(ctx, blk.Messages)
	require.NoError(t, err)
	assert.Len(t, msgs, 1) // This is the good message
}

func TestGenerateSetsBasicFields(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}
	minerAddr := addrs[4]
	minerOwnerAddr := addrs[3]

	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	h := types.Uint64(100)
	w := types.Uint64(1000)
	baseBlock := block.Block{
		Height:        h,
		ParentWeight:  w,
		StateRoot:     newCid(),
		ElectionProof: consensus.MakeFakeElectionProofForTest(),
	}
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	ticket := mining.NthTicket(7)
	blk, err := worker.Generate(ctx, baseTipSet, ticket, consensus.MakeFakeElectionProofForTest(), 0)
	assert.NoError(t, err)

	assert.Equal(t, h+1, blk.Height)
	assert.Equal(t, minerAddr, blk.Miner)
	assert.Equal(t, ticket, blk.Ticket)

	blk, err = worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeElectionProofForTest(), 1)
	assert.NoError(t, err)

	assert.Equal(t, h+2, blk.Height)
	assert.Equal(t, w+10.0, blk.ParentWeight)
	assert.Equal(t, minerAddr, blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(bs)

	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	assert.Len(t, pool.Pending(), 0)
	baseBlock := block.Block{
		Parents:       block.NewTipSetKey(newCid()),
		Height:        types.Uint64(100),
		StateRoot:     newCid(),
		ElectionProof: consensus.MakeFakeElectionProofForTest(),
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeElectionProofForTest(), 0)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0) // This is the temporary failure.

	assert.Equal(t, types.EmptyMessagesCID, blk.Messages.SecpRoot)
	assert.Equal(t, types.EmptyMessagesCID, blk.Messages.BLSRoot)
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, ts block.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts block.TipSet, newBlockHeight *types.BlockHeight) ([]block.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(bs)
	worker := mining.NewDefaultWorker(mining.WorkerParameters{
		API: th.NewDefaultFakeWorkerPorcelainAPI(blockSignerAddr),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		WorkerSigner:   mockSigner,

		TipSetMetadata: fakeTSMetadata{shouldError: true},
		GetStateTree:   getStateTree,
		GetWeight:      getWeightTest,
		GetAncestors:   getAncestors,
		Election:       &consensus.FakeElectionMachine{},
		TicketGen:      &consensus.FakeTicketMachine{},

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		Blockstore:    bs,
		MessageStore:  messages,
		Clock:         th.NewFakeClock(time.Unix(1234567890, 0)),
	})

	// This is actually okay and should result in a receipt
	msg := types.NewMeteredMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, types.SendMethodID, nil, types.NewGasPrice(0), types.NewGasUnits(0))
	smsg, err := types.NewSignedMessage(*msg, &mockSigner)
	require.NoError(t, err)
	_, err = pool.Add(ctx, smsg, 0)
	require.NoError(t, err)

	assert.Len(t, pool.Pending(), 1)
	baseBlock := block.Block{
		Parents:       block.NewTipSetKey(newCid()),
		Height:        types.Uint64(100),
		StateRoot:     newCid(),
		ElectionProof: consensus.MakeFakeElectionProofForTest(),
	}
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, block.Ticket{VRFProof: []byte{0}}, consensus.MakeFakeElectionProofForTest(), 0)
	assert.Error(t, err, "boom")
	assert.Nil(t, blk)

	assert.Len(t, pool.Pending(), 1) // No messages are removed from the pool.
}

func getWeightTest(_ context.Context, ts block.TipSet) (uint64, error) {
	w, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}
	// consensus.ecV = 10
	return w + uint64(ts.Len())*10, nil
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
