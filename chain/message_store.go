package chain

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

// MessageReader is an interface exposing the read methods of the
// MessageStore.
type MessageReader interface {
	LoadMessages(context.Context, cid.Cid) ([]*types.SignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]*types.MessageReceipt, error)
}

// MessageWriter is an interface exposing the write methods of the
// MessageStore.
type MessageWriter interface {
	StoreMessages(context.Context, []*types.SignedMessage) (cid.Cid, error)
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
func (ms *MessageStore) LoadMessages(ctx context.Context, c cid.Cid) ([]*types.SignedMessage, error) {
	// TODO #1324 message collection shouldn't be a slice
	var out []*types.SignedMessage
	err := ms.ipldStore.Get(ctx, c, &out)
	return out, err
}

// StoreMessages puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreMessages(ctx context.Context, msgs []*types.SignedMessage) (cid.Cid, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	return ms.ipldStore.Put(ctx, msgs)
}

// LoadReceipts loads the signed messages in the collection with cid c from ipld
// storage and returns the slice implied by the collection
func (ms *MessageStore) LoadReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	var out []*types.MessageReceipt
	err := ms.ipldStore.Get(ctx, c, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreReceipts(ctx context.Context, receipts []*types.MessageReceipt) (cid.Cid, error) {
	// For now the collection is just a slice (cbor array)
	// TODO #1324 put these messages in a merkelized collection
	return ms.ipldStore.Put(ctx, receipts)
}
