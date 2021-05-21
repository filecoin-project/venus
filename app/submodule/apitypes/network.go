package apitypes

import "github.com/filecoin-project/venus/pkg/constants"

// Version provides various build-time information
type Version struct {
	Version string

	// APIVersion is a binary encoded semver version of the remote implementing
	// this api
	//
	// See APIVersion in build/version.go
	APIVersion constants.Version
}
