package main

import (
	"log"
	"path/filepath"

	"github.com/filecoin-project/venus/pkg/chain"
	market1 "github.com/filecoin-project/venus/pkg/market"
	"github.com/filecoin-project/venus/pkg/net/helloprotocol"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	types2 "github.com/filecoin-project/venus/venus-shared/actors/types"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
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
				types.MessageRoot{},
				types.BlockMsg{},
				types.ExpTipSet{},
				types.PaymentInfo{},
				types.Event{},
				types.EventEntry{},
				types.GasTrace{},
				types.MessageTrace{},
				types.ReturnTrace{},
				types.ExecutionTrace{},
			},
		},
		{
			dir: "../venus-shared/actors/types/",
			types: []interface{}{
				types2.Message{},
				types2.SignedMessage{},
				types.ActorV4{},
				types.Actor{},
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
				market.TimeStamp{},
				market.SignedStorageAsk{},
			},
		},
		{
			dir: "../pkg/market",
			types: []interface{}{
				market1.FundedAddressState{},
			},
		},
		{
			dir: "../pkg/net/helloprotocol",
			types: []interface{}{
				helloprotocol.HelloMessage{},
				helloprotocol.LatencyMessage{},
			},
		},
		{
			dir: "../pkg/vm/dispatch",
			types: []interface{}{
				dispatch.SimpleParams{},
			},
		},
		{
			dir: "../pkg/state/tree",
			types: []interface{}{
				tree.StateRoot{},
			},
		},
		{
			dir: "../pkg/chain",
			types: []interface{}{
				chain.TSState{},
			},
		},
		{
			dir: "../venus-shared/blockstore",
			types: []interface{}{
				blockstore.NetRPCReq{},
				blockstore.NetRPCResp{},
				blockstore.NetRPCErr{},
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
