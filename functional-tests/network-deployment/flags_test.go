package networkdeployment_test

import (
	"flag"

	"github.com/filecoin-project/go-filecoin/testhelpers"
)

var binaryFlag = flag.String("binary", "", "Path to binary to use for tests")

func GetBinary() string {
	if len(*binaryFlag) == 0 {
		return testhelpers.MustGetFilecoinBinary()
	} else {
		return *binaryFlag
	}
}
