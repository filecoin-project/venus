module github.com/filecoin-project/go-filecoin/tools/migration

go 1.12

require (
	github.com/filecoin-project/go-filecoin v0.0.0-20190412003003-39405ed2fb8b
	github.com/ipfs/go-ipfs-cmds v0.0.5
	github.com/ipfs/go-log v0.0.1
	github.com/pkg/errors v0.8.1
	github.com/prometheus/common v0.3.0
	github.com/stretchr/testify v1.3.0
)

// uncomment below for testing ? see
// https://github.com/golang/go/wiki/Modules#is-it-possible-to-add-a-module-to-a-multi-module-repository
replace github.com/filecoin-project/go-filecoin v0.0.0-20190325235425-966fe7591975 => ../../
