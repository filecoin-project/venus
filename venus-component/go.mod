module github.com/filecoin-project/venus/venus-component

go 1.16

require (
	github.com/filecoin-project/go-cbor-util v0.0.1
	github.com/filecoin-project/venus/venus-shared v0.0.0
	github.com/ipfs/go-cid v0.3.2
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/libp2p/go-libp2p v0.23.2
	go.opencensus.io v0.23.0
	go.uber.org/fx v1.15.0
)

replace github.com/filecoin-project/venus/venus-shared => github.com/dtynn/venus/venus-shared v0.0.0-20211123072147-edbf49c4507e
