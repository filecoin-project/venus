package eth

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/venus-shared/actors"
	types2 "github.com/filecoin-project/venus/venus-shared/actors/types"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// The address used in messages to actors that have since been deleted.
//
// 0xff0000000000000000000000ffffffffffffffff
var revertedEthAddress types.EthAddress

func init() {
	revertedEthAddress[0] = 0xff
	for i := 20 - 8; i < 20; i++ {
		revertedEthAddress[i] = 0xff
	}
}

func getTipsetByBlockNumber(ctx context.Context, store *chain.Store, blkParam string, strict bool) (*types.TipSet, error) {
	if blkParam == "earliest" {
		return nil, fmt.Errorf("block param \"earliest\" is not supported")
	}

	head := store.GetHead()
	switch blkParam {
	case "pending":
		return head, nil
	case "latest":
		parent, err := store.GetTipSet(ctx, head.Parents())
		if err != nil {
			return nil, fmt.Errorf("cannot get parent tipset")
		}
		return parent, nil
	case "safe":
		latestHeight := head.Height() - 1
		safeHeight := latestHeight - types.SafeEpochDelay
		ts, err := store.GetTipSetByHeight(ctx, head, safeHeight, true)
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", safeHeight)
		}
		return ts, nil
	case "finalized":
		latestHeight := head.Height() - 1
		safeHeight := latestHeight - constants.Finality
		ts, err := store.GetTipSetByHeight(ctx, head, safeHeight, true)
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", safeHeight)
		}
		return ts, nil
	default:
		var num types.EthUint64
		err := num.UnmarshalJSON([]byte(`"` + blkParam + `"`))
		if err != nil {
			return nil, fmt.Errorf("cannot parse block number: %v", err)
		}
		if abi.ChainEpoch(num) > head.Height()-1 {
			return nil, fmt.Errorf("requested a future epoch (beyond 'latest')")
		}
		ts, err := store.GetTipSetByHeight(ctx, head, abi.ChainEpoch(num), true)
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", num)
		}
		if strict && ts.Height() != abi.ChainEpoch(num) {
			return nil, ErrNullRound
		}
		return ts, nil
	}
}

func getTipsetByEthBlockNumberOrHash(ctx context.Context, store *chain.Store, blkParam types.EthBlockNumberOrHash) (*types.TipSet, error) {
	head := store.GetHead()

	predefined := blkParam.PredefinedBlock
	if predefined != nil {
		if *predefined == "earliest" {
			return nil, fmt.Errorf("block param \"earliest\" is not supported")
		} else if *predefined == "pending" {
			return head, nil
		} else if *predefined == "latest" {
			parent, err := store.GetTipSet(ctx, head.Parents())
			if err != nil {
				return nil, fmt.Errorf("cannot get parent tipset")
			}
			return parent, nil
		} else {
			return nil, fmt.Errorf("unknown predefined block %s", *predefined)
		}
	}

	if blkParam.BlockNumber != nil {
		height := abi.ChainEpoch(*blkParam.BlockNumber)
		if height > head.Height()-1 {
			return nil, fmt.Errorf("requested a future epoch (beyond 'latest')")
		}
		ts, err := store.GetTipSetByHeight(ctx, head, height, true)
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", height)
		}
		return ts, nil
	}

	if blkParam.BlockHash != nil {
		ts, err := store.GetTipSetByCid(ctx, blkParam.BlockHash.ToCid())
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset by hash: %v", err)
		}

		// verify that the tipset is in the canonical chain
		if blkParam.RequireCanonical {
			// walk up the current chain (our head) until we reach ts.Height()
			walkTS, err := store.GetTipSetByHeight(ctx, head, ts.Height(), true)
			if err != nil {
				return nil, fmt.Errorf("cannot get tipset at height: %v", ts.Height())
			}

			// verify that it equals the expected tipset
			if !walkTS.Equals(ts) {
				return nil, fmt.Errorf("tipset is not canonical")
			}
		}

		return ts, nil
	}

	return nil, errors.New("invalid block param")
}

