package constants

import (
	"github.com/filecoin-project/venus/build/flags"
)

// BuildVersion is the local build version, set by build system
const BuildVersion = "0.9.7"

func UserVersion() string {
	return BuildVersion + flags.GitCommit
}

type Version uint32

func newVer(major, minor, patch uint8) Version {
	return Version(uint32(major)<<16 | uint32(minor)<<8 | uint32(patch))
}

// semver versions of the rpc api exposed
var (
	FullAPIVersion0 = newVer(1, 3, 0)
	FullAPIVersion1 = newVer(2, 1, 0)
)
