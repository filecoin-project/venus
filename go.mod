module github.com/filecoin-project/venus

go 1.16

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	contrib.go.opencensus.io/exporter/prometheus v0.4.0
	github.com/Gurpartap/async v0.0.0-20180927173644-4f7f499dd9ee
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d
	github.com/ahmetb/go-linq/v3 v3.2.0
	github.com/awnumar/memguard v0.22.2
	github.com/bluele/gcache v0.0.0-20190518031135-bc40bd653833
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/detailyang/go-fallocate v0.0.0-20180908115635-432fa640bd2e
	github.com/dgraph-io/badger/v2 v2.2007.3
	github.com/docker/go-units v0.4.0
	github.com/drand/drand v1.3.0
	github.com/drand/kyber v1.1.7
	github.com/fatih/color v1.13.0
	github.com/filecoin-project/filecoin-ffi v0.30.4-0.20200910194244-f640612a1a1f
	github.com/filecoin-project/go-address v0.0.6
	github.com/filecoin-project/go-amt-ipld/v2 v2.1.1-0.20201006184820-924ee87a1349 // indirect
	github.com/filecoin-project/go-bitfield v0.2.4
	github.com/filecoin-project/go-cbor-util v0.0.1
	github.com/filecoin-project/go-commp-utils v0.1.3
	github.com/filecoin-project/go-crypto v0.0.1
	github.com/filecoin-project/go-data-transfer v1.15.1
	github.com/filecoin-project/go-fil-commcid v0.1.0
	github.com/filecoin-project/go-fil-markets v1.20.1
	github.com/filecoin-project/go-jsonrpc v0.1.4-0.20210217175800-45ea43ac2bec
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/filecoin-project/go-paramfetch v0.0.4
	github.com/filecoin-project/go-state-types v0.1.3
	github.com/filecoin-project/specs-actors v0.9.14
	github.com/filecoin-project/specs-actors/v2 v2.3.6
	github.com/filecoin-project/specs-actors/v3 v3.1.1
	github.com/filecoin-project/specs-actors/v4 v4.0.1
	github.com/filecoin-project/specs-actors/v5 v5.0.4
	github.com/filecoin-project/specs-actors/v6 v6.0.1
	github.com/filecoin-project/specs-actors/v7 v7.0.0
	github.com/filecoin-project/specs-storage v0.2.2
	github.com/filecoin-project/test-vectors/schema v0.0.5
	github.com/filecoin-project/venus-auth v1.3.2
	github.com/fxamacker/cbor/v2 v2.4.0
	github.com/gbrlsnchs/jwt/v3 v3.0.1
	github.com/go-errors/errors v1.0.1
	github.com/go-kit/kit v0.12.0
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/golang/mock v1.6.0
	github.com/google/flatbuffers v2.0.0+incompatible // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/hannahhoward/go-pubsub v0.0.0-20200423002714-8d62886cc36e
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/howeyc/gopass v0.0.0-20190910152052-7cb4b85ec19c
	github.com/ipfs-force-community/metrics v1.0.1-0.20211022060227-11142a08b729
	github.com/ipfs-force-community/venus-common-utils v0.0.0-20210924063144-1d3a5b30de87
	github.com/ipfs/go-bitswap v0.5.1
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.2.1
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-datastore v0.5.1
	github.com/ipfs/go-ds-badger2 v0.1.2
	github.com/ipfs/go-fs-lock v0.0.7
	github.com/ipfs/go-graphsync v0.13.1
	github.com/ipfs/go-ipfs-blockstore v1.1.2
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-cmdkit v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.6.1-0.20220212012746-40b8fddb899f
	github.com/ipfs/go-ipfs-ds-help v1.1.0
	github.com/ipfs/go-ipfs-exchange-interface v0.1.0
	github.com/ipfs/go-ipfs-exchange-offline v0.1.1
	github.com/ipfs/go-ipfs-files v0.0.9
	github.com/ipfs/go-ipfs-routing v0.2.1
	github.com/ipfs/go-ipld-cbor v0.0.6
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-log/v2 v2.5.0
	github.com/ipfs/go-merkledag v0.5.1
	github.com/ipfs/go-path v0.2.1
	github.com/ipfs/go-unixfs v0.3.1
	github.com/ipld/go-car v0.3.3
	github.com/jbenet/goprocess v0.1.4
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/libp2p/go-eventbus v0.2.1
	github.com/libp2p/go-libp2p v0.18.0-rc3
	github.com/libp2p/go-libp2p-core v0.14.0
	github.com/libp2p/go-libp2p-kad-dht v0.15.0
	github.com/libp2p/go-libp2p-peerstore v0.6.0
	github.com/libp2p/go-libp2p-pubsub v0.6.1
	github.com/libp2p/go-libp2p-swarm v0.10.1
	github.com/libp2p/go-libp2p-yamux v0.8.1
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/multiformats/go-multihash v0.1.0
	github.com/onsi/gomega v1.15.0 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pborman/uuid v1.2.0
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/raulk/clock v1.1.0
	github.com/rogpeppe/go-internal v1.6.2 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1 // indirect
	github.com/streadway/handy v0.0.0-20200128134331-0f66f006fb2e
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/whyrusleeping/cbor-gen v0.0.0-20220302191723-37c43cae8e14
	github.com/whyrusleeping/go-logging v0.0.1
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	github.com/whyrusleeping/pubsub v0.0.0-20190708150250-92bcb0691325
	go.etcd.io/bbolt v1.3.6 // indirect
	go.opencensus.io v0.23.0
	go.opentelemetry.io/otel/sdk v1.3.0 // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20211209193657-4570a0811e8b
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/api v0.43.0 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/grpc v1.43.0 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gorm.io/driver/mysql v1.1.1
	gorm.io/gorm v1.21.12
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi
	github.com/filecoin-project/go-jsonrpc => github.com/ipfs-force-community/go-jsonrpc v0.1.4-0.20210731021807-68e5207079bc
	github.com/filecoin-project/test-vectors => ./extern/test-vectors
)
