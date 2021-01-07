package chain

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	cbor2 "github.com/filecoin-project/go-state-types/cbor"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"

	"github.com/filecoin-project/venus/pkg/config"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	adt0 "github.com/filecoin-project/specs-actors/actors/util/adt" // todo block headers use adt0
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
)

// MessageProvider is an interface exposing the load methods of the
// MessageStore.
type MessageProvider interface {
	LoadTipSetMessage(ctx context.Context, ts *block.TipSet) ([]block.BlockMessagesInfo, error)
	LoadMetaMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error)
	ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error)
	LoadUnsignedMessagesFromCids(blsCids []cid.Cid) ([]*types.UnsignedMessage, error)
	LoadSignedMessagesFromCids(secpCids []cid.Cid) ([]*types.SignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]types.MessageReceipt, error)
	LoadTxMeta(context.Context, cid.Cid) (types.TxMeta, error)
}

// MessageWriter is an interface exposing the write methods of the
// MessageStore.
type MessageWriter interface {
	StoreMessages(ctx context.Context, secpMessages []*types.SignedMessage, blsMessages []*types.UnsignedMessage) (cid.Cid, error)
	StoreReceipts(context.Context, []types.MessageReceipt) (cid.Cid, error)
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

// LoadMetaMessages loads the signed messages in the collection with cid c from ipld
// storage.
func (ms *MessageStore) LoadMetaMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	// load txmeta
	meta, err := ms.LoadTxMeta(ctx, metaCid)
	if err != nil {
		return nil, nil, err
	}

	secpCids, err := ms.loadAMTCids(ctx, meta.SecpRoot)
	if err != nil {
		return nil, nil, err
	}

	// load secp messages from cids
	secpMsgs, err := ms.LoadSignedMessagesFromCids(secpCids)
	if err != nil {
		return nil, nil, err
	}

	blsCids, err := ms.loadAMTCids(ctx, meta.BLSRoot)
	if err != nil {
		return nil, nil, err
	}

	// load bls messages from cids
	blsMsgs, err := ms.LoadUnsignedMessagesFromCids(blsCids)
	if err != nil {
		return nil, nil, err
	}

	return secpMsgs, blsMsgs, nil
}

func (ms *MessageStore) ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error) {
	meta, err := ms.LoadTxMeta(ctx, mmc)
	if err != nil {
		return nil, nil, err
	}

	secpCids, err := ms.loadAMTCids(ctx, meta.SecpRoot)
	if err != nil {
		return nil, nil, err
	}
	blsCids, err := ms.loadAMTCids(ctx, meta.BLSRoot)
	if err != nil {
		return nil, nil, err
	}
	return blsCids, secpCids, nil
}

func (ms *MessageStore) LoadMessage(mid cid.Cid) (types.ChainMsg, error) {
	m, err := ms.LoadUnsignedMessage(mid)
	if err == nil {
		return m, nil
	}

	if err != bstore.ErrNotFound {
		log.Warnf("GetCMessage: unexpected error getting unsigned message: %s", err)
	}

	return ms.LoadSignedMessage(mid)
}

func (ms *MessageStore) LoadUnsignedMessage(mid cid.Cid) (*types.UnsignedMessage, error) {
	messageBlock, err := ms.bs.Get(mid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bls message %s", mid)
	}
	message := &types.UnsignedMessage{}
	if err := message.UnmarshalCBOR(bytes.NewReader(messageBlock.RawData())); err != nil {
		return nil, errors.Wrapf(err, "could not decode bls message %s", mid)
	}
	return message, nil
}

func (ms *MessageStore) LoadSignedMessage(mid cid.Cid) (*types.SignedMessage, error) {
	messageBlock, err := ms.bs.Get(mid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bls message %s", mid)
	}

	message := &types.SignedMessage{}
	if err := message.UnmarshalCBOR(bytes.NewReader(messageBlock.RawData())); err != nil {
		return nil, errors.Wrapf(err, "could not decode secp message %s", mid)
	}

	return message, nil
}

