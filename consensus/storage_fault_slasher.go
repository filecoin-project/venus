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

// monitorPlumbing is an interface for the functionality StorageFaultSlasher needs
type monitorPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// slashingMsgOutbox is the interface for the functionality of Outbox StorageFaultSlasher needs
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

// StorageFaultSlasher checks for unreported storage faults by miners, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultSlasher struct {
	log       logging.EventLogger
	msgSender address.Address   // what signs the slashing message and receives slashing reward
	outbox    slashingMsgOutbox // what sends the slashing message
	plumbing  monitorPlumbing   // what does the message query
}

// NewStorageFaultSlasher creates a new StorageFaultSlasher with the provided plumbing, outbox and message sender.
// Message sender must be an account actor address.
func NewStorageFaultSlasher(plumbing monitorPlumbing, outbox slashingMsgOutbox, msgSender address.Address) *StorageFaultSlasher {
	return &StorageFaultSlasher{
		plumbing:  plumbing,
		log:       logging.Logger("StorFltMon"),
		outbox:    outbox,
		msgSender: msgSender,
	}
}

// Slash checks for miners with unreported faults, then slashes them
// Slashing messages are not broadcast to the network, but included in the next block mined by the slashing
// node.
func (sfm *StorageFaultSlasher) Slash(ctx context.Context, currentHeight *types.BlockHeight) error {
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
	sfm.log.Debugf("there are %d late miners\n", len(*lms))

	// Slash late miners.
	for minerStr, state := range *lms {
		minerAddr, err := address.NewFromString(minerStr)
		if err != nil {
			return errors.FaultErrorWrap(err, "could not create minerAddr string")
		}

		// add slash message to message pull w/o broadcasting
		sfm.log.Debugf("Slashing %s with state %d\n", minerStr, state)

		_, err = sfm.outbox.Send(ctx, sfm.msgSender, minerAddr, types.ZeroAttoFIL, types.NewAttoFILFromFIL(1),
			types.NewGasUnits(300), false, "slashStorageFault")
		if err != nil {
			return errors.FaultErrorWrap(err, "slashStorageFault message failed")
		}
	}
	return nil
}
