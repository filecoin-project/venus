package chain

import (
	"context"

	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// MessageProvider is an interface exposing the load methods of the
// MessageStore.
type MessageProvider interface {
	LoadMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]vm.MessageReceipt, error)
	LoadTxMeta(context.Context, cid.Cid) (types.TxMeta, error)
}

// MessageWriter is an interface exposing the write methods of the
// MessageStore.
type MessageWriter interface {
	StoreMessages(ctx context.Context, secpMessages []*types.SignedMessage, blsMessages []*types.UnsignedMessage) (cid.Cid, error)
	StoreReceipts(context.Context, []vm.MessageReceipt) (cid.Cid, error)
	StoreTxMeta(context.Context, types.TxMeta) (cid.Cid, error)
}

// MessageStore stores and loads collections of signed messages and receipts.
type MessageStore struct {
	bs blockstore.Blockstore
}

// NewMessageStore creates and returns a new store
func NewMessageStore(bs blockstore.Blockstore) *MessageStore {
	return &MessageStore{bs: bs}
}

// LoadMessages loads the signed messages in the collection with cid c from ipld
// storage.
func (ms *MessageStore) LoadMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	// load txmeta
	meta, err := ms.LoadTxMeta(ctx, metaCid)
	if err != nil {
		return nil, nil, err
	}

	secpCids, err := ms.loadAMTCids(ctx, meta.SecpRoot.Cid)
	if err != nil {
		return nil, nil, err
	}

	// load secp messages from cids
	secpMsgs := make([]*types.SignedMessage, len(secpCids))
	for i, c := range secpCids {
		messageBlock, err := ms.bs.Get(c)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to get secp message %s", c)
		}

		message := &types.SignedMessage{}
		if err := encoding.Decode(messageBlock.RawData(), message); err != nil {
			return nil, nil, errors.Wrapf(err, "could not decode secp message %s", c)
		}
		secpMsgs[i] = message
	}

	blsCids, err := ms.loadAMTCids(ctx, meta.BLSRoot.Cid)
	if err != nil {
		return nil, nil, err
	}

	// load bls messages from cids
	blsMsgs := make([]*types.UnsignedMessage, len(blsCids))
	for i, c := range blsCids {
		messageBlock, err := ms.bs.Get(c)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to get bls message %s", c)
		}

		message := &types.UnsignedMessage{}
		if err := encoding.Decode(messageBlock.RawData(), message); err != nil {
			return nil, nil, errors.Wrapf(err, "could not decode bls message %s", c)
		}
		blsMsgs[i] = message
	}

	return secpMsgs, blsMsgs, nil
}

// StoreMessages puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreMessages(ctx context.Context, secpMessages []*types.SignedMessage, blsMessages []*types.UnsignedMessage) (cid.Cid, error) {
	var ret types.TxMeta
	var err error

	// store secp messages
	secpCids, err := ms.storeSignedMessages(secpMessages)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp messages")
	}

	secpRaw, err := ms.storeAMTCids(ctx, secpCids)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp cids as AMT")
	}
	ret.SecpRoot = e.NewCid(secpRaw)

	// store bls messages
	blsCids, err := ms.storeUnsignedMessages(blsMessages)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp cids as AMT")
	}
	blsRaw, err := ms.storeAMTCids(ctx, blsCids)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store bls cids as AMT")
	}
	ret.BLSRoot = e.NewCid(blsRaw)

	return ms.StoreTxMeta(ctx, ret)
}

// LoadReceipts loads the signed messages in the collection with cid c from ipld
// storage and returns the slice implied by the collection
func (ms *MessageStore) LoadReceipts(ctx context.Context, c cid.Cid) ([]vm.MessageReceipt, error) {
	receiptCids, err := ms.loadAMTCids(ctx, c)
	if err != nil {
		return nil, err
	}

	// load receipts from cids
	receipts := make([]vm.MessageReceipt, len(receiptCids))
	for i, c := range receiptCids {
		receiptBlock, err := ms.bs.Get(c)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get receipt %s", c)
		}

		receipt := vm.MessageReceipt{}
		if err := encoding.Decode(receiptBlock.RawData(), &receipt); err != nil {
			return nil, errors.Wrapf(err, "could not decode receipt %s", c)
		}
		receipts[i] = receipt
	}

	return receipts, nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreReceipts(ctx context.Context, receipts []vm.MessageReceipt) (cid.Cid, error) {
	// store secp messages
	cids, err := ms.storeMessageReceipts(receipts)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp messages")
	}

	return ms.storeAMTCids(ctx, cids)
}

