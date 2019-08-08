module github.com/filecoin-project/go-filecoin

go 1.12

require (
	cloud.google.com/go v0.43.0 // indirect
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Workiva/go-datastructures v1.0.50
	github.com/btcsuite/goleveldb v1.0.0 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.13+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/cskr/pubsub v1.0.2
	github.com/dgraph-io/badger v1.6.0 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v0.7.3-0.20190315170154-87d593639c77
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/go-critic/go-critic v0.0.0-20181204210945-ee9bf5809ead // indirect
	github.com/go-kit/kit v0.9.0 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/golangci/golangci-lint v1.17.0
	github.com/golangci/govet v0.0.0-20180818181408-44ddbe260190 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/pprof v0.0.0-20190723021845-34ac40c74b70 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.5 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/ipfs/go-bitswap v0.1.5
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.0.2
	github.com/ipfs/go-car v0.0.1
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.0.5
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ds-leveldb v0.0.2 // indirect
	github.com/ipfs/go-fs-lock v0.0.1
	github.com/ipfs/go-graphsync v0.0.1-filecoin
	github.com/ipfs/go-hamt-ipld v0.0.1
	github.com/ipfs/go-ipfs-blockstore v0.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.1
	github.com/ipfs/go-ipfs-cmdkit v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.1
	github.com/ipfs/go-ipfs-flags v0.0.1 // indirect
	github.com/ipfs/go-ipfs-keystore v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.1
	github.com/ipfs/go-ipld-format v0.0.1
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.0.2
	github.com/ipfs/go-path v0.0.1
	github.com/ipfs/go-unixfs v0.0.1
	github.com/ipfs/iptb v1.3.8-0.20190401234037-98ccf4228a73
	github.com/ipld/go-ipld-prime v0.0.1-filecoin
	github.com/ipsn/go-secp256k1 v0.0.0-20180726113642-9d62b9f0bc52
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jbenet/goprocess v0.1.3
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/json-iterator/go v1.1.7 // indirect
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/libp2p/go-libp2p v0.2.1
	github.com/libp2p/go-libp2p-autonat-svc v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.1.0
	github.com/libp2p/go-libp2p-core v0.0.9
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-host v0.0.1
	github.com/libp2p/go-libp2p-interface-connmgr v0.0.1
	github.com/libp2p/go-libp2p-kad-dht v0.1.1
	github.com/libp2p/go-libp2p-metrics v0.0.1
	github.com/libp2p/go-libp2p-net v0.1.0
	github.com/libp2p/go-libp2p-peer v0.2.0
	github.com/libp2p/go-libp2p-peerstore v0.1.2
	github.com/libp2p/go-libp2p-protocol v0.0.1
	github.com/libp2p/go-libp2p-pubsub v0.1.0
	github.com/libp2p/go-libp2p-routing v0.1.0
	github.com/libp2p/go-libp2p-swarm v0.1.1
	github.com/libp2p/go-stream-muxer v0.0.1
	github.com/libp2p/go-testutil v0.0.1 // indirect
	github.com/magiconair/properties v1.8.1
	github.com/miekg/dns v1.1.15 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/minio/sha256-simd v0.1.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multiaddr-net v0.0.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.6
	github.com/multiformats/go-multistream v0.1.0
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/otiai10/copy v1.0.1
	github.com/otiai10/curr v0.0.0-20150429015615-9b4961190c95 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/polydawn/refmt v0.0.0-20190408063855-01bf1e26dd14
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.6.0 // indirect
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/rogpeppe/fastuuid v1.2.0 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.4.0 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/ugorji/go v1.1.7 // indirect
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	github.com/whyrusleeping/go-smux-yamux v2.0.9+incompatible // indirect
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.1.0
	go.etcd.io/bbolt v1.3.3 // indirect
	go.opencensus.io v0.22.0
	golang.org/x/exp v0.0.0-20190718202018-cfdd5522f6f6 // indirect
	golang.org/x/image v0.0.0-20190703141733-d6a02ce849c9 // indirect
	golang.org/x/mobile v0.0.0-20190719004257-d2bd2a29d028 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/tools v0.0.0-20190726230722-1bd56024c620 // indirect
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7 // indirect
	google.golang.org/grpc v1.22.1 // indirect
	gotest.tools v2.2.0+incompatible
	sourcegraph.com/sourcegraph/go-diff v0.5.1-0.20190210232911-dee78e514455 // indirect
)
