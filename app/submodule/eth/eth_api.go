package eth

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	"github.com/filecoin-project/go-state-types/builtin/v10/evm"
	init10 "github.com/filecoin-project/go-state-types/builtin/v10/init"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors"
	builtinactors "github.com/filecoin-project/venus/venus-shared/actors/builtin"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
)

type ethAPI struct {
	em    *EthSubModule
	chain v1.IChain
	mpool v1.IMessagePool
}

func (a *ethAPI) EthBlockNumber(ctx context.Context) (types.EthUint64, error) {
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthUint64(0), err
	}
	return types.EthUint64(head.Height()), nil
}

func (a *ethAPI) EthAccounts(context.Context) ([]types.EthAddress, error) {
	// The lotus node is not expected to hold manage accounts, so we'll always return an empty array
	return []types.EthAddress{}, nil
}

func (a *ethAPI) countTipsetMsgs(ctx context.Context, ts *types.TipSet) (int, error) {
	msgs, err := a.em.chainModule.MessageStore.LoadTipSetMessage(ctx, ts)
	if err != nil {
		return 0, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	return len(msgs), nil
}

func (a *ethAPI) EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthUint64) (types.EthUint64, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(blkNum), false)
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}

	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthUint64(count), err
}

func (a *ethAPI) EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthUint64, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthUint64(count), err
}

func (a *ethAPI) EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	return a.newEthBlockFromFilecoinTipSet(ctx, ts, fullTxInfo)
}

func (a *ethAPI) EthGetBlockByNumber(ctx context.Context, blkNum string, fullTxInfo bool) (types.EthBlock, error) {
	typ, num, err := types.ParseBlkNumOption(blkNum)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("cannot parse block number: %v", err)
	}
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("failed to got head %v", err)
	}
	switch typ {
	case types.BlkNumLatest:
		num = types.EthUint64(head.Height()) - 1
	case types.BlkNumPending:
		num = types.EthUint64(head.Height())
	}

	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(num), false)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	return a.newEthBlockFromFilecoinTipSet(ctx, ts, fullTxInfo)
}

func (a *ethAPI) EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error) {
	// Ethereum's behavior is to return null when the txHash is invalid, so we use nil to check if txHash is valid
	if txHash == nil {
		return nil, nil
	}

	cid := txHash.ToCid()

	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, cid, constants.LookbackNoLimit, true)
	if err != nil {
		return nil, nil
	}

	tx, err := a.ethTxFromFilecoinMessageLookup(ctx, msgLookup)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (a *ethAPI) EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkParam string) (types.EthUint64, error) {
	addr, err := sender.ToFilecoinAddress()
	if err != nil {
		return types.EthUint64(0), err
	}
	nonce, err := a.mpool.MpoolGetNonce(ctx, addr)
	if err != nil {
		return types.EthUint64(0), err
	}
	return types.EthUint64(nonce), nil
}

// todo: 实现 StateReplay 接口
func (a *ethAPI) EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error) {
	// cid := txHash.ToCid()

	// msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, cid, constants.LookbackNoLimit, true)
	// if err != nil {
	// 	return types.EthTxReceipt{}, err
	// }

	// tx, err := a.ethTxFromFilecoinMessageLookup(ctx, msgLookup)
	// if err != nil {
	// 	return types.EthTxReceipt{}, err
	// }

	// replay, err := a.chain.StateReplay(ctx, types.EmptyTSK, cid)
	// if err != nil {
	// 	return types.EthTxReceipt{}, err
	// }

	// receipt, err := types.NewEthTxReceipt(tx, msgLookup, replay)
	// if err != nil {
	// 	return types.EthTxReceipt{}, err
	// }
	// return receipt, nil
	return &types.EthTxReceipt{}, nil
}

func (a *ethAPI) EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, nil
}

func (a *ethAPI) EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, nil
}

