package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"

	"github.com/filecoin-project/venus/app/submodule/mining"
	venustypes "github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"

	lotusrepo "github.com/filecoin-project/lotus/node/repo"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/pkg/block"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	lotusapi "github.com/filecoin-project/lotus/api"
	client2 "github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/venus/app/client"
	ipfscid "github.com/ipfs/go-cid"
	cli2 "github.com/urfave/cli/v2"
)

var resultNotMatch = "result not match"

type Nodes struct {
	VenusNode   client.FullNode
	VenusCloser jsonrpc.ClientCloser
	LotusNode   api.FullNode
	LotusClose  jsonrpc.ClientCloser
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, "\n", string(debug.Stack()))
		}
	}()
	ctx := context.Background()
	n := &Nodes{}
	defer n.Close()

	venusNode, closer, err := client.NewFullNode(ctx)
	judgeError(err)
	n.VenusNode = venusNode
	n.VenusCloser = closer

	lotusNode, lotusCloser, err := newLotusNode(ctx)
	judgeError(err)
	n.LotusNode = lotusNode
	n.LotusClose = lotusCloser

	//n.ChainHead(ctx) // ok
	//n.ChainReadObj(ctx) // ok
	//n.ChainGetTipSetByHeight(ctx) // fali lotus 当tipsetkey为空时，使用HeaviestTipSet ok
	//n.ChainGetBlock(ctx)          // ok
	//n.ChainGetBlockMessages(ctx) // ok
	//n.ChainGetMessages(ctx) // ok
	//n.ChainHasObj(ctx)    // ok
	//n.ChainGetTipSet(ctx) // ok

	//n.StateSectorPreCommitInfo(ctx) //ok
	//n.StateSectorGetInfo(ctx)       // ok
	//n.StateSectorPartition(ctx)     // ok
	//n.StateMinerPreCommitDepositForPower(ctx) // fail ok
	//n.StateMarketStorageDeal(ctx) // ok
	//n.StateMinerInitialPledgeCollateral(ctx) // ok
	//n.StateMinerInitialPledgeCollateral(ctx) // ok
	//n.StateNetworkVersion(ctx)       // ok
	//n.StateMinerInfo(ctx)            // ok
	//n.StateMinerSectorSize(ctx)    // lotus 无对应函数 ok
	//n.StateMinerWorkerAddress(ctx) // lotus 无对应函数 ok
	//n.StateMinerSectorAllocated(ctx) // ok
	//n.StateMinerSectors(ctx)    // ok
	//n.StateMinerDeadlines(ctx)  // ok
	//n.StateMinerPartitions(ctx) // ok
	//n.StateMinerProvingDeadline(ctx) // ok
	//n.StateGetActor(ctx)             // ok
	//n.StateMinerFaults(ctx)          // ok
	//n.StateMinerRecoveries(ctx) // ok
	//n.StateAccountKey(ctx) // ok
	//n.StateGetReceipt(ctx) // ok
	//n.StateCall(ctx) // 结果有差异 lotus gas写死的0 ok
	//n.StateSearchMsg(ctx) // 结果有差异 高度差距1 ok
	//n.StateWaitMsg(ctx) // ok
	//n.ChainGetRandomnessFromBeacon(ctx)  // ok
	//n.ChainGetRandomnessFromTickets(ctx) // ok

	// winning poster
	//n.BeaconGetEntry(ctx) // ok
	//n.ChainTipSetWeight(ctx) // ok

	//n.WalletSign(ctx) // fail ok

	//n.MinerGetBaseInfo(ctx) // fail ok
	//n.SyncSubmitBlock(ctx)  // fail 返回值不一致
	//n.MinerCreateBlock(ctx) // fail

	//testJsonUnmarshal()
}

func newLotusNode(ctx context.Context) (api.FullNode, jsonrpc.ClientCloser, error) {
	app := cli2.NewApp()
	app.Metadata = map[string]interface{}{
		"repoType": lotusrepo.FullNode,
	}
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	temp := ""
	fs.StringVar(&temp, "repo", "~/.lotusDevnet", "")

	cctx := cli2.NewContext(app, fs, nil)
	addr, headers, err := cli.GetRawAPI(cctx, lotusrepo.FullNode)
	if err != nil {
		return nil, nil, err
	}

	return client2.NewFullNodeRPC(ctx, addr, headers)
}

