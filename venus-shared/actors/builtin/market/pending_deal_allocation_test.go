package market

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	verifregtypes "github.com/filecoin-project/go-state-types/builtin/v9/verifreg"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/actors/adt"
)

func TestGetAllocationIdForPendingDeal(t *testing.T) {
	ctx := context.Background()
	store := adt.WrapStore(ctx, cbor.NewCborStore(blockstore.NewBlockstore(datastore.NewMapDatastore())))

	tests := []struct {
		name        string
		av          actorstypes.Version
		unsupported bool
	}{
		{name: "nv28 and earlier report no allocation", av: actorstypes.Version18},
		{name: "nv29 is unsupported", av: actorstypes.Version19, unsupported: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, err := MakeState(store, tc.av)
			require.NoError(t, err)

			id, err := st.GetAllocationIdForPendingDeal(abi.DealID(1))
			if tc.unsupported {
				require.EqualError(t, err, "unsupported from actors v19")
				return
			}

			require.NoError(t, err)
			require.Equal(t, verifregtypes.NoAllocationID, id)
		})
	}
}
