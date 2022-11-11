package chain

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	cbor2 "github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state/tree"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// MessageProvider is an interface exposing the load methods of the
// MessageStore.
type MessageProvider interface {
	LoadTipSetMessage(ctx context.Context, ts *types.TipSet) ([]types.BlockMessagesInfo, error)
	LoadMetaMessages(context.Context, cid.Cid) ([]*types.SignedMessage, []*types.Message, error)
	ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error)
	LoadUnsignedMessagesFromCids(ctx context.Context, blsCids []cid.Cid) ([]*types.Message, error)
	LoadSignedMessagesFromCids(ctx context.Context, secpCids []cid.Cid) ([]*types.SignedMessage, error)
	LoadReceipts(context.Context, cid.Cid) ([]types.MessageReceipt, error)
	LoadTxMeta(context.Context, cid.Cid) (types.MessageRoot, error)
}

// MessageWriter is an interface exposing the write methods of the
// MessageStore.
type MessageWriter interface {
	StoreMessages(ctx context.Context, secpMessages []*types.SignedMessage, blsMessages []*types.Message) (cid.Cid, error)
	StoreReceipts(context.Context, []types.MessageReceipt) (cid.Cid, error)
	StoreTxMeta(context.Context, types.MessageRoot) (cid.Cid, error)
}

// MessageStore stores and loads collections of signed messages and receipts.
type MessageStore struct {
	bs    blockstoreutil.Blockstore
	fkCfg *config.ForkUpgradeConfig
}

// NewMessageStore creates and returns a new store
func NewMessageStore(bs blockstoreutil.Blockstore, fkCfg *config.ForkUpgradeConfig) *MessageStore {
	return &MessageStore{bs: bs, fkCfg: fkCfg}
}

// LoadMetaMessages loads the signed messages in the collection with cid c from ipld
// storage.
func (ms *MessageStore) LoadMetaMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, []*types.Message, error) {
	// load txmeta
	meta, err := ms.LoadTxMeta(ctx, metaCid)
	if err != nil {
		return nil, nil, err
	}

	secpCids, err := ms.loadAMTCids(ctx, meta.SecpkRoot)
	if err != nil {
		return nil, nil, err
	}

	// load secp messages from cids
	secpMsgs, err := ms.LoadSignedMessagesFromCids(ctx, secpCids)
	if err != nil {
		return nil, nil, err
	}

	blsCids, err := ms.loadAMTCids(ctx, meta.BlsRoot)
	if err != nil {
		return nil, nil, err
	}

	// load bls messages from cids
	blsMsgs, err := ms.LoadUnsignedMessagesFromCids(ctx, blsCids)
	if err != nil {
		return nil, nil, err
	}

	return secpMsgs, blsMsgs, nil
}

// ReadMsgMetaCids load messager from message meta cid
func (ms *MessageStore) ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error) {
	meta, err := ms.LoadTxMeta(ctx, mmc)
	if err != nil {
		return nil, nil, err
	}

	secpCids, err := ms.loadAMTCids(ctx, meta.SecpkRoot)
	if err != nil {
		return nil, nil, err
	}
	blsCids, err := ms.loadAMTCids(ctx, meta.BlsRoot)
	if err != nil {
		return nil, nil, err
	}
	return blsCids, secpCids, nil
}

// LoadMessage load message of specify message cid
// First get the unsigned message. If it is not found, then get the signed message. If still not found, an error will be returned
func (ms *MessageStore) LoadMessage(ctx context.Context, mid cid.Cid) (types.ChainMsg, error) {
	m, err := ms.LoadUnsignedMessage(ctx, mid)
	if err == nil {
		return m, nil
	}

	if !ipld.IsNotFound(err) {
		log.Warnf("GetCMessage: unexpected error getting unsigned message: %s", err)
	}

	return ms.LoadSignedMessage(ctx, mid)
}

// LoadUnsignedMessage load unsigned messages in tipset
func (ms *MessageStore) LoadUnsignedMessage(ctx context.Context, mid cid.Cid) (*types.Message, error) {
	messageBlock, err := ms.bs.Get(ctx, mid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bls message %s", mid)
	}
	message := &types.Message{}
	if err := message.UnmarshalCBOR(bytes.NewReader(messageBlock.RawData())); err != nil {
		return nil, errors.Wrapf(err, "could not decode bls message %s", mid)
	}
	return message, nil
}

