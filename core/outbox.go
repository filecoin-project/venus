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

// Outbox validates and marshals messages for sending and maintains the outbound message queue.
type Outbox struct {
	// Signs messages
	signer types.Signer
	// Validates messages before sending them.
	validator consensus.SignedMessageValidator
	// Holds messages sent from this node but not yet mined.
	queue *MessageQueue
	// Publishes a signed message to the network.
	publisher publisher
	// Maintains message queue in response to new tipsets.
	policy QueuePolicy

	chains outboxChainProvider
	actors actorProvider

	// Protects the "next nonce" calculation to avoid collisions.
	nonceLock sync.Mutex
}

type outboxChainProvider interface {
	GetHead() types.TipSetKey
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
}

type actorProvider interface {
	// GetActorAt returns the actor state defined by the chain up to some tipset
	GetActorAt(ctx context.Context, tipset types.TipSetKey, addr address.Address) (*actor.Actor, error)
}

type publisher interface {
	Publish(ctx context.Context, message *types.SignedMessage, height uint64, bcast bool) error
}

var msgSendErrCt = metrics.NewInt64Counter("message_sender_error", "Number of errors encountered while sending a message")

// NewOutbox creates a new outbox
func NewOutbox(signer types.Signer, validator consensus.SignedMessageValidator, queue *MessageQueue,
	publisher publisher, policy QueuePolicy, chains outboxChainProvider, actors actorProvider) *Outbox {
	return &Outbox{
		signer:    signer,
		validator: validator,
		queue:     queue,
		publisher: publisher,
		policy:    policy,
		chains:    chains,
		actors:    actors,
	}
}

// Queue returns the outbox's outbound message queue.
func (ob *Outbox) Queue() *MessageQueue {
	return ob.queue
}

// Send marshals and sends a message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) Send(ctx context.Context, from, to address.Address, value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method string, params ...interface{}) (out cid.Cid, err error) {
	defer func() {
		if err != nil {
			msgSendErrCt.Inc(ctx, 1)
		}
	}()

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid params")
	}

	// Lock to avoid a race inspecting the actor state and message queue to calculate next nonce.
	ob.nonceLock.Lock()
	defer ob.nonceLock.Unlock()

	head := ob.chains.GetHead()

	fromActor, err := ob.actors.GetActorAt(ctx, head, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "no actor at address %s", from)
	}

	nonce, err := nextNonce(fromActor, ob.queue, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed calculating nonce for actor at %s", from)
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

	// Add to the local message queue at the last possible moment before
	// calling Publish.
	if err := ob.queue.Enqueue(ctx, signed, height); err != nil {
		return cid.Undef, errors.Wrap(err, "failed to add message to outbound queue")
	}
	err = ob.publisher.Publish(ctx, signed, height, bcast)
	if err != nil {
		return cid.Undef, err
	}

	return signed.Cid()
}

// HandleNewHead maintains the message queue in response to a new head tipset.
func (ob *Outbox) HandleNewHead(ctx context.Context, oldHead, newHead types.TipSet) error {
	return ob.policy.HandleNewHead(ctx, ob.queue, oldHead, newHead)
}

// nextNonce returns the next expected nonce value for an account actor. This is the larger
// of the actor's nonce value, or one greater than the largest nonce from the actor found in the message queue.
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

func tipsetHeight(provider outboxChainProvider, key types.TipSetKey) (uint64, error) {
	head, err := provider.GetTipSet(key)
	if err != nil {
		return 0, err
	}
	return head.Height()
}
