package message

import (
	"context"
	"sync"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// ErrNoDefaultFromAddress is returned when a default address to send from couldn't be determined (eg, there are zero addresses in the wallet).
var ErrNoDefaultFromAddress = errors.New("unable to determine a default address to send the message from")

// Topic is the network pubsub topic identifier on which new messages are announced.
const Topic = "/fil/msgs"

// Publish is a function the Sender calls to publish a message to the network.
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

	// Locking in send reduces the change of nonce collision.
	l sync.Mutex
}

// NewSender returns a new sender. There should be exactly one of these per node because
// sending locks to reduce nonce collisions.
func NewSender(repo repo.Repo, wallet *wallet.Wallet, chainReader chain.ReadStore, msgPool *core.MessagePool, publish PublishFunc) *Sender {
	return &Sender{repo: repo, wallet: wallet, chainReader: chainReader, msgPool: msgPool, publish: publish}
}

// Send sends a message. It uses the default from address if none is given and signs the
// message using the wallet. This method "sends" in the sense that it enqueues the
// message in the msg pool and broadcasts it to the network; it does not wait for the
// message to go on chain.
func (s *Sender) Send(ctx context.Context, from, to address.Address, value *types.AttoFIL, method string, params ...interface{}) (cid.Cid, error) {
	// If the from address isn't set attempt to use the default address.
	if from == (address.Address{}) {
		ret, err := GetAndMaybeSetDefaultSenderAddress(s.repo, s.wallet)
		if (err != nil && err == ErrNoDefaultFromAddress) || ret == (address.Address{}) {
			return cid.Undef, ErrNoDefaultFromAddress
		}
		from = ret
	}

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "invalid params")
	}

	// Lock to avoid race for message nonce.
	s.l.Lock()
	defer s.l.Unlock()

	nonce, err := NextNonce(ctx, s.chainReader, s.msgPool, from)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "couldn't get next nonce")
	}

	msg := types.NewMessage(from, to, nonce, value, method, encodedParams)
	smsg, err := types.NewSignedMessage(*msg, s.wallet)
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

	return smsg.Cid()
}

// NextNonce returns the next nonce for the given address. It checks
// the actor's memory and also scans the message pool for any pending
// messages.
func NextNonce(ctx context.Context, chainReader chain.ReadStore, msgPool *core.MessagePool, address address.Address) (nonce uint64, err error) {
	st, err := chainReader.LatestState(ctx)
	if err != nil {
		return 0, err
	}

	nonce, err = core.NextNonce(ctx, st, msgPool, address)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}

// GetAndMaybeSetDefaultSenderAddress returns a default address from which to
// send messsages. If none is set it picks the first address in the wallet and
// sets it as the default in the config.
//
// Note: this is a silly pattern and should probably be replaced with something
// less magical.
func GetAndMaybeSetDefaultSenderAddress(repo repo.Repo, wallet *wallet.Wallet) (address.Address, error) {
	ret, err := repo.Config().Get("wallet.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || addr != (address.Address{}) {
		return addr, err
	}

	// No default is set; pick the 0th and make it the deafult.
	if len(wallet.Addresses()) > 0 {
		addr := wallet.Addresses()[0]
		newConfig := repo.Config()
		newConfig.Wallet.DefaultAddress = addr
		if err := repo.ReplaceConfig(newConfig); err != nil {
			return address.Address{}, err
		}

		return addr, nil
	}

	return address.Address{}, ErrNoDefaultFromAddress
}
