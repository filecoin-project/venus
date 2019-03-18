package series

import (
	"time"

	"github.com/filecoin-project/go-filecoin/mining"
)

// GlobalSleepDelay is the time value that is used to wait for block processing
// in series
var GlobalSleepDelay = mining.DefaultBlockTime

// SleepDelay is a helper method to make sure people
// don't call `time.Sleep` themselves in series.
func SleepDelay() {
	time.Sleep(GlobalSleepDelay)
}
