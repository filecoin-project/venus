module github.com/filecoin-project/go-filecoin/tools/migration

go 1.12

require (
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/filecoin-project/go-filecoin v0.0.1
	github.com/ipfs/go-ipfs-api v0.0.1 // indirect
	github.com/ipfs/go-ipfs-cmds v0.0.5
	github.com/ipfs/go-log v0.0.1
	github.com/pkg/errors v0.8.1
	github.com/prometheus/common v0.3.0
	github.com/sabhiram/go-gitignore v0.0.0-20180611051255-d3107576ba94 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/whyrusleeping/gx v0.14.2 // indirect
	github.com/whyrusleeping/json-filter v0.0.0-20160615203754-ff25329a9528 // indirect
	github.com/whyrusleeping/progmeter v0.0.0-20180725015555-f3e57218a75b // indirect
	github.com/whyrusleeping/stump v0.0.0-20160611222256-206f8f13aae1 // indirect
)

// uncomment below for testing ? see
// https://github.com/golang/go/wiki/Modules#is-it-possible-to-add-a-module-to-a-multi-module-repository
replace github.com/filecoin-project/go-filecoin v0.0.1 => ../../