func (n *Nodes) Close() {
	if n.VenusCloser != nil {
		n.VenusCloser()
	}
	if n.LotusClose != nil {
		n.LotusClose()
	}
}

func (n *Nodes) GetChainHead(ctx context.Context) *block.TipSet {
	ts, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	return ts
}

func trim(s string) string {
	s = strings.Replace(s, " ", "", -1)
	s = strings.Trim(s, "{")
	s = strings.Trim(s, "}")
	s = strings.Trim(s, "]")
	s = strings.Trim(s, "[")

	return s
}

func judgeError(err error) {
	if err != nil {
		panic(err)
	}
}

func minerAddr() address.Address {
	addr, err := address.NewFromString("t01000")
	judgeError(err)

	return addr
}

var cidStr = `[
  {
    "/": "bafy2bzaceamcvshqhq4rgs4cudti3c6dc3e65tvub4zdqsevhnfh2krkgi7bq"
  }
]`

func getMessageCID() ipfscid.Cid {
	tsk := types.TipSetKey{}
	err := tsk.UnmarshalJSON([]byte(cidStr))
	judgeError(err)
	cid := tsk.Cids()[0]

	return cid
}

// chain

func (n *Nodes) ChainHead(ctx context.Context) {
	lotusHead, err := n.LotusNode.ChainHead(ctx)
	judgeError(err)

	venusHead, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)

	vStr := trim(venusHead.String())
	lStr := trim(lotusHead.String())
	if vStr != lStr {
		log.Fatal(fmt.Printf("venus head(%s) not match lotus head(%s)", vStr, lStr))
	}
	fmt.Println(venusHead.String(), " ", lotusHead.Key().String())
}

func (n *Nodes) ChainGetRandomnessFromBeacon(ctx context.Context) {
	head := n.GetChainHead(ctx)
	h, err := head.Height()
	judgeError(err)
	tag := crypto.DomainSeparationTag(2)
	b := []byte("Ynl0ZSBhcnJheQ==")

	rand, err := n.VenusNode.ChainGetRandomnessFromBeacon(ctx, head.Key(), tag, h, b)
	judgeError(err)

	rand2, err2 := n.LotusNode.ChainGetRandomnessFromBeacon(ctx, types.NewTipSetKey(head.Key().ToSlice()...), tag, h, b)
	judgeError(err2)

	fmt.Println("ChainGetRandomnessFromBeacon: ", rand)
	fmt.Println("ChainGetRandomnessFromBeacon: ", rand2)
}

func (n *Nodes) ChainGetRandomnessFromTickets(ctx context.Context) {
	head := n.GetChainHead(ctx)
	h, err := head.Height()
	judgeError(err)
	tag := crypto.DomainSeparationTag(2)
	b := []byte("Ynl0ZSBhcnJheQ==")
	rand, err := n.VenusNode.ChainGetRandomnessFromTickets(ctx, head.Key(), tag, h, b)
	judgeError(err)

	rand2, err2 := n.LotusNode.ChainGetRandomnessFromTickets(ctx, types.NewTipSetKey(head.Key().ToSlice()...), tag, h, b)
	judgeError(err2)

	fmt.Println("ChainGetRandomnessFromTickets: ", rand)
	fmt.Println("ChainGetRandomnessFromTickets: ", rand2)
}

func (n *Nodes) ChainReadObj(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	blk := head.Blocks()[0]
	cid := blk.Cid()

	b, err := n.VenusNode.ChainReadObj(ctx, cid)
	judgeError(err)

	b2, err2 := n.LotusNode.ChainReadObj(ctx, cid)
	judgeError(err2)

	fmt.Println(b, " ", b2)
	if !bytes.Equal(b, b2) {
		log.Fatalf("ChainReadObj %s %s != %s", resultNotMatch, string(b), string(b2))
	}
}

