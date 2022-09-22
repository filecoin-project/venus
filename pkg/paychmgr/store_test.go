package paychmgr

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	tutils "github.com/filecoin-project/specs-actors/support/testing"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	pchTypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

func TestStore(t *testing.T) {
	tf.UnitTest(t)
	store := NewStore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	ctx := context.Background()
	addrs, err := store.ListChannels(ctx)
	require.NoError(t, err)
	require.Len(t, addrs, 0)

	ch := tutils.NewIDAddr(t, 100)
	ci := &pchTypes.ChannelInfo{
		Channel: &ch,
		Control: tutils.NewIDAddr(t, 101),
		Target:  tutils.NewIDAddr(t, 102),

		Direction: pchTypes.DirOutbound,
		Vouchers:  []*pchTypes.VoucherInfo{{Voucher: nil, Proof: []byte{}}},
	}

	ch2 := tutils.NewIDAddr(t, 200)
	ci2 := &pchTypes.ChannelInfo{
		Channel: &ch2,
		Control: tutils.NewIDAddr(t, 201),
		Target:  tutils.NewIDAddr(t, 202),

		Direction: pchTypes.DirOutbound,
		Vouchers:  []*pchTypes.VoucherInfo{{Voucher: nil, Proof: []byte{}}},
	}

	// Track the channel
	_, err = store.TrackChannel(ctx, ci)
	require.NoError(t, err)

	// Tracking same channel again should error
	_, err = store.TrackChannel(ctx, ci)
	require.Error(t, err)

	// Track another channel
	_, err = store.TrackChannel(ctx, ci2)
	require.NoError(t, err)

	// List channels should include all channels
	addrs, err = store.ListChannels(ctx)
	require.NoError(t, err)
	require.Len(t, addrs, 2)
	t0100, err := address.NewIDAddress(100)
	require.NoError(t, err)
	t0200, err := address.NewIDAddress(200)
	require.NoError(t, err)
	require.Contains(t, addrs, t0100)
	require.Contains(t, addrs, t0200)

	// Request vouchers for channel
	vouchers, err := store.VouchersForPaych(ctx, *ci.Channel)
	require.NoError(t, err)
	require.Len(t, vouchers, 1)

	// Requesting voucher for non-existent channel should error
	_, err = store.VouchersForPaych(ctx, tutils.NewIDAddr(t, 300))
	require.Equal(t, err, ErrChannelNotTracked)

	// Allocate lane for channel
	lane, err := store.AllocateLane(ctx, *ci.Channel)
	require.NoError(t, err)
	require.Equal(t, lane, uint64(0))

	// Allocate next lane for channel
	lane, err = store.AllocateLane(ctx, *ci.Channel)
	require.NoError(t, err)
	require.Equal(t, lane, uint64(1))

	// Allocate next lane for non-existent channel should error
	_, err = store.AllocateLane(ctx, tutils.NewIDAddr(t, 300))
	require.Equal(t, err, ErrChannelNotTracked)
}
