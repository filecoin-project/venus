package initactor_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/address"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestInitActorCreateInitActor(t *testing.T) {
	tf.UnitTest(t)

	initExecActor := &Actor{}

	stmap := th.VMStorage()
	initActor := &actor.Actor{}
	storage := stmap.NewStorage(address.InitAddress, initActor)

	// create state with a network name
	initExecActor.InitializeState(storage, "foo")
	stmap.Flush()

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

	msg := types.NewUnsignedMessage(address.TestAddress, address.InitAddress, 0, types.ZeroAttoFIL, GetNetwork, []byte{})
	vmctx := vm2.NewFakeVMContext(msg, state)

	actor := &Impl{}
	network, code, err := actor.GetNetwork(vmctx)
	require.NoError(t, err)
	require.Equal(t, uint8(0), code)

	assert.Equal(t, "bar", network)
}
