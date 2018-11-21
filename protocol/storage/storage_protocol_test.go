package storage_test

import (
	"context"
	"sync"
	"testing"
	"time"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	unixfs "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	mactor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/node"
	. "github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializeProposal(t *testing.T) {
	t.Parallel()

	p := &DealProposal{}
	p.Size = types.NewBytesAmount(5)
	v, _ := cid.Decode("QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk")
	p.PieceRef = v
	_, err := cbor.DumpObject(p)
	if err != nil {
		t.Fatal(err)
	}
}

// TODO: we need to really rethink how this sort of testing can be done
// cleaner. The gengen stuff helps, but its still difficult to make actor
// method invocations
func TestStorageProtocolBasic(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	seed := node.MakeChainSeed(t, node.TestGenCfg)

	// make two nodes, one of which is the miner (and gets the miner peer key)
	miner := node.NodeWithChainSeed(t, seed, node.PeerKeyOpt(node.PeerKeys[0]), node.AutoSealIntervalSecondsOpt(1))
	client := node.NodeWithChainSeed(t, seed)
	minerAPI := impl.New(miner)

	// Give the miner node the right private key, and set them up with
	// the miner actor
	seed.GiveKey(t, miner, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, miner, 0)

	seed.GiveKey(t, client, 1)

	cni := NewClientNodeImpl(
		dag.NewDAGService(client.BlockService()),
		client.Host(),
		client.Lookup(),
		func(_ context.Context, _ address.Address, _ string, _ []byte, _ *address.Address) ([][]byte, uint8, error) {
			// This is only used for getting the price of an ask.
			a := &mactor.Ask{
				Price: types.NewAttoFILFromFIL(50),
			}

			enc, err := cbor.DumpObject(a)
			if err != nil {
				return nil, 0, err
			}

			return [][]byte{enc}, 0, nil
		},
	)
	c := NewClient(cni)
	m, err := NewMiner(ctx, mineraddr, minerOwnerAddr, miner)
	assert.NoError(err)
	_ = m

	assert.NoError(miner.Start(ctx))
	assert.NoError(client.Start(ctx))

	node.ConnectNodes(t, miner, client)
	err = minerAPI.Mining().Start(ctx)
	assert.NoError(err)
	defer minerAPI.Mining().Stop(ctx)

	sectorSize, err := miner.SectorBuilder().GetMaxUserBytesPerStagedSector()
	require.NoError(err)

	data := unixfs.NewFSNode(unixfs.TFile)
	bytes := make([]byte, sectorSize)
	for i := 0; uint64(i) < sectorSize; i++ {
		bytes[i] = byte(i)
	}
	data.SetData(bytes)

	raw, err := data.GetBytes()
	assert.NoError(err)
	protonode := dag.NodeWithData(raw)

	assert.NoError(client.BlockService().AddBlock(protonode))

	var foundCommit bool
	var foundPoSt bool

	var wg sync.WaitGroup
	wg.Add(2)

	old := miner.AddNewlyMinedBlock
	var bCount, mCount int
	miner.AddNewlyMinedBlock = func(ctx context.Context, blk *types.Block) {
		bCount++
		mCount += len(blk.Messages)
		old(ctx, blk)

		if !foundCommit {
			for i, msg := range blk.Messages {
				if msg.Message.Method == "commitSector" {
					assert.False(foundCommit, "multiple commitSector submissions must not happen")
					assert.Equal(uint8(0), blk.MessageReceipts[i].ExitCode, "seal submission failed")
					foundCommit = true
					wg.Done()
				}
			}
		}
		if !foundPoSt {
			for i, msg := range blk.Messages {
				if msg.Message.Method == "submitPoSt" {
					assert.False(foundPoSt, "multiple post submissions must not happen")
					assert.Equal(uint8(0), blk.MessageReceipts[i].ExitCode, "post submission failed")
					foundPoSt = true
					wg.Done()
				}
			}
		}
	}

	ref, err := c.ProposeDeal(ctx, mineraddr, protonode.Cid(), 1, 150)
	assert.NoError(err)
	requireQueryDeal := func() (DealState, string) {
		resp, err := c.QueryDeal(ctx, ref.Proposal)
		require.NoError(err)
		return resp.State, resp.Message
	}

	time.Sleep(time.Millisecond * 100) // Bad dignifiedquire, bad!
	var done bool
	for i := 0; i < 5; i++ {
		state, message := requireQueryDeal()
		assert.NotEqual(Failed, state, message)
		if state == Staged {
			done = true
			break
		}
		time.Sleep(time.Millisecond * 500)
	}

	require.True(done)
	if waitTimeout(&wg, 120*time.Second) {
		state, message := requireQueryDeal()
		require.NotEqual(Failed, state, message)
		assert.Failf("TestStorageProtocolBasic failed", "waiting for submission timed out. Saw %d blocks with %d messages while waiting", bCount, mCount)
	}
	require.True(foundCommit, "no commitSector on chain")
	require.True(foundPoSt, "no submitPoSt on chain")

	// Now all things should be ready
	done = false
	for i := 0; i < 10; i++ {
		resp, err := c.QueryDeal(ctx, ref.Proposal)
		assert.NoError(err)
		assert.NotEqual(Failed, resp.State, resp.Message)
		if resp.State == Posted {
			done = true
			assert.True(resp.ProofInfo.SectorID > 0)
			break
		}
		time.Sleep(time.Millisecond * 500)
	}

	assert.True(done, "failed to finish transfer")
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
