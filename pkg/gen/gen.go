package gen

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/venus/pkg/gen/genesis"
)

//CarWalkFunc get each child node under the node (nd)
func CarWalkFunc(nd format.Node) (out []*format.Link, err error) {
	for _, link := range nd.Links() {
		pref := link.Cid.Prefix()
		if pref.Codec == cid.FilCommitmentSealed || pref.Codec == cid.FilCommitmentUnsealed {
			continue
		}
		out = append(out, link)
	}

	return out, nil
}

var rootkeyMultisig = genesis.MultisigMeta{
	Signers:         []address.Address{remAccTestKey},
	Threshold:       1,
	VestingDuration: 0,
	VestingStart:    0,
}

var DefaultVerifregRootkeyActor = genesis.Actor{
	Type:    genesis.TMultisig,
	Balance: big.NewInt(0),
	Meta:    rootkeyMultisig.ActorMeta(),
}

var remAccTestKey, _ = address.NewFromString("t1ceb34gnsc6qk5dt6n7xg6ycwzasjhbxm3iylkiy")
var remAccMeta = genesis.MultisigMeta{
	Signers:   []address.Address{remAccTestKey},
	Threshold: 1,
}

var DefaultRemainderAccountActor = genesis.Actor{
	Type:    genesis.TMultisig,
	Balance: big.NewInt(0),
	Meta:    remAccMeta.ActorMeta(),
}
