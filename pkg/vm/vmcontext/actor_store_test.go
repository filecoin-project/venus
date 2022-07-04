package vmcontext_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/filecoin-project/venus/pkg/config"
	cbor "github.com/ipfs/go-ipld-cbor"

	cbor2 "github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	vmr "github.com/filecoin-project/venus/pkg/vm/runtime"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	typegen "github.com/whyrusleeping/cbor-gen"
)

func TestActorStore(t *testing.T) {
	ctx := context.Background()
	raw := cbor.NewCborStore(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	gasTank := gas.NewGasTracker(1e6)
	priceSchedule := gas.NewPricesSchedule(config.DefaultForkUpgradeParam)
	t.Run("abort on put serialization failure", func(t *testing.T) {
		store := vmcontext.NewActorStorage(ctx, raw, gasTank, priceSchedule.PricelistByEpoch(0))
		_, thrown := tryPut(store, cannotCBOR{})
		abort, ok := thrown.(vmr.ExecutionPanic)
		assert.NotNil(t, thrown)
		assert.True(t, ok, "expected abort")
		assert.Equal(t, exitcode.ErrSerialization, abort.Code())
	})

	t.Run("abort on get serialization failure", func(t *testing.T) {
		store := vmcontext.NewActorStorage(ctx, raw, gasTank, priceSchedule.PricelistByEpoch(0))
		v := typegen.CborInt(0)

		c, thrown := tryPut(store, &v)
		assert.True(t, c.Defined())
		require.Nil(t, thrown)

		var v2 typegen.CborCid
		thrown = tryGet(store, c, &v2) // Attempt decode into wrong type
		assert.Contains(t, thrown.(string), "failed To get object *typegen.CborCid ")
	})

	t.Run("panic on get storage failure", func(t *testing.T) {
		store := vmcontext.NewActorStorage(ctx, raw, gasTank, priceSchedule.PricelistByEpoch(0))
		var v typegen.CborInt
		thrown := tryGet(store, cid.Undef, &v)
		_, ok := thrown.(vmr.ExecutionPanic)
		assert.NotNil(t, thrown)
		assert.False(t, ok, "expected non-abort panic")
	})
}

func tryPut(s *vmcontext.ActorStorage, v cbor2.Marshaler) (c cid.Cid, thrown interface{}) {
	defer func() {
		thrown = recover()
	}()
	c = s.StorePut(v)
	return
}

func tryGet(s *vmcontext.ActorStorage, c cid.Cid, v cbor2.Unmarshaler) (thrown interface{}) {
	defer func() {
		thrown = recover()
	}()
	s.StoreGet(c, v)
	return
}

type cannotCBOR struct {
}

func (c cannotCBOR) MarshalCBOR(w io.Writer) error {
	return fmt.Errorf("no")
}
