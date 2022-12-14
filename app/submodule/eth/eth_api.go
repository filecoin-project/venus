package eth

import (
	"context"
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-state-types/abi"
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
	return types.EthBlock{}, nil
}

func (a *ethAPI) EthGetBlockByNumber(ctx context.Context, blkNum types.EthInt, fullTxInfo bool) (types.EthBlock, error) {
	return types.EthBlock{}, nil
}

func (a *ethAPI) EthGetTransactionByHash(ctx context.Context, txHash types.EthHash) (types.EthTx, error) {
	return types.EthTx{}, nil
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
	return types.EthInt(0), nil
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

var _ v1.IETH = (*ethAPI)(nil)