// EthGetCode returns string value of the compiled bytecode
func (a *ethAPI) EthGetCode(ctx context.Context, ethAddr types.EthAddress) (types.EthBytes, error) {
	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	// use the system actor as the caller
	from, err := address.NewIDAddress(0)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system sender address: %w", err)
	}
	msg := &types.Message{
		From:       from,
		To:         to,
		Value:      big.Zero(),
		Method:     builtintypes.MethodsEVM.GetBytecode,
		Params:     nil,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}

	// Try calling until we find a height with no migration.
	var res *vm.Ret
	for {
		res, err = a.em.chainModule.Stmgr.Call(ctx, msg, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("call failed: %w", err)
	}

	if res.Receipt.ExitCode.IsError() {
		return nil, fmt.Errorf("message execution failed: exit %s, reason: %s", &res.Receipt.ExitCode, res.ActorErr)
	}

	var bytecodeCid cbg.CborCid
	if err := bytecodeCid.UnmarshalCBOR(bytes.NewReader(res.Receipt.Return)); err != nil {
		return nil, fmt.Errorf("failed to decode EVM bytecode CID: %w", err)
	}

	blk, err := a.em.chainModule.ChainReader.Blockstore().Get(ctx, cid.Cid(bytecodeCid))
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM bytecode: %w", err)
	}

	return blk.RawData(), nil
}

func (a *ethAPI) EthGetStorageAt(ctx context.Context, ethAddr types.EthAddress, position types.EthBytes, blkParam string) (types.EthBytes, error) {
	l := len(position)
	if l > 32 {
		return nil, fmt.Errorf("supplied storage key is too long")
	}

	// pad with zero bytes if smaller than 32 bytes
	position = append(make([]byte, 32-l), position...)

	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	// use the system actor as the caller
	from, err := address.NewIDAddress(0)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system sender address: %w", err)
	}

	// TODO super duper hack (raulk). The EVM runtime actor uses the U256 parameter type in
	//  GetStorageAtParams, which serializes as a hex-encoded string. It should serialize
	//  as bytes. We didn't get to fix in time for Iron, so for now we just pass
	//  through the hex-encoded value passed through the Eth JSON-RPC API, by remarshalling it.
	//  We don't fix this at origin (builtin-actors) because we are not updating the bundle
	//  for Iron.
	tmp, err := position.MarshalJSON()
	if err != nil {
		panic(err)
	}
	params, err := actors.SerializeParams(&evm.GetStorageAtParams{
		StorageKey: tmp[1 : len(tmp)-1], // TODO strip the JSON-encoding quotes -- yuck
	})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}

	msg := &types.Message{
		From:       from,
		To:         to,
		Value:      big.Zero(),
		Method:     builtintypes.MethodsEVM.GetStorageAt,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}

	// Try calling until we find a height with no migration.
	var res *vm.Ret
	for {
		res, err = a.em.chainModule.Stmgr.Call(ctx, msg, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("call failed: %w", err)
	}

	return res.Receipt.Return, nil
}

func (a *ethAPI) EthGetBalance(ctx context.Context, address types.EthAddress, blkParam string) (types.EthBigInt, error) {
	filAddr, err := address.ToFilecoinAddress()
	if err != nil {
		return types.EthBigInt{}, err
	}

	actor, err := a.chain.StateGetActor(ctx, filAddr, types.EmptyTSK)
	if err != nil {
		return types.EthBigInt{}, err
	}

	return types.EthBigInt{Int: actor.Balance.Int}, nil
}

func (a *ethAPI) EthChainId(ctx context.Context) (types.EthUint64, error) {
	return types.EthUint64(types.Eip155ChainID), nil
}

func (a *ethAPI) NetVersion(ctx context.Context) (string, error) {
	// Note that networkId is not encoded in hex
	nv, err := a.chain.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(uint64(nv), 10), nil
}

func (a *ethAPI) NetListening(ctx context.Context) (bool, error) {
	return true, nil
}

