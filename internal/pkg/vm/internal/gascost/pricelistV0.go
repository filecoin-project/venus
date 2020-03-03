package gascost

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

type pricelistV0 struct {
	///////////////////////////////////////////////////////////////////////////
	// System operations
	///////////////////////////////////////////////////////////////////////////

	// Gas cost charged to the originator of an on-chain message (regardless of
	// whether it succeeds or fails in application) is given by:
	//   OnChainMessageBase + len(serialized message)*OnChainMessagePerByte
	// Together, these account for the cost of message propagation and validation,
	// up to but excluding any actual processing by the VM.
	// This is the cost a block producer burns when including an invalid message.
	onChainMessageBase    gas.Unit
	onChainMessagePerByte gas.Unit

	// Gas cost charged to the originator of a non-nil return value produced
	// by an on-chain message is given by:
	//   len(return value)*OnChainReturnValuePerByte
	onChainReturnValuePerByte gas.Unit

	// Gas cost for any message send execution(including the top-level one
	// initiated by an on-chain message).
	// This accounts for the cost of loading sender and receiver actors and
	// (for top-level messages) incrementing the sender's sequence number.
	// Load and store of actor sub-state is charged separately.
	sendBase gas.Unit

	// Gas cost charged, in addition to SendBase, if a message send
	// is accompanied by any nonzero currency amount.
	// Accounts for writing receiver's new balance (the sender's state is
	// already accounted for).
	sendTransferFunds gas.Unit

	// Gas cost charged, in addition to SendBase, if a message invokes
	// a method on the receiver.
	// Accounts for the cost of loading receiver code and method dispatch.
	sendInvokeMethod gas.Unit

	// Gas cost (Base + len*PerByte) for any Get operation to the IPLD store
	// in the runtime VM context.
	ipldGetBase    gas.Unit
	ipldGetPerByte gas.Unit

	// Gas cost (Base + len*PerByte) for any Put operation to the IPLD store
	// in the runtime VM context.
	//
	// Note: these costs should be significantly higher than the costs for Get
	// operations, since they reflect not only serialization/deserialization
	// but also persistent storage of chain data.
	ipldPutBase    gas.Unit
	ipldPutPerByte gas.Unit

	// Gas cost for creating a new actor (via InitActor's Exec method).
	//
	// Note: this costs assume that the extra will be partially or totally refunded while
	// the base is covering for the put.
	createActorBase  gas.Unit
	createActorExtra gas.Unit

	// Gas cost for deleting an actor.
	//
	// Note: this partially refunds the create cost to incentivise the deletion of the actors.
	deleteActor gas.Unit

	verifySignature map[crypto.SigType]func(gas.Unit) gas.Unit

	hashingBase    gas.Unit
	hashingPerByte gas.Unit

	computeUnsealedSectorCidBase gas.Unit
	verifySealBase               gas.Unit
	verifyPostBase               gas.Unit
	verifyConsensusFault         gas.Unit
}

var _ Pricelist = (*pricelistV0)(nil)

// OnChainMessage returns the gas used for storing a message of a given size in the chain.
func (pl *pricelistV0) OnChainMessage(msgSize int) gas.Unit {
	return pl.onChainMessageBase + pl.onChainMessagePerByte*gas.Unit(msgSize)
}

// OnChainReturnValue returns the gas used for storing the response of a message in the chain.
func (pl *pricelistV0) OnChainReturnValue(receipt *message.Receipt) gas.Unit {
	return gas.Unit(len(receipt.ReturnValue)) * pl.onChainReturnValuePerByte
}

// OnMethodInvocation returns the gas used when invoking a method.
func (pl *pricelistV0) OnMethodInvocation(value abi.TokenAmount, methodNum abi.MethodNum) gas.Unit {
	ret := pl.sendBase
	if value != abi.NewTokenAmount(0) {
		ret += pl.sendTransferFunds
	}
	if methodNum != builtin.MethodSend {
		ret += pl.sendInvokeMethod
	}
	return ret
}

// OnIpldGet returns the gas used for storing an object
func (pl *pricelistV0) OnIpldGet(dataSize int) gas.Unit {
	return pl.ipldGetBase + gas.Unit(dataSize)*pl.ipldGetPerByte
}

// OnIpldPut returns the gas used for storing an object
func (pl *pricelistV0) OnIpldPut(dataSize int) gas.Unit {
	return pl.ipldPutBase + gas.Unit(dataSize)*pl.ipldPutPerByte
}

// OnCreateActor returns the gas used for creating an actor
func (pl *pricelistV0) OnCreateActor() gas.Unit {
	return pl.createActorBase + pl.createActorExtra
}

// OnDeleteActor returns the gas used for deleting an actor
func (pl *pricelistV0) OnDeleteActor() gas.Unit {
	return pl.deleteActor
}

// OnVerifySignature
func (pl *pricelistV0) OnVerifySignature(sigType crypto.SigType, planTextSize int) gas.Unit {
	costFn, ok := pl.verifySignature[sigType]
	if !ok {
		runtime.Abortf(exitcode.SysErrInternal, "Cost function for signature type %d not supported", sigType)
	}
	return costFn(gas.Unit(planTextSize))
}

// OnHashing
func (pl *pricelistV0) OnHashing(dataSize int) gas.Unit {
	return pl.hashingBase + gas.Unit(dataSize)*pl.hashingPerByte
}

// OnComputeUnsealedSectorCid
func (pl *pricelistV0) OnComputeUnsealedSectorCid(proofType abi.RegisteredProof, pieces *[]abi.PieceInfo) gas.Unit {
	// TODO: this needs more cost tunning, check with @lotus
	return pl.computeUnsealedSectorCidBase
}

// OnVerifySeal
func (pl *pricelistV0) OnVerifySeal(info abi.SealVerifyInfo) gas.Unit {
	// TODO: this needs more cost tunning, check with @lotus
	return pl.verifySealBase
}

// OnVerifyPost
func (pl *pricelistV0) OnVerifyPost(info abi.PoStVerifyInfo) gas.Unit {
	// TODO: this needs more cost tunning, check with @lotus
	return pl.verifyPostBase
}

// OnVerifyConsensusFault
func (pl *pricelistV0) OnVerifyConsensusFault() gas.Unit {
	return pl.verifyConsensusFault
}
