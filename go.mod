module github.com/filecoin-project/go-filecoin

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Workiva/go-datastructures v1.0.50
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/btcsuite/goleveldb v1.0.0 // indirect
	github.com/cskr/pubsub v1.0.2
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v0.7.3-0.20190315170154-87d593639c77
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/filecoin-project/go-amt-ipld v0.0.0-20190920035751-ae3c37184616
	github.com/filecoin-project/go-bls-sigs v0.0.0
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/filecoin-project/go-sectorbuilder v0.0.0-20190801004428-e75bc9b0aaea
	github.com/go-check/check v0.0.0-20190902080502-41f04d3bba15 // indirect
	github.com/golang/groupcache v0.0.0-20191027212112-611e8accdfc9 // indirect
	github.com/golangci/golangci-lint v1.21.0
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/ipfs/go-bitswap v0.1.5
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-car v0.0.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.1.1
	github.com/ipfs/go-ds-badger v0.0.7
	github.com/ipfs/go-fs-lock v0.0.1
	github.com/ipfs/go-graphsync v0.0.3
	github.com/ipfs/go-hamt-ipld v0.0.13
	github.com/ipfs/go-ipfs-blockstore v0.1.0
	github.com/ipfs/go-ipfs-chunker v0.0.1
	github.com/ipfs/go-ipfs-cmdkit v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.3
	github.com/ipfs/go-ipfs-keystore v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-log v0.0.2-0.20190920042044-a609c1ae5144
	github.com/ipfs/go-merkledag v0.2.4
	github.com/ipfs/go-path v0.0.1
	github.com/ipfs/go-unixfs v0.2.2
	github.com/ipfs/iptb v1.3.8-0.20190401234037-98ccf4228a73
	github.com/ipld/go-ipld-prime v0.0.1-filecoin
	github.com/ipsn/go-secp256k1 v0.0.0-20180726113642-9d62b9f0bc52
	github.com/jbenet/goprocess v0.1.3
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/libp2p/go-flow-metrics v0.0.2 // indirect
	github.com/libp2p/go-libp2p v0.4.1-0.20191006140250-5f60501a04d5
	github.com/libp2p/go-libp2p-autonat-svc v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.1.3
	github.com/libp2p/go-libp2p-core v0.2.4
	github.com/libp2p/go-libp2p-kad-dht v0.1.1
	github.com/libp2p/go-libp2p-peerstore v0.1.4
	github.com/libp2p/go-libp2p-pubsub v0.2.1
	github.com/libp2p/go-libp2p-secio v0.2.1 // indirect
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/libp2p/go-libp2p-testing v0.1.1 // indirect
	github.com/libp2p/go-stream-muxer v0.0.1
	github.com/libp2p/go-yamux v1.2.4 // indirect
	github.com/magiconair/properties v1.8.1
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/multiformats/go-multiaddr v0.1.1
	github.com/multiformats/go-multiaddr-dns v0.2.0
	github.com/multiformats/go-multiaddr-net v0.1.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.8
	github.com/multiformats/go-multistream v0.1.0
	github.com/onsi/ginkgo v1.10.3 // indirect
	github.com/onsi/gomega v1.7.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/otiai10/copy v1.0.1
	github.com/otiai10/curr v0.0.0-20150429015615-9b4961190c95 // indirect
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/polydawn/refmt v0.0.0-20190408063855-01bf1e26dd14
	github.com/prometheus/client_golang v1.2.1
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.5.0 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20191001154818-b4b5288fcb86
	github.com/whyrusleeping/go-logging v0.0.1
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	go.opencensus.io v0.22.1
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.12.0
	golang.org/x/crypto v0.0.0-20191112222119-e1110fd1c708 // indirect
	golang.org/x/net v0.0.0-20191101175033-0deb6923b6d9 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20191104094858-e8c54fb511f6 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.0.0-20191104182621-d415e1c6f957 // indirect
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898
	google.golang.org/api v0.13.0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20191028173616-919d9bdd9fe6 // indirect
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/urfave/cli.v2 v2.0.0-20180128182452-d3ae77c26ac8
	gopkg.in/yaml.v2 v2.2.5 // indirect
	gotest.tools v2.2.0+incompatible
)

replace github.com/filecoin-project/go-bls-sigs => ./vendors/go-bls-sigs

replace github.com/filecoin-project/go-sectorbuilder => ./vendors/go-sectorbuilder