func (a *ethAPI) EthProtocolVersion(ctx context.Context) (types.EthUint64, error) {
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthUint64(0), err
	}

	return types.EthUint64(a.em.chainModule.Fork.GetNetworkVersion(ctx, head.Height())), nil
}

func (a *ethAPI) EthMaxPriorityFeePerGas(ctx context.Context) (types.EthBigInt, error) {
	gasPremium, err := a.mpool.GasEstimateGasPremium(ctx, 0, builtin.SystemActorAddr, 10000, types.EmptyTSK)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}
	return types.EthBigInt(gasPremium), nil
}

func (a *ethAPI) EthGasPrice(ctx context.Context) (types.EthBigInt, error) {
	// According to Geth's implementation, eth_gasPrice should return base + tip
	// Ref: https://github.com/ethereum/pm/issues/328#issuecomment-853234014

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}
	baseFee := ts.Blocks()[0].ParentBaseFee

	premium, err := a.EthMaxPriorityFeePerGas(ctx)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}

	gasPrice := big.Add(baseFee, big.Int(premium))
	return types.EthBigInt(gasPrice), nil
}

func (a *ethAPI) EthSendRawTransaction(ctx context.Context, rawTx types.EthBytes) (types.EthHash, error) {
	txArgs, err := types.ParseEthTxArgs(rawTx)
	if err != nil {
		return types.EmptyEthHash, err
	}

	smsg, err := txArgs.ToSignedMessage()
	if err != nil {
		return types.EmptyEthHash, err
	}

	_, err = a.chain.StateGetActor(ctx, smsg.Message.To, types.EmptyTSK)
	if err != nil {
		// if actor does not exist on chain yet, set the method to 0 because
		// embryos only implement method 0
		smsg.Message.Method = builtin.MethodSend
	}

	cid, err := a.mpool.MpoolPush(ctx, smsg)
	if err != nil {
		return types.EmptyEthHash, err
	}
	return types.NewEthHashFromCid(cid)
}

func (a *ethAPI) applyEvmMsg(ctx context.Context, tx types.EthCall) (*types.InvocResult, error) {
	// FIXME: this is a workaround, remove this when f4 address is ready
	var from address.Address
	var err error
	if tx.From[0] == 0xff && tx.From[1] == 0 && tx.From[2] == 0 {
		addr, err := tx.From.ToFilecoinAddress()
		if err != nil {
			return nil, err
		}
		from = addr
	} else {
		id := uint64(100)
		for ; id < 300; id++ {
			idAddr, err := address.NewIDAddress(id)
			if err != nil {
				return nil, err
			}
			from = idAddr
			act, err := a.chain.StateGetActor(ctx, idAddr, types.EmptyTSK)
			if err != nil {
				return nil, err
			}
			if builtinactors.IsAccountActor(act.Code) {
				break
			}
		}
		if id == 300 {
			return nil, fmt.Errorf("cannot find a dummy account")
		}
	}

	var params []byte
	var to address.Address
	if tx.To == nil {
		to = builtintypes.InitActorAddr
		constructorParams, err := actors.SerializeParams(&evm.ConstructorParams{
			Initcode: tx.Data,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to serialize constructor params: %w", err)
		}

		evmActorCid, ok := actors.GetActorCodeID(actorstypes.Version8, "evm")
		if !ok {
			return nil, fmt.Errorf("failed to lookup evm actor code CID")
		}

		params, err = actors.SerializeParams(&init10.ExecParams{
			CodeCID:           evmActorCid,
			ConstructorParams: constructorParams,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to serialize init actor exec params: %w", err)
		}
	} else {
		addr, err := tx.To.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
		}
		to = addr
		var buf bytes.Buffer
		if err := cbg.WriteByteArray(&buf, tx.Data); err != nil {
			return nil, fmt.Errorf("failed to encode tx input into a cbor byte-string %v", err)
		}
		params = buf.Bytes()
	}

	msg := &types.Message{
		From:       from,
		To:         to,
		Value:      big.Int(tx.Value),
		Method:     builtintypes.MethodsEVM.InvokeContract,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}
	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}

	// Try calling until we find a height with no migration.
	var res *vmcontext.Ret
	for {
		res, err = a.em.chainModule.Stmgr.CallWithGas(ctx, msg, []types.ChainMsg{}, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("CallWithGas failed: %w", err)
	}
	if res.Receipt.ExitCode.IsError() {
		return nil, fmt.Errorf("message execution failed: exit %s, reason: %s", res.Receipt.ExitCode, res.ActorErr)
	}
	var errStr string
	if res.ActorErr != nil {
		errStr = res.ActorErr.Error()
	}
	return &types.InvocResult{
		MsgCid:         msg.Cid(),
		Msg:            msg,
		MsgRct:         &res.Receipt,
		ExecutionTrace: res.GasTracker.ExecutionTrace,
		Error:          errStr,
		Duration:       res.Duration,
	}, nil
}

func (a *ethAPI) EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error) {
	invokeResult, err := a.applyEvmMsg(ctx, tx)
	if err != nil {
		return types.EthUint64(0), err
	}
	ret := invokeResult.MsgRct.GasUsed
	return types.EthUint64(ret), nil
}

