package chain

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessageProvider is an interface exposing the load methods of the
// MessageStore.
type MessageProvider interface {
	LoadMessages(context.Context, types.TxMeta) ([]*types.SignedMessage, []*types.SignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]*types.MessageReceipt, error)
}

// MessageWriter is an interface exposing the write methods of the
// MessageStore.
type MessageWriter interface {
	StoreMessages(ctx context.Context, secpMessages, blsMessages []*types.SignedMessage) (types.TxMeta, error)
	StoreReceipts(context.Context, []*types.MessageReceipt) (cid.Cid, error)
}

// MessageStore stores and loads collections of signed messages and receipts.
type MessageStore struct {
	ipldStore *hamt.CborIpldStore
}

// NewMessageStore creates and returns a new store
func NewMessageStore(cst *hamt.CborIpldStore) *MessageStore {
	return &MessageStore{
		ipldStore: cst,
	}
}

// LoadMessages loads the signed messages in the collection with cid c from ipld
// storage.
func (ms *MessageStore) LoadMessages(ctx context.Context, meta types.TxMeta) ([]*types.SignedMessage, []*types.SignedMessage, error) {
	// TODO #1324 message collection shouldn't be a slice
	var secp types.MessageCollection
	err := ms.ipldStore.Get(ctx, meta.SecpRoot, &secp)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load secp messages %s", meta.SecpRoot.String())
	}
	var bls types.MessageCollection
	err = ms.ipldStore.Get(ctx, meta.BLSRoot, &bls)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load bls messages %s", meta.BLSRoot.String())
	}
	return []*types.SignedMessage(secp), []*types.SignedMessage(bls), nil
}

// StoreMessages puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreMessages(ctx context.Context, secpMessages, blsMessages []*types.SignedMessage) (types.TxMeta, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	var ret types.TxMeta
	secpMsgCollection := types.MessageCollection(secpMessages)
	secpRoot, err := ms.ipldStore.Put(ctx, secpMsgCollection)
	if err != nil {
		return ret, err
	}

	blsMsgCollection := types.MessageCollection(blsMessages)
	blsRoot, err := ms.ipldStore.Put(ctx, blsMsgCollection)
	if err != nil {
		return ret, err
	}

	ret.SecpRoot = secpRoot
	ret.BLSRoot = blsRoot
	return ret, nil
}

// LoadReceipts loads the signed messages in the collection with cid c from ipld
// storage and returns the slice implied by the collection
func (ms *MessageStore) LoadReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	var out types.ReceiptCollection
	err := ms.ipldStore.Get(ctx, c, &out)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load receipts %s", c.String())
	}
	return []*types.MessageReceipt(out), nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreReceipts(ctx context.Context, receipts []*types.MessageReceipt) (cid.Cid, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	rcptCollection := types.ReceiptCollection(receipts)
	return ms.ipldStore.Put(ctx, rcptCollection)
}
