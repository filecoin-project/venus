package splitstore

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestSplitstore(t *testing.T) {

	ctx := context.Background()

	path := "/root/tanlang/venus/.vscode/root/.venus_1206_1154800/splitstore_copy"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus_1206_1154800/badger_copy")
	require.NoError(t, err)

	c1 := cid.MustParse("bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6")

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	err = WalkHeader(ctx, initStore, c1, 0)
	require.NoError(t, err)
}

func TestBadgerSync(t *testing.T) {
	p := "/root/tanlang/venus/.vscode/badger"
	s, err := openStore(p)
	require.NoError(t, err)
	testBlock := newBlock("test")

	has, err := s.Has(context.Background(), testBlock.Cid())
	require.NoError(t, err)
	fmt.Println(has)
	if !has {
		err = s.Put(context.Background(), testBlock)
		require.NoError(t, err)
	}
}

func TestHeadChange(t *testing.T) {
	ctx := context.Background()

	path := "/root/tanlang/venus/.vscode/root/badger"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus_1212_1152279/badger_copy")
	require.NoError(t, err)

	c1 := cid.MustParse("bafy2bzaced33mlvgpaxydgywwl7f5poent5bkcwa6hbnrd5547lroo5nxrh2m")

	has, err := initStore.Has(ctx, c1)
	require.NoError(t, err)
	fmt.Printf("check tsk exist %v\n", has)
	if has {
		return
	}

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	tsk, err := ss.getTipsetKey(ctx, c1)
	require.NoError(t, err)
	ts, err := ss.getTipset(ctx, tsk)
	require.NoError(t, err)

	tempPath := t.TempDir()
	tempSs, err := NewSplitstore(tempPath, initStore, opt)
	require.NoError(t, err)

	err = tempSs.HeadChange(nil, []*types.TipSet{ts})
	require.NoError(t, err)

	has, err = initStore.Has(ctx, c1)
	require.NoError(t, err)
	require.True(t, has)
}

func TestDiff(t *testing.T) {

	ctx := context.Background()

	path := "/root/.venus/splitstore"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus/badger")
	require.NoError(t, err)

	c1 := cid.MustParse("bafy2bzacedmwacbii4vmc5fmb5x6dt75nwkodzulxxdgsvjxz3yxy4c3arijs")

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	// tempPath := t.TempDir()
	// tempSs, err := NewSplitstore(tempPath, initStore, opt)
	// require.NoError(t, err)

	tsk, err := ss.getTipsetKey(ctx, c1)
	require.NoError(t, err)
	fmt.Println(tsk)

	blk, err := ss.getBlock(ctx, tsk.Cids()[0])
	require.NoError(t, err)
	nTsk := types.NewTipSetKey(blk.Parents...)

	// ts, err := ss.getTipset(ctx, tsk)
	// require.NoError(t, err)

	// err = tempSs.HeadChange(nil, []*types.TipSet{ts})
	// require.NoError(t, err)

	c, err := nTsk.Cid()
	require.NoError(t, err)

	cids, err := diffFrom(ctx, c, ss, initStore)
	require.NoError(t, err)
	fmt.Println(cids)
	fmt.Printf("has %d different blocks\n", len(cids))
}

func TestWalkHead(t *testing.T) {
	ctx := context.Background()

	initStore, err := openStore("/root/.venus/badger")
	require.NoError(t, err)

	c1 := cid.MustParse("bafy2bzacecp37mrr74cqagjhirt5u23dgltegcinzv7ssbubajkgcir7dsnlw")

	v := NewSyncVisitor()
	// try to walk chain
	err = walkChain(ctx, initStore, c1, v, 1165000, false)
	require.NoError(t, err)
}

func TestStoreOperate(t *testing.T) {
	ctx := context.Background()

	initStore, err := openStore("/root/.venus/badger")
	require.NoError(t, err)

	c1 := cid.MustParse("bafy2bzacebeg3tctrxgxwho2hmv4oasgeuqldd4uuvmig3ot6zizlpmhmbiie")

	has, err := initStore.Has(ctx, c1)
	require.NoError(t, err)
	require.True(t, has)

	v := NewSyncVisitor()
	v.RegisterVisitHook(func(c cid.Cid) bool {
		if v.Len() > 100 {
			return false
		}
		return true
	})

	walkObject(ctx, initStore, c1, v)
}

func TestSplitstoreOperate(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus/splitstore_1214"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus/badger")
	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzacecbjk73ouvygfayane4w7653vfz7pi335tqyaut6gji2shy6tdqhi")

	has, err := ss.Has(ctx, c1)
	require.NoError(t, err)
	require.True(t, has)
}