func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam string) (types.EthBytes, error) {
	invokeResult, err := a.applyEvmMsg(ctx, tx)
	if err != nil {
		return nil, err
	}
	if len(invokeResult.MsgRct.Return) > 0 {
		return cbg.ReadByteArray(bytes.NewReader(invokeResult.MsgRct.Return), uint64(len(invokeResult.MsgRct.Return)))
	}
	return types.EthBytes{}, nil
}

func (a *ethAPI) newEthBlockFromFilecoinTipSet(ctx context.Context, ts *types.TipSet, fullTxInfo bool) (types.EthBlock, error) {
	parent, err := a.chain.ChainGetTipSet(ctx, ts.Parents())
	if err != nil {
		return types.EthBlock{}, err
	}
	parentKeyCid, err := parent.Key().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	parentBlkHash, err := types.NewEthHashFromCid(parentKeyCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	blkCid, err := ts.Key().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	blkHash, err := types.NewEthHashFromCid(blkCid)

	msgs, err := a.em.chainModule.MessageStore.MessagesForTipset(ts)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	block := types.NewEthBlock()

	// this seems to be a very expensive way to get gasUsed of the block. may need to find an efficient way to do it
	gasUsed := int64(0)
	for _, msg := range msgs {
		msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, msg.Cid(), constants.LookbackNoLimit, true)
		if err != nil {
			return types.EthBlock{}, nil
		}
		gasUsed += msgLookup.Receipt.GasUsed

		if fullTxInfo {
			tx, err := a.ethTxFromFilecoinMessageLookup(ctx, msgLookup)
			if err != nil {
				return types.EthBlock{}, nil
			}
			block.Transactions = append(block.Transactions, tx)
		} else {
			hash, err := types.NewEthHashFromCid(msg.Cid())
			if err != nil {
				return types.EthBlock{}, err
			}
			block.Transactions = append(block.Transactions, hash.String())
		}
	}

	block.Hash = blkHash
	block.Number = types.EthUint64(ts.Height())
	block.ParentHash = parentBlkHash
	block.Timestamp = types.EthUint64(ts.Blocks()[0].Timestamp)
	block.BaseFeePerGas = types.EthBigInt{Int: ts.Blocks()[0].ParentBaseFee.Int}
	block.GasUsed = types.EthUint64(gasUsed)
	return block, nil
}

