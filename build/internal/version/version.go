package version

import (
	"strconv"
	"strings"
)

// Check ensures that we are using the correct version of go
func Check(version string) bool {
	pieces := strings.Split(version, ".")

	if pieces[0] != "go1" {
		return false
	}

	minorVersion, _ := strconv.Atoi(pieces[1])

	if minorVersion > 13 {
		return true
	}

	if minorVersion < 13 {
		return false
	}

	if len(pieces) < 3 {
		return false
	}

	patchVersion, _ := strconv.Atoi(pieces[2])
	return patchVersion >= 1
}
