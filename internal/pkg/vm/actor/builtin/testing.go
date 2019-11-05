package builtin

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/stretchr/testify/require"
)

// RequireReadState constructs vm storage from the storage map and reads the chunk at the given actor's head
func RequireReadState(t *testing.T, vms storagemap.StorageMap, addr address.Address, act *actor.Actor, state interface{}) {
	chunk, err := vms.NewStorage(addr, act).Get(act.Head) // address arbitrary
	require.NoError(t, err)

	err = encoding.Decode(chunk, state)
	require.NoError(t, err)
}
