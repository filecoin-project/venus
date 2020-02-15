package gascost

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
)

// OnChainMessage returns the FIL cost of storing a message of a given size in the chain.
func OnChainMessage(size uint32) gas.Unit {
	// TODO: set true cost when Spec has them (issue: ????)
	return gas.NewGas(7)
}

// OnChainReturnValue returns the FIL cost of storing the response of a message in the chain.
func OnChainReturnValue(receipt *message.Receipt) gas.Unit {
	// TODO: set true cost when Spec has them (issue: ????)
	return gas.NewGas(3)
}

type methodInvocationArgs interface{}

// OnMethodInvocation returns the FIL cost of invoking a method.
func OnMethodInvocation(args methodInvocationArgs) gas.Unit {
	// TODO: set true cost when Spec has them (issue: ????)
	return gas.NewGas(1)
}
