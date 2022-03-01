module github.com/filecoin-project/venus/venus-component

go 1.16

require (
	github.com/filecoin-project/go-cbor-util v0.0.1
	github.com/filecoin-project/venus/venus-shared v0.0.0
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/libp2p/go-libp2p-core v0.11.0
	go.opencensus.io v0.23.0
	go.uber.org/fx v1.15.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/filecoin-project/venus/venus-shared => github.com/dtynn/venus/venus-shared v0.0.0-20211123072147-edbf49c4507e