// LoadUnsignedMessagesFromCids load unsigned messages of cid array
func (ms *MessageStore) LoadSignedMessage(ctx context.Context, mid cid.Cid) (*types.SignedMessage, error) {
	messageBlock, err := ms.bs.Get(ctx, mid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bls message %s", mid)
	}

	message := &types.SignedMessage{}
	if err := message.UnmarshalCBOR(bytes.NewReader(messageBlock.RawData())); err != nil {
		return nil, errors.Wrapf(err, "could not decode secp message %s", mid)
	}

	return message, nil
}

// LoadUnsignedMessagesFromCids load unsigned messages of cid array
func (ms *MessageStore) LoadUnsignedMessagesFromCids(ctx context.Context, blsCids []cid.Cid) ([]*types.Message, error) {
	blsMsgs := make([]*types.Message, len(blsCids))
	for i, c := range blsCids {
		message, err := ms.LoadUnsignedMessage(ctx, c)
		if err != nil {
			return nil, err
		}
		blsMsgs[i] = message
	}
	return blsMsgs, nil
}

// LoadSignedMessagesFromCids load signed messages of cid array
func (ms *MessageStore) LoadSignedMessagesFromCids(ctx context.Context, secpCids []cid.Cid) ([]*types.SignedMessage, error) {
	secpMsgs := make([]*types.SignedMessage, len(secpCids))
	for i, c := range secpCids {
		message, err := ms.LoadSignedMessage(ctx, c)
		if err != nil {
			return nil, err
		}
		secpMsgs[i] = message
	}
	return secpMsgs, nil
}

// StoreMessages puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreMessages(ctx context.Context, secpMessages []*types.SignedMessage, blsMessages []*types.Message) (cid.Cid, error) {
	var ret types.MessageRoot
	var err error

	// store secp messages
	as := cbor.NewCborStore(ms.bs)
	secpMsgArr := adt.MakeEmptyArray(adt.WrapStore(ctx, as))
	for i, msg := range secpMessages {
		secpCid, err := ms.StoreMessage(msg)
		if err != nil {
			return cid.Undef, errors.Wrap(err, "could not store secp messages")
		}
		err = secpMsgArr.Set(uint64(i), (*cbg.CborCid)(&secpCid))
		if err != nil {
			return cid.Undef, errors.Wrap(err, "could not store secp messages cid")
		}
	}

	secpRaw, err := secpMsgArr.Root()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store secp cids as AMT")
	}
	ret.SecpkRoot = secpRaw

	// store bls messages
	blsMsgArr := adt.MakeEmptyArray(adt.WrapStore(ctx, as))
	for i, msg := range blsMessages {
		blsCid, err := ms.StoreMessage(msg)
		if err != nil {
			return cid.Undef, errors.Wrap(err, "could not store bls messages")
		}
		err = blsMsgArr.Set(uint64(i), (*cbg.CborCid)(&blsCid))
		if err != nil {
			return cid.Undef, errors.Wrap(err, "could not store secp messages cid")
		}
	}

	blsRaw, err := blsMsgArr.Root()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not store bls cids as AMT")
	}
	ret.BlsRoot = blsRaw

	return ms.StoreTxMeta(ctx, ret)
}

// load message from tipset NOTICE skip message with the same nonce
func (ms *MessageStore) LoadTipSetMesssages(ctx context.Context, ts *types.TipSet) ([][]*types.SignedMessage, [][]*types.Message, error) {
	var secpMessages [][]*types.SignedMessage
	var blsMessages [][]*types.Message

	applied := make(map[address.Address]uint64)

	vms := cbor.NewCborStore(ms.bs)
	st, err := tree.LoadState(ctx, vms, ts.Blocks()[0].ParentStateRoot)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load state tree %s", ts.Blocks()[0].ParentStateRoot.String())
	}

	selectMsg := func(m *types.Message) (bool, error) {
		var sender address.Address
		if ts.Height() >= ms.fkCfg.UpgradeHyperdriveHeight {
			sender, err = st.LookupID(m.From)
			if err != nil {
				return false, err
			}
		} else {
			sender = m.From
		}

		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[sender]; !ok {
			applied[sender] = m.Nonce
		}

		if applied[sender] != m.Nonce {
			return false, nil
		}

		applied[sender]++

		return true, nil
	}

	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, blk.Messages)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		var blksecpMessages []*types.SignedMessage
		var blkblsMessages []*types.Message

		for _, msg := range blsMsgs {
			b, err := selectMsg(msg)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to decide whether to select message for block")
			}
			if b {
				blkblsMessages = append(blkblsMessages, msg)
			}
		}

		for _, msg := range secpMsgs {
			b, err := selectMsg(&msg.Message)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to decide whether to select message for block")
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
	as := cbor.NewCborStore(ms.bs)
	fmt.Println(c.String())
	a, err := adt.AsArray(adt.WrapStore(ctx, as), c)
	if err != nil {
		return nil, err
	}

	receipts := make([]types.MessageReceipt, a.Length())
	for i := uint64(0); i < a.Length(); i++ {
		var rec types.MessageReceipt
		if found, err := a.Get(i, &rec); err != nil {
			return nil, errors.Wrapf(err, "could not retrieve %d bytes from AMT", i)
		} else if !found {
			return nil, errors.Errorf("failed to find receipt %d", i)
		}
		receipts[i] = rec
	}
	return receipts, nil
}

