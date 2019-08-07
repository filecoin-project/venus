package consensus_test

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
	. "github.com/filecoin-project/go-filecoin/consensus"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestStorageFaultSlasher_Slash(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	signer, _ := types.NewMockSignersAndKeyInfo(1)

	t.Run("When there are no miners, functions normally by returning empty late miners list", func(t *testing.T) {

		height := types.NewBlockHeight(1)
		data, err := cbor.DumpObject(&map[string]uint64{})
		require.NoError(t, err)
		queryer := makeQueryer([][]byte{data})
		minerOwnerAddr := signer.Addresses[0]

		ob := outbox{}
		fm := NewStorageFaultMonitor(&storageFaultMonitorPorcelain{false, false, queryer}, &ob, minerOwnerAddr)
		err = fm.Slash(ctx, height)
		require.NoError(t, err)
	})

	t.Run("When 3 late miners, sends 3 messages", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)

		addr1 := getf().String()
		addr2 := getf().String()
		addr3 := getf().String()

		data, err := cbor.DumpObject(&map[string]uint64{
			addr1: miner.PoStStateAfterProvingPeriod,
			addr2: miner.PoStStateAfterGenerationAttackThreshold,
			addr3: miner.PoStStateAfterProvingPeriod,
		})
		require.NoError(t, err)

		queryer := makeQueryer([][]byte{data})
		ob := outbox{}
		minerOwnerAddr := signer.Addresses[0]
		fm := NewStorageFaultMonitor(&storageFaultMonitorPorcelain{false, false, queryer}, &ob, minerOwnerAddr)
		err = fm.Slash(ctx, height)
		assert.NoError(t, err)
		assert.Equal(t, 3, ob.msgCount)
	})

	t.Run("When Send fails, return error", func(t *testing.T) {
		getf := address.NewForTestGetter()
		height := types.NewBlockHeight(100)
		addr1 := getf().String()
		addr2 := getf().String()

		data, err := cbor.DumpObject(&map[string]uint64{
			addr1: miner.PoStStateAfterGenerationAttackThreshold,
			addr2: miner.PoStStateAfterGenerationAttackThreshold,
		})
		require.NoError(t, err)

		queryer := makeQueryer([][]byte{data})
		ob := outbox{failSend: true, failErr: "Boom"}
		minerOwnerAddr := signer.Addresses[0]
		fm := NewStorageFaultMonitor(&storageFaultMonitorPorcelain{false, false, queryer}, &ob, minerOwnerAddr)
		err = fm.Slash(ctx, height)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Boom\nBoom")
		assert.Equal(t, 0, ob.msgCount)
	})

}

func makeQueryer(returnData [][]byte) msgQueryer {
	return func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
		_, err := abi.ToEncodedValues(params...)
		if err != nil {
			return nil, err
		}
		return returnData, nil
	}
}

type msgQueryer func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)

type storageFaultMonitorPorcelain struct {
	actorFail   bool
	actorChFail bool
	Queryer     msgQueryer
}

func (tmp *storageFaultMonitorPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	return tmp.Queryer(ctx, optFrom, to, method, params)
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
