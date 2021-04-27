package gas

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
)

//GasCharge amount of gas consumed at one time
type GasCharge struct { //nolint
	Name  string
	Extra interface{}

	ComputeGas int64
	StorageGas int64

	VirtualCompute int64
	VirtualStorage int64
}

//Total return all gas in one time
func (g GasCharge) Total() int64 {
	return g.ComputeGas + g.StorageGas
}

func (g GasCharge) WithVirtual(compute, storage int64) GasCharge {
	out := g
	out.VirtualCompute = compute
	out.VirtualStorage = storage
	return out
}

func (g GasCharge) WithExtra(extra interface{}) GasCharge {
	out := g
	out.Extra = extra
	return out
}

func NewGasCharge(name string, computeGas int64, storageGas int64) GasCharge {
	return GasCharge{
		Name:       name,
		ComputeGas: computeGas,
		StorageGas: storageGas,
	}
}

// Pricelist provides prices for operations in the VM.
//
// Note: this interface should be APPEND ONLY since last chain checkpoint
type Pricelist interface {
	// OnChainMessage returns the gas used for storing a message of a given size in the chain.
	OnChainMessage(msgSize int) GasCharge
	// OnChainReturnValue returns the gas used for storing the response of a message in the chain.
	OnChainReturnValue(dataSize int) GasCharge

	// OnMethodInvocation returns the gas used when invoking a method.
	OnMethodInvocation(value abi.TokenAmount, methodNum abi.MethodNum) GasCharge

	// OnIpldGet returns the gas used for storing an object
	OnIpldGet() GasCharge
	// OnIpldPut returns the gas used for storing an object
	OnIpldPut(dataSize int) GasCharge

	// OnCreateActor returns the gas used for creating an actor
	OnCreateActor() GasCharge
	// OnDeleteActor returns the gas used for deleting an actor
	OnDeleteActor() GasCharge

	OnVerifySignature(sigType crypto.SigType, planTextSize int) (GasCharge, error)
	OnHashing(dataSize int) GasCharge
	OnComputeUnsealedSectorCid(proofType abi.RegisteredSealProof, pieces []abi.PieceInfo) GasCharge
	OnVerifySeal(info proof.SealVerifyInfo) GasCharge
	OnVerifyPost(info proof.WindowPoStVerifyInfo) GasCharge
	OnVerifyConsensusFault() GasCharge
}

//PricesSchedule schedule gas prices for different network version
type PricesSchedule struct {
	prices map[abi.ChainEpoch]Pricelist
}

//NewPricesSchedule new gasprice schedule from netowrk parameters
func NewPricesSchedule(forkParams *config.ForkUpgradeConfig) *PricesSchedule {
	var prices = map[abi.ChainEpoch]Pricelist{
		abi.ChainEpoch(0): &pricelistV0{
			computeGasMulti: 1,
			storageGasMulti: 1000,

			onChainMessageComputeBase:    38863,
			onChainMessageStorageBase:    36,
			onChainMessageStoragePerByte: 1,

			onChainReturnValuePerByte: 1,

			sendBase:                29233,
			sendTransferFunds:       27500,
			sendTransferOnlyPremium: 159672,
			sendInvokeMethod:        -5377,

			ipldGetBase:    75242,
			ipldPutBase:    84070,
			ipldPutPerByte: 1,

			createActorCompute: 1108454,
			createActorStorage: 36 + 40,
			deleteActor:        -(36 + 40), // -createActorStorage

			verifySignature: map[crypto.SigType]int64{
				crypto.SigTypeBLS:       16598605,
				crypto.SigTypeSecp256k1: 1637292,
			},

			hashingBase:                  31355,
			computeUnsealedSectorCidBase: 98647,
			verifySealBase:               2000, // TODO gas , it VerifySeal syscall is not used
			verifyPostLookup: map[abi.RegisteredPoStProof]scalingCost{
				abi.RegisteredPoStProof_StackedDrgWindow512MiBV1: {
					flat:  123861062,
					scale: 9226981,
				},
				abi.RegisteredPoStProof_StackedDrgWindow32GiBV1: {
					flat:  748593537,
					scale: 85639,
				},
				abi.RegisteredPoStProof_StackedDrgWindow64GiBV1: {
					flat:  748593537,
					scale: 85639,
				},
			},
			verifyPostDiscount:   true,
			verifyConsensusFault: 495422,
		},
		forkParams.UpgradeCalicoHeight: &pricelistV0{
			computeGasMulti: 1,
			storageGasMulti: 1300,

			onChainMessageComputeBase:    38863,
			onChainMessageStorageBase:    36,
			onChainMessageStoragePerByte: 1,

			onChainReturnValuePerByte: 1,

			sendBase:                29233,
			sendTransferFunds:       27500,
			sendTransferOnlyPremium: 159672,
			sendInvokeMethod:        -5377,

			ipldGetBase:    114617,
			ipldPutBase:    353640,
			ipldPutPerByte: 1,

			createActorCompute: 1108454,
			createActorStorage: 36 + 40,
			deleteActor:        -(36 + 40), // -createActorStorage

			verifySignature: map[crypto.SigType]int64{
				crypto.SigTypeBLS:       16598605,
				crypto.SigTypeSecp256k1: 1637292,
			},

			hashingBase:                  31355,
			computeUnsealedSectorCidBase: 98647,
			verifySealBase:               2000, // TODO gas , it VerifySeal syscall is not used
			verifyPostLookup: map[abi.RegisteredPoStProof]scalingCost{
				abi.RegisteredPoStProof_StackedDrgWindow512MiBV1: {
					flat:  117680921,
					scale: 43780,
				},
				abi.RegisteredPoStProof_StackedDrgWindow32GiBV1: {
					flat:  117680921,
					scale: 43780,
				},
				abi.RegisteredPoStProof_StackedDrgWindow64GiBV1: {
					flat:  117680921,
					scale: 43780,
				},
			},
			verifyPostDiscount:   false,
			verifyConsensusFault: 495422,
		},
	}
	return &PricesSchedule{prices: prices}
}

// PricelistByEpoch finds the latest prices for the given epoch
func (schedule *PricesSchedule) PricelistByEpoch(epoch abi.ChainEpoch) Pricelist {
	// since we are storing the prices as map or epoch to price
	// we need to get the price with the highest epoch that is lower or equal to the `epoch` arg
	bestEpoch := abi.ChainEpoch(0)
	bestPrice := schedule.prices[bestEpoch]
	for e, pl := range schedule.prices {
		// if `e` happened after `bestEpoch` and `e` is earlier or equal to the target `epoch`
		if e > bestEpoch && e <= epoch {
			bestEpoch = e
			bestPrice = pl
		}
	}
	if bestPrice == nil {
		panic(fmt.Sprintf("bad setup: no gas prices available for epoch %d", epoch))
	}
	return bestPrice
}
