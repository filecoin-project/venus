package main

import (
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/filecoin-project/venus/pkg/fvm"
	"github.com/filecoin-project/venus/pkg/market"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	if err := gen.WriteTupleEncodersToFile("./pkg/paychmgr/cbor_gen.go", "paychmgr",
		paychmgr.VoucherInfo{},
		paychmgr.ChannelInfo{},
		paychmgr.MsgInfo{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/market/cbor_gen.go", "market",
		market.FundedAddressState{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/discovery/cbor_gen.go", "discovery",
		discovery.HelloMessage{},
		discovery.LatencyMessage{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/vm/dispatch/cbor_gen.go", "dispatch",
		dispatch.SimpleParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/state/tree/cbor_gen.go", "tree",
		tree.StateRoot{},
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
		chain.TSState{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./pkg/fvm/cbor_gen.go", "fvm",
		fvm.FvmExecutionTrace{},
	); err != nil {
		panic(err)
	}
}
