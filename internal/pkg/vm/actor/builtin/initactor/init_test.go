package initactor_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	dag "github.com/ipfs/go-merkledag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitActorCreateInitActor(t *testing.T) {
	tf.UnitTest(t)

	initExecActor := &Actor{}

	storageMap := th.VMStorage()
	initActor := &actor.Actor{}
	storage := storageMap.NewStorage(address.InitAddress, initActor)

	// create state with a network name
	initExecActor.InitializeState(storage, "foo")
	storageMap.Flush()

	// retrieve state directly and assert it's constructed correctly
	state, err := storage.Get(initActor.Head)
	require.NoError(t, err)

	var initState State
	err = encoding.Decode(state, &initState)
	require.NoError(t, err)

	assert.Equal(t, "foo", initState.Network)
}

func TestInitActorGetNetwork(t *testing.T) {
	tf.UnitTest(t)

	state := &State{
		Network: "bar",
	}

	msg := types.NewUnsignedMessage(address.TestAddress, address.InitAddress, 0, types.NewAttoFILFromFIL(53), GetNetwork, []byte{})
	vmctx := vm.NewFakeVMContext(msg, state)

	actor := &Impl{}
	network, code, err := actor.GetNetwork(vmctx)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)

	assert.Equal(t, "bar", network)
}

func TestInitActorExec(t *testing.T) {
	tf.UnitTest(t)

	msg := types.NewUnsignedMessage(address.TestAddress, address.InitAddress, 0, types.ZeroAttoFIL, Exec, []byte{})

	newState := func() *State {
		return &State{
			Network: "bar",
			NextID:  42,
		}
	}
	initParams := []interface{}{[]byte("one"), []byte("two")}
	act := &Impl{}

	t.Run("exec charges gas", func(t *testing.T) {
		var charge types.GasUnits
		vmctx := vm.NewFakeVMContext(msg, newState())
		vmctx.Charger = func(cost types.GasUnits) error { charge = cost; return nil }

		_, _, err := act.Exec(vmctx, types.MinerActorCodeCid, initParams)
		require.NoError(t, err)

		assert.Equal(t, types.NewGasUnits(actor.DefaultGasCost), charge)
	})

	t.Run("exec refuses to construct a non builtin actor", func(t *testing.T) {
		code := dag.NewRawNode([]byte("nonesuch")).Cid()
		_, _, err := act.Exec(vm.NewFakeVMContext(msg, newState()), code, initParams)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a builtin actor")
	})

	t.Run("exec refuses to construct singleton actor", func(t *testing.T) {
		code := types.StorageMarketActorCodeCid // storage market is a singleton actor
		_, _, err := act.Exec(vm.NewFakeVMContext(msg, newState()), code, initParams)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot launch another actor of this type")
	})

	t.Run("exec constructs a permanent address and creates a mapping to the id", func(t *testing.T) {
		vmctx := vm.NewFakeVMContext(msg, newState())

		// set up enough storage so that it actually uses the hamts
		sm := vm.NewStorageMap(blockstore.NewBlockstore(datastore.NewMapDatastore()))
		actorModel := actor.NewActor(types.InitActorCodeCid, types.ZeroAttoFIL)
		vmctx.StorageValue = sm.NewStorage(address.InitAddress, actorModel)
		err := (*Actor)(act).InitializeState(vmctx.Storage(), "network")
		require.NoError(t, err)

		addr, _, err := act.Exec(vmctx, types.MinerActorCodeCid, initParams)
		require.NoError(t, err)

		_, _, err = act.GetActorIDForAddress(vmctx, addr)
		require.NoError(t, err)
	})

	t.Run("exec creates a new actor at the id address", func(t *testing.T) {
		var actualAddr address.Address
		var actualCode cid.Cid
		vmctx := vm.NewFakeVMContext(msg, newState())
		vmctx.ActorCreator = func(addr address.Address, code cid.Cid) error {
			actualAddr = addr
			actualCode = code
			return nil
		}
		_, _, err := act.Exec(vmctx, types.MinerActorCodeCid, initParams)
		require.NoError(t, err)

		assert.Equal(t, address.ID, actualAddr.Protocol())
		assert.Equal(t, types.MinerActorCodeCid, actualCode)
	})

	t.Run("exec calls constructor and sends funds", func(t *testing.T) {
		var constructedAddr address.Address
		var actualTo address.Address
		var actualMethod types.MethodID
		var actualValue types.AttoFIL
		var actualParams []interface{}
		vmctx := vm.NewFakeVMContext(msg, newState())
		vmctx.ActorCreator = func(addr address.Address, code cid.Cid) error {
			constructedAddr = addr
			return nil
		}
		vmctx.Sender = func(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
			actualTo = to
			actualMethod = method
			actualValue = value
			actualParams = params
			return [][]byte{}, 0, nil
		}
		_, _, err := act.Exec(vmctx, types.MinerActorCodeCid, initParams)
		require.NoError(t, err)

		// sends to actor it just created
		assert.Equal(t, constructedAddr, actualTo)

		// sends to constructor method
		assert.Equal(t, types.ConstructorMethodID, actualMethod)

		// sends value given it on to actor
		assert.Equal(t, msg.Value, actualValue)

		// sends given parameters on to constructor
		require.Equal(t, 2, len(actualParams))
		assert.Equal(t, initParams[0], actualParams[0])
		assert.Equal(t, initParams[1], actualParams[1])
	})
}
