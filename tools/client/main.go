package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/filecoin-project/venus/app/client"
	"github.com/filecoin-project/venus/app/client/v0api"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
)

const Filecoin = "Filecoin"

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
			fmt.Println(string(debug.Stack()))
		}
	}()

	ctx := context.Background()

	addrBase := "/ip4/127.0.0.1/tcp/3453"
	addr, err := dialArgs(addrBase, "v0")
	checkErr(err)
	addr2, err := dialArgs(addrBase, "v1")
	checkErr(err)

	handler := http.Header{}
	var cliV0 v0api.FullNodeStruct
	var cliV1 client.FullNodeStruct

	closeV0, err := jsonrpc.NewClient(ctx, addr, Filecoin, &cliV0, handler)
	checkErr(err)
	defer closeV0()

	closeV1, err := jsonrpc.NewClient(ctx, addr2, Filecoin, &cliV1, handler)
	checkErr(err)
	defer closeV1()

	v, err := cliV0.Version(ctx)
	checkErr(err)
	fmt.Println("Version", v)

	head, err := cliV0.ChainHead(ctx)
	checkErr(err)
	fmt.Println(head)

	var checkOver bool
	var i int
	for ; !checkOver; i++ {
		fullBlock, err := cliV0.GetFullBlock(ctx, head.Blocks()[0].Cid())
		checkErr(err)
		fmt.Println("GetFullBlock ", fullBlock.Header)

		for _, m := range fullBlock.BLSMessages {
			checkOver = true
			msgLookup, err := cliV0.StateWaitMsg(ctx, m.Cid(), constants.DefaultConfidence)
			checkErr(err)
			fmt.Println("StateWaitMsg ", msgLookup)

			msgLookup, err = cliV0.StateSearchMsg(ctx, m.Cid())
			checkErr(err)
			fmt.Println("StateSearchMsg ", msgLookup)

			receipt, err := cliV0.StateGetReceipt(ctx, m.Cid(), types.TipSetKey{})
			checkErr(err)
			fmt.Println("StateGetReceipt ", receipt)

			break
		}

		for _, m := range fullBlock.BLSMessages {
			checkOver = true
			msgLookup, err := cliV1.StateWaitMsg(ctx, m.Cid(), constants.DefaultConfidence, constants.LookbackNoLimit, true)
			checkErr(err)
			fmt.Println("StateWaitMsg ", msgLookup)

			msgLookup, err = cliV1.StateSearchMsg(ctx, types.TipSetKey{}, m.Cid(), constants.LookbackNoLimit, true)
			checkErr(err)
			fmt.Println("StateSearchMsg ", msgLookup)

			break
		}

		pt, err := cliV0.ChainGetTipSet(ctx, head.Parents())
		checkErr(err)
		fmt.Println("parent ", pt, "height ", pt.Height())
		head = pt
	}

}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// eg. "bafy2bzaceb2ff6egw54sjcarqvl22mkitmr7q5rmlinza6nnhk6p44t5eee24"
func toCid(cidStr string) cid.Cid { // nolint
	cid, err := cid.Decode(cidStr)
	checkErr(err)
	return cid
}

func dialArgs(addr string, version string) (string, error) {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err == nil {
		_, addr, err := manet.DialArgs(ma)
		if err != nil {
			return "", err
		}

		return "ws://" + addr + "/rpc/" + version, nil
	}

	_, err = url.Parse(addr)
	if err != nil {
		return "", err
	}
	return addr + "/rpc/" + version, nil
}