// StoreReceipts puts the input signed messages to a collection and then writes
// this collection to ipld storage.  The cid of the collection is returned.
func (ms *MessageStore) StoreReceipts(ctx context.Context, receipts []types.MessageReceipt) (cid.Cid, error) {
	tmp := blockstoreutil.NewTemporary()
	rectarr := adt.MakeEmptyArray(adt.WrapStore(ctx, cbor.NewCborStore(tmp)))

	for i, receipt := range receipts {
		if err := rectarr.Set(uint64(i), &receipt); err != nil {
			return cid.Undef, errors.Wrap(err, "failed to build receipts amt")
		}
	}

	root, err := rectarr.Root()
	if err != nil {
		return cid.Undef, err
	}

	err = blockstoreutil.CopyParticial(ctx, tmp, ms.bs, root)
	if err != nil {
		return cid.Undef, err
	}

	return rectarr.Root()
}

func (ms *MessageStore) loadAMTCids(ctx context.Context, c cid.Cid) ([]cid.Cid, error) {
	as := cbor.NewCborStore(ms.bs)
	a, err := adt.AsArray(adt.WrapStore(ctx, as), c)
	if err != nil {
		return []cid.Cid{}, err
	}

	cids := make([]cid.Cid, a.Length())
	for i := uint64(0); i < a.Length(); i++ {
		oc := cbg.CborCid(c)
		if found, err := a.Get(i, &oc); err != nil {
			return nil, errors.Wrapf(err, "could not retrieve %d cid from AMT", i)
		} else if !found {
			return nil, errors.Errorf("failed to find receipt %d", i)
		}

		cids[i] = cid.Cid(oc)
	}

	return cids, nil
}

// LoadTxMeta loads the secproot, blsroot data from the message store
func (ms *MessageStore) LoadTxMeta(ctx context.Context, c cid.Cid) (types.MessageRoot, error) {
	metaBlock, err := ms.bs.Get(ctx, c)
	if err != nil {
		return types.MessageRoot{}, errors.Wrapf(err, "failed to get tx meta %s", c)
	}

	var meta types.MessageRoot
	if err := meta.UnmarshalCBOR(bytes.NewReader(metaBlock.RawData())); err != nil {
		return types.MessageRoot{}, errors.Wrapf(err, "could not decode tx meta %s", c)
	}
	return meta, nil
}

// LoadTipSetMessage message from tipset NOTICE skip message with the same nonce
func (ms *MessageStore) LoadTipSetMessage(ctx context.Context, ts *types.TipSet) ([]types.BlockMessagesInfo, error) {
	// gather message
	applied := make(map[address.Address]uint64)

	vms := cbor.NewCborStore(ms.bs)
	st, err := tree.LoadState(ctx, vms, ts.Blocks()[0].ParentStateRoot)
	if err != nil {
		return nil, errors.Errorf("failed to load state tree")
	}

	selectMsg := func(m *types.Message) (bool, error) {
		var sender address.Address
		if ts.Height() >= ms.fkCfg.UpgradeHyperdriveHeight {
			sender, err = st.LookupID(m.From)
			if err != nil {
				return false, err
			}
		} else {
			sender = m.From
		}

		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[sender]; !ok {
			applied[sender] = m.Nonce
		}

		if applied[sender] != m.Nonce {
			return false, nil
		}

		applied[sender]++

		return true, nil
	}

	var blockMsg []types.BlockMessagesInfo
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, blk.Messages) // Corresponding to  MessagesForBlock of lotus
		if err != nil {
			return nil, errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		sBlsMsg := make([]types.ChainMsg, 0, len(blsMsgs))
		sSecpMsg := make([]types.ChainMsg, 0, len(secpMsgs))
		for _, msg := range blsMsgs {
			b, err := selectMsg(msg)
			if err != nil {
				return nil, errors.Wrap(err, "failed to decide whether to select message for block")
			}
			if b {
				sBlsMsg = append(sBlsMsg, msg)
			}
		}
		for _, msg := range secpMsgs {
			b, err := selectMsg(&msg.Message)
			if err != nil {
				return nil, errors.Wrap(err, "failed to decide whether to select message for block")
			}
			if b {
				sSecpMsg = append(sSecpMsg, msg)
			}
		}

		blockMsg = append(blockMsg, types.BlockMessagesInfo{
			BlsMessages:   sBlsMsg,
			SecpkMessages: sSecpMsg,
			Block:         blk,
		})
	}

	return blockMsg, nil
}

