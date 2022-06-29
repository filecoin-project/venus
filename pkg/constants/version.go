package constants

import (
	"os"
)

// BuildVersion is the local build version, set by build system
const BuildVersion = "1.6.0"

var CurrentCommit string

// software version
func UserVersion() string {
	if os.Getenv("VENUS_VERSION_IGNORE_COMMIT") == "1" {
		return BuildVersion
	}

	return BuildVersion + CurrentCommit
}
