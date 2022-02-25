package constants

import (
	"os"

	"github.com/filecoin-project/venus/build/flags"
)

// BuildVersion is the local build version, set by build system
const BuildVersion = "1.2.1"

// software version
func UserVersion() string {
	if os.Getenv("VENUS_VERSION_IGNORE_COMMIT") == "1" {
		return BuildVersion
	}

	return BuildVersion + flags.GitCommit
}