// MessagesForTipset return of message ( bls message + secp message) of tipset
func (ms *MessageStore) MessagesForTipset(ts *types.TipSet) ([]types.ChainMsg, error) {
	bmsgs, err := ms.LoadTipSetMessage(context.TODO(), ts)
	if err != nil {
		return nil, err
	}

	var out []types.ChainMsg
	for _, bm := range bmsgs {
		out = append(out, bm.BlsMessages...)
		out = append(out, bm.SecpkMessages...)
	}

	return out, nil
}

// StoreMessage put message(include signed message and unsigned message) to database
func (ms *MessageStore) StoreMessage(message types.ChainMsg) (cid.Cid, error) {
	return cbor.NewCborStore(ms.bs).Put(context.TODO(), message)
}

// StoreTxMeta writes the secproot, blsroot block to the message store
func (ms *MessageStore) StoreTxMeta(ctx context.Context, meta types.MessageRoot) (cid.Cid, error) {
	return cbor.NewCborStore(ms.bs).Put(ctx, &meta)
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

// todo move to a more suitable position
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

// todo move to a more suitable position
func (ms *MessageStore) ComputeBaseFee(ctx context.Context, ts *types.TipSet, upgrade *config.ForkUpgradeConfig) (abi.TokenAmount, error) {
	zero := abi.NewTokenAmount(0)
	baseHeight := ts.Height()

	if upgrade.UpgradeBreezeHeight >= 0 && baseHeight > upgrade.UpgradeBreezeHeight && baseHeight < upgrade.UpgradeBreezeHeight+upgrade.BreezeGasTampingDuration {
		return abi.NewTokenAmount(100), nil
	}

	// totalLimit is sum of GasLimits of unique messages in a tipset
	totalLimit := int64(0)

	seen := make(map[cid.Cid]struct{})

	for _, b := range ts.Blocks() {
		secpMsgs, blsMsgs, err := ms.LoadMetaMessages(ctx, b.Messages)
		if err != nil {
			return zero, errors.Wrapf(err, "error getting messages for: %s", b.Cid())
		}

		for _, m := range blsMsgs {
			c := m.Cid()
			if _, ok := seen[c]; !ok {
				totalLimit += m.GasLimit
				seen[c] = struct{}{}
			}
		}
		for _, m := range secpMsgs {
			c := m.Cid()
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
	as := cbor.NewCborStore(bs)
	rectarr := adt.MakeEmptyArray(adt.WrapStore(context.TODO(), as))
	for i, receipt := range receipts {
		if err := rectarr.Set(uint64(i), &receipt); err != nil {
			return cid.Undef, errors.Wrapf(err, "failed to build receipts amt")
		}
	}
	return rectarr.Root()
}

func GetChainMsgRoot(ctx context.Context, messages []types.ChainMsg) (cid.Cid, error) {
	tmpbs := blockstoreutil.NewTemporary()
	tmpstore := adt.WrapStore(ctx, cbor.NewCborStore(tmpbs))

	arr := adt.MakeEmptyArray(tmpstore)

	for i, m := range messages {
		b, err := m.ToStorageBlock()
		if err != nil {
			return cid.Undef, err
		}
		k := cbg.CborCid(b.Cid())
		if err := arr.Set(uint64(i), &k); err != nil {
			return cid.Undef, errors.Wrap(err, "failed to put message")
		}
	}

	return arr.Root()
}

// computeMsgMeta computes the root CID of the combined arrays of message CIDs
// of both types (BLS and Secpk).
func ComputeMsgMeta(bs blockstore.Blockstore, bmsgCids, smsgCids []cid.Cid) (cid.Cid, error) {
	// block headers use adt0
	store := adt.WrapStore(context.TODO(), cbor.NewCborStore(bs))
	bmArr := adt.MakeEmptyArray(store)
	smArr := adt.MakeEmptyArray(store)

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

	mrcid, err := store.Put(store.Context(), &types.MessageRoot{
		BlsRoot:   bmroot,
		SecpkRoot: smroot,
	})
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to put msgmeta")
	}

	return mrcid, nil
}