func (ms *MessageStore) LoadUnsignedMessagesFromCids(blsCids []cid.Cid) ([]*types.UnsignedMessage, error) {
	blsMsgs := make([]*types.UnsignedMessage, len(blsCids))
	for i, c := range blsCids {
		message, err := ms.LoadUnsignedMessage(c)
		if err != nil {
			return nil, err
		}
		blsMsgs[i] = message
	}
	return blsMsgs, nil
}

func (ms *MessageStore) LoadSignedMessagesFromCids(secpCids []cid.Cid) ([]*types.SignedMessage, error) {
	secpMsgs := make([]*types.SignedMessage, len(secpCids))
	for i, c := range secpCids {
		message, err := ms.LoadSignedMessage(c)
		if err != nil {
			return nil, err
		}
		secpMsgs[i] = message
	}
	return secpMsgs, nil
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
	ret.SecpRoot = secpRaw

	// store bls messages
	blsCids, err := ms.storeUnsignedMessages(blsMessages)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp cids as AMT")
	}
	blsRaw, err := ms.storeAMTCids(ctx, blsCids)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store bls cids as AMT")
	}
	ret.BLSRoot = blsRaw

	return ms.StoreTxMeta(ctx, ret)
}

//load message from tipset NOTICE skip message with the same nonce
func (ms *MessageStore) LoadTipSetMesssages(ctx context.Context, ts *block.TipSet) ([][]*types.SignedMessage, [][]*types.UnsignedMessage, error) {
	var secpMessages [][]*types.SignedMessage
	var blsMessages [][]*types.UnsignedMessage

	applied := make(map[address.Address]uint64)

	selectMsg := func(m *types.UnsignedMessage) (bool, error) {
		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[m.From]; !ok {
			applied[m.From] = m.Nonce
		}

		if applied[m.From] != m.Nonce {
			return false, nil
		}

		applied[m.From]++

		return true, nil
	}

	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, blk.Messages)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		var blksecpMessages []*types.SignedMessage
		var blkblsMessages []*types.UnsignedMessage

		for _, msg := range blsMsgs {
			b, err := selectMsg(msg)
			if err != nil {
				return nil, nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				blkblsMessages = append(blkblsMessages, msg)
			}
		}

		for _, msg := range secpMsgs {
			b, err := selectMsg(&msg.Message)
			if err != nil {
				return nil, nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				blksecpMessages = append(blksecpMessages, msg)
			}
		}

		blsMessages = append(blsMessages, blkblsMessages)
		secpMessages = append(secpMessages, blksecpMessages)
	}

	return secpMessages, blsMessages, nil
}

