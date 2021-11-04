package constants

import (
	"os"

	"github.com/filecoin-project/venus/build/flags"
)

// BuildVersion is the local build version, set by build system
const BuildVersion = "1.1.2"

// software version
func UserVersion() string {
	if os.Getenv("VENUS_VERSION_IGNORE_COMMIT") == "1" {
		return BuildVersion
	}

	return BuildVersion + flags.GitCommit
}

type Version uint32

func newVer(major, minor, patch uint8) Version {
	return Version(uint32(major)<<16 | uint32(minor)<<8 | uint32(patch))
}

// semver versions of the rpc api exposed
var (
	FullAPIVersion0 = newVer(1, 4, 0)
	FullAPIVersion1 = newVer(2, 1, 0)
)
