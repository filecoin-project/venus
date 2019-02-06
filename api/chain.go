package api

import (
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
)

// Chain is the interface that defines methods to inspect the Filecoin blockchain.
// Deprecated: use the plumbing API instead
type Chain interface {
	Head() ([]cid.Cid, error)
}