func ethCallToFilecoinMessage(_ context.Context, tx types.EthCall) (*types.Message, error) {
	var from address.Address
	if tx.From == nil || *tx.From == (types.EthAddress{}) {
		var err error
		from, err = (types.EthAddress{}).ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to construct the ethereum system address: %w", err)
		}
	} else {
		// The from address must be translatable to an f4 address.
		var err error
		from, err = tx.From.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to translate sender address (%s): %w", tx.From.String(), err)
		}
		if p := from.Protocol(); p != address.Delegated {
			return nil, fmt.Errorf("expected a class 4 address, got: %d: %w", p, err)
		}
	}

	var params []byte
	if len(tx.Data) > 0 {
		initcode := abi.CborBytes(tx.Data)
		params2, err := actors.SerializeParams(&initcode)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize params: %w", err)
		}
		params = params2
	}

	var to address.Address
	var method abi.MethodNum
	if tx.To == nil {
		// this is a contract creation
		to = builtin.EthereumAddressManagerActorAddr
		method = builtin.MethodsEAM.CreateExternal
	} else {
		addr, err := tx.To.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
		}
		to = addr

		method = builtin.MethodsEVM.InvokeContract
	}

	return &types.Message{
		From:       from,
		To:         to,
		Value:      big.Int(tx.Value),
		Method:     method,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}, nil
}

func newEthBlockFromFilecoinTipSet(ctx context.Context, ts *types.TipSet, fullTxInfo bool, ms *chain.MessageStore, stmgr *statemanger.Stmgr) (types.EthBlock, error) {
	parentKeyCid, err := ts.Parents().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	parentBlkHash, err := types.EthHashFromCid(parentKeyCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	bn := types.EthUint64(ts.Height())

	tsk := ts.Key()
	blkCid, err := tsk.Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	blkHash, err := types.EthHashFromCid(blkCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	stRoot, msgs, rcpts, err := executeTipset(ctx, ts, ms, stmgr)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("failed to retrieve messages and receipts: %w", err)
	}

	st, err := stmgr.TipsetState(ctx, ts)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("failed to load state-tree root %q: %w", stRoot, err)
	}

	block := types.NewEthBlock(len(msgs) > 0, len(ts.Blocks()))

	gasUsed := int64(0)
	for i, msg := range msgs {
		rcpt := rcpts[i]
		ti := types.EthUint64(i)
		gasUsed += rcpt.GasUsed
		var smsg *types.SignedMessage
		switch msg := msg.(type) {
		case *types.SignedMessage:
			smsg = msg
		case *types.Message:
			smsg = &types.SignedMessage{
				Message: *msg,
				Signature: crypto.Signature{
					Type: crypto.SigTypeBLS,
				},
			}
		default:
			return types.EthBlock{}, fmt.Errorf("failed to get signed msg %s: %w", msg.Cid(), err)
		}
		tx, err := newEthTxFromSignedMessage(ctx, smsg, st)
		if err != nil {
			return types.EthBlock{}, fmt.Errorf("failed to convert msg to ethTx: %w", err)
		}

		tx.BlockHash = &blkHash
		tx.BlockNumber = &bn
		tx.TransactionIndex = &ti

		if fullTxInfo {
			block.Transactions = append(block.Transactions, tx)
		} else {
			block.Transactions = append(block.Transactions, tx.Hash.String())
		}
	}

	block.Hash = blkHash
	block.Number = bn
	block.ParentHash = parentBlkHash
	block.Timestamp = types.EthUint64(ts.Blocks()[0].Timestamp)
	block.BaseFeePerGas = types.EthBigInt{Int: ts.Blocks()[0].ParentBaseFee.Int}
	block.GasUsed = types.EthUint64(gasUsed)
	return block, nil
}

func messagesAndReceipts(ctx context.Context, ts *types.TipSet, ms *chain.MessageStore, stmgr *statemanger.Stmgr) ([]types.ChainMsg, []types.MessageReceipt, error) {
	msgs, err := ms.MessagesForTipset(ts)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	_, rcptRoot, err := stmgr.RunStateTransition(ctx, ts, nil, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compute state: %w", err)
	}

	rcpts, err := ms.LoadReceipts(ctx, rcptRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading receipts for tipset: %v: %w", ts, err)
	}

	if len(msgs) != len(rcpts) {
		return nil, nil, fmt.Errorf("receipts and message array lengths didn't match for tipset: %v: %w", ts, err)
	}

	return msgs, rcpts, nil
}