// LoadReceipts loads the signed messages in the collection with cid c from ipld
// storage and returns the slice implied by the collection
func (ms *MessageStore) LoadReceipts(ctx context.Context, c cid.Cid) ([]types.MessageReceipt, error) {
	rawReceipts, err := ms.loadAMTRaw(ctx, c)
	if err != nil {
		return nil, err
	}

	// load receipts from cids
	receipts := make([]types.MessageReceipt, len(rawReceipts))
	for i, raw := range rawReceipts {
		receipt := types.MessageReceipt{}
		if err := receipt.UnmarshalCBOR(bytes.NewReader(raw)); err != nil {
			return nil, errors.Wrapf(err, "could not decode receipt %s", c)
		}
		receipts[i] = receipt
	}

	return receipts, nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreReceipts(ctx context.Context, receipts []types.MessageReceipt) (cid.Cid, error) {
	// store secp messages
	rawReceipts := make([][]byte, len(receipts))
	tmp := blockstoreutil.NewTemporary()
	for i, rcpt := range receipts {
		_, rcptBlock, err := ms.storeBlock(tmp, &rcpt)
		if err != nil {
			return cid.Undef, err
		}
		rawReceipts[i] = rcptBlock.RawData()
	}

	reccid, err := ms.storeAMTRaw(ctx, tmp, rawReceipts)
	if err != nil {
		return cid.Undef, err
	}
	err = blockstoreutil.CopyBlockstore(ctx, tmp, ms.bs)
	if err != nil {
		return cid.Undef, err
	}
	return reccid, nil
}

func (ms *MessageStore) loadAMTCids(ctx context.Context, c cid.Cid) ([]cid.Cid, error) {
	as := cbor.NewCborStore(ms.bs)
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

func (ms *MessageStore) loadAMTRaw(ctx context.Context, c cid.Cid) ([][]byte, error) {
	as := cbor.NewCborStore(ms.bs)
	a, err := amt.LoadAMT(ctx, as, c)
	if err != nil {
		return nil, err
	}

	raws := make([][]byte, a.Count)
	for i := uint64(0); i < a.Count; i++ {
		var raw cbg.Deferred
		if err := a.Get(ctx, i, &raw); err != nil {
			return nil, errors.Wrapf(err, "could not retrieve %d bytes from AMT", i)
		}

		raws[i] = raw.Raw
	}
	return raws, nil
}

// LoadTxMeta loads the secproot, blsroot data from the message store
func (ms *MessageStore) LoadTxMeta(ctx context.Context, c cid.Cid) (types.TxMeta, error) {
	metaBlock, err := ms.bs.Get(c)
	if err != nil {
		return types.TxMeta{}, errors.Wrapf(err, "failed to get tx meta %s", c)
	}

	var meta types.TxMeta
	if err := meta.UnmarshalCBOR(bytes.NewReader(metaBlock.RawData())); err != nil {
		return types.TxMeta{}, errors.Wrapf(err, "could not decode tx meta %s", c)
	}
	return meta, nil
}

func (ms *MessageStore) LoadTipSetMessage(ctx context.Context, ts *block.TipSet) ([]block.BlockMessagesInfo, error) {
	//gather message
	applied := make(map[address.Address]uint64)

	selectMsg := func(m *types.UnsignedMessage) (bool, error) {
		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[m.From]; !ok {
			applied[m.From] = m.Nonce
		}

		if applied[m.From] != m.Nonce {
			return false, nil
		}

		applied[m.From]++

		return true, nil
	}

	blockMsg := []block.BlockMessagesInfo{}
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, blk.Messages) // Corresponding to  MessagesForBlock of lotus
		if err != nil {
			return nil, errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		var sBlsMsg []types.ChainMsg
		var sSecpMsg []types.ChainMsg
		for _, msg := range blsMsgs {
			b, err := selectMsg(msg)
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				sBlsMsg = append(sBlsMsg, msg)
			}
		}
		for _, msg := range secpMsgs {
			b, err := selectMsg(&msg.Message)
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				sSecpMsg = append(sSecpMsg, msg)
			}
		}

		blockMsg = append(blockMsg, block.BlockMessagesInfo{
			BlsMessages:   sBlsMsg,
			SecpkMessages: sSecpMsg,
			Block:         blk,
		})
	}

	return blockMsg, nil
}

func (ms *MessageStore) MessagesForTipset(ts *block.TipSet) ([]types.ChainMsg, error) {
	bmsgs, err := ms.LoadTipSetMessage(context.TODO(), ts)
	if err != nil {
		return nil, err
	}

	var out []types.ChainMsg
	for _, bm := range bmsgs {
		for _, blsm := range bm.BlsMessages {
			out = append(out, blsm)
		}

		for _, secm := range bm.SecpkMessages {
			out = append(out, secm)
		}
	}

	return out, nil
}

func (ms *MessageStore) storeUnsignedMessages(messages []*types.UnsignedMessage) ([]cid.Cid, error) {
	cids := make([]cid.Cid, len(messages))
	var err error
	for i, msg := range messages {
		cids[i], _, err = ms.storeBlock(ms.bs, msg)
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
		cids[i], _, err = ms.storeBlock(ms.bs, msg)
		if err != nil {
			return nil, err
		}
	}
	return cids, nil
}

