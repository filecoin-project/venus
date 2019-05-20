package core

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

// Topic is the network pubsub topic identifier on which new messages are announced.
const Topic = "/fil/msgs"

// Outbox validates and marshals messages for sending and maintains the outbound message queue.
type Outbox struct {
	// Signs messages
	signer types.Signer
	// Validates messages before sending them.
	validator consensus.SignedMessageValidator
	// Holds messages sent from this node but not yet mined.
	queue *MessageQueue
	// Invoked to publish a message to the network
	publisher publisher

	chains chainProvider
	actors actorProvider

	// Protects the "next nonce" calculation to avoid collisions.
	nonceLock sync.Mutex
}

type chainProvider interface {
	GetHead() types.SortedCidSet
	GetTipSet(tsKey types.SortedCidSet) (*types.TipSet, error)
}

type actorProvider interface {
	// GetActor returns the actor state defined by the chain up to some tipset
	GetActor(ctx context.Context, tipset types.SortedCidSet, addr address.Address) (*actor.Actor, error)
}

type publisher interface {
	Publish(ctx context.Context, message *types.SignedMessage, height uint64) error
}

var msgSendErrCt = metrics.NewInt64Counter("message_sender_error", "Number of errors encountered while sending a message")

// NewOutbox creates a new outbox
func NewOutbox(signer types.Signer, validator consensus.SignedMessageValidator, queue *MessageQueue,
	publisher publisher, chains chainProvider, actors actorProvider) *Outbox {
	return &Outbox{
		signer:    signer,
		validator: validator,
		queue:     queue,
		publisher: publisher,
		chains:    chains,
		actors:    actors,
	}
}

// Send marshals and sends a message, retaining it in the outbound message queue.
func (ob *Outbox) Send(ctx context.Context, from, to address.Address, value *types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (out cid.Cid, err error) {
	defer func() {
		if err != nil {
			msgSendErrCt.Inc(ctx, 1)
		}
	}()

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid params")
	}

	// Lock to avoid race for message nonce.
	ob.nonceLock.Lock()
	defer ob.nonceLock.Unlock()

	head := ob.chains.GetHead()

	fromActor, err := ob.actors.GetActor(ctx, head, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "no GetActor at address %s", from)
	}

	nonce, err := nextNonce(fromActor, ob.queue, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed calculating nonce for GetActor %s", from)
	}

	rawMsg := types.NewMessage(from, to, nonce, value, method, encodedParams)
	signed, err := types.NewSignedMessage(*rawMsg, ob.signer, gasPrice, gasLimit)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to sign message")
	}

	err = ob.validator.Validate(ctx, signed, fromActor)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid message")
	}

	height, err := tipsetHeight(ob.chains, head)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to get block height")
	}

	// Add to the local message queue/pool at the last possible moment before broadcasting to network.
	if err := ob.queue.Enqueue(signed, height); err != nil {
		return cid.Undef, errors.Wrap(err, "failed to add message to outbound queue")
	}

	err = ob.publisher.Publish(ctx, signed, height)
	if err != nil {
		return cid.Undef, err
	}

	return signed.Cid()
}

// nextNonce returns the next expected nonce value for an account GetActor. This is the larger
// of the GetActor's nonce value, or one greater than the largest nonce from the GetActor found in the message queue.
func nextNonce(act *actor.Actor, queue *MessageQueue, address address.Address) (uint64, error) {
	actorNonce, err := actor.NextNonce(act)
	if err != nil {
		return 0, err
	}

	poolNonce, found := queue.LargestNonce(address)
	if found && poolNonce >= actorNonce {
		return poolNonce + 1, nil
	}
	return actorNonce, nil
}

func tipsetHeight(provider chainProvider, key types.SortedCidSet) (uint64, error) {
	head, err := provider.GetTipSet(key)
	if err != nil {
		return 0, err
	}
	return head.Height()
}
