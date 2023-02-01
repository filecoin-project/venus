package messagepool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const dsKeyActorNonce = "ActorNextNonce"

type MpoolNonceAPI interface {
	GetNonce(context.Context, address.Address, types.TipSetKey) (uint64, error)
	GetActor(context.Context, address.Address, types.TipSetKey) (*types.Actor, error)
}

// MessageSigner keeps track of nonces per address, and increments the nonce
// when signing a message
type MessageSigner struct {
	wallet wallet.WalletIntersection
	lk     sync.Mutex
	mpool  MpoolNonceAPI
	ds     datastore.Batching
}

func NewMessageSigner(wallet wallet.WalletIntersection, mpool MpoolNonceAPI, ds datastore.Batching) *MessageSigner {
	ds = namespace.Wrap(ds, datastore.NewKey("/message-signer/"))
	return &MessageSigner{
		wallet: wallet,
		mpool:  mpool,
		ds:     ds,
	}
}

// SignMessage increments the nonce for the message From address, and signs
// the message
func (ms *MessageSigner) SignMessage(ctx context.Context, msg *types.Message, cb func(*types.SignedMessage) error) (*types.SignedMessage, error) {
	ms.lk.Lock()
	defer ms.lk.Unlock()

	// Get the next message nonce
	nonce, err := ms.nextNonce(ctx, msg.From)
	if err != nil {
		return nil, fmt.Errorf("failed to create nonce: %w", err)
	}

	// Sign the message with the nonce
	msg.Nonce = nonce

	sb, err := msg.SigningBytes(types.AddressProtocol2SignType(msg.From.Protocol()))
	if err != nil {
		return nil, err
	}

	mb, err := msg.ToStorageBlock()
	if err != nil {
		return nil, fmt.Errorf("serializing message: %w", err)
	}

	sig, err := ms.wallet.WalletSign(ctx, msg.From, sb, types.MsgMeta{
		Type:  types.MTChainMsg,
		Extra: mb.RawData(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	// Callback with the signed message
	smsg := &types.SignedMessage{
		Message:   *msg,
		Signature: *sig,
	}
	err = cb(smsg)
	if err != nil {
		return nil, err
	}

	// If the callback executed successfully, write the nonce to the datastore
	if err := ms.saveNonce(ctx, msg.From, nonce); err != nil {
		return nil, fmt.Errorf("failed to save nonce: %w", err)
	}

	return smsg, nil
}

// nextNonce gets the next nonce for the given address.
// If there is no nonce in the datastore, gets the nonce from the message pool.
func (ms *MessageSigner) nextNonce(ctx context.Context, addr address.Address) (uint64, error) {
	// Nonces used to be created by the mempool and we need to support nodes
	// that have mempool nonces, so first check the mempool for a nonce for
	// this address. Note that the mempool returns the actor state's nonce
	// by default.
	nonce, err := ms.mpool.GetNonce(ctx, addr, types.EmptyTSK)
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce from mempool: %w", err)
	}

	// Get the next nonce for this address from the datastore
	addrNonceKey := ms.dstoreKey(addr)
	dsNonceBytes, err := ms.ds.Get(ctx, addrNonceKey)

	switch {
	case errors.Is(err, datastore.ErrNotFound):
		// If a nonce for this address hasn't yet been created in the
		// datastore, just use the nonce from the mempool
		return nonce, nil

	case err != nil:
		return 0, fmt.Errorf("failed to get nonce from datastore: %w", err)

	default:
		// There is a nonce in the datastore, so unmarshall it
		maj, dsNonce, err := cbg.CborReadHeader(bytes.NewReader(dsNonceBytes))
		if err != nil {
			return 0, fmt.Errorf("failed to parse nonce from datastore: %w", err)
		}
		if maj != cbg.MajUnsignedInt {
			return 0, fmt.Errorf("bad cbor type parsing nonce from datastore")
		}

		// The message pool nonce should be <= than the datastore nonce
		if nonce <= dsNonce {
			nonce = dsNonce
		} else {
			log.Warnf("mempool nonce was larger than datastore nonce (%d > %d)", nonce, dsNonce)
		}

		return nonce, nil
	}
}

// saveNonce increments the nonce for this address and writes it to the
// datastore
func (ms *MessageSigner) saveNonce(ctx context.Context, addr address.Address, nonce uint64) error {
	// Increment the nonce
	nonce++

	// Write the nonce to the datastore
	addrNonceKey := ms.dstoreKey(addr)
	buf := bytes.Buffer{}
	_, err := buf.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, nonce))
	if err != nil {
		return fmt.Errorf("failed to marshall nonce: %w", err)
	}
	err = ms.ds.Put(ctx, addrNonceKey, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write nonce to datastore: %w", err)
	}
	return nil
}

func (ms *MessageSigner) dstoreKey(addr address.Address) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyActorNonce, addr.String()})
}
