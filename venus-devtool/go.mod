module github.com/filecoin-project/venus/venus-devtool

go 1.16

require (
	github.com/filecoin-project/go-jsonrpc v0.1.5
	github.com/filecoin-project/lotus v1.12.0
	github.com/filecoin-project/venus v0.0.0-00010101000000-000000000000
	github.com/filecoin-project/venus/venus-shared v0.0.1
	github.com/ipfs/go-ipfs-http-client v0.1.0 // indirect
	github.com/urfave/cli/v2 v2.3.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20211110122933-f57984553008
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
)

replace (
	github.com/filecoin-project/venus => ../
	github.com/filecoin-project/venus/venus-shared => ../venus-shared/
	github.com/ipfs/go-ipfs-cmds => github.com/ipfs-force-community/go-ipfs-cmds v0.6.1-0.20210521090123-4587df7fa0ab
	github.com/multiformats/go-multiaddr => github.com/multiformats/go-multiaddr v0.3.0
)