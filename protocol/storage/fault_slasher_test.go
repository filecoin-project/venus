package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	. "github.com/filecoin-project/go-filecoin/protocol/storage"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestFaultSlasher_OnNewHeaviestTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	signer, _ := types.NewMockSignersAndKeyInfo(3)

	data, err := cbor.DumpObject(&map[string]uint64{})
	require.NoError(t, err)
	snapshot := makeSnapshot([][]byte{data})

	getf := address.NewForTestGetter()
	ownMiner := getf()
	ownWorker := signer.Addresses[0]

	ob := outbox{}
	sp := slasherPlumbing{
		Snapshot:   snapshot,
		minerAddr:  ownMiner,
		workerAddr: ownWorker,
	}
	fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

	t.Run("with bad tipset", func(t *testing.T) {
		ts := types.UndefTipSet
		err := fm.OnNewHeaviestTipSet(ctx, ts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tipset height")
	})

	t.Run("calls slasher if tipset can get height", func(t *testing.T) {
		store := chain.NewBuilder(t, signer.Addresses[0])
		baseTs := store.NewGenesis()
		assert.NoError(t, fm.OnNewHeaviestTipSet(ctx, baseTs))
	})
}

func TestFaultSlasher_Slash(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	signer, _ := types.NewMockSignersAndKeyInfo(3)
	ownWorker := signer.Addresses[0]

	t.Run("ok with no miners", func(t *testing.T) {
		height := types.NewBlockHeight(1)
		data, err := cbor.DumpObject(&map[string]uint64{})
		require.NoError(t, err)
		snapshot := makeSnapshot([][]byte{data})

		getf := address.NewForTestGetter()
		ownMiner := getf()

		ob := outbox{}
		sp := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, height)
		require.NoError(t, err)
		assert.Equal(t, 0, ob.msgCount)
	})

	t.Run("slashes multiple miners", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)

		ownMiner := getf()

		badMiner1 := getf().String()
		badMiner2 := getf().String()
		badMiner3 := getf().String()

		data, err := cbor.DumpObject(&map[string]uint64{
			badMiner1: miner.PoStStateUnrecoverable,
			badMiner2: miner.PoStStateUnrecoverable,
			badMiner3: miner.PoStStateUnrecoverable,
		})
		require.NoError(t, err)

		snapshot := makeSnapshot([][]byte{data})
		ob := outbox{}
		sp := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)
		err = fm.Slash(ctx, height)
		assert.NoError(t, err)
		assert.Equal(t, 3, ob.msgCount)
	})

	t.Run("slashes miner only once", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)

		ownMiner := getf()

		badMiner1 := getf().String()

		data1, err := cbor.DumpObject(&map[string]uint64{
			badMiner1: miner.PoStStateUnrecoverable,
		})
		require.NoError(t, err)

		snapshot := makeSnapshot([][]byte{data1})
		ob := outbox{}
		plumbing := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&plumbing, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, height)
		assert.NoError(t, err)
		assert.Equal(t, 1, ob.msgCount)

		err = fm.Slash(ctx, height.Add(types.NewBlockHeight(1)))
		assert.NoError(t, err)
		assert.Equal(t, 1, ob.msgCount) // No change.

		// A second miner becomes slashable.
		badMiner2 := getf().String()
		data2, err := cbor.DumpObject(&map[string]uint64{
			badMiner1: miner.PoStStateUnrecoverable,
			badMiner2: miner.PoStStateUnrecoverable,
		})
		require.NoError(t, err)
		plumbing.Snapshot = makeSnapshot([][]byte{data2})
		err = fm.Slash(ctx, height)
		assert.NoError(t, err)
		assert.Equal(t, 2, ob.msgCount) // A new slashing message.
	})

	t.Run("does not slash own worker address", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)

		ownMiner := getf()

		data1, err := cbor.DumpObject(&map[string]uint64{
			ownMiner.String(): miner.PoStStateUnrecoverable,
		})
		require.NoError(t, err)

		snapshot := makeSnapshot([][]byte{data1})
		ob := outbox{}
		plumbing := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&plumbing, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, height)
		assert.NoError(t, err)
		assert.Equal(t, 0, ob.msgCount)
		assert.Equal(t, ownWorker, plumbing.workerAddr)
	})

	t.Run("error when send fails", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)

		ownMiner := getf()

		badMiner1 := getf().String()
		badMiner2 := getf().String()

		data, err := cbor.DumpObject(&map[string]uint64{
			badMiner1: miner.PoStStateUnrecoverable,
			badMiner2: miner.PoStStateUnrecoverable,
		})
		require.NoError(t, err)

		snapshot := makeSnapshot([][]byte{data})
		ob := outbox{failSend: true, failErr: "Boom"}
		sp := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, height)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Boom")
		assert.Equal(t, 0, ob.msgCount)
	})

	t.Run("error when getLateMiners fails", func(t *testing.T) {
		getf := address.NewForTestGetter()
		ownMiner := getf()

		ob := outbox{}
		snapshot := func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
			if method == "getLateMiners" {
				return nil, errors.New("message query failed")
			}
			return nil, errors.New("test failed")
		}
		sp := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err := fm.Slash(ctx, types.NewBlockHeight(42))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message query failed")
	})

	t.Run("error when when message response malformed", func(t *testing.T) {
		getf := address.NewForTestGetter()
		ownMiner := getf()

		ob := outbox{}

		badBytes, err := cbor.DumpObject("junk")
		require.NoError(t, err)

		snapshot := func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
			if method == "getLateMiners" {
				return [][]byte{badBytes}, nil
			}
			return nil, errors.New("test failed")
		}
		sp := slasherPlumbing{
			Snapshot:   snapshot,
			minerAddr:  ownMiner,
			workerAddr: ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, types.NewBlockHeight(42))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deserializing MinerPoStStates failed")

	})

	t.Run("when MinerGetWorkerAddress fails, returns error", func(t *testing.T) {
		getf := address.NewForTestGetter()
		ownMiner := getf()

		data, err := cbor.DumpObject(&map[string]uint64{})
		require.NoError(t, err)
		snapshot := makeSnapshot([][]byte{data})
		ob := outbox{}
		sp := slasherPlumbing{
			workerAddrFail: true,
			Snapshot:       snapshot,
			minerAddr:      ownMiner,
			workerAddr:     ownWorker,
		}
		fm := NewFaultSlasher(&sp, &ob, DefaultFaultSlasherGasPrice, DefaultFaultSlasherGasLimit)

		err = fm.Slash(ctx, types.NewBlockHeight(99))
		assert.EqualError(t, err, "could not get worker address: actor not found")
	})
}

