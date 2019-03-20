package builtin

import (
	"testing"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/vm"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

// RequireReadState constructs vm storage from the storage map and reads the chunk at the given actor's head
func RequireReadState(t *testing.T, vms vm.StorageMap, addr address.Address, act *actor.Actor, state interface{}) {
	chunk, err := vms.NewStorage(addr, act).Get(act.Head) // address arbitrary
	require.NoError(t, err)

	err = cbor.DecodeInto(chunk, state)
	require.NoError(t, err)
}