// lookupEthAddress makes its best effort at finding the Ethereum address for a
// Filecoin address. It does the following:
//
//  1. If the supplied address is an f410 address, we return its payload as the EthAddress.
//  2. Otherwise (f0, f1, f2, f3), we look up the actor on the state tree. If it has a predictable address, we return it if it's f410 address.
//  3. Otherwise, we fall back to returning a masked ID Ethereum address. If the supplied address is an f0 address, we
//     use that ID to form the masked ID address.
//  4. Otherwise, we fetch the actor's ID from the state tree and form the masked ID with it.
func (a *ethAPI) lookupEthAddress(ctx context.Context, addr address.Address) (types.EthAddress, error) {
	// Attempt to convert directly.
	if ethAddr, ok, err := types.TryEthAddressFromFilecoinAddress(addr, false); err != nil {
		return types.EthAddress{}, err
	} else if ok {
		return ethAddr, nil
	}

	// Lookup on the target actor.
	actor, err := a.chain.StateGetActor(ctx, addr, types.EmptyTSK)
	if err != nil {
		return types.EthAddress{}, err
	}
	if actor.Address != nil {
		if ethAddr, ok, err := types.TryEthAddressFromFilecoinAddress(*actor.Address, false); err != nil {
			return types.EthAddress{}, err
		} else if ok {
			return ethAddr, nil
		}
	}

	// Check if we already have an ID addr, and use it if possible.
	if ethAddr, ok, err := types.TryEthAddressFromFilecoinAddress(addr, true); err != nil {
		return types.EthAddress{}, err
	} else if ok {
		return ethAddr, nil
	}

	// Otherwise, resolve the ID addr.
	idAddr, err := a.chain.StateLookupID(ctx, addr, types.EmptyTSK)
	if err != nil {
		return types.EthAddress{}, err
	}
	return types.EthAddressFromFilecoinAddress(idAddr)
}

func (a *ethAPI) ethTxFromFilecoinMessageLookup(ctx context.Context, msgLookup *types.MsgLookup) (types.EthTx, error) {
	if msgLookup == nil {
		return types.EthTx{}, fmt.Errorf("msg does not exist")
	}
	cid := msgLookup.Message
	txHash, err := types.NewEthHashFromCid(cid)
	if err != nil {
		return types.EthTx{}, err
	}

	tsCid, err := msgLookup.TipSet.Cid()
	if err != nil {
		return types.EthTx{}, err
	}

	blkHash, err := types.NewEthHashFromCid(tsCid)
	if err != nil {
		return types.EthTx{}, err
	}

	msg, err := a.chain.ChainGetMessage(ctx, msgLookup.Message)
	if err != nil {
		return types.EthTx{}, err
	}

	fromEthAddr, err := a.lookupEthAddress(ctx, msg.From)
	if err != nil {
		return types.EthTx{}, err
	}

	toEthAddr, err := a.lookupEthAddress(ctx, msg.To)
	if err != nil {
		return types.EthTx{}, err
	}

	toAddr := &toEthAddr
	input := msg.Params
	// Check to see if we need to decode as contract deployment.
	// We don't need to resolve the to address, because there's only one form (an ID).
	if msg.To == builtintypes.EthereumAddressManagerActorAddr {
		switch msg.Method {
		case builtintypes.MethodsEAM.Create:
			toAddr = nil
			var params eam.CreateParams
			err = params.UnmarshalCBOR(bytes.NewReader(msg.Params))
			input = params.Initcode
		case builtintypes.MethodsEAM.Create2:
			toAddr = nil
			var params eam.Create2Params
			err = params.UnmarshalCBOR(bytes.NewReader(msg.Params))
			input = params.Initcode
		}
		if err != nil {
			return types.EthTx{}, err
		}
	}
	// Otherwise, try to decode as a cbor byte array.
	// TODO: Actually check if this is an ethereum call. This code will work for demo purposes, but is not correct.
	if toAddr != nil {
		if decodedParams, err := cbg.ReadByteArray(bytes.NewReader(msg.Params), uint64(len(msg.Params))); err == nil {
			input = decodedParams
		}
	}

	tx := types.EthTx{
		ChainID:              types.EthUint64(types.Eip155ChainID),
		Hash:                 txHash,
		BlockHash:            blkHash,
		BlockNumber:          types.EthUint64(msgLookup.Height),
		From:                 fromEthAddr,
		To:                   toAddr,
		Value:                types.EthBigInt(msg.Value),
		Type:                 types.EthUint64(2),
		Gas:                  types.EthUint64(msg.GasLimit),
		MaxFeePerGas:         types.EthBigInt(msg.GasFeeCap),
		MaxPriorityFeePerGas: types.EthBigInt(msg.GasPremium),
		V:                    types.EthBytes{},
		R:                    types.EthBytes{},
		S:                    types.EthBytes{},
		Input:                input,
	}
	return tx, nil
}