func executeTipset(ctx context.Context, ts *types.TipSet, ms *chain.MessageStore, stmgr *statemanger.Stmgr) (cid.Cid, []types.ChainMsg, []types.MessageReceipt, error) {
	msgs, err := ms.MessagesForTipset(ts)
	if err != nil {
		return cid.Undef, nil, nil, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	stRoot, rcptRoot, err := stmgr.RunStateTransition(ctx, ts, nil, false)
	if err != nil {
		return cid.Undef, nil, nil, fmt.Errorf("failed to compute tipset state: %w", err)
	}

	rcpts, err := ms.LoadReceipts(ctx, rcptRoot)
	if err != nil {
		return cid.Undef, nil, nil, fmt.Errorf("error loading receipts for tipset: %v: %w", ts, err)
	}

	if len(msgs) != len(rcpts) {
		return cid.Undef, nil, nil, fmt.Errorf("receipts and message array lengths didn't match for tipset: %v: %w", ts, err)
	}

	return stRoot, msgs, rcpts, nil
}

const errorFunctionSelector = "\x08\xc3\x79\xa0" // Error(string)
const panicFunctionSelector = "\x4e\x48\x7b\x71" // Panic(uint256)
// Eth ABI (solidity) panic codes.
var panicErrorCodes = map[uint64]string{
	0x00: "Panic()",
	0x01: "Assert()",
	0x11: "ArithmeticOverflow()",
	0x12: "DivideByZero()",
	0x21: "InvalidEnumVariant()",
	0x22: "InvalidStorageArray()",
	0x31: "PopEmptyArray()",
	0x32: "ArrayIndexOutOfBounds()",
	0x41: "OutOfMemory()",
	0x51: "CalledUninitializedFunction()",
}

// Parse an ABI encoded revert reason. This reason should be encoded as if it were the parameters to
// an `Error(string)` function call.
//
// See https://docs.soliditylang.org/en/latest/control-structures.html#panic-via-assert-and-error-via-require
func parseEthRevert(ret []byte) string {
	if len(ret) == 0 {
		return "none"
	}
	var cbytes abi.CborBytes
	if err := cbytes.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
		return "ERROR: revert reason is not cbor encoded bytes"
	}
	if len(cbytes) == 0 {
		return "none"
	}
	// If it's not long enough to contain an ABI encoded response, return immediately.
	if len(cbytes) < 4+32 {
		return types.EthBytes(cbytes).String()
	}
	switch string(cbytes[:4]) {
	case panicFunctionSelector:
		cbytes := cbytes[4 : 4+32]
		// Read the and check the code.
		code, err := types.EthUint64FromBytes(cbytes)
		if err != nil {
			// If it's too big, just return the raw value.
			codeInt := big.PositiveFromUnsignedBytes(cbytes)
			return fmt.Sprintf("Panic(%s)", types.EthBigInt(codeInt).String())
		}
		if s, ok := panicErrorCodes[uint64(code)]; ok {
			return s
		}
		return fmt.Sprintf("Panic(0x%x)", code)
	case errorFunctionSelector:
		cbytes := cbytes[4:]
		cbytesLen := types.EthUint64(len(cbytes))
		// Read the and check the offset.
		offset, err := types.EthUint64FromBytes(cbytes[:32])
		if err != nil {
			break
		}
		if cbytesLen < offset {
			break
		}

		// Read and check the length.
		if cbytesLen-offset < 32 {
			break
		}
		start := offset + 32
		length, err := types.EthUint64FromBytes(cbytes[offset : offset+32])
		if err != nil {
			break
		}
		if cbytesLen-start < length {
			break
		}
		// Slice the error message.
		return fmt.Sprintf("Error(%s)", cbytes[start:start+length])
	}
	return types.EthBytes(cbytes).String()
}

