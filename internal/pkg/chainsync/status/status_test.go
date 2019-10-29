package status_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/stretchr/testify/assert"

	//"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestStatus(t *testing.T) {
	tf.UnitTest(t)

	sr := status.NewReporter()
	assert.Equal(t, *status.NewDefaultChainStatus(), sr.Status())
	assert.Equal(t, status.NewDefaultChainStatus().String(), sr.Status().String())

	// single update
	cidFn := types.NewCidForTestGetter()

	// multi update
	t2 := block.NewTipSetKey(cidFn())
	t3 := block.NewTipSetKey(cidFn())
	expStatus := status.Status{
		SyncingHead:          t2,
		SyncingHeight:        456,
		SyncingTrusted:       true,
		SyncingStarted:       123,
		SyncingComplete:      false,
		SyncingFetchComplete: true,
		FetchingHead:         t3,
		FetchingHeight:       789,
	}
	sr.UpdateStatus(status.SyncingStarted(123), status.SyncHead(t2),
		status.SyncHeight(456), status.SyncTrusted(true), status.SyncComplete(false), status.SyncFetchComplete(true),
		status.FetchHead(t3), status.FetchHeight(789))
	assert.Equal(t, expStatus, sr.Status())
}