func TestSyncState(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus/splitstore_back"
	opt := Option{
		MaxLayerCount: 30,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus/badger")
	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)
	c1 := cid.MustParse("bafy2bzacedho6cope4fp5sss43t4swtvc6its5upv22yawtivnic7rkbudmcu")

	ss.layers.ForEach(func(idx int, s Layer) bool {
		has, err := s.Has(ctx, c1)
		require.NoError(t, err)
		if has {
			fmt.Printf("store %d has %s\n", idx, c1)
		}
		return true
	})

	has, err := ss.Has(ctx, c1)
	require.NoError(t, err)
	require.True(t, has)
}

func ExampleSplitstore() {
	ctx := context.Background()

	path := "/root/.venus/splitstore"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus/badger")
	if err != nil {
		return
	}

	ss, err := NewSplitstore(path, initStore, opt)
	if err != nil {
		return
	}
	c1 := cid.MustParse("bafy2bzacecbsjrqcq4gigx6cluazarota6466vqwasaqhdd7oxrqogd5k5p2q")

	has, err := ss.Has(ctx, c1)
	if err != nil {
		return
	}
	if has {
		fmt.Println("has")
	}

	tsk, err := ss.getTipsetKey(ctx, c1)
	if err != nil {
		log.Errorf("get tipset key error %v", err)
		return
	}
	v := NewSyncVisitor()
	v.RegisterErrorHook(func(c cid.Cid, err error) {
		log.Warnf("visit error %v", err)
	})

	next := tsk
	for i := 0; i < 100; i++ {
		next = walkTipset(ctx, ss, next, v)
	}

	fmt.Println(runtime.NumCPU())
	// Output:
	// has
	// 32
}

func ExampleTrace() {
	ctx := context.Background()

	path := "/root/.venus/splitstore"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus/badger")
	if err != nil {
		return
	}

	ss, err := NewSplitstore(path, initStore, opt)
	if err != nil {
		return
	}
	c1 := cid.MustParse("bafy2bzaceb4fkaq2mxkhpw463fz5ur3ixj37ybjcacs2lfxy54jq2eds7ayoc")

	target := cid.MustParse("bafy2bzacebnm4ehs3mx644nedilcf55flmei5entdpxiomahxrdcmacfokyl6")

	v := NewSyncVisitor()
	tsk, err := ss.getTipsetKey(ctx, c1)
	if err != nil {
		log.Errorf("get tipset key error %v", err)
		return
	}

	objCid := scanTipset(ctx, ss, tsk, v)
	log.Infoln("res :", objCid)

	for _, c := range objCid {
		if c.Equals(target) {
			log.Infof("find target %s", c)
			break
		}
		route := traceObject(ctx, ss, target, []cid.Cid{c}, v)
		if route != nil {
			log.Infof("find target %s", c)
			log.Infoln(route)
			break
		}
	}

	fmt.Println("ok")
	// Output:
	// ok

}

func TestComposeWalk(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus_back/splitstore"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	initStore, err := openStore("/root/.venus_back/badger")
	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzacedmwacbii4vmc5fmb5x6dt75nwkodzulxxdgsvjxz3yxy4c3arijs")

	cs := NewComposeStore(ss.composeStore(), initStore)

	has, err := ss.Has(ctx, c1)
	require.NoError(t, err)
	require.Truef(t, has, "split store")

	has, err = cs.Has(ctx, c1)
	require.NoError(t, err)
	require.Truef(t, has, "compose store")

	// WalkHeader(ctx, cs, c1, 1151800)
	v := NewSyncVisitor()
	err = walkChain(ctx, ss, c1, v, 1151800, false)
}

func TestCid(t *testing.T) {
	c := cid.Cid{}
	fmt.Println(c)
	fmt.Println(c.String())

}

func TestScanObjectDeep(t *testing.T) {
	ctx := context.Background()

	// path := "/root/tanlang/venus/.vscode/root/.venus/splitstore"
	path := "/root/.venus/splitstore"
	opt := Option{
		MaxLayerCount: 10,
		LayerSize:     1000,
	}

	// initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus/badger")
	initStore, err := openStore("/root/.venus/badger")

	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzacealcviqybxjr6jgw7hoiptmedp4q6tgku6tchjq5moexhscs4mdwm")

	f, err := os.OpenFile("./scan.log", os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)

	v := NewSyncVisitor()
	v.RegisterErrorHook(func(c cid.Cid, err error) {
		log.Errorln(err)
	})
	v.RegisterVisitHook(func(c cid.Cid) bool {
		fmt.Fprintln(f, c)
		return true
	})

	walkObject(ctx, ss, c1, v)
	require.NoError(t, err)
}