// lookupEthAddress makes its best effort at finding the Ethereum address for a
// Filecoin address. It does the following:
//
//  1. If the supplied address is an f410 address, we return its payload as the EthAddress.
//  2. Otherwise (f0, f1, f2, f3), we look up the actor on the state tree. If it has a delegated address, we return it if it's f410 address.
//  3. Otherwise, we fall back to returning a masked ID Ethereum address. If the supplied address is an f0 address, we
//     use that ID to form the masked ID address.
//  4. Otherwise, we fetch the actor's ID from the state tree and form the masked ID with it.
//
// If the actor doesn't exist in the state-tree but we have its ID, we use a masked ID address. It could have been deleted.
func lookupEthAddress(ctx context.Context, addr address.Address, state tree.Tree) (types.EthAddress, error) {
	// Attempt to convert directly, if it's an f4 address.
	ethAddr, err := types.EthAddressFromFilecoinAddress(addr)
	if err == nil && !ethAddr.IsMaskedID() {
		return ethAddr, nil
	}

	// Otherwise, resolve the ID addr.
	idAddr, err := state.LookupID(addr)
	if err != nil {
		return types.EthAddress{}, err
	}

	// revive:disable:empty-block easier to grok when the cases are explicit

	// Lookup on the target actor and try to get an f410 address.
	if actor, found, err := state.GetActor(ctx, idAddr); errors.Is(err, types.ErrActorNotFound) && !found {
		// Not found -> use a masked ID address
	} else if err != nil {
		// Any other error -> fail.
		return types.EthAddress{}, err
	} else if actor.DelegatedAddress == nil {
		// No delegated address -> use masked ID address.
	} else if ethAddr, err := types.EthAddressFromFilecoinAddress(*actor.DelegatedAddress); err == nil && !ethAddr.IsMaskedID() {
		// Conversable into an eth address, use it.
		return ethAddr, nil
	}

	// Otherwise, use the masked address.
	return types.EthAddressFromFilecoinAddress(idAddr)
}

func ethTxHashFromMessageCid(ctx context.Context, c cid.Cid, ms *chain.MessageStore) (types.EthHash, error) {
	smsg, err := ms.LoadSignedMessage(ctx, c)
	if err == nil {
		// This is an Eth Tx, Secp message, Or BLS message in the mpool
		return ethTxHashFromSignedMessage(smsg)
	}

	_, err = ms.LoadMessage(ctx, c)
	if err == nil {
		// This is a BLS message
		return types.EthHashFromCid(c)
	}

	return types.EthHashFromCid(c)
}

func ethTxHashFromSignedMessage(smsg *types.SignedMessage) (types.EthHash, error) {
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		tx, err := types.EthTransactionFromSignedFilecoinMessage(smsg)
		if err != nil {
			return types.EthHash{}, fmt.Errorf("failed to convert from signed message: %w", err)
		}

		return tx.TxHash()
	} else if smsg.Signature.Type == crypto.SigTypeSecp256k1 {
		return types.EthHashFromCid(smsg.Cid())
	}
	// else BLS message
	return types.EthHashFromCid(smsg.Message.Cid())
}

func newEthTxFromSignedMessage(ctx context.Context, smsg *types.SignedMessage, state tree.Tree) (types.EthTx, error) {
	var tx types.EthTx
	var err error

	// This is an eth tx
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		ethTx, err := types.EthTransactionFromSignedFilecoinMessage(smsg)
		if err != nil {
			return types.EthTx{}, fmt.Errorf("failed to convert from signed message: %w", err)
		}
		tx, err = ethTx.ToEthTx(smsg)
		if err != nil {
			return types.EthTx{}, fmt.Errorf("failed to convert from signed message: %w", err)
		}
	} else if smsg.Signature.Type == crypto.SigTypeSecp256k1 { // Secp Filecoin Message
		tx, err = ethTxFromNativeMessage(ctx, smsg.VMMessage(), state)
		if err != nil {
			return types.EthTx{}, err
		}
		tx.Hash, err = types.EthHashFromCid(smsg.Cid())
		if err != nil {
			return types.EthTx{}, err
		}
	} else { // BLS Filecoin message
		tx, err = ethTxFromNativeMessage(ctx, smsg.VMMessage(), state)
		if err != nil {
			return types.EthTx{}, err
		}
		tx.Hash, err = types.EthHashFromCid(smsg.Message.Cid())
		if err != nil {
			return types.EthTx{}, err
		}
	}

	return tx, nil
}

