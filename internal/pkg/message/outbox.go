package message

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Outbox validates and marshals messages for sending and maintains the outbound message queue.
// The code arrangement here is not quite right. We probably want to factor out the bits that
// build and sign a message from those that add to the local queue/pool and broadcast it.
// See discussion in
// https://github.com/filecoin-project/go-filecoin/pull/3178#discussion_r311593312
// and https://github.com/filecoin-project/go-filecoin/issues/3052#issuecomment-513643661
type Outbox struct {
	// Signs messages
	signer types.Signer
	// Validates messages before sending them.
	validator consensus.MessageValidator
	// Holds messages sent from this node but not yet mined.
	queue *Queue
	// Publishes a signed message to the network.
	publisher publisher
	// Maintains message queue in response to new tipsets.
	policy QueuePolicy

	chains chainProvider
	actors actorProvider

	// Protects the "next nonce" calculation to avoid collisions.
	nonceLock sync.Mutex

	journal journal.Writer
}

type actorProvider interface {
	// GetActorAt returns the actor state defined by the chain up to some tipset
	GetActorAt(ctx context.Context, tipset block.TipSetKey, addr address.Address) (*actor.Actor, error)
}

type publisher interface {
	Publish(ctx context.Context, message *types.SignedMessage, height uint64, bcast bool) error
}

var msgSendErrCt = metrics.NewInt64Counter("message_sender_error", "Number of errors encountered while sending a message")

// NewOutbox creates a new outbox
func NewOutbox(signer types.Signer, validator consensus.MessageValidator, queue *Queue,
	publisher publisher, policy QueuePolicy, chains chainProvider, actors actorProvider, jw journal.Writer) *Outbox {
	return &Outbox{
		signer:    signer,
		validator: validator,
		queue:     queue,
		publisher: publisher,
		policy:    policy,
		chains:    chains,
		actors:    actors,
		journal:   jw,
	}
}

// Queue returns the outbox's outbound message queue.
func (ob *Outbox) Queue() *Queue {
	return ob.queue
}

// Send marshals and sends a message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) Send(ctx context.Context, from, to address.Address, value types.AttoFIL,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	defer func() {
		if err != nil {
			msgSendErrCt.Inc(ctx, 1)
		}
		ob.journal.Write("Send",
			"to", to.String(), "from", from.String(), "value", value.AsBigInt().Uint64(), "method", method,
			"gasPrice", gasPrice.AsBigInt().Uint64(), "gasLimit", uint64(gasLimit), "bcast", bcast,
			"params", params, "error", err, "cid", out.String())
	}()

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "invalid params")
	}

	// Lock to avoid a race inspecting the actor state and message queue to calculate next nonce.
	ob.nonceLock.Lock()
	defer ob.nonceLock.Unlock()

	head := ob.chains.GetHead()

	fromActor, err := ob.actors.GetActorAt(ctx, head, from)
	if err != nil {
		return cid.Undef, nil, errors.Wrapf(err, "no actor at address %s", from)
	}

	nonce, err := nextNonce(fromActor, ob.queue, from)
	if err != nil {
		return cid.Undef, nil, errors.Wrapf(err, "failed calculating nonce for actor at %s", from)
	}

	rawMsg := types.NewMeteredMessage(from, to, nonce, value, method, encodedParams, gasPrice, gasLimit)
	signed, err := types.NewSignedMessage(*rawMsg, ob.signer)

	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to sign message")
	}

	// Slightly awkward: it would be better validate before signing but the MeteredMessage construction
	// is hidden inside NewSignedMessage.
	err = ob.validator.Validate(ctx, &signed.Message, fromActor)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "invalid message")
	}

	return sendSignedMsg(ctx, ob, signed, bcast)
}

// SignedSend send a signed message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) SignedSend(ctx context.Context, signed *types.SignedMessage, bcast bool) (out cid.Cid, pubErrCh chan error, err error) {
	defer func() {
		if err != nil {
			msgSendErrCt.Inc(ctx, 1)
		}
	}()

	return sendSignedMsg(ctx, ob, signed, bcast)
}

// sendSignedMsg add signed message in pool and return cid
func sendSignedMsg(ctx context.Context, ob *Outbox, signed *types.SignedMessage, bcast bool) (cid.Cid, chan error, error) {
	head := ob.chains.GetHead()

	height, err := tipsetHeight(ob.chains, head)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to get block height")
	}

	// Add to the local message queue/pool at the last possible moment before broadcasting to network.
	if err := ob.queue.Enqueue(ctx, signed, height); err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to add message to outbound queue")
	}

	c, err := signed.Cid()
	if err != nil {
		return cid.Undef, nil, err
	}
	pubErrCh := make(chan error)

	go func() {
		err = ob.publisher.Publish(ctx, signed, height, bcast)
		if err != nil {
			log.Errorf("error: %s publishing message %s", err, c.String())
		}
		pubErrCh <- err
		close(pubErrCh)
	}()

	return c, pubErrCh, nil
}

// HandleNewHead maintains the message queue in response to a new head tipset.
func (ob *Outbox) HandleNewHead(ctx context.Context, oldTips, newTips []block.TipSet) error {
	return ob.policy.HandleNewHead(ctx, ob.queue, oldTips, newTips)
}

// nextNonce returns the next expected nonce value for an account actor. This is the larger
// of the actor's nonce value, or one greater than the largest nonce from the actor found in the message queue.
func nextNonce(act *actor.Actor, queue *Queue, address address.Address) (uint64, error) {
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

func tipsetHeight(provider chainProvider, key block.TipSetKey) (uint64, error) {
	head, err := provider.GetTipSet(key)
	if err != nil {
		return 0, err
	}
	return head.Height()
}
