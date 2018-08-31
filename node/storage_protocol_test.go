package node

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"
)

func TestSerializeProposal(t *testing.T) {
	p := &StorageDealProposal{}
	p.Size = types.NewBytesAmount(5)
	v, _ := cid.Decode("QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk")
	p.PieceRef = v
	_, err := cbor.DumpObject(p)
	if err != nil {
		t.Fatal(err)
	}
}

func mustGenKey(seed int64) crypto.PrivKey {
	r := rand.New(rand.NewSource(seed))
	priv, _, err := crypto.GenerateEd25519Key(r)
	if err != nil {
		panic(err)
	}

	return priv
}

func mustPeerID(k crypto.PrivKey) peer.ID {
	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		panic(err)
	}
	return pid
}

var peerKeys = []crypto.PrivKey{
	mustGenKey(101),
}

var testGenCfg = &gengen.GenesisCfg{
	Keys: []string{"foo", "bar"},
	Miners: []gengen.Miner{
		{
			Owner:  "foo",
			Power:  100,
			PeerID: mustPeerID(peerKeys[0]).Pretty(),
		},
	},
	PreAlloc: map[string]string{
		"foo": "10000",
		"bar": "10000",
	},
}

func TestStorageProtocolBasic(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	ctx := context.Background()

	seed := MakeChainSeed(t, testGenCfg)

	// make two nodes, one of which is the miner (and gets the miner peer key)
	miner := NodeWithChainSeed(t, seed, PeerKeyOpt(peerKeys[0]))
	client := NodeWithChainSeed(t, seed)

	// Give the miner node the right private key, and set them up with
	// the miner actor
	seed.GiveKey(t, miner, "foo")
	mineraddr := seed.GiveMiner(t, miner, 0)

	seed.GiveKey(t, client, "bar")

	c := NewStorageMinerClient(client)
	m := NewStorageMiner(miner)
	_ = m

	assert.NoError(miner.Start(ctx))
	assert.NoError(client.Start(ctx))

	ConnectNodes(t, miner, client)

	data := dag.NewRawNode([]byte("cats"))

	err := client.Blockservice.AddBlock(data)
	assert.NoError(err)

	ref, err := c.TryToStoreData(ctx, mineraddr, data.Cid(), 10, types.NewAttoFILFromFIL(60))
	assert.NoError(err)

	time.Sleep(time.Millisecond * 100) // Bad whyrusleeping, bad!

	resp, err := c.Query(ctx, ref)
	assert.NoError(err)

	assert.Equal(Complete, resp.State)
}
