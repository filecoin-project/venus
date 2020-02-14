package types

import (
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31

// BootstrapMinerActorCodeCid is the cid of the above object
// Dragons: this needs to be deleted once we bring the new actors in
var BootstrapMinerActorCodeCid cid.Cid

func init() {
	builder := cid.V1Builder{Codec: cid.Raw, MhType: mh.IDENTITY}
	makeBuiltin := func(s string) cid.Cid {
		c, err := builder.Sum([]byte(s))
		if err != nil {
			panic(err)
		}
		return c
	}

	BootstrapMinerActorCodeCid = makeBuiltin("fil/1/bootstrap")
}
