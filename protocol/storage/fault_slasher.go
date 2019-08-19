package storage

import (
	"context"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// DefaultFaultSlasherGasPrice is the default gas price to be used when sending messages
var DefaultFaultSlasherGasPrice = types.NewAttoFILFromFIL(1)

// DefaultFaultSlasherGasLimit is the default gas limit to be used when sending messages
var DefaultFaultSlasherGasLimit = types.NewGasUnits(300)

// monitorPlumbing is an interface for the functionality FaultSlasher needs
type monitorPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// slashingMsgOutbox is the interface for the functionality of Outbox FaultSlasher needs
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

// FaultSlasher checks for unreported storage faults by miners, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type FaultSlasher struct {
	gasPrice  types.AttoFIL  // gas price to use when sending messages
	gasLimit  types.GasUnits // gas limit to use when sending messages
	log       logging.EventLogger
	msgSender address.Address   // what signs the slashing message and receives slashing reward
	outbox    slashingMsgOutbox // what sends the slashing message
	plumbing  monitorPlumbing   // what does the message query
}

// NewFaultSlasher creates a new FaultSlasher with the provided plumbing, outbox and message sender.
// Message sender must be an account actor address.
func NewFaultSlasher(plumbing monitorPlumbing, outbox slashingMsgOutbox, msgSender address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits) *FaultSlasher {
	return &FaultSlasher{
		plumbing:  plumbing,
		log:       logging.Logger("StorFltMon"),
		outbox:    outbox,
		msgSender: msgSender,
		gasPrice:  gasPrice,
		gasLimit:  gasLimit,
	}
}

// OnNewHeaviestTipSet is a wrapper for calling the Slash function, after getting the TipSet height.
func (sfm *FaultSlasher) OnNewHeaviestTipSet(ctx context.Context, ts types.TipSet) error {
	height, err := ts.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get tipset height")
	}

	return sfm.Slash(ctx, types.NewBlockHeight(height))
}

// Slash checks for miners with unreported faults, then slashes them
// Slashing messages are not broadcast to the network, but included in the next block mined by the slashing
// node.
func (sfm *FaultSlasher) Slash(ctx context.Context, currentHeight *types.BlockHeight) error {
	res, err := sfm.plumbing.MessageQuery(ctx, sfm.msgSender, address.StorageMarketAddress, "getLateMiners")
	if err != nil {
		return errors.Wrap(err, "getLateMiners message failed")
	}

	lateMiners, err := abi.Deserialize(res[0], abi.MinerPoStStates)
	if err != nil {
		return errors.Wrap(err, "deserializing MinerPoStStates failed")
	}

	lms, ok := lateMiners.Val.(*map[string]uint64)
	if !ok {
		return errors.Wrapf(err, "expected *map[string]uint64 but got %T", lms)
	}
	sfm.log.Debugf("there are %d late miners\n", len(*lms))

	// Slash late miners.
	for minerStr, state := range *lms {
		minerAddr, err := address.NewFromString(minerStr)
		if err != nil {
			return errors.Wrap(err, "could not create minerAddr string")
		}

		// add slash message to message pull w/o broadcasting
		sfm.log.Debugf("Slashing %s with state %d\n", minerStr, state)

		_, err = sfm.outbox.Send(ctx, sfm.msgSender, minerAddr, types.ZeroAttoFIL, sfm.gasPrice,
			sfm.gasLimit, false, "slashStorageFault")
		if err != nil {
			return errors.Wrap(err, "slashStorageFault message failed")
		}
	}
	return nil
}