func makeSnapshot(returnData [][]byte) msgSnapshot {
	return func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
		_, err := abi.ToEncodedValues(params...)
		if err != nil {
			return nil, err
		}
		return returnData, nil
	}
}

type msgSnapshot func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)

type slasherPlumbing struct {
	workerAddrFail        bool
	Snapshot              msgSnapshot
	minerAddr, workerAddr address.Address
}

func (tmp *slasherPlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return tmp.minerAddr, nil
}

func (tmp *slasherPlumbing) ChainHeadKey() types.TipSetKey {
	return types.NewTipSetKey()
}

func (tmp *slasherPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, _ types.TipSetKey, params ...interface{}) ([][]byte, error) {
	return tmp.Snapshot(ctx, optFrom, to, method, params)
}

func (tmp *slasherPlumbing) MinerGetWorkerAddress(_ context.Context, _ address.Address, _ types.TipSetKey) (address.Address, error) {
	if tmp.workerAddrFail {
		return address.Undef, errors.New("actor not found")
	}
	return tmp.workerAddr, nil
}

type outbox struct {
	failSend bool
	failErr  string
	msgCount int
}

func (ob *outbox) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	bcast bool,
	method string,
	params ...interface{}) (out cid.Cid, err error) {

	_, err = abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, err
	}

	if gasPrice.LessEqual(types.ZeroAttoFIL) {
		return cid.Undef, errors.New("gas price must be >0")
	}

	if gasLimit < types.Uint64(300) {
		return cid.Undef, errors.New("gas limit must be >= 300")
	}

	if ob.failSend {
		return cid.Undef, errors.New(ob.failErr)
	}
	ob.msgCount++
	// we ignore the CID returned from Send anyway
	return cid.Undef, nil
}