func (ms *MessageStore) StoreMessage(message types.ChainMsg) (cid.Cid, error) {
	c, _, err := ms.storeBlock(ms.bs, message)
	return c, err
}

// StoreTxMeta writes the secproot, blsroot block to the message store
func (ms *MessageStore) StoreTxMeta(ctx context.Context, meta types.TxMeta) (cid.Cid, error) {
	c, _, err := ms.storeBlock(ms.bs, &meta)
	return c, err
}

func (ms *MessageStore) storeBlock(bs bstore.Blockstore, data cbor2.Marshaler) (cid.Cid, blocks.Block, error) {
	sblk, err := MakeBlock(data)
	if err != nil {
		return cid.Undef, nil, err
	}

	if err := bs.Put(sblk); err != nil {
		return cid.Undef, nil, err
	}

	return sblk.Cid(), sblk, nil
}

func MakeBlock(obj cbor2.Marshaler) (blocks.Block, error) {
	buf := new(bytes.Buffer)
	err := obj.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	c, err := constants.DefaultCidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (ms *MessageStore) storeAMTRaw(ctx context.Context, bsstore blockstore.Blockstore, bs [][]byte) (cid.Cid, error) {
	as := cbor.NewCborStore(bsstore)

	rawMarshallers := make([]cbg.CBORMarshaler, len(bs))
	for i, raw := range bs {
		rawMarshallers[i] = &cbg.Deferred{Raw: raw}
	}
	return amt.FromArray(ctx, as, rawMarshallers)
}

func (ms *MessageStore) storeAMTCids(ctx context.Context, cids []cid.Cid) (cid.Cid, error) {
	as := cbor.NewCborStore(ms.bs)

	cidMarshallers := make([]cbg.CBORMarshaler, len(cids))
	for i, c := range cids {
		cidMarshaller := cbg.CborCid(c)
		cidMarshallers[i] = &cidMarshaller
	}
	return amt.FromArray(ctx, as, cidMarshallers)
}

func ComputeNextBaseFee(baseFee abi.TokenAmount, gasLimitUsed int64, noOfBlocks int, epoch abi.ChainEpoch, upgrade *config.ForkUpgradeConfig) abi.TokenAmount {
	// deta := gasLimitUsed/noOfBlocks - constants.BlockGasTarget
	// change := baseFee * deta / BlockGasTarget
	// nextBaseFee = baseFee + change
	// nextBaseFee = max(nextBaseFee, constants.MinimumBaseFee)

	var delta int64
	if epoch > upgrade.UpgradeSmokeHeight {
		delta = gasLimitUsed / int64(noOfBlocks)
		delta -= constants.BlockGasTarget
	} else {
		delta = constants.PackingEfficiencyDenom * gasLimitUsed / (int64(noOfBlocks) * constants.PackingEfficiencyNum)
		delta -= constants.BlockGasTarget
	}

	// cap change at 12.5% (BaseFeeMaxChangeDenom) by capping delta
	if delta > constants.BlockGasTarget {
		delta = constants.BlockGasTarget
	}
	if delta < -constants.BlockGasTarget {
		delta = -constants.BlockGasTarget
	}

	change := big.Mul(baseFee, big.NewInt(delta))
	change = big.Div(change, big.NewInt(constants.BlockGasTarget))
	change = big.Div(change, big.NewInt(constants.BaseFeeMaxChangeDenom))

	nextBaseFee := big.Add(baseFee, change)
	if big.Cmp(nextBaseFee, big.NewInt(constants.MinimumBaseFee)) < 0 {
		nextBaseFee = big.NewInt(constants.MinimumBaseFee)
	}
	return nextBaseFee
}

func (ms *MessageStore) ComputeBaseFee(ctx context.Context, ts *block.TipSet, upgrade *config.ForkUpgradeConfig) (abi.TokenAmount, error) {
	zero := abi.NewTokenAmount(0)
	baseHeight, err := ts.Height()
	if err != nil {
		return zero, err
	}

	if upgrade.UpgradeBreezeHeight >= 0 && baseHeight > upgrade.UpgradeBreezeHeight && baseHeight < upgrade.UpgradeBreezeHeight+upgrade.BreezeGasTampingDuration {
		return abi.NewTokenAmount(100), nil
	}

	// totalLimit is sum of GasLimits of unique messages in a tipset
	totalLimit := int64(0)

	seen := make(map[cid.Cid]struct{})

	for _, b := range ts.Blocks() {
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, b.Messages)
		if err != nil {
			return zero, xerrors.Errorf("error getting messages for: %s: %w", b.Cid(), err)
		}

		for _, m := range blsMsgs {
			c, err := m.Cid()
			if err != nil {
				return zero, xerrors.Errorf("error getting cid for message: %v: %w", m, err)
			}
			if _, ok := seen[c]; !ok {
				totalLimit += m.GasLimit
				seen[c] = struct{}{}
			}
		}
		for _, m := range secpMsgs {
			c, err := m.Cid()
			if err != nil {
				return zero, xerrors.Errorf("error getting cid for signed message: %v: %w", m, err)
			}
			if _, ok := seen[c]; !ok {
				totalLimit += m.Message.GasLimit
				seen[c] = struct{}{}
			}
		}
	}

	parentBaseFee := ts.Blocks()[0].ParentBaseFee

	return ComputeNextBaseFee(parentBaseFee, totalLimit, len(ts.Blocks()), baseHeight, upgrade), nil
}

