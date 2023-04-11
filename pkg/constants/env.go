package constants

import (
	"os"
	"strconv"

	logging "github.com/ipfs/go-log/v2"
)

var envLog = logging.Logger("env")

// FevmEnableEthRPC enables eth rpc, and enables storing a mapping of eth transaction hashes to filecoin message Cids.
var FevmEnableEthRPC = os.Getenv("VENUS_FEVM_ENABLEETHRPC") == "1"

var badgerCacheStr = os.Getenv("VENUS_BADGER_CACHE")

var BadgerCache int64 = 10 * 10000

func init() {
	if len(badgerCacheStr) != 0 {
		count, err := strconv.ParseInt(badgerCacheStr, 10, 64)
		if err == nil {
			BadgerCache = count
		} else {
			envLog.Errorf("badger cache error: %s %v", badgerCacheStr, err)
		}
	}
}
