package initactor_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/address"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-ipld-cbor"
	"github.com/magiconair/properties/assert"
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
	storage.Flush()

	// retrieve state directly and assert it's constructed correctly
	state, err := storage.Get(initActor.Head)
	require.NoError(t, err)

	var initState State
	err = cbornode.DecodeInto(state, &initState)
	require.NoError(t, err)

	assert.Equal(t, "foo", initState.Network)
}

func TestInitActorGetNetwork(t *testing.T) {
	tf.UnitTest(t)

	initExecActor := &Actor{}
	state := &State{
		Network: "bar",
	}

	msg := types.NewUnsignedMessage(address.TestAddress, address.InitAddress, 0, types.ZeroAttoFIL, "getAddress", []byte{})
	vmctx := th.NewFakeVMContext(msg, state)

	network, code, err := initExecActor.GetNetwork(vmctx)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)

	assert.Equal(t, "bar", network)
}
