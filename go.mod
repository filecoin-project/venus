module github.com/filecoin-project/venus

go 1.14

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/GeertJohan/go.rice v1.0.0
	github.com/Gurpartap/async v0.0.0-20180927173644-4f7f499dd9ee
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/cskr/pubsub v1.0.2
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/detailyang/go-fallocate v0.0.0-20180908115635-432fa640bd2e
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v0.7.3-0.20190315170154-87d593639c77
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0
	github.com/drand/drand v1.2.1
	github.com/drand/kyber v1.1.4
	github.com/fatih/color v1.9.0
	github.com/filecoin-project/filecoin-ffi v0.30.4-0.20200716204036-cddc56607e1d
	github.com/filecoin-project/go-address v0.0.4
	github.com/filecoin-project/go-amt-ipld/v2 v2.1.1-0.20201006184820-924ee87a1349
	github.com/filecoin-project/go-bitfield v0.2.2
	github.com/filecoin-project/go-crypto v0.0.0-20191218222705-effae4ea9f03
	github.com/filecoin-project/go-data-transfer v0.9.0
	github.com/filecoin-project/go-fil-commcid v0.0.0-20200716160307-8f644712406f
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/filecoin-project/go-multistore v0.0.3
	github.com/filecoin-project/go-paramfetch v0.0.2-0.20200701152213-3e0f0afdc261
	github.com/filecoin-project/go-state-types v0.0.0-20201013222834-41ea465f274f
	github.com/filecoin-project/go-storedcounter v0.0.0-20200421200003-1c99c62e8a5b
	github.com/filecoin-project/specs-actors v0.9.13
	github.com/filecoin-project/specs-actors/v2 v2.2.0
	github.com/filecoin-project/test-vectors/schema v0.0.5
	github.com/fxamacker/cbor/v2 v2.2.0
	github.com/go-errors/errors v1.0.1
	github.com/go-kit/kit v0.10.0
	github.com/golangci/golangci-lint v1.21.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-bitswap v0.2.20
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.4-0.20200624145336-a978cec6e834
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ds-badger2 v0.1.1-0.20200708190120-187fc06f714e
	github.com/ipfs/go-fs-lock v0.0.6
	github.com/ipfs/go-graphsync v0.3.1
	github.com/ipfs/go-ipfs-blockstore v1.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-cmdkit v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.1.0
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-keystore v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.5-0.20200428170625-a0bd04d3cbdf
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-log/v2 v2.1.2-0.20200626104915-0016c0b4b3e4
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-path v0.0.7
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/iptb v1.4.0
	github.com/ipld/go-car v0.1.1-0.20201015032735-ff6ccdc46acc
	github.com/ipld/go-ipld-prime v0.5.1-0.20200910124733-350032422383
	github.com/ipsn/go-secp256k1 v0.0.0-20180726113642-9d62b9f0bc52
	github.com/jbenet/goprocess v0.1.4
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/libp2p/go-eventbus v0.2.1
	github.com/libp2p/go-libp2p v0.11.0
	github.com/libp2p/go-libp2p-autonat-svc v0.2.0
	github.com/libp2p/go-libp2p-circuit v0.3.1
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-kad-dht v0.8.3
	github.com/libp2p/go-libp2p-mplex v0.2.4
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/libp2p/go-libp2p-pubsub v0.3.6
	github.com/libp2p/go-libp2p-swarm v0.2.8
	github.com/libp2p/go-libp2p-yamux v0.2.8
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multiaddr-dns v0.2.0
	github.com/multiformats/go-multiaddr-net v0.2.0
	github.com/multiformats/go-multihash v0.0.14
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.6.0
	github.com/prometheus/common v0.10.0
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.5.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/supranational/blst v0.1.1
	github.com/urfave/cli/v2 v2.3.0 // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20200826160007-0b9f6c5fb163
	github.com/whyrusleeping/go-logging v0.0.1
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.15.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.3.1-0.20200828183125-ce943fd02449 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20201008141435-b3e1573b7520
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/urfave/cli.v2 v2.0.0-20180128182452-d3ae77c26ac8
	gotest.tools v2.2.0+incompatible
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect

)

replace github.com/filecoin-project/filecoin-ffi => ./vendors/filecoin-ffi

replace github.com/filecoin-project/test-vectors => ./vendors/test-vectors

replace github.com/supranational/blst => ./vendors/fil-blst/blst

replace github.com/filecoin-project/fil-blst => ./vendors/fil-blst
