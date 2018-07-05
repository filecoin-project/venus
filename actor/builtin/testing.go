package builtin

import (
	"testing"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RequireReadState constructs vm storage from the storage map and reads the chunk at the given actor's head
func RequireReadState(t *testing.T, vms vm.StorageMap, addr types.Address, act *types.Actor, state interface{}) {
	chunk, ok, err := vms.NewStorage(addr, act).Get(act.Head) // address arbitrary
	require.NoError(t, err)
	assert.True(t, ok)

	err = cbor.DecodeInto(chunk, state)
	require.NoError(t, err)
}