func (n *Nodes) ChainGetTipSetByHeight(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	tsk := head.Key()
	height, err := head.Height()
	judgeError(err)

	ts, err := n.VenusNode.ChainGetTipSetByHeight(ctx, height, tsk)
	judgeError(err)

	ts2, err2 := n.LotusNode.ChainGetTipSetByHeight(ctx, height, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	fmt.Println("ChainGetTipSetByHeight: ", ts)
	fmt.Println("ChainGetTipSetByHeight: ", ts2)

	height = 300
	ts, err = n.VenusNode.ChainGetTipSetByHeight(ctx, height, block.TipSetKey{})
	fmt.Println("ChainGetTipSetByHeight: ", ts, " error: ", err)

	ts2, err2 = n.LotusNode.ChainGetTipSetByHeight(ctx, height, types.TipSetKey{})
	fmt.Println("ChainGetTipSetByHeight: ", ts2, " error: ", err2)
}

func (n *Nodes) ChainGetBlock(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	blk := head.Blocks()[0]
	cid := blk.Cid()

	blk, err = n.VenusNode.ChainGetBlock(ctx, cid)
	judgeError(err)
	blkHeader, err2 := n.LotusNode.ChainGetBlock(ctx, cid)
	judgeError(err2)
	fmt.Println("ChainGetBlock: ", blk)
	fmt.Printf("ChainGetBlock: %+v\n", *blkHeader)
}

func (n *Nodes) ChainGetBlockMessages(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	blk := head.Blocks()[0]
	cid := blk.Cid()

	msg, err := n.VenusNode.ChainGetBlockMessages(ctx, cid)
	judgeError(err)
	msg2, err2 := n.LotusNode.ChainGetBlockMessages(ctx, cid)
	judgeError(err2)
	fmt.Printf("ChainGetBlockMessages: %+v\n", msg)
	fmt.Printf("ChainGetBlockMessages: %+v\n", msg2)
}

// 结果不一致
func (n *Nodes) ChainGetMessage(ctx context.Context) {
	cid := getMessageCID()

	fmt.Println("cid: ", cid)

	msg, err := n.VenusNode.ChainGetMessage(ctx, cid)
	//judgeError(err)
	msg2, err2 := n.LotusNode.ChainGetMessage(ctx, cid)
	judgeError(err2)
	fmt.Println("ChainGetMessage: ", msg, " xxx ", err)
	fmt.Println("ChainGetMessage: ", msg2, " xxx ", err2)
}

func (n *Nodes) ChainHasObj(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	blk := head.Blocks()[0]
	cid := blk.Cid()

	found, err := n.VenusNode.ChainHasObj(ctx, cid)
	judgeError(err)
	found2, err2 := n.LotusNode.ChainHasObj(ctx, cid)
	judgeError(err2)

	fmt.Println("ChainHasObj: ", found, found2)
}

func (n *Nodes) ChainGetTipSet(ctx context.Context) {
	head, err := n.VenusNode.ChainHead(ctx)
	judgeError(err)
	tsk := head.Key()

	ts, err := n.VenusNode.ChainGetTipSet(tsk)
	judgeError(err)
	ts2, err2 := n.LotusNode.ChainGetTipSet(ctx, types.NewTipSetKey(ts.Key().ToSlice()...))
	judgeError(err2)

	fmt.Printf("ChainGetTipSet: %+v\n", ts)
	fmt.Printf("ChainGetTipSet: %+v\n", ts2)
}

//--- state ---

func (n *Nodes) StateSectorPreCommitInfo(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	addr := minerAddr()
	sectorNum := abi.SectorNumber(1)

	info, err := n.VenusNode.StateSectorPreCommitInfo(ctx, addr, sectorNum, tsk)
	//judgeError(err)
	info2, err2 := n.LotusNode.StateSectorPreCommitInfo(ctx, addr, sectorNum, types.NewTipSetKey(tsk.ToSlice()...))
	//judgeError(err2)

	fmt.Println("StateSectorPreCommitInfo: ", info, err)
	fmt.Println("StateSectorPreCommitInfo: ", info2, err2)
}

func (n *Nodes) StateSectorGetInfo(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	addr := minerAddr()
	sectorNum := abi.SectorNumber(1)

	info, err := n.VenusNode.StateSectorGetInfo(ctx, addr, sectorNum, tsk)
	judgeError(err)
	info2, err2 := n.LotusNode.StateSectorGetInfo(ctx, addr, sectorNum, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	fmt.Println("StateSectorGetInfo: ", info)
	fmt.Println("StateSectorGetInfo: ", info2)
}

func (n *Nodes) StateSectorPartition(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	addr := minerAddr()
	sectorNum := abi.SectorNumber(1)

	sectorLocation, err := n.VenusNode.StateSectorPartition(ctx, addr, sectorNum, tsk)
	judgeError(err)
	sectorLocation2, err2 := n.LotusNode.StateSectorPartition(ctx, addr, sectorNum, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	fmt.Println("StateSectorPartition: ", sectorLocation)
	fmt.Println("StateSectorPartition: ", sectorLocation2)
}

// lotus无对应函数
func (n *Nodes) StateMinerSectorSize(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	maddr := minerAddr()

	addr, err := n.VenusNode.StateMinerSectorSize(ctx, maddr, tsk)
	judgeError(err)
	//addr2, err2 := n.LotusNode.StateMinerSectorSize(ctx, maddr, types.NewTipSetKey(tsk.ToSlice()...))
	//judgeError(err2)

	fmt.Println("StateMinerWorkerAddress: ", addr)
	//fmt.Println("StateMinerWorkerAddress: ", addr2)
}

// lotus无对应函数
func (n *Nodes) StateMinerWorkerAddress(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	maddr := minerAddr()

	addr, err := n.VenusNode.StateMinerWorkerAddress(ctx, maddr, tsk)
	judgeError(err)
	//addr2, err2 := n.LotusNode.StateMinerWorkerAddress(ctx, maddr,types.NewTipSetKey(tsk.ToSlice()...))
	//judgeError(err2)

	fmt.Println("StateMinerWorkerAddress: ", addr)
	//fmt.Println("StateMinerWorkerAddress: ", addr2)
}

var infoStr = `{
    "SealProof": 0,
    "SectorNumber": 1,
    "SealedCID": {
      "/": "bagboea4b5abcbtv66pwhtgrecd2acedvlgoqtm2gzgr72u4v32zope3x6mnaqbtd"
    },
    "SealRandEpoch": 255,
    "DealIDs": null,
    "Expiration": 255,
    "ReplaceCapacity": true,
    "ReplaceSectorDeadline": 42,
    "ReplaceSectorPartition": 42,
    "ReplaceSectorNumber": 1
  }`

// fail
func (n *Nodes) StateMinerPreCommitDepositForPower(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()
	var info miner.SectorPreCommitInfo
	err := json.Unmarshal([]byte(infoStr), &info)
	judgeError(err)
	i, err := n.VenusNode.StateMinerPreCommitDepositForPower(ctx, addr, info, head.Key())
	//judgeError(err)
	i2, err2 := n.LotusNode.StateMinerPreCommitDepositForPower(ctx, addr, info, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerPreCommitDepositForPower: ", i, err)
	fmt.Println("StateMinerPreCommitDepositForPower: ", i2, err2)
}

// fail
func (n *Nodes) StateMinerInitialPledgeCollateral(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	var info miner.SectorPreCommitInfo
	err := json.Unmarshal([]byte(infoStr), &info)
	judgeError(err)

	i, err := n.VenusNode.StateMinerInitialPledgeCollateral(ctx, addr, info, head.Key())
	judgeError(err)
	i2, err2 := n.LotusNode.StateMinerInitialPledgeCollateral(ctx, addr, info, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerInitialPledgeCollateral: ", i)
	fmt.Println("StateMinerInitialPledgeCollateral: ", i2)
}

func (n *Nodes) StateMarketStorageDeal(ctx context.Context) {
	head := n.GetChainHead(ctx)
	dealID := abi.DealID(0)

	deal, err := n.VenusNode.StateMarketStorageDeal(ctx, dealID, head.Key())
	judgeError(err)
	deal2, err2 := n.LotusNode.StateMarketStorageDeal(ctx, dealID, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMarketStorageDeal: ", deal)
	fmt.Println("StateMarketStorageDeal: ", deal2)
}

func (n *Nodes) StateNetworkVersion(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()

	vn, err := n.VenusNode.StateNetworkVersion(ctx, tsk)
	judgeError(err)
	vn2, err2 := n.LotusNode.StateNetworkVersion(ctx, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	fmt.Println("StateNetworkVersion: ", vn)
	fmt.Println("StateNetworkVersion: ", vn2)
}

func (n *Nodes) StateMinerInfo(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()
	addr := minerAddr()

	info, err := n.VenusNode.StateMinerInfo(ctx, addr, tsk)
	judgeError(err)
	info2, err2 := n.LotusNode.StateMinerInfo(ctx, addr, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerInfo: ", info, " \n", info2)
}

func (n *Nodes) StateMinerSectorAllocated(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()
	sectorNum := abi.SectorNumber(1)

	flag, err := n.VenusNode.StateMinerSectorAllocated(ctx, addr, sectorNum, head.Key())
	judgeError(err)
	flag2, err2 := n.LotusNode.StateMinerSectorAllocated(ctx, addr, sectorNum, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerSectorAllocated: ", flag)
	fmt.Println("StateMinerSectorAllocated: ", flag2)
}

func (n *Nodes) StateMinerSectors(ctx context.Context) {
	ts := n.GetChainHead(ctx)
	tsk := ts.Key()
	addr := minerAddr()

	info, err := n.VenusNode.StateMinerSectors(ctx, addr, nil, tsk)
	judgeError(err)

	info2, err2 := n.LotusNode.StateMinerSectors(ctx, addr, nil, types.NewTipSetKey(tsk.ToSlice()...))
	judgeError(err2)

	//fmt.Printf("StateMinerSectors: %+v\n %+v", info, info2)
	for _, sInfo := range info {
		fmt.Println("StateMinerSectors: ", *sInfo)
	}

	for _, sInfo := range info2 {
		fmt.Println("StateMinerSectors: ", *sInfo)
	}
}

func (n *Nodes) StateMinerDeadlines(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	deadLine, err := n.VenusNode.StateMinerDeadlines(ctx, addr, head.Key())
	judgeError(err)
	deadLine2, err := n.LotusNode.StateMinerDeadlines(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err)

	fmt.Println("StateMinerDeadlines: ", deadLine, "\n", deadLine2)
}

func (n *Nodes) StateMinerPartitions(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()
	dlIdx := uint64(2)

	partition, err := n.VenusNode.StateMinerPartitions(ctx, addr, dlIdx, head.Key())
	judgeError(err)
	partition2, err2 := n.LotusNode.StateMinerPartitions(ctx, addr, dlIdx, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerPartitions: ", partition, "\n", partition2)
}

func (n *Nodes) StateMinerProvingDeadline(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	info, err := n.VenusNode.StateMinerProvingDeadline(ctx, addr, head.Key())
	judgeError(err)
	info2, err2 := n.LotusNode.StateMinerProvingDeadline(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerProvingDeadline： ", info, "\n", info2)
}

func (n *Nodes) StateGetActor(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	actor, err := n.VenusNode.StateGetActor(ctx, addr, head.Key())
	judgeError(err)
	actor2, err2 := n.LotusNode.StateGetActor(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateGetActor: ", actor)
	fmt.Println("StateGetActor: ", actor2)
}

func (n *Nodes) StateMinerFaults(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	bitField, err := n.VenusNode.StateMinerFaults(ctx, addr, head.Key())
	judgeError(err)
	bitField2, err2 := n.LotusNode.StateMinerFaults(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerFaults: ", bitField)
	fmt.Println("StateMinerFaults: ", bitField2)
}

func (n *Nodes) StateMinerRecoveries(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()

	bitField, err := n.VenusNode.StateMinerRecoveries(ctx, addr, head.Key())
	judgeError(err)
	bitField2, err2 := n.LotusNode.StateMinerRecoveries(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateMinerRecoveries: ", bitField)
	fmt.Println("StateMinerRecoveries: ", bitField2)
}

func (n *Nodes) StateAccountKey(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr, err := address.NewFromString("t3vc6mw75s2sww5uajh62fouxgsomvvvcxwstbq7dfek4g6rqkjebculfcqt5l3w7msapyjviebjia46zfttea")
	judgeError(err)

	addr2, err := n.VenusNode.StateAccountKey(ctx, addr, head.Key())
	judgeError(err)
	fmt.Println("StateAccountKey: ", addr2)

	addr3, err2 := n.LotusNode.StateAccountKey(ctx, addr, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateAccountKey: ", addr3)
}

func (n *Nodes) StateGetReceipt(ctx context.Context) {
	head, err := n.LotusNode.ChainHead(ctx)
	judgeError(err)

	cid := getMessageCID()
	fmt.Println("cid: ", cid)

	receipt, err := n.VenusNode.StateGetReceipt(ctx, cid, block.NewTipSetKey(head.Key().Cids()...))
	//judgeError(err)
	receipt2, err2 := n.LotusNode.StateGetReceipt(ctx, cid, head.Key())
	//judgeError(err2)

	fmt.Printf("StateGetReceipt: %+v, error: %v\n", receipt, err)
	fmt.Printf("StateGetReceipt: %+v, error: %v\n", receipt2, err2)
}

func (n *Nodes) StateCall(ctx context.Context) {
	head := n.GetChainHead(ctx)

	cid := getMessageCID()

	msg, err := n.LotusNode.ChainGetMessage(ctx, cid)
	judgeError(err)
	fmt.Println(msg)

	umsg := &venustypes.UnsignedMessage{
		Version:    int64(msg.Version),
		From:       msg.From,
		To:         msg.To,
		Nonce:      msg.Nonce,
		Value:      msg.Value,
		GasLimit:   venustypes.Unit(msg.GasLimit),
		GasPremium: msg.GasPremium,
		GasFeeCap:  msg.GasFeeCap,
		Method:     msg.Method,
		Params:     []byte{},
	}
	bb, _ := msg.ToStorageBlock()
	fmt.Println(bb.RawData())
	fmt.Println(msg.Cid())

	bbb, _ := umsg.ToStorageBlock()
	fmt.Println(bbb.RawData())
	fmt.Println(umsg.Cid())

	result, err := n.VenusNode.StateCall(ctx, umsg, head)
	judgeError(err)
	result2, err2 := n.LotusNode.StateCall(ctx, msg, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("StateCall: ", result.MsgCid, result.Msg, result.Duration, result.GasCost, result.ExecutionTrace, result.MsgRct)
	fmt.Println("StateCall: ", result2.MsgCid, result2.Msg, result2.Duration, result2.GasCost, result2.ExecutionTrace, result2.MsgRct)
}

func (n *Nodes) StateSearchMsg(ctx context.Context) {
	cid := getMessageCID()
	fmt.Println(cid)

	msgLookup, err := n.VenusNode.StateSearchMsg(ctx, cid)
	judgeError(err)
	msgLookup2, err2 := n.LotusNode.StateSearchMsg(ctx, cid)
	judgeError(err2)

	fmt.Printf("StateSearchMsg: %+v\n", *msgLookup)
	fmt.Printf("StateSearchMsg: %+v\n", *msgLookup2)
}

func (n *Nodes) StateWaitMsg(ctx context.Context) {
	h := abi.ChainEpoch(42)
	cid := getMessageCID()
	fmt.Println(cid)
	lookup, err := n.VenusNode.StateWaitMsg(ctx, cid, h)
	judgeError(err)
	lookup2, err2 := n.LotusNode.StateWaitMsg(ctx, cid, 42)
	judgeError(err2)

	fmt.Println("StateWaitMsg: ", lookup)
	fmt.Println("StateWaitMsg: ", lookup2)
}

// --- winning poster ---

func (n *Nodes) BeaconGetEntry(ctx context.Context) {
	head := n.GetChainHead(ctx)
	height, err := head.Height()
	judgeError(err)

	beaconEntry, err := n.VenusNode.BeaconGetEntry(ctx, height)
	judgeError(err)
	beaconEntry2, err2 := n.LotusNode.BeaconGetEntry(ctx, height)
	judgeError(err2)

	fmt.Println("BeaconGetEntry: ", beaconEntry)
	fmt.Println("BeaconGetEntry: ", beaconEntry2)
}

func (n *Nodes) ChainTipSetWeight(ctx context.Context) {
	head := n.GetChainHead(ctx)

	weight, err := n.VenusNode.ChainTipSetWeight(ctx, head.Key())
	judgeError(err)
	weight2, err2 := n.LotusNode.ChainTipSetWeight(ctx, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Println("BeaconGetEntry: ", weight)
	fmt.Println("BeaconGetEntry: ", weight2)
}

func (n *Nodes) MinerGetBaseInfo(ctx context.Context) {
	head := n.GetChainHead(ctx)
	addr := minerAddr()
	height, err := head.Height()
	judgeError(err)

	weight, err := n.VenusNode.MinerGetBaseInfo(ctx, addr, height, head.Key())
	judgeError(err)
	weight2, err2 := n.LotusNode.MinerGetBaseInfo(ctx, addr, height, types.NewTipSetKey(head.Key().ToSlice()...))
	judgeError(err2)

	fmt.Printf("BeaconGetEntry: %+v\n", weight)
	fmt.Printf("BeaconGetEntry: %+v\n", weight2)
}

func (n *Nodes) WalletSign(ctx context.Context) {
	addr, err := address.NewFromString("t3vc6mw75s2sww5uajh62fouxgsomvvvcxwstbq7dfek4g6rqkjebculfcqt5l3w7msapyjviebjia46zfttea")
	judgeError(err)
	b := []byte("sing")

	sign, err := n.VenusNode.WalletSign(ctx, addr, b, wallet.MsgMeta{})
	judgeError(err)
	sign2, err2 := n.LotusNode.WalletSign(ctx, addr, b)
	judgeError(err2)

	fmt.Println("WalletSign: ", sign)
	fmt.Println("WalletSign: ", sign2)
}

// 结果不一致
func (n *Nodes) SyncSubmitBlock(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()

	var blkTmp mining.BlockTemplate
	err := json.Unmarshal([]byte(blk), &blkTmp)
	blkTmp.Parents = tsk
	judgeError(err)

	var blkTmp2 lotusapi.BlockTemplate
	err = json.Unmarshal([]byte(blk), &blkTmp2)
	judgeError(err)
	blkTmp2.Parents = types.NewTipSetKey(tsk.ToSlice()...)

	msg, err := n.VenusNode.MinerCreateBlock(ctx, &blkTmp)
	judgeError(err)
	msg2, err := n.LotusNode.MinerCreateBlock(ctx, &blkTmp2)
	judgeError(err)

	err = n.VenusNode.SyncSubmitBlock(ctx, msg)
	err2 := n.LotusNode.SyncSubmitBlock(ctx, msg2)

	fmt.Println("SyncSubmitBlock: ", err)
	fmt.Println("SyncSubmitBlock: ", err2)
}

var blk = `
  {
    "Miner": "f01000",
    "Parents": [{"/": "bafy2bzacebbqasuizscj3wezcvmkabunm7zdm4gzoat7qav2fmmnxnfhs3j5a"}],
    "Ticket": {
	  "VRFProof": "ixqtjbi1odyKze46vDM+ZtEak1xPI6AiSjcSK5X57Ug3YrV/fHePb6U33hYTSBoVDrourp/4pyisO3fhi9mWkN9qgZFUXq9pVAXDiiYvkpqNDwIYFugbFyZBN28PkBbT"
    },
    "ElectionProof": {
	  "WinCount": 8,
	  "VRFProof": "t8/Il+YpV2Muk6Y7ueTNoOWHNAU3eII4pO+Q4aVzEtd/vpfAAHHYS6qfqnNvX0afBgNZlEF4+u37jh5wDKnpZ+ySuiJi39/Dj/CrXblK/C95YM127s0P2MumaOzRFKKs"
    },
    "BeaconValues": null,
    "Messages": null,
    "Epoch": 226,
    "Timestamp": 1607501505,
    "WinningPoStProof": null
  }
`

// 结果不一致
func (n *Nodes) MinerCreateBlock(ctx context.Context) {
	head := n.GetChainHead(ctx)
	tsk := head.Key()

	var blkTmp mining.BlockTemplate
	err := json.Unmarshal([]byte(blk), &blkTmp)
	blkTmp.Parents = tsk
	judgeError(err)

	var blkTmp2 lotusapi.BlockTemplate
	err = json.Unmarshal([]byte(blk), &blkTmp2)
	judgeError(err)
	blkTmp2.Parents = types.NewTipSetKey(tsk.ToSlice()...)

	msg, err := n.VenusNode.MinerCreateBlock(ctx, &blkTmp)
	judgeError(err)
	msg2, err := n.LotusNode.MinerCreateBlock(ctx, &blkTmp2)
	judgeError(err)

	fmt.Println("MinerCreateBlock: ", msg)
	fmt.Println("MinerCreateBlock: ", *msg2.Header, msg2.BlsMessages, msg2.SecpkMessages)
	fmt.Println("MinerCreateBlock: ", string(msg2.Header.Ticket.VRFProof), msg2.Header.BlockSig.Type, string(msg2.Header.BlockSig.Data),
		msg2.Header.BLSAggregate.Type, string(msg2.Header.BLSAggregate.Data), msg2.Header.ElectionProof)
}

///// test

//type T interface {
//	String()
//}
//
//type A struct {
//	str string
//}
//
//type B struct {
//	T T
//}
//
//func (a *A) String() {
//	fmt.Println(a.str)
//}
//
//func testJsonUnmarshal() {
//	a := A{str: "aaa"}
//	b := B{&a}
//	bt, err := json.Marshal(b)
//	judgeError(err)
//	var b2 B
//	err = json.Unmarshal(bt, &b2)
//	judgeError(err)
//	fmt.Println("xxxxxx", b2)
//}
