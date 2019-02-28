package msg

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// Topic is the network pubsub topic identifier on which new messages are announced.
const Topic = "/fil/msgs"

// PublishFunc is a function the Sender calls to publish a message to the network.
type PublishFunc func(topic string, data []byte) error

// Sender is plumbing implementation that knows how to send a message.
type Sender struct {
	// For getting the default address.
	repo   repo.Repo
	wallet *wallet.Wallet

	// For getting the latest nonce and enqueuing messages.
	chainReader chain.ReadStore
	msgPool     *core.MessagePool

	// To publish the new message to the network.
	publish PublishFunc

	// Locking in send reduces the chance of nonce collision.
	l sync.Mutex
}

// NewSender returns a new Sender. There should be exactly one of these per node because
// sending locks to reduce nonce collisions.
func NewSender(repo repo.Repo, wallet *wallet.Wallet, chainReader chain.ReadStore, msgPool *core.MessagePool, publish PublishFunc) *Sender {
	return &Sender{repo: repo, wallet: wallet, chainReader: chainReader, msgPool: msgPool, publish: publish}
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

	nonce, err := nextNonce(ctx, st, s.msgPool, from)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "couldn't get next nonce")
	}

	msg := types.NewMessage(from, to, nonce, value, method, encodedParams)
	smsg, err := types.NewSignedMessage(*msg, s.wallet, gasPrice, gasLimit)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to sign message")
	}

	smsgdata, err := smsg.Marshal()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal message")
	}

	if _, err := s.msgPool.Add(smsg); err != nil {
		return cid.Undef, errors.Wrap(err, "failed to add message to the message pool")
	}

	if err = s.publish(Topic, smsgdata); err != nil {
		return cid.Undef, errors.Wrap(err, "couldnt publish new message to network")
	}

	log.Debugf("MessageSend with message: %s", smsg)

	return smsg.Cid()
}

// nextNonce returns the next expected nonce value for an account actor. This is the larger
// of the actor's nonce value, or one greater than the largest nonce from the actor found in the message pool.
// The address must be the address of an account actor, or be not contained in, in the provided state tree.
func nextNonce(ctx context.Context, st state.Tree, pool *core.MessagePool, address address.Address) (nonce uint64, err error) {
	act, err := st.GetActor(ctx, address)
	if err != nil && !state.IsActorNotFoundError(err) {
		return 0, err
	}
	actorNonce, err := actor.NextNonce(act)
	if err != nil {
		return 0, err
	}

	poolNonce, found := core.LargestNonce(pool, address)
	if found && poolNonce >= actorNonce {
		return poolNonce + 1, nil
	}
	return actorNonce, nil
}