func (a *ethAPI) EthFeeHistory(ctx context.Context, blkCount types.EthUint64, newestBlkNum string, rewardPercentiles []float64) (types.EthFeeHistory, error) {
	if blkCount > 1024 {
		return types.EthFeeHistory{}, fmt.Errorf("block count should be smaller than 1024")
	}

	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("failed to got head %v", err)
	}
	newestBlkHeight := uint64(head.Height())

	// TODO https://github.com/filecoin-project/ref-fvm/issues/1016
	var blkNum types.EthUint64
	err = blkNum.UnmarshalJSON([]byte(`"` + newestBlkNum + `"`))
	if err == nil && uint64(blkNum) < newestBlkHeight {
		newestBlkHeight = uint64(blkNum)
	}

	// Deal with the case that the chain is shorter than the number of
	// requested blocks.
	oldestBlkHeight := uint64(1)
	if uint64(blkCount) <= newestBlkHeight {
		oldestBlkHeight = newestBlkHeight - uint64(blkCount) + 1
	}

	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(newestBlkHeight), false)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("cannot load find block height: %v", newestBlkHeight)
	}

	// FIXME: baseFeePerGas should include the next block after the newest of the returned range, because this
	// can be inferred from the newest block. we use the newest block's baseFeePerGas for now but need to fix it
	// In other words, due to deferred execution, we might not be returning the most useful value here for the client.
	baseFeeArray := []types.EthBigInt{types.EthBigInt(ts.Blocks()[0].ParentBaseFee)}
	gasUsedRatioArray := []float64{}

	for ts.Height() >= abi.ChainEpoch(oldestBlkHeight) {
		// Unfortunately we need to rebuild the full message view so we can
		// totalize gas used in the tipset.
		block, err := a.newEthBlockFromFilecoinTipSet(ctx, ts, false)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("cannot create eth block: %v", err)
		}

		// both arrays should be reversed at the end
		baseFeeArray = append(baseFeeArray, types.EthBigInt(ts.Blocks()[0].ParentBaseFee))
		gasUsedRatioArray = append(gasUsedRatioArray, float64(block.GasUsed)/float64(constants.BlockGasLimit))

		parentTsKey := ts.Parents()
		ts, err = a.chain.ChainGetTipSet(ctx, parentTsKey)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("cannot load tipset key: %v", parentTsKey)
		}
	}

	// Reverse the arrays; we collected them newest to oldest; the client expects oldest to newest.

	for i, j := 0, len(baseFeeArray)-1; i < j; i, j = i+1, j-1 {
		baseFeeArray[i], baseFeeArray[j] = baseFeeArray[j], baseFeeArray[i]
	}
	for i, j := 0, len(gasUsedRatioArray)-1; i < j; i, j = i+1, j-1 {
		gasUsedRatioArray[i], gasUsedRatioArray[j] = gasUsedRatioArray[j], gasUsedRatioArray[i]
	}

	return types.EthFeeHistory{
		OldestBlock:   oldestBlkHeight,
		BaseFeePerGas: baseFeeArray,
		GasUsedRatio:  gasUsedRatioArray,
	}, nil
}

var _ v1.IETH = (*ethAPI)(nil)
