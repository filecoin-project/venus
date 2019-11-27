package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestChainEpochClock(t *testing.T) {
	tf.UnitTest(t)

	now := int64(123456789)

	cec := clock.NewChainClock(uint64(now))

	epoch0 := time.Unix(now, 0)
	epoch1 := time.Unix(now+clock.EpochCount, 0)
	assert.Equal(t, int64(0), cec.EpochAtTime(epoch0))
	assert.Equal(t, int64(1), cec.EpochAtTime(epoch1))

	epoch2 := time.Unix(now+clock.EpochCount*2, 0)
	epoch2again := time.Unix((now+clock.EpochCount*3)-1, 0)
	assert.Equal(t, int64(2), cec.EpochAtTime(epoch2))
	assert.Equal(t, int64(2), cec.EpochAtTime(epoch2again))

	epoch200 := time.Unix(now+clock.EpochCount*200, 0)
	assert.Equal(t, int64(200), cec.EpochAtTime(epoch200))

	epochBeforeGenesis := time.Unix(now-clock.EpochCount, 0)
	epochWayBeforeGenesis := time.Unix(now-clock.EpochCount*30, 0)
	assert.Equal(t, int64(-1), cec.EpochAtTime(epochBeforeGenesis))
	assert.Equal(t, int64(-30), cec.EpochAtTime(epochWayBeforeGenesis))
}
