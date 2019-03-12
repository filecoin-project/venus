package msg

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// Topic is the network pubsub topic identifier on which new messages are announced.
const Topic = "/fil/msgs"

// PublishFunc is a function the Sender calls to publish a message to the network.
type PublishFunc func(topic string, data []byte) error

// Sender is plumbing implementation that knows how to send a message.
type Sender struct {
	// Signs messages.
	signer types.Signer
	// Provides actor state (we could reduce this dependency to only LatestState()).
	chainReader chain.ReadStore
	// Pool of existing message, and receiver of the sent message.
	msgPool *core.MessagePool
	// Validates messages before sending them.
	validator consensus.SignedMessageValidator
	// Invoked to publish the new message to the network.
	publish PublishFunc
	// Protects the "next nonce" calculation to avoid collisions.
	l sync.Mutex
}

// NewSender returns a new Sender. There should be exactly one of these per node because
// sending locks to reduce nonce collisions.
func NewSender(signer types.Signer, chainReader chain.ReadStore, msgPool *core.MessagePool, validator consensus.SignedMessageValidator, publish PublishFunc) *Sender {
	return &Sender{signer: signer, chainReader: chainReader, msgPool: msgPool, validator: validator, publish: publish}
}

// Send sends a message. See api description.
func (s *Sender) Send(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid params")
	}

	// Lock to avoid race for message nonce.
	s.l.Lock()
	defer s.l.Unlock()

	st, err := s.chainReader.LatestState(ctx)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to load state from chain")
	}

	fromActor, err := st.GetActor(ctx, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "no actor at address %s", from)
	}

	nonce, err := nextNonce(ctx, st, s.msgPool, from)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed calculating nonce for actor %s", from)
	}

	msg := types.NewMessage(from, to, nonce, value, method, encodedParams)
	smsg, err := types.NewSignedMessage(*msg, s.signer, gasPrice, gasLimit)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to sign message")
	}

	err = s.validator.Validate(ctx, smsg, fromActor)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid message")
	}

	smsgdata, err := smsg.Marshal()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal message")
	}

	// Add to the local message pool at the last possible moment before broadcasting to network.
	if _, err := s.msgPool.Add(smsg); err != nil {
		return cid.Undef, errors.Wrap(err, "failed to add message to the message pool")
	}

	if err = s.publish(Topic, smsgdata); err != nil {
		return cid.Undef, errors.Wrap(err, "failed to publish message to network")
	}

	log.Debugf("MessageSend with message: %s", smsg)
	return smsg.Cid()
}

// nextNonce returns the next expected nonce value for an account actor. This is the larger
// of the actor's nonce value, or one greater than the largest nonce from the actor found in the message pool.
// The address must be the address of an account actor, or be not contained in, in the provided state tree.
func nextNonce(ctx context.Context, st state.Tree, pool *core.MessagePool, address address.Address) (uint64, error) {
	act, err := st.GetActor(ctx, address)
	if err != nil && !state.IsActorNotFoundError(err) {
		return 0, err
	}
	actorNonce, err := actor.NextNonce(act)
	if err != nil {
		return 0, err
	}

	poolNonce, found := pool.LargestNonce(address)
	if found && poolNonce >= actorNonce {
		return poolNonce + 1, nil
	}
	return actorNonce, nil
}
