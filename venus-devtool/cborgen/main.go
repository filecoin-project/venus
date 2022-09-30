package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/fvm"
	market1 "github.com/filecoin-project/venus/pkg/market"
	"github.com/filecoin-project/venus/pkg/net/helloprotocol"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/venus-devtool/util"
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
				types.SignedMessage{},
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
			dir: "../pkg/fvm",
			types: []interface{}{
				fvm.FvmExecutionTrace{},
				fvm.FvmGasCharge{},
			},
		},
	}

	for _, target := range targets {
		pkg := target.pkg
		if pkg == "" {
			pkg = filepath.Base(target.dir)
		}

		if err := WriteTupleEncodersToFile(filepath.Join(target.dir, "cbor_gen.go"), pkg, target.types...); err != nil {
			log.Fatalf("gen for %s: %s", target.dir, err)
		}
	}
}

// WriteTupleEncodersToFile copy from https://github.com/whyrusleeping/cbor-gen/blob/master/writefile.go#L16
func WriteTupleEncodersToFile(fname, pkg string, types ...interface{}) error {
	buf := new(bytes.Buffer)

	typeInfos := make([]*gen.GenTypeInfo, len(types))
	for i, t := range types {
		gti, err := gen.ParseTypeInfo(t)
		if err != nil {
			return fmt.Errorf("failed to parse type info: %w", err)
		}
		typeInfos[i] = gti
	}

	if err := gen.PrintHeaderAndUtilityMethods(buf, pkg, typeInfos); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, t := range typeInfos {
		if err := gen.GenTupleEncodersForType(t, buf); err != nil {
			return fmt.Errorf("failed to generate encoders: %w", err)
		}
	}

	srcData := buf.Bytes()
	if strings.Contains(fname, "pkg/fvm") {
		srcData = bytes.ReplaceAll(srcData, []byte(`internal "github.com/filecoin-project/venus/venus-shared/internal"`), []byte{})
		srcData = bytes.ReplaceAll(srcData, []byte("internal."), []byte("types."))
	}

	data, err := util.FmtFile("", srcData)
	if err != nil {
		return err
	}

	fi, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	_, err = fi.Write(data)
	if err != nil {
		_ = fi.Close()
		return err
	}
	_ = fi.Close()

	return nil
}
