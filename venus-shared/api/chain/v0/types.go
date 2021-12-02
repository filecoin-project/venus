package v0

import (
	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/ipfs/go-cid"
)

type HeadChange struct {
	Type string
	Val  *chain.TipSet
}

// BlsMessages[x].cid = Cids[x]
// SecpkMessages[y].cid = Cids[BlsMessages.length + y]
type BlockMessages struct {
	BlsMessages   []*chain.Message
	SecpkMessages []*chain.SignedMessage

	Cids []cid.Cid
}

type Message struct {
	Cid     cid.Cid
	Message *chain.Message
}

type ObjStat struct {
	Size  uint64
	Links uint64
}

type IpldObject struct {
	Cid cid.Cid
	Obj interface{}
}