func parseEthTopics(topics types.EthTopicSpec) (map[string][][]byte, error) {
	keys := map[string][][]byte{}
	for idx, vals := range topics {
		if len(vals) == 0 {
			continue
		}
		// Ethereum topics are emitted using `LOG{0..4}` opcodes resulting in topics1..4
		key := fmt.Sprintf("t%d", idx+1)
		for _, v := range vals {
			v := v // copy the ethhash to avoid repeatedly referencing the same one.
			keys[key] = append(keys[key], v[:])
		}
	}
	return keys, nil
}

// Convert a native message to an eth transaction.
//
//   - The state-tree must be from after the message was applied (ideally the following tipset).
//   - In some cases, the "to" address may be `0xff0000000000000000000000ffffffffffffffff`. This
//     means that the "to" address has not been assigned in the passed state-tree and can only
//     happen if the transaction reverted.
//
// ethTxFromNativeMessage does NOT populate:
// - BlockHash
// - BlockNumber
// - TransactionIndex
// - Hash
func ethTxFromNativeMessage(ctx context.Context, msg *types.Message, state tree.Tree) (types.EthTx, error) {
	// Lookup the from address. This must succeed.
	from, err := lookupEthAddress(ctx, msg.From, state)
	if err != nil {
		return types.EthTx{}, fmt.Errorf("failed to lookup sender address %s when converting a native message to an eth txn: %w", msg.From, err)
	}
	// Lookup the to address. If the recipient doesn't exist, we replace the address with a
	// known sentinel address.
	to, err := lookupEthAddress(ctx, msg.To, state)
	if err != nil {
		if !errors.Is(err, types.ErrActorNotFound) {
			return types.EthTx{}, fmt.Errorf("failed to lookup receiver address %s when converting a native message to an eth txn: %w", msg.To, err)
		}
		to = revertedEthAddress
	}

	// For empty, we use "0" as the codec. Otherwise, we use CBOR for message
	// parameters.
	var codec uint64
	if len(msg.Params) > 0 {
		codec = uint64(multicodec.Cbor)
	}

	maxFeePerGas := types.EthBigInt(msg.GasFeeCap)
	maxPriorityFeePerGas := types.EthBigInt(msg.GasPremium)

	// We decode as a native call first.
	ethTx := types.EthTx{
		To:                   &to,
		From:                 from,
		Input:                encodeFilecoinParamsAsABI(msg.Method, codec, msg.Params),
		Nonce:                types.EthUint64(msg.Nonce),
		ChainID:              types.EthUint64(types2.Eip155ChainID),
		Value:                types.EthBigInt(msg.Value),
		Type:                 types.EIP1559TxType,
		Gas:                  types.EthUint64(msg.GasLimit),
		MaxFeePerGas:         &maxFeePerGas,
		MaxPriorityFeePerGas: &maxPriorityFeePerGas,
		AccessList:           []types.EthHash{},
	}

	// Then we try to see if it's "special". If we fail, we ignore the error and keep treating
	// it as a native message. Unfortunately, the user is free to send garbage that may not
	// properly decode.
	if msg.Method == builtin.MethodsEVM.InvokeContract {
		// try to decode it as a contract invocation first.
		if inp, err := decodePayload(msg.Params, codec); err == nil {
			ethTx.Input = []byte(inp)
		}
	} else if msg.To == builtin.EthereumAddressManagerActorAddr && msg.Method == builtin.MethodsEAM.CreateExternal {
		// Then, try to decode it as a contract deployment from an EOA.
		if inp, err := decodePayload(msg.Params, codec); err == nil {
			ethTx.Input = []byte(inp)
			ethTx.To = nil
		}
	}

	return ethTx, nil
}

