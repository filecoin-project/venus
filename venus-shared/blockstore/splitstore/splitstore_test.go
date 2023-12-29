package splitstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
	cid "github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestNewSplitstore(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	mockTipset := []*types.TipSet{
		newTipSet(100),
		newTipSet(200),
		newTipSet(300),
		newTipSet(400),
		newTipSet(500),
	}

	initStore, err := openStore(tempDir + "/badger.db")
	require.NoError(t, err)

	var tskCids []cid.Cid
	for i, ts := range mockTipset {
		tsk := ts.Key()
		tskCid, err := tsk.Cid()
		require.NoError(t, err)
		tskCids = append(tskCids, tskCid)

		s, err := newLayer(tempDir, int64(100+i*100), tskCid)
		require.NoError(t, err)

		rawBlk, err := ts.Blocks()[0].ToStorageBlock()
		require.NoError(t, err)
		err = s.Put(ctx, rawBlk)
		require.NoError(t, err)

		rawTsk, err := tsk.ToStorageBlock()
		require.NoError(t, err)
		err = s.Put(ctx, rawTsk)
		require.NoError(t, err)

		if s, ok := s.Blockstore.(Closer); ok {
			err := s.Close()
			require.NoError(t, err)
		}
	}

	ss, err := NewSplitstore(tempDir, initStore)
	require.NoError(t, err)
	require.Equal(t, 5, ss.layers.Len())
}

func TestSethead(t *testing.T) {
	tempDir := t.TempDir()

	ss, err := NewSplitstore(tempDir, nil)
	require.NoError(t, err)

	var c1, c2 cid.Cid
	c2 = cid.MustParse("bafy2bzacedqrlux7zaeaoka7b5udzwvdguzf3vqsgxglrdtakislgofte3ehi")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c1 = ss.nextHead()
	}()

	runtime.Gosched()
	ss.setHead(c2)
	ss.setHead(c2)

	wg.Wait()
	fmt.Println(c1, c2)
	require.True(t, c1.Equals(c2))
}

func TestScan(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	mockTipset := []*types.TipSet{
		newTipSet(100),
		newTipSet(200),
		newTipSet(300),
	}

	var tskCids []cid.Cid
	for i, ts := range mockTipset {
		tsk := ts.Key()
		tskCid, err := tsk.Cid()
		require.NoError(t, err)
		tskCids = append(tskCids, tskCid)

		s, err := newLayer(tempDir, int64(100+i*100), tskCid)
		require.NoError(t, err)

		rawBlk, err := ts.Blocks()[0].ToStorageBlock()
		require.NoError(t, err)
		err = s.Put(ctx, rawBlk)
		require.NoError(t, err)

		rawTsk, err := tsk.ToStorageBlock()
		require.NoError(t, err)
		err = s.Put(ctx, rawTsk)
		require.NoError(t, err)

		if s, ok := s.Blockstore.(Closer); ok {
			err := s.Close()
			require.NoError(t, err)
		}
	}

	// any.db will not be scanned in
	err := os.MkdirAll(filepath.Join(tempDir, "any.db"), 0777)
	require.NoError(t, err)

	bs, err := scan(tempDir)
	require.NoError(t, err)

	t.Run("scan in", func(t *testing.T) {
		require.Len(t, bs, len(mockTipset))

		for i, c := range tskCids {
			require.Equal(t, c, bs[i].Base())
		}
	})

	t.Run("clean up", func(t *testing.T) {
		for i := range bs {
			store := bs[i]
			err := store.Clean()
			require.NoError(t, err)
		}

		bs, err = scan(tempDir)
		require.NoError(t, err)
		require.Len(t, bs, 0)
	})
}

func TestExtractHeightAndCid(t *testing.T) {
	h, _, err := extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6.db")
	require.NoError(t, err)
	require.Equal(t, int64(10), h)

	_, _, err = extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6")
	require.Error(t, err)

	_, _, err = extractHeightAndCid("base_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6")
	require.Error(t, err)

	_, _, err = extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6.db.del")
	require.Error(t, err)
}

