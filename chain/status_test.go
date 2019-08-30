package chain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	//"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestStatus(t *testing.T) {
	tf.UnitTest(t)

	sr := NewStatusReporter()
	assert.Equal(t, *newDefaultChainStatus(), sr.Status())
	assert.Equal(t, newDefaultChainStatus().String(), sr.Status().String())

	// single update
	cidFn := types.NewCidForTestGetter()
	t0 := types.NewTipSetKey(cidFn())
	sr.UpdateStatus(validateHead(t0))
	assert.Equal(t, t0, sr.Status().ValidatedHead)

	// multi update
	t1 := types.NewTipSetKey(cidFn())
	t2 := types.NewTipSetKey(cidFn())
	t3 := types.NewTipSetKey(cidFn())
	expStatus := Status{
		ValidatedHead:        t1,
		ValidatedHeadHeight:  1,
		ValidatedStarted:     123,
		SyncingHead:          t2,
		SyncingHeight:        456,
		SyncingTrusted:       true,
		SyncingComplete:      false,
		SyncingFetchComplete: true,
		FetchingHead:         t3,
		FetchingHeight:       789,
	}
	sr.UpdateStatus(validateHead(t1), validateHeight(1), validateStarted(123), syncHead(t2),
		syncHeight(456), syncTrusted(true), syncComplete(false), syncFetchComplete(true),
		fetchHead(t3), fetchHeight(789))
	assert.Equal(t, expStatus, sr.Status())
}
