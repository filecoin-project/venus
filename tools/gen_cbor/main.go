package main

import (
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/state"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	if err := gen.WriteTupleEncodersToFile("./pkg/types/cbor_gen.go", "types",
		types.MessageReceipt{},
		types.SignedMessage{},
		types.UnsignedMessage{},
		types.TxMeta{},
		types.Actor{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/discovery/cbor_gen.go", "discovery",
		discovery.HelloMessage{},
		discovery.LatencyMessage{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/crypto/cbor_gen.go", "crypto",
		crypto.KeyInfo{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/vm/dispatch/cbor_gen.go", "dispatch",
		dispatch.SimpleParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/vm/state/cbor_gen.go", "state",
		state.StateRoot{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/chainsync/exchange/cbor_gen.go", "exchange",
		exchange.Request{},
		exchange.Response{},
		exchange.CompactedMessages{},
		exchange.BSTipSet{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/chain/cbor_gen.go", "chain",
		chain.TsState{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/block/cbor_gen.go", "block",
		block.BeaconEntry{},
		block.Block{},
		block.Ticket{},
		block.ElectionProof{},
		block.PoStProof{},
		block.BlockMsg{},
		/*
			types.ExpTipSet{},

			types.StateRoot{},
			types.StateInfo0{},

		*/

	); err != nil {
		panic(err)
	}
}