// newEthTxFromMessageLookup creates an ethereum transaction from filecoin message lookup. If a negative txIdx is passed
// into the function, it looks up the transaction index of the message in the tipset, otherwise it uses the txIdx passed into the
// function
func newEthTxFromMessageLookup(ctx context.Context, msgLookup *types.MsgLookup, txIdx int, ms *chain.MessageStore, cr *chain.Store) (types.EthTx, error) {
	if msgLookup == nil {
		return types.EthTx{}, fmt.Errorf("msg does not exist")
	}

	ts, err := cr.GetTipSet(ctx, msgLookup.TipSet)
	if err != nil {
		return types.EthTx{}, err
	}

	// This tx is located in the parent tipset
	parentTS, err := cr.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return types.EthTx{}, err
	}

	parentTSCid, err := parentTS.Key().Cid()
	if err != nil {
		return types.EthTx{}, err
	}

	// lookup the transactionIndex
	if txIdx < 0 {
		msgs, err := ms.MessagesForTipset(parentTS)
		if err != nil {
			return types.EthTx{}, err
		}
		for i, msg := range msgs {
			if msg.Cid() == msgLookup.Message {
				txIdx = i
				break
			}
		}
		if txIdx < 0 {
			return types.EthTx{}, fmt.Errorf("cannot find the msg in the tipset")
		}
	}

	state, err := cr.GetTipSetState(ctx, parentTS)
	if err != nil {
		return types.EthTx{}, fmt.Errorf("failed to load message state tree: %w", err)
	}

	return newEthTx(ctx, state, parentTS.Height(), parentTSCid, msgLookup.Message, txIdx, ms)
}

func newEthTx(ctx context.Context,
	state tree.Tree,
	blockHeight abi.ChainEpoch,
	msgTSCid cid.Cid,
	msgCid cid.Cid,
	txIdx int,
	ms *chain.MessageStore,
) (types.EthTx, error) {
	smsg, err := getSignedMessage(ctx, ms, msgCid)
	if err != nil {
		return types.EthTx{}, fmt.Errorf("failed to get signed msg: %w", err)
	}

	tx, err := newEthTxFromSignedMessage(ctx, smsg, state)
	if err != nil {
		return types.EthTx{}, err
	}

	var (
		bn = types.EthUint64(blockHeight)
		ti = types.EthUint64(txIdx)
	)

	blkHash, err := types.EthHashFromCid(msgTSCid)
	if err != nil {
		return types.EthTx{}, err
	}

	tx.ChainID = types.EthUint64(types2.Eip155ChainID)
	tx.BlockHash = &blkHash
	tx.BlockNumber = &bn
	tx.TransactionIndex = &ti
	return tx, nil
}

