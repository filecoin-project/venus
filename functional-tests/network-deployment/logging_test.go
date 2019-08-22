package networkdeployment_test

import (
	logging "github.com/ipfs/go-log"
	oldlogging "github.com/whyrusleeping/go-logging"
)

func init() {
	logging.SetAllLoggers(oldlogging.INFO)
}
