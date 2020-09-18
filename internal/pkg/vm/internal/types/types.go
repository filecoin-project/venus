package types

import (
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/runtime"
)

// SendReturn is the return values for the Send method.
type SendReturn struct {
	Return runtime.CBORBytes
	Code   exitcode.ExitCode
}