func newEthTxReceipt(ctx context.Context,
	tx types.EthTx,
	baseFee big.Int,
	msgReceipt types.MessageReceipt,
	ev *ethEventAPI,
) (types.EthTxReceipt, error) {
	var (
		transactionIndex types.EthUint64
		blockHash        types.EthHash
		blockNumber      types.EthUint64
	)

	if tx.TransactionIndex != nil {
		transactionIndex = *tx.TransactionIndex
	}
	if tx.BlockHash != nil {
		blockHash = *tx.BlockHash
	}
	if tx.BlockNumber != nil {
		blockNumber = *tx.BlockNumber
	}

	receipt := types.EthTxReceipt{
		TransactionHash:  tx.Hash,
		From:             tx.From,
		To:               tx.To,
		TransactionIndex: transactionIndex,
		BlockHash:        blockHash,
		BlockNumber:      blockNumber,
		Type:             tx.Type,
		Logs:             []types.EthLog{}, // empty log array is compulsory when no logs, or libraries like ethers.js break
		LogsBloom:        types.EmptyEthBloom[:],
	}

	if msgReceipt.ExitCode.IsSuccess() {
		receipt.Status = 1
	}
	if msgReceipt.ExitCode.IsError() {
		receipt.Status = 0
	}

	receipt.GasUsed = types.EthUint64(msgReceipt.GasUsed)

	// TODO: handle CumulativeGasUsed
	receipt.CumulativeGasUsed = types.EmptyEthInt

	gasFeeCap, err := tx.GasFeeCap()
	if err != nil {
		return types.EthTxReceipt{}, fmt.Errorf("failed to get gas fee cap: %w", err)
	}
	gasPremium, err := tx.GasPremium()
	if err != nil {
		return types.EthTxReceipt{}, fmt.Errorf("failed to get gas premium: %w", err)
	}

	gasOutputs := gas.ComputeGasOutputs(msgReceipt.GasUsed, int64(tx.Gas), baseFee, big.Int(gasFeeCap),
		big.Int(gasPremium), true)
	totalSpent := big.Sum(gasOutputs.BaseFeeBurn, gasOutputs.MinerTip, gasOutputs.OverEstimationBurn)

	effectiveGasPrice := big.Zero()
	if msgReceipt.GasUsed > 0 {
		effectiveGasPrice = big.Div(totalSpent, big.NewInt(msgReceipt.GasUsed))
	}
	receipt.EffectiveGasPrice = types.EthBigInt(effectiveGasPrice)

	if receipt.To == nil && msgReceipt.ExitCode.IsSuccess() {
		// Create and Create2 return the same things.
		var ret eam.CreateExternalReturn
		if err := ret.UnmarshalCBOR(bytes.NewReader(msgReceipt.Return)); err != nil {
			return types.EthTxReceipt{}, fmt.Errorf("failed to parse contract creation result: %w", err)
		}
		addr := types.EthAddress(ret.EthAddress)
		receipt.ContractAddress = &addr
	}

	if rct := msgReceipt; rct.EventsRoot != nil {
		logs, err := ev.getEthLogsForBlockAndTransaction(ctx, &blockHash, tx.Hash)
		if err != nil {
			return types.EthTxReceipt{}, fmt.Errorf("failed to get eth logs for block and transaction: %w", err)
		}
		if len(logs) > 0 {
			receipt.Logs = logs
		}
	}

	for _, log := range receipt.Logs {
		for _, topic := range log.Topics {
			types.EthBloomSet(receipt.LogsBloom, topic[:])
		}
		types.EthBloomSet(receipt.LogsBloom, log.Address[:])
	}

	return receipt, nil
}

func encodeFilecoinParamsAsABI(method abi.MethodNum, codec uint64, params []byte) []byte {
	buf := []byte{0x86, 0x8e, 0x10, 0xc4} // Native method selector.
	return append(buf, encodeAsABIHelper(uint64(method), codec, params)...)
}

func encodeFilecoinReturnAsABI(exitCode exitcode.ExitCode, codec uint64, data []byte) []byte {
	return encodeAsABIHelper(uint64(exitCode), codec, data)
}

// Format 2 numbers followed by an arbitrary byte array as solidity ABI. Both our native
// inputs/outputs follow the same pattern, so we can reuse this code.
func encodeAsABIHelper(param1 uint64, param2 uint64, data []byte) []byte {
	const evmWordSize = 32

	// The first two params are "static" numbers. Then, we record the offset of the "data" arg,
	// then, at that offset, we record the length of the data.
	//
	// In practice, this means we have 4 256-bit words back to back where the third arg (the
	// offset) is _always_ '32*3'.
	staticArgs := []uint64{param1, param2, evmWordSize * 3, uint64(len(data))}
	// We always pad out to the next EVM "word" (32 bytes).
	totalWords := len(staticArgs) + (len(data) / evmWordSize)
	if len(data)%evmWordSize != 0 {
		totalWords++
	}
	len := totalWords * evmWordSize
	buf := make([]byte, len)
	offset := 0
	// Below, we use copy instead of "appending" to preserve all the zero padding.
	for _, arg := range staticArgs {
		// Write each "arg" into the last 8 bytes of each 32 byte word.
		offset += evmWordSize
		start := offset - 8
		binary.BigEndian.PutUint64(buf[start:offset], arg)
	}

	// Finally, we copy in the data.
	copy(buf[offset:], data)

	return buf
}
