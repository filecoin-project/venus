package consensus

import (
	"context"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// monitorPlumbing is an interface for the functionality StorageFaultMonitor needs
type monitorPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

type slashingMsgOutbox interface {
	Send(ctx context.Context,
		from, to address.Address,
		value types.AttoFIL,
		gasPrice types.AttoFIL,
		gasLimit types.GasUnits,
		bcast bool,
		method string,
		params ...interface{}) (out cid.Cid, err error)
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	log       logging.EventLogger
	msgSender address.Address   // what signs the slashing message and receives slashing reward
	outbox    slashingMsgOutbox // what sends the slashing message
	plumbing  monitorPlumbing   // what does the message query
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain and function
// to get miner power
func NewStorageFaultMonitor(plumbing monitorPlumbing, outbox slashingMsgOutbox, msgSender address.Address) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		plumbing:  plumbing,
		log:       logging.Logger("StorFltMon"),
		outbox:    outbox,
		msgSender: msgSender,
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and looks for missing, expected submitPoSts
// Miners without power and those that posted proofs to newTs are skipped
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, currentHeight *types.BlockHeight) error {
	res, err := sfm.plumbing.MessageQuery(ctx, sfm.msgSender, address.StorageMarketAddress, "getLateMiners")
	if err != nil {
		return errors.FaultErrorWrap(err, "getLateMiners message failed")
	}

	lateMiners, err := abi.Deserialize(res[0], abi.MinerPoStStates)
	if err != nil {
		return errors.FaultErrorWrap(err, "deserializing MinerPoStStates failed")
	}

	lms, ok := lateMiners.Val.(*map[string]uint64)
	if !ok {
		return errors.FaultErrorWrapf(err, "expected *map[string]uint64 but got %T", lms)
	}
	sfm.log.Debugf("there are %d late miners", len(*lms))
	// Slash late miners.
	for minerStr, state := range *lms {
		minerAddr, err := address.NewFromString(minerStr)
		if err != nil {
			return errors.FaultErrorWrap(err, "could not create minerAddr string")
		}

		// send slash message, don't broadcast it, and don't wait for message to appear on chain.
		sfm.log.Debugf("Slashing %s with state %d", minerStr, state)

		_, err = sfm.outbox.Send(ctx, sfm.msgSender, minerAddr, types.ZeroAttoFIL, types.NewAttoFILFromFIL(1),
			types.NewGasUnits(300), false, "slashStorageFault")
		if err != nil {
			return errors.FaultErrorWrap(err, "slashStorageFault message failed")
		}
	}
	return nil
}
