package main

import (
	"log"
	"path/filepath"

	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
	"github.com/filecoin-project/venus/venus-shared/libp2p/hello"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"

	gen "github.com/whyrusleeping/cbor-gen"
)

type genTarget struct {
	dir   string
	pkg   string
	types []interface{}
}

func main() {
	targets := []genTarget{
		{
			dir: "../venus-shared/libp2p/hello/",
			types: []interface{}{
				hello.GreetingMessage{},
				hello.LatencyMessage{},
			},
		},
		{
			dir: "../venus-shared/libp2p/exchange/",
			types: []interface{}{
				exchange.Request{},
				exchange.Response{},
				exchange.CompactedMessages{},
				exchange.BSTipSet{},
			},
		},
		{
			dir: "../venus-shared/types/",
			types: []interface{}{
				types.BlockHeader{},
				types.Ticket{},
				types.ElectionProof{},
				types.BeaconEntry{},
				//types.Message{},
				types.SignedMessage{},
				//types.Actor{},
				types.MessageRoot{},
				types.MessageReceipt{},
				types.BlockMsg{},
				types.ExpTipSet{},
				types.PaymentInfo{},
			},
		},
		{
			dir: "../venus-shared/internal/",
			types: []interface{}{
				types.Actor{},
				types.Message{},
			},
		},
		{
			dir: "../venus-shared/types/market",
			types: []interface{}{
				market.FundedAddressState{},
				market.MsgInfo{},
				market.ChannelInfo{},
				market.VoucherInfo{},
				market.MinerDeal{},
				market.RetrievalAsk{},
				market.ProviderDealState{},
			},
		},
	}

	for _, target := range targets {
		pkg := target.pkg
		if pkg == "" {
			pkg = filepath.Base(target.dir)
		}

		if err := gen.WriteTupleEncodersToFile(filepath.Join(target.dir, "cbor_gen.go"), pkg, target.types...); err != nil {
			log.Fatalf("gen for %s: %s", target.dir, err)
		}
	}
}
