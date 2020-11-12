package message

import (
	"context"
	"golang.org/x/xerrors"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/journal"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm"
)

// Outbox validates and marshals messages for sending and maintains the outbound message queue.
// The code arrangement here is not quite right. We probably want to factor out the bits that
// build and sign a message from those that add to the local queue/pool and broadcast it.
// See discussion in
// https://github.com/filecoin-project/venus/pull/3178#discussion_r311593312
// and https://github.com/filecoin-project/venus/issues/3052#issuecomment-513643661
type Outbox struct {
	// Signs messages
	signer types.Signer
	// Validates messages before sending them.
	validator messageValidator
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

	gp gasPredictor
}

type messageValidator interface {
	// Validate checks a message for validity.
	ValidateSignedMessageSyntax(ctx context.Context, msg *types.SignedMessage) error
}

type actorProvider interface {
	// GetActorAt returns the actor state defined by the chain up to some tipset
	GetActorAt(ctx context.Context, tipset block.TipSetKey, addr address.Address) (*types.Actor, error)
}

type publisher interface {
	Publish(ctx context.Context, message *types.SignedMessage, height abi.ChainEpoch, bcast bool) error
}

type gasPredictor interface {
	CallWithGas(ctx context.Context, msg *types.UnsignedMessage) (*vm.Ret, error)
}

var msgSendErrCt = metrics.NewInt64Counter("message_sender_error", "Number of errors encountered while sending a message")

// NewOutbox creates a new outbox
func NewOutbox(signer types.Signer, validator messageValidator, queue *Queue, publisher publisher, policy QueuePolicy, chains chainProvider,
	actors actorProvider, jw journal.Writer, gp gasPredictor) *Outbox {
	return &Outbox{
		signer:    signer,
		validator: validator,
		queue:     queue,
		publisher: publisher,
		policy:    policy,
		chains:    chains,
		actors:    actors,
		journal:   jw,
		gp:        gp,
	}
}

// Queue returns the outbox's outbound message queue.
func (ob *Outbox) Queue() *Queue {
	return ob.queue
}

// Send marshals and sends a message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) Send(ctx context.Context, from, to address.Address, value types.AttoFIL,
	baseFee types.AttoFIL, gasPremium types.AttoFIL, gasLimit types.Unit, bcast bool, method abi.MethodNum, params interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	encodedParams, err := encoding.Encode(params)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "invalid params")
	}

	return ob.SendEncoded(ctx, from, to, value, baseFee, gasPremium, gasLimit, bcast, method, encodedParams)
}

// SendEncoded sends an encoded message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) SendEncoded(ctx context.Context, from, to address.Address, value types.AttoFIL,
	baseFee types.AttoFIL, gasPremium types.AttoFIL, gasLimit types.Unit, bcast bool, method abi.MethodNum, encodedParams []byte) (out cid.Cid, pubErrCh chan error, err error) {
	defer func() {
		if err != nil {
			msgSendErrCt.Inc(ctx, 1)
		}
		ob.journal.Write("SendEncoded",
			"to", to.String(), "from", from.String(), "value", value.Int.Uint64(), "method", method,
			"baseFee", baseFee.Int.Uint64(), "gasPremium", gasPremium.Int.Uint64(), "gasLimit", uint64(gasLimit), "bcast", bcast,
			"encodedParams", encodedParams, "error", err, "cid", out.String())
	}()

	// The spec's message syntax validation rules restricts empty parameters
	//  to be encoded as an empty byte string not cbor null
	if encodedParams == nil {
		encodedParams = []byte{}
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

	rawMsg := types.NewMeteredMessage(from, to, nonce, value, method, encodedParams, baseFee, gasPremium, gasLimit)
	msg, err := ob.GasEstimateMessageGas(ctx, rawMsg, nil, block.TipSetKey{})
	if err != nil {
		return cid.Undef, nil, xerrors.Errorf("GasEstimateMessageGas error: %w", err)
	}

	if msg.GasPremium.GreaterThan(msg.GasFeeCap) {
		return cid.Undef, nil, xerrors.Errorf("After estimation, GasPremium is greater than GasFeeCap")
	}

	signed, err := types.NewSignedMessage(ctx, *msg, ob.signer)

	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to sign message")
	}

	// Slightly awkward: it would be better validate before signing but the MeteredMessage construction
	// is hidden inside NewSignedMessage.
	err = ob.validator.ValidateSignedMessageSyntax(ctx, signed)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "invalid message")
	}

	return sendSignedMsg(ctx, ob, signed, bcast)
}

// Send marshals and sends a message, retaining it in the outbound message queue.
// If bcast is true, the publisher broadcasts the message to the network at the current block height.
func (ob *Outbox) UnSignedSend(ctx context.Context, message types.UnsignedMessage) (out cid.Cid, pubErrCh chan error, err error) {
	ob.nonceLock.Lock()
	defer ob.nonceLock.Unlock()

	head := ob.chains.GetHead()

	fromActor, err := ob.actors.GetActorAt(ctx, head, message.From)
	if err != nil {
		return cid.Undef, nil, errors.Wrapf(err, "no actor at address %s", message.From)
	}

	nonce, err := nextNonce(fromActor, ob.queue, message.From)
	if err != nil {
		return cid.Undef, nil, errors.Wrapf(err, "failed calculating nonce for actor at %s", message.From)
	}
	message.CallSeqNum = nonce

	signed, err := types.NewSignedMessage(ctx, message, ob.signer)

	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to sign message")
	}

	// Slightly awkward: it would be better validate before signing but the MeteredMessage construction
	// is hidden inside NewSignedMessage.
	err = ob.validator.ValidateSignedMessageSyntax(ctx, signed)
	if err != nil {
		return cid.Undef, nil, errors.Wrap(err, "invalid message")
	}

	return sendSignedMsg(ctx, ob, signed, true)
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
	if err := ob.queue.Enqueue(ctx, signed, uint64(height)); err != nil {
		return cid.Undef, nil, errors.Wrap(err, "failed to add message to outbound queue")
	}

	var c cid.Cid
	if signed.Message.From.Protocol() == address.BLS {
		// drop signature before generating Cid to match cid of message retrieved from block.
		c, err = signed.Message.Cid()
	} else {
		c, err = signed.Cid()
	}
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
func (ob *Outbox) HandleNewHead(ctx context.Context, oldTips, newTips []*block.TipSet) error {
	return ob.policy.HandleNewHead(ctx, ob.queue, oldTips, newTips)
}

// nextNonce returns the next expected nonce value for an account actor. This is the larger
// of the actor's nonce value, or one greater than the largest nonce from the actor found in the message queue.
func nextNonce(act *types.Actor, queue *Queue, address address.Address) (uint64, error) {
	if !(act.Empty() || builtin.IsAccountActor(act.Code.Cid)) {
		return 0, errors.New("next nonce only defined for account or empty actors")
	}

	poolNonce, found := queue.LargestNonce(address)
	if found && poolNonce >= act.CallSeqNum {
		return poolNonce + 1, nil
	}
	return act.CallSeqNum, nil
}

func tipsetHeight(provider chainProvider, key block.TipSetKey) (abi.ChainEpoch, error) {
	head, err := provider.GetTipSet(key)
	if err != nil {
		return 0, err
	}
	return head.Height()
}