func (ms *MessageStore) loadAMTCids(ctx context.Context, c cid.Cid) ([]cid.Cid, error) {
	as := cborutil.NewIpldStore(ms.bs)
	a, err := amt.LoadAMT(ctx, as, c)
	if err != nil {
		return []cid.Cid{}, err
	}

	cids := make([]cid.Cid, a.Count)
	for i := uint64(0); i < a.Count; i++ {
		var c cid.Cid
		if err := a.Get(ctx, i, &c); err != nil {
			return nil, errors.Wrapf(err, "could not retrieve %d cid from AMT", i)
		}

		cids[i] = c
	}

	return cids, nil
}

// LoadTxMeta loads the secproot, blsroot data from the message store
func (ms *MessageStore) LoadTxMeta(ctx context.Context, c cid.Cid) (types.TxMeta, error) {
	metaBlock, err := ms.bs.Get(c)
	if err != nil {
		return types.TxMeta{}, errors.Wrapf(err, "failed to get tx meta %s", c)
	}

	var meta types.TxMeta
	if err := encoding.Decode(metaBlock.RawData(), &meta); err != nil {
		return types.TxMeta{}, errors.Wrapf(err, "could not decode tx meta %s", c)
	}
	return meta, nil
}

func (ms *MessageStore) storeUnsignedMessages(messages []*types.UnsignedMessage) ([]cid.Cid, error) {
	cids := make([]cid.Cid, len(messages))
	var err error
	for i, msg := range messages {
		cids[i], err = ms.storeBlock(msg)
		if err != nil {
			return nil, err
		}
	}
	return cids, nil
}

func (ms *MessageStore) storeSignedMessages(messages []*types.SignedMessage) ([]cid.Cid, error) {
	cids := make([]cid.Cid, len(messages))
	var err error
	for i, msg := range messages {
		cids[i], err = ms.storeBlock(msg)
		if err != nil {
			return nil, err
		}
	}
	return cids, nil
}

// StoreTxMeta writes the secproot, blsroot block to the message store
func (ms *MessageStore) StoreTxMeta(ctx context.Context, meta types.TxMeta) (cid.Cid, error) {
	return ms.storeBlock(meta)
}

func (ms *MessageStore) storeMessageReceipts(receipts []vm.MessageReceipt) ([]cid.Cid, error) {
	cids := make([]cid.Cid, len(receipts))
	var err error
	for i, msg := range receipts {
		cids[i], err = ms.storeBlock(msg)
		if err != nil {
			return nil, err
		}
	}
	return cids, nil
}

func (ms *MessageStore) storeBlock(data interface{}) (cid.Cid, error) {
	sblk, err := makeBlock(data)
	if err != nil {
		return cid.Undef, err
	}

	if err := ms.bs.Put(sblk); err != nil {
		return cid.Undef, err
	}

	return sblk.Cid(), nil
}

func makeBlock(obj interface{}) (blocks.Block, error) {
	data, err := encoding.Encode(obj)
	if err != nil {
		return nil, err
	}

	pre := cid.NewPrefixV1(cid.DagCBOR, multihash.BLAKE2B_MIN+31)
	c, err := pre.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (ms *MessageStore) storeAMTCids(ctx context.Context, cids []cid.Cid) (cid.Cid, error) {
	as := cborutil.NewIpldStore(ms.bs)

	cidMarshallers := make([]cbg.CBORMarshaler, len(cids))
	for i, c := range cids {
		cidMarshaller := cbg.CborCid(c)
		cidMarshallers[i] = &cidMarshaller
	}
	return amt.FromArray(ctx, as, cidMarshallers)
}
