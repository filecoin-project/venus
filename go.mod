module github.com/filecoin-project/go-filecoin

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/cskr/pubsub v1.0.2
	github.com/davidlazar/go-crypto v0.0.0-20190912175916-7055855a373f // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v0.7.3-0.20190315170154-87d593639c77
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/drand/drand v0.8.1
	github.com/drand/kyber v1.0.1-0.20200331114745-30e90cc60f99
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/filecoin-project/chain-validation v0.0.6-0.20200512234642-6304037e1db6
	github.com/filecoin-project/filecoin-ffi v0.26.1-0.20200508175440-05b30afeb00d
	github.com/filecoin-project/go-address v0.0.2-0.20200218010043-eb9bb40ed5be
	github.com/filecoin-project/go-amt-ipld/v2 v2.0.1-0.20200424220931-6263827e49f2
	github.com/filecoin-project/go-bitfield v0.0.1
	github.com/filecoin-project/go-crypto v0.0.0-20191218222705-effae4ea9f03
	github.com/filecoin-project/go-data-transfer v0.3.0
	github.com/filecoin-project/go-fil-commcid v0.0.0-20200208005934-2b8bd03caca5
	github.com/filecoin-project/go-fil-markets v0.2.5
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/filecoin-project/go-paramfetch v0.0.2-0.20200505180321-973f8949ea8e
	github.com/filecoin-project/go-statemachine v0.0.0-20200314004106-2e7830fc1948 // indirect
	github.com/filecoin-project/go-statestore v0.1.0
	github.com/filecoin-project/go-storedcounter v0.0.0-20200421200003-1c99c62e8a5b
	github.com/filecoin-project/sector-storage v0.0.0-20200508203401-a74812ba12f3
	github.com/filecoin-project/specs-actors v0.5.3
	github.com/filecoin-project/specs-storage v0.0.0-20200427220711-ae5a7f26c65f // indirect
	github.com/filecoin-project/storage-fsm v0.0.0-20200508212339-4980cb4c92b1
	github.com/fxamacker/cbor v1.5.0
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golangci/golangci-lint v1.21.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/ipfs/go-bitswap v0.2.8
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.3
	github.com/ipfs/go-cid v0.0.5
	github.com/ipfs/go-datastore v0.4.4
	github.com/ipfs/go-ds-badger2 v0.0.0-20200211201106-609c9d2a39c7
	github.com/ipfs/go-fs-lock v0.0.1
	github.com/ipfs/go-graphsync v0.0.6-0.20200504202014-9d5f2c26a103
	github.com/ipfs/go-hamt-ipld v0.1.1-0.20200501020327-d53d20a7063e
	github.com/ipfs/go-ipfs-blockstore v1.0.0
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-cmdkit v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-keystore v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.5-0.20200204214505-252690b78669
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.3
	github.com/ipfs/go-log/v2 v2.0.4 // indirect
	github.com/ipfs/go-merkledag v0.3.1
	github.com/ipfs/go-path v0.0.7
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/iptb v1.3.8-0.20190401234037-98ccf4228a73
	github.com/ipld/go-car v0.1.1-0.20200429200904-c222d793c339
	github.com/ipld/go-ipld-prime v0.0.2-0.20200428162820-8b59dc292b8e
	github.com/ipsn/go-secp256k1 v0.0.0-20180726113642-9d62b9f0bc52
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/libp2p/go-libp2p v0.8.1
	github.com/libp2p/go-libp2p-autonat-svc v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.2.1
	github.com/libp2p/go-libp2p-core v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.1.1
	github.com/libp2p/go-libp2p-peerstore v0.2.2
	github.com/libp2p/go-libp2p-pubsub v0.2.6
	github.com/libp2p/go-libp2p-swarm v0.2.3
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multiaddr-dns v0.2.0
	github.com/multiformats/go-multiaddr-net v0.1.4
	github.com/multiformats/go-multibase v0.0.2 // indirect
	github.com/multiformats/go-multihash v0.0.13
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/otiai10/copy v1.0.2
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/prometheus/procfs v0.0.11 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.5.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/whyrusleeping/cbor-gen v0.0.0-20200501014322-5f9941ef88e0
	github.com/whyrusleeping/go-logging v0.0.1
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.14.1
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543
	google.golang.org/api v0.13.0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/urfave/cli.v2 v2.0.0-20180128182452-d3ae77c26ac8
	gotest.tools v2.2.0+incompatible
	howett.net/plist v0.0.0-20200419221736-3b63eb3a43b5 // indirect
)

replace github.com/filecoin-project/filecoin-ffi => ./vendors/filecoin-ffi
