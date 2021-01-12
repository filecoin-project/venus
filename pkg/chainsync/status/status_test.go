package status_test

import (
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"testing"

	"github.com/stretchr/testify/assert"

	//"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/chainsync/status"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestStatus(t *testing.T) {
	tf.UnitTest(t)

	sr := status.NewReporter()
	assert.Equal(t, *status.NewDefaultChainStatus(), sr.Status())
	assert.Equal(t, status.NewDefaultChainStatus().String(), sr.Status().String())

	// multi update
	t2 := testhelpers.RequireTipset(t)
	t3 := testhelpers.RequireTipset(t)
	expStatus := status.Status{
		SyncingHead:          t2,
		SyncingTrusted:       true,
		SyncingStarted:       123,
		SyncingComplete:      false,
		SyncingFetchComplete: true,
		FetchingHead:         t3,
	}
	sr.UpdateStatus(status.SyncingStarted(123), status.SyncHead(t2), status.SyncTrusted(true), status.SyncComplete(false), status.SyncFetchComplete(true),
		status.FetchHead(t3))
	assert.Equal(t, expStatus, sr.Status())
}
