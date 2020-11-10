package clock_test

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/clock"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestChainEpochClock(t *testing.T) {
	tf.UnitTest(t)

	now := int64(123456789)
	bt := clock.DefaultEpochDuration
	pd := clock.DefaultPropagationDelay
	cec := clock.NewChainClock(uint64(now), bt, pd)

	epoch0Start := time.Unix(now, 0)
	epoch1Start := epoch0Start.Add(bt)

	assert.Equal(t, abi.ChainEpoch(0), cec.EpochAtTime(epoch0Start))
	assert.Equal(t, abi.ChainEpoch(1), cec.EpochAtTime(epoch1Start))

	epoch2Start := epoch1Start.Add(bt)
	epoch2Middle := epoch2Start.Add(bt / time.Duration(5))
	assert.Equal(t, abi.ChainEpoch(2), cec.EpochAtTime(epoch2Start))
	assert.Equal(t, abi.ChainEpoch(2), cec.EpochAtTime(epoch2Middle))

	epoch200Start := epoch0Start.Add(time.Duration(200) * bt)
	assert.Equal(t, abi.ChainEpoch(200), cec.EpochAtTime(epoch200Start))
}
