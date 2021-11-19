module github.com/filecoin-project/venus/venus-devtool

go 1.16

require (
	github.com/filecoin-project/lotus v1.12.0
	github.com/filecoin-project/venus/venus-shared v0.0.1
	github.com/urfave/cli/v2 v2.2.0 // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20211110122933-f57984553008
)

replace github.com/filecoin-project/venus/venus-shared => ../venus-shared/
