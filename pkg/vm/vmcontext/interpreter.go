package vmcontext

import (
	"github.com/filecoin-project/venus/pkg/state/tree"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent State.
type VMInterpreter interface {
	Interface

	StateTree() tree.Tree
}
