package storage

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/spooky/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/spooky/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// DefaultFaultSlasherGasPrice is the default gas price to be used when sending messages
var DefaultFaultSlasherGasPrice = types.NewAttoFILFromFIL(1)

// DefaultFaultSlasherGasLimit is the default gas limit to be used when sending messages
var DefaultFaultSlasherGasLimit = types.NewGasUnits(300)

// monitorPlumbing is an interface for the functionality FaultSlasher needs
type monitorPlumbing interface {
	ChainHeadKey() block.TipSetKey
	ConfigGet(string) (interface{}, error)
	MessageQuery(context.Context, address.Address, address.Address, string, block.TipSetKey, ...interface{}) ([][]byte, error)
	MinerGetWorkerAddress(context.Context, address.Address, block.TipSetKey) (address.Address, error)
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
	gasPrice types.AttoFIL  // gas price to use when sending messages
	gasLimit types.GasUnits // gas limit to use when sending messages
	log      logging.EventLogger
	outbox   slashingMsgOutbox // what sends the slashing message
	plumbing monitorPlumbing   // what does the message query

	// Records addresses of miners the slasher has already attempted to penalise.
	slashed map[string]struct{}
}

// NewFaultSlasher creates a new FaultSlasher with the provided plumbing and outbox
// Message sender must be an account actor address.
func NewFaultSlasher(plumbing monitorPlumbing, outbox slashingMsgOutbox, gasPrice types.AttoFIL, gasLimit types.GasUnits) *FaultSlasher {
	return &FaultSlasher{
		plumbing: plumbing,
		log:      logging.Logger("StorFltMon"),
		outbox:   outbox,
		gasPrice: gasPrice,
		gasLimit: gasLimit,

		slashed: make(map[string]struct{}),
	}
}

// OnNewHeaviestTipSet is a wrapper for calling the Slash function, after getting the TipSet height.
func (sfm *FaultSlasher) OnNewHeaviestTipSet(ctx context.Context, ts block.TipSet) error {
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

	myMinerActorAddr, err := sfm.plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return err
	}

	myWorkerAddr, err := sfm.plumbing.MinerGetWorkerAddress(ctx, myMinerActorAddr.(address.Address), sfm.plumbing.ChainHeadKey())
	if err != nil {
		return errors.Wrap(err, "could not get worker address")
	}

	res, err := sfm.plumbing.MessageQuery(ctx, myWorkerAddr, address.StorageMarketAddress, "getLateMiners", sfm.plumbing.ChainHeadKey())
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
	sfm.log.Debugf("there are %d late miners", len(*lms))

	// Slash late miners.
	for lateMinerActor, state := range *lms {
		if _, ok := sfm.slashed[lateMinerActor]; ok {
			// Skip slashed miner.
			// This logic is not perfect. A miner that appears to be late could redeem itself
			// (e.g. due to a chain re-org) such that a submitted slashing message fails, but
			// this slasher will remember that it has attempted to slash that miner, and won't
			// re-attempt to do so in the future.
			// The `slashed` cache key should include the proving period for which the attempt was
			// made so that if a miner does enter another period, it becomes slashable again.
			// Getting that information requires painful plumbing of data through actor methods
			// and the ABI, until we have better direct state access ability for non-actor code.
			// Alternatives are discussed in #3358
			continue
		}

		lateMinerActorAddr, err := address.NewFromString(lateMinerActor)
		if err != nil {
			return errors.Wrap(err, "could not create minerAddr string")
		}

		if lateMinerActorAddr == myMinerActorAddr {
			sfm.log.Warnf("skip slashing self %s", lateMinerActorAddr)
			continue
		}

		// add slash message to message pull w/o broadcasting
		sfm.log.Debugf("Slashing %s with state %d", lateMinerActorAddr, state)

		_, err = sfm.outbox.Send(ctx, myWorkerAddr, lateMinerActorAddr, types.ZeroAttoFIL, sfm.gasPrice,
			sfm.gasLimit, false, "slashStorageFault")
		if err != nil {
			return errors.Wrap(err, "slashStorageFault message failed")
		}
		sfm.slashed[lateMinerActor] = struct{}{}
	}
	return nil
}
