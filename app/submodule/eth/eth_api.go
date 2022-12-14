package eth

import (
	"context"
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/constants"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ethAPI struct {
	em    *EthSubModule
	chain v1.IChain
}

func (a *ethAPI) EthBlockNumber(ctx context.Context) (types.EthInt, error) {
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthInt(0), err
	}
	return types.EthInt(head.Height()), nil
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

func (a *ethAPI) EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthInt) (types.EthInt, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(blkNum), false)
	if err != nil {
		return types.EthInt(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}

	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthInt(count), err
}

func (a *ethAPI) EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthInt, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthInt(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthInt(count), err
}

func (a *ethAPI) EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	return a.ethBlockFromFilecoinTipSet(ctx, ts, fullTxInfo)
}

func (a *ethAPI) EthGetBlockByNumber(ctx context.Context, blkNum types.EthInt, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(blkNum), false)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	return a.ethBlockFromFilecoinTipSet(ctx, ts, fullTxInfo)
}

func (a *ethAPI) EthGetTransactionByHash(ctx context.Context, txHash types.EthHash) (types.EthTx, error) {
	cid := txHash.ToCid()

	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, cid, constants.LookbackNoLimit, true)
	if err != nil {
		return types.EthTx{}, nil
	}

	tx, err := a.ethTxFromFilecoinMessageLookup(ctx, msgLookup)
	if err != nil {
		return types.EthTx{}, err
	}
	return tx, nil
}

func (a *ethAPI) EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkParam string) (types.EthInt, error) {
	return types.EthInt(0), nil
}

func (a *ethAPI) EthGetTransactionReceipt(ctx context.Context, blkHash types.EthHash) (types.EthTxReceipt, error) {
	return types.EthTxReceipt{}, nil
}

func (a *ethAPI) EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthInt) (types.EthTx, error) {
	return types.EthTx{}, nil
}

func (a *ethAPI) EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthInt, txIndex types.EthInt) (types.EthTx, error) {
	return types.EthTx{}, nil
}

// EthGetCode returns string value of the compiled bytecode
func (a *ethAPI) EthGetCode(ctx context.Context, address types.EthAddress) (string, error) {
	return "", nil
}

func (a *ethAPI) EthGetStorageAt(ctx context.Context, address types.EthAddress, position types.EthInt, blkParam string) (string, error) {
	return "", nil
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

func (a *ethAPI) EthChainId(ctx context.Context) (types.EthInt, error) {
	return types.EthInt(a.em.networkCfg.Eip155ChainID), nil
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

func (a *ethAPI) EthProtocolVersion(ctx context.Context) (types.EthInt, error) {
	return types.EthInt(0), nil
}

func (a *ethAPI) EthMaxPriorityFeePerGas(ctx context.Context) (types.EthInt, error) {
	return types.EthInt(0), nil
}

func (a *ethAPI) EthGasPrice(ctx context.Context) (types.EthInt, error) {
	return types.EthInt(0), nil
}

func (a *ethAPI) EthEstimateGas(ctx context.Context, tx types.EthCall, blkParam string) (types.EthInt, error) {
	return types.EthInt(0), nil
}

func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam string) (string, error) {
	return "", nil
}

func (a *ethAPI) ethBlockFromFilecoinTipSet(ctx context.Context, ts *types.TipSet, fullTxInfo bool) (types.EthBlock, error) {
	parent, err := a.chain.ChainGetTipSet(ctx, ts.Parents())
	if err != nil {
		return types.EthBlock{}, err
	}
	parentKeyCid, err := parent.Key().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	parentBlkHash, err := types.EthHashFromCid(parentKeyCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	// blkMsgs, err := a.Chain.BlockMsgsForTipset(ctx, ts)
	// if err != nil {
	// 	return types.EthBlock{}, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	// }
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
			hash, err := types.EthHashFromCid(msg.Cid())
			if err != nil {
				return types.EthBlock{}, err
			}
			block.Transactions = append(block.Transactions, hash.String())
		}
	}

	block.Number = types.EthInt(ts.Height())
	block.ParentHash = parentBlkHash
	block.Timestamp = types.EthInt(ts.Blocks()[0].Timestamp)
	block.BaseFeePerGas = types.EthBigInt{Int: ts.Blocks()[0].ParentBaseFee.Int}
	block.GasUsed = types.EthInt(gasUsed)
	return block, nil
}

func (a *ethAPI) ethTxFromFilecoinMessageLookup(ctx context.Context, msgLookup *types.MsgLookup) (types.EthTx, error) {
	cid := msgLookup.Message
	txHash, err := types.EthHashFromCid(cid)
	if err != nil {
		return types.EthTx{}, err
	}

	tsCid, err := msgLookup.TipSet.Cid()
	if err != nil {
		return types.EthTx{}, err
	}

	blkHash, err := types.EthHashFromCid(tsCid)
	if err != nil {
		return types.EthTx{}, err
	}

	msg, err := a.chain.ChainGetMessage(ctx, msgLookup.Message)
	if err != nil {
		return types.EthTx{}, err
	}

	fromFilIDAddr, err := a.chain.StateLookupID(ctx, msg.From, types.EmptyTSK)
	if err != nil {
		return types.EthTx{}, err
	}

	fromEthAddr, err := types.EthAddressFromFilecoinIDAddress(fromFilIDAddr)
	if err != nil {
		return types.EthTx{}, err
	}

	toFilAddr, err := a.chain.StateLookupID(ctx, msg.From, types.EmptyTSK)
	if err != nil {
		return types.EthTx{}, err
	}

	toEthAddr, err := types.EthAddressFromFilecoinIDAddress(toFilAddr)
	if err != nil {
		return types.EthTx{}, err
	}

	tx := types.EthTx{
		ChainID:              types.EthInt(a.em.networkCfg.Eip155ChainID),
		Hash:                 txHash,
		BlockHash:            blkHash,
		BlockNumber:          types.EthInt(msgLookup.Height),
		From:                 fromEthAddr,
		To:                   toEthAddr,
		Value:                types.EthBigInt(msg.Value),
		Type:                 types.EthInt(2),
		Gas:                  types.EthInt(msg.GasLimit),
		MaxFeePerGas:         types.EthBigInt(msg.GasFeeCap),
		MaxPriorityFeePerGas: types.EthBigInt(msg.GasPremium),
		V:                    types.EthBigIntZero,
		R:                    types.EthBigIntZero,
		S:                    types.EthBigIntZero,
		// TODO: Input:
	}
	return tx, nil
}

var _ v1.IETH = (*ethAPI)(nil)