func TestCheck(t *testing.T) {
	ctx := context.Background()

	// path := "/root/tanlang/venus/.vscode/root/.venus/splitstore"
	path := "/root/.venus/splitstore"
	opt := Option{
		MaxLayerCount: 50,
		LayerSize:     1000,
	}

	// initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus/badger")
	initStore, err := openStore("/root/.venus/badger")

	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzaceai2f5d6jtbmxozwlmj4rcs7ezuefv6wxuklnessixtpl7o4eezdy")

	tsk, err := ss.getTipsetKey(ctx, c1)
	require.NoError(t, err)
	blk, err := ss.getBlock(ctx, tsk.Cids()[0])
	require.NoError(t, err)

	state := blk.ParentStateRoot

	walkLayer := func(i, j int, object cid.Cid) {
		bs := []blockstore.Blockstore{initStore}
		ss.layers.Slice(i, j).ForEach(func(idx int, s Layer) bool {
			bs = append(bs, s)
			return true
		})
		s := NewComposeStore(bs...)
		v := NewSyncVisitor()
		errorTrace := make(map[cid.Cid]error)
		v.RegisterErrorHook(func(c cid.Cid, err error) {
			errorTrace[c] = err
		})
		walkObject(ctx, s, object, v)
		buf := make([]byte, 0, 1024)
		buf = append(buf, fmt.Sprintf("layer %d-%d, visit %d, miss %d\n", i, j, v.Len(), len(errorTrace))...)
		for c := range errorTrace {
			buf = append(buf, fmt.Sprintf("%s\n", c)...)
		}
		fmt.Println(string(buf))
	}

	for i := 0; i < ss.Len(); i++ {
		walkLayer(i, ss.Len(), state)
	}
}

func TestFind(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus/splitstore_back"
	opt := Option{
		MaxLayerCount: 50,
		LayerSize:     1000,
	}

	// initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus/badger")
	initStore, err := openStore("/root/.venus/badger")

	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzacedmpr4stk6ezxfxtnnklx2yyoohvkooqgymjj4ddxxmcg52nuzwia")
	c2 := cid.MustParse("bafy2bzacedho6cope4fp5sss43t4swtvc6its5upv22yawtivnic7rkbudmcu")

	tsk, err := ss.getTipsetKey(ctx, c1)
	require.NoError(t, err)
	blk, err := ss.getBlock(ctx, tsk.Cids()[0])
	require.NoError(t, err)
	state := blk.ParentStateRoot

	v := NewSyncVisitor()
	v.RegisterVisitHook(func(c cid.Cid) bool {
		if c.Equals(c2) {
			log.Infoln("find %s from %s", c2, c1)
			return false
		}
		return true
	})

	walkObject(ctx, ss, state, v)

}

func Test2Find(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus/splitstore_back"
	opt := Option{
		MaxLayerCount: 50,
		LayerSize:     1000,
	}

	// initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus/badger")
	initStore, err := openStore("/root/.venus/badger")

	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c2 := cid.MustParse("bafy2bzacedho6cope4fp5sss43t4swtvc6its5upv22yawtivnic7rkbudmcu")

	wg := sync.WaitGroup{}
	wg.Add(ss.Len())
	for i := 0; i < ss.Len(); i++ {
		layers := []blockstore.Blockstore{
			initStore,
		}
		ss.layers.Slice(0, i).ForEach(func(idx int, s Layer) bool {
			layers = append(layers, s)
			return true
		})
		s := NewComposeStore(layers...)

		baseCid := ss.layers.At(i).Base()
		v := NewSyncVisitor()
		v.RegisterVisitHook(func(c cid.Cid) bool {
			if c.Equals(c2) {
				log.Infof("find %s from %s", c2, baseCid)
				return false
			}
			return true
		})

		tsk, err := ss.getTipsetKey(ctx, baseCid)
		require.NoError(t, err)
		blk, err := ss.getBlock(ctx, tsk.Cids()[0])
		require.NoError(t, err)
		go walkChain(ctx, s, baseCid, v, blk.Height-2000, true)
	}
	wg.Wait()

}

func TestSPScan(t *testing.T) {
	ctx := context.Background()

	path := "/root/.venus/splitstore_back"
	opt := Option{
		MaxLayerCount: 50,
		LayerSize:     1000,
	}

	// initStore, err := openStore("/root/tanlang/venus/.vscode/root/.venus/badger")
	initStore, err := openStore("/root/.venus/badger")

	require.NoError(t, err)

	ss, err := NewSplitstore(path, initStore, opt)
	require.NoError(t, err)
	require.Equal(t, opt.MaxLayerCount, ss.maxLayerCount)

	c1 := cid.MustParse("bafy2bzacedblczrsl5qf5aslqhmlr4dmr2dokkpekwfcg7hhcum4et7z674mq")

	ss.layers.ForEach(func(idx int, s Layer) bool {
		has, err := s.Has(ctx, c1)
		require.NoError(t, err)
		if has {
			fmt.Printf("store %d has %s\n", idx, c1)
		}
		return true
	})

	has, err := ss.Has(ctx, c1)
	require.NoError(t, err)
	require.True(t, has)
}
