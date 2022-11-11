// stm: #unit
package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"golang.org/x/net/context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/clock"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestChainEpochClock(t *testing.T) {
	tf.UnitTest(t)

	// stm: @CLOCK_CHAIN_CLOCK_NEW_001
	now := time.Now().Unix()
	bt := clock.DefaultEpochDuration
	cec := clock.NewChainClock(uint64(now), bt)

	epoch0Start := time.Unix(now, 0)
	epoch1Start := epoch0Start.Add(bt)

	// stm: @CLOCK_CHAIN_CLOCK_EPOCH_AT_TIME_001
	assert.Equal(t, abi.ChainEpoch(0), cec.EpochAtTime(epoch0Start))
	assert.Equal(t, abi.ChainEpoch(1), cec.EpochAtTime(epoch1Start))

	epoch2Start := epoch1Start.Add(bt)
	epoch2Middle := epoch2Start.Add(bt / time.Duration(5))
	assert.Equal(t, abi.ChainEpoch(2), cec.EpochAtTime(epoch2Start))
	assert.Equal(t, abi.ChainEpoch(2), cec.EpochAtTime(epoch2Middle))

	epoch200Start := epoch0Start.Add(time.Duration(200) * bt)
	assert.Equal(t, abi.ChainEpoch(200), cec.EpochAtTime(epoch200Start))

	expectedStartEpoch := 10
	// stm: @CLOCK_CHAIN_CLOCK_EPOCH_RANGE_AT_TIMESTAMP_001
	first, last := cec.EpochRangeAtTimestamp(uint64(now) + uint64(builtin.EpochDurationSeconds*expectedStartEpoch) - 1)
	assert.Equal(t, int(first), expectedStartEpoch-1)
	assert.Equal(t, int(last), expectedStartEpoch)

	// stm: @CLOCK_CHAIN_CLOCK_START_TIME_OF_EPOCH_001
	startTime = cec.StartTimeOfEpoch(abi.ChainEpoch(expectedStartEpoch))
	assert.Equal(t, startTime.Unix(), now+(builtin.EpochDurationSeconds*int64(expectedStartEpoch)))

	waitEpoch := abi.ChainEpoch(1)
	var expectedNextEpoch abi.ChainEpoch
	var wg sync.WaitGroup

	ctx := context.Background()

	wg.Add(2)
	go func() {
		defer wg.Done()
		// stm: @CLOCK_CHAIN_CLOCK_WAIT_NEXT_EPOCH_001
		expectedNextEpoch = cec.WaitNextEpoch(ctx)
	}()

	go func() {
		defer wg.Done()
		// stm: @CLOCK_CHAIN_CLOCK_WAIT_FOR_EPOCH_001
		cec.WaitForEpoch(ctx, waitEpoch)
	}()

	t.Logf("waitting for next epoch.")
	wg.Wait()

	assert.Equal(t, waitEpoch, expectedNextEpoch)
}