const blkRaw = `{"Miner":"t038057","Ticket":{"VRFProof":"kfggWR2GcEbfTuJ20hkAFNRbF7xusDuAQR7XwTjJ2/gc1rwIDmaXbSVxXe4j1njcCBoMhmlYIn9D/BLqQuIOayMHPYvDmOJGc9M27Hwg1UZkiuJmXji+iM/JBNYaOA61"},"ElectionProof":{"WinCount":1,"VRFProof":"tI7cWWM9sGsKc69N9DjN41glaO5Hg7r742H56FPzg7szbhTrxj8kw0OsiJzcPJdiAa6D5jZ1S2WKoLK7lwg2R5zYvCRwwWLGDiExqbqsvqmH5z/e6YGpaD7ghTPRH1SR"},"BeaconEntries":[{"Round":2118576,"Data":"rintMKcvVAslYpn9DcshDBmlPN6hUR+wjvVQSkkVUK5klx1kOSpcDvzODSc2wXFQA7BVbEcXJW/5KLoL0KHx2alLUWDOwxhsIQnAydXdZqG8G76nTIgogthfIMgSGdB2"}],"WinPoStProof":[{"PoStProof":3,"ProofBytes":"t0ZgPHFv0ao9fVZJ/fxbBrzATmOiIv9/IueSyAjtcqEpxqWViqchaaxnz1afwzAbhahpfZsGiGWyc408WYh7Q8u0Aa52KGPmUNtf3pAvxWfsUDMz9QUfhLZVg/p8/PUVC/O/E7RBNq4YPrRK5b6Q8PVwzIOxGOS14ge6ys8Htq+LfNJbcqY676qOYF4lzMuMtQIe3CxMSAEaEBfNpHhAEs83dO6vll9MZKzcXYpNWeqmMIz4xSdF18StQq9vL/Lo"}],"Parents":[{"/":"bafy2bzacecf4wtqz3kgumeowhdulejk3xbfzgibfyhs42x4vx2guqgudem2hg"},{"/":"bafy2bzacebkpxh2k63xreigl6a3ggdr2adwk67b4zw5dddckhqex2tmha6hee"},{"/":"bafy2bzacecor3xq4ykmhhrgq55rdo5w7up65elc4qwx5uwjy25ffynidskbxw"},{"/":"bafy2bzacedr2mztmef65fodqzvyjcdnsgpcjthstseinll4maqg24avnv7ljo"}],"ParentWeight":"21779626255","Height":1164251,"ParentStateRoot":{"/":"bafy2bzacecypgutbewmyop2wfuafvxt7dm7ew4u3ssy2p4rn457f6ynrj2i6a"},"ParentMessageReceipts":{"/":"bafy2bzaceaflsspsxuxew2y4g6o72wp5i2ewp3fcolga6n2plw3gycam7s4lg"},"Messages":{"/":"bafy2bzaceanux5ivzlxzvhqxtwc5vkktcfqepubwtwgv26dowzbl3rtgqk54k"},"BLSAggregate":{"Type":2,"Data":"lQg9jBfYhY2vvjB/RPlWg6i+MBTlH1u0lmdasiab5BigsKAuZSeLNlTGbdoVZhAsDUT59ZdGsMmueHjafygDUN2KLhZoChFf6LQHH42PTSXFlkRVHvmKVz9DDU03FLMB"},"Timestamp":1658988330,"BlockSig":{"Type":2,"Data":"rMOv2tXKqV5VDOq5IQ35cP0cCAzGmaugVr/g5JTrilhAn4LYK0h6ByPL5cX5ONzlDTx9+zYZFteIzaenirZhw7G510Lh0J8lbTLP5X2EX251rEA8dpkPZPcNylzN0r8X"},"ForkSignaling":0,"ParentBaseFee":"100"}`

func newTipSet(height abi.ChainEpoch) *types.TipSet {
	var blk types.BlockHeader
	err := json.Unmarshal([]byte(blkRaw), &blk)
	if err != nil {
		panic(err)
	}
	blk.Height = height

	ts, err := types.NewTipSet([]*types.BlockHeader{&blk})
	if err != nil {
		panic(err)
	}
	return ts
}