func GetReceiptRoot(receipts []types.MessageReceipt) (cid.Cid, error) {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	rawReceipts := make([][]byte, len(receipts))
	for i, rcpt := range receipts {
		sblk, err := MakeBlock(&rcpt)
		if err != nil {
			return cid.Undef, err
		}
		rawReceipts[i] = sblk.RawData()
	}

	as := cbor.NewCborStore(bs)
	rawMarshallers := make([]cbg.CBORMarshaler, len(rawReceipts))
	for i, raw := range rawReceipts {
		rawMarshallers[i] = &cbg.Deferred{Raw: raw}
	}
	return amt.FromArray(context.TODO(), as, rawMarshallers)
}

func GetChainMsgRoot(ctx context.Context, bs blockstore.Blockstore, messages []types.ChainMsg) (cid.Cid, error) {
	tmpbs := bstore.NewTemporary()
	tmpstore := adt0.WrapStore(ctx, cbor.NewCborStore(tmpbs))

	arr := adt0.MakeEmptyArray(tmpstore)

	for i, m := range messages {
		b, err := m.ToStorageBlock()
		if err != nil {
			return cid.Undef, err
		}

		err = tmpbs.Put(b)
		if err != nil {
			return cid.Undef, err
		}

		k := cbg.CborCid(b.Cid())
		if err := arr.Set(uint64(i), &k); err != nil {
			return cid.Undef, xerrors.Errorf("failed to put message: %v", err)
		}
	}

	return arr.Root()
}

// computeMsgMeta computes the root CID of the combined arrays of message CIDs
// of both types (BLS and Secpk).
func ComputeMsgMeta(bs blockstore.Blockstore, bmsgCids, smsgCids []cid.Cid) (cid.Cid, error) {
	// block headers use adt0
	store := adt0.WrapStore(context.TODO(), cbor.NewCborStore(bs))
	bmArr := adt0.MakeEmptyArray(store)
	smArr := adt0.MakeEmptyArray(store)

	for i, m := range bmsgCids {
		c := cbg.CborCid(m)
		if err := bmArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	for i, m := range smsgCids {
		c := cbg.CborCid(m)
		if err := smArr.Set(uint64(i), &c); err != nil {
			return cid.Undef, err
		}
	}

	bmroot, err := bmArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	smroot, err := smArr.Root()
	if err != nil {
		return cid.Undef, err
	}

	mrcid, err := store.Put(store.Context(), &types.TxMeta{
		BLSRoot:  bmroot,
		SecpRoot: smroot,
	})
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to put msgmeta: %w", err)
	}

	return mrcid, nil
}
