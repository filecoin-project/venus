module github.com/filecoin-project/venus/venus-devtool

go 1.16

require (
	github.com/filecoin-project/go-address v0.0.6
	github.com/filecoin-project/go-bitfield v0.2.4
	github.com/filecoin-project/go-data-transfer v1.15.1
	github.com/filecoin-project/go-fil-markets v1.20.1
	github.com/filecoin-project/go-jsonrpc v0.1.5
	github.com/filecoin-project/go-state-types v0.1.3
	github.com/filecoin-project/lotus v1.14.0
	github.com/filecoin-project/venus v0.0.0-00010101000000-000000000000
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-graphsync v0.13.1
	github.com/ipfs/go-ipfs-http-client v0.1.0 // indirect
	github.com/ipld/go-ipld-selector-text-lite v0.0.1
	github.com/libp2p/go-libp2p-core v0.14.0
	github.com/libp2p/go-libp2p-pubsub v0.6.1
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20220302191723-37c43cae8e14
)

replace (
	github.com/filecoin-project/filecoin-ffi => .././extern/filecoin-ffi
	github.com/filecoin-project/go-jsonrpc => github.com/ipfs-force-community/go-jsonrpc v0.1.4-0.20210731021807-68e5207079bc
	github.com/filecoin-project/venus => ../
	github.com/ipfs/go-ipfs-cmds => github.com/ipfs-force-community/go-ipfs-cmds v0.6.1-0.20210521090123-4587df7fa0ab
	github.com/multiformats/go-multiaddr => github.com/multiformats/go-multiaddr v0.3.0
)
