package gen

import (
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

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
