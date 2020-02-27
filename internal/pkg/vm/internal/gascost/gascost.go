package gascost

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
)

// TODO: assign all of these.
// TODO: this needs to be upgradeable, it can change at a given epoch

var (
	gasAmountPlaceholder                = gas.NewGas(1)
	gasAmountPlaceholderUpdateStateTree = gasAmountPlaceholder
)

var (
	///////////////////////////////////////////////////////////////////////////
	// System operations
	///////////////////////////////////////////////////////////////////////////

	// Gas cost charged to the originator of an on-chain message (regardless of
	// whether it succeeds or fails in application) is given by:
	//   OnChainMessageBase + len(serialized message)*OnChainMessagePerByte
	// Together, these account for the cost of message propagation and validation,
	// up to but excluding any actual processing by the VM.
	// This is the cost a block producer burns when including an invalid message.
	onChainMessageBase    = gas.Zero
	onChainMessagePerByte = gas.NewGas(2)

	// Gas cost charged to the originator of a non-nil return value produced
	// by an on-chain message is given by:
	//   len(return value)*OnChainReturnValuePerByte
	onChainReturnValuePerByte = gas.Zero

	// Gas cost for any message send execution(including the top-level one
	// initiated by an on-chain message).
	// This accounts for the cost of loading sender and receiver actors and
	// (for top-level messages) incrementing the sender's sequence number.
	// Load and store of actor sub-state is charged separately.
	sendBase = gas.Zero

	// Gas cost charged, in addition to SendBase, if a message send
	// is accompanied by any nonzero currency amount.
	// Accounts for writing receiver's new balance (the sender's state is
	// already accounted for).
	sendTransferFunds = gas.NewGas(10)

	// Gas cost charged, in addition to SendBase, if a message invokes
	// a method on the receiver.
	// Accounts for the cost of loading receiver code and method dispatch.
	sendInvokeMethod = gas.NewGas(5)

	// Gas cost (Base + len*PerByte) for any Get operation to the IPLD store
	// in the runtime VM context.
	ipldGetBase    = gas.NewGas(10)
	ipldGetPerByte = gas.NewGas(1)

	// Gas cost (Base + len*PerByte) for any Put operation to the IPLD store
	// in the runtime VM context.
	//
	// Note: these costs should be significantly higher than the costs for Get
	// operations, since they reflect not only serialization/deserialization
	// but also persistent storage of chain data.
	ipldPutBase    = gas.NewGas(20)
	ipldPutPerByte = gas.NewGas(2)

	// Gas cost for updating an actor's substate (i.e., UpdateRelease).
	// This is in addition to a per-byte fee for the state as for IPLD Get/Put.
	UpdateActorSubstate = gasAmountPlaceholderUpdateStateTree

	// Gas cost for creating a new actor (via InitActor's Exec method).
	// Actor sub-state is charged separately.
	ExecNewActor = gasAmountPlaceholder

	// Gas cost for deleting an actor.
	DeleteActor = gasAmountPlaceholder

	///////////////////////////////////////////////////////////////////////////
	// Pure functions (VM ABI)
	///////////////////////////////////////////////////////////////////////////

	// Gas cost charged per public-key cryptography operation (e.g., signature
	// verification).
	PublicKeyCryptoOp = gasAmountPlaceholder
)

// OnChainMessage returns the gas used for storing a message of a given size in the chain.
func OnChainMessage(msgSize int) gas.Unit {
	return onChainMessageBase + onChainMessagePerByte*gas.Unit(msgSize)
}

// OnChainReturnValue returns the gas used for storing the response of a message in the chain.
func OnChainReturnValue(receipt *message.Receipt) gas.Unit {
	return gas.Unit(len(receipt.ReturnValue)) * onChainReturnValuePerByte
}

type methodInvocationArgs interface{}

// OnMethodInvocation returns the gas used when invoking a method.
func OnMethodInvocation(value abi.TokenAmount, methodNum abi.MethodNum) gas.Unit {
	ret := sendBase
	if value != abi.NewTokenAmount(0) {
		ret += sendTransferFunds
	}
	if methodNum != builtin.MethodSend {
		ret += sendInvokeMethod
	}
	return ret
}

// OnIpldGet returns the gas used for storing an object
func OnIpldGet(dataSize int) gas.Unit {
	return ipldGetBase + gas.Unit(dataSize)*ipldGetPerByte
}

// OnIpldPut returns the gas used for storing an object
func OnIpldPut(dataSize int) gas.Unit {
	return ipldPutBase + gas.Unit(dataSize)*ipldPutPerByte
}
