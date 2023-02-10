package eth

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"

	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var ErrModuleDisabled = errors.New("module disabled, enable with Fevm.EnableEthRPC / VENUS_FEVM_ENABLEETHRPC")

type ethAPIDummy struct{}

func (e *ethAPIDummy) EthGetMessageCidByTransactionHash(ctx context.Context, txHash *types.EthHash) (*cid.Cid, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionHashByCid(ctx context.Context, cid cid.Cid) (*types.EthHash, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthBlockNumber(ctx context.Context) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthAccounts(ctx context.Context) ([]types.EthAddress, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthUint64) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error) {
	return types.EthBlock{}, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetBlockByNumber(ctx context.Context, blkNum string, fullTxInfo bool) (types.EthBlock, error) {
	return types.EthBlock{}, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkOpt string) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetCode(ctx context.Context, address types.EthAddress, blkOpt string) (types.EthBytes, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetStorageAt(ctx context.Context, address types.EthAddress, position types.EthBytes, blkParam string) (types.EthBytes, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGetBalance(ctx context.Context, address types.EthAddress, blkParam string) (types.EthBigInt, error) {
	return types.EthBigIntZero, ErrModuleDisabled
}

func (e *ethAPIDummy) EthFeeHistory(ctx context.Context, blkCount types.EthUint64, newestBlk string, rewardPercentiles []float64) (types.EthFeeHistory, error) {
	return types.EthFeeHistory{}, ErrModuleDisabled
}

func (e *ethAPIDummy) EthChainId(ctx context.Context) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) NetVersion(ctx context.Context) (string, error) {
	return "", ErrModuleDisabled
}

func (e *ethAPIDummy) NetListening(ctx context.Context) (bool, error) {
	return false, ErrModuleDisabled
}

func (e *ethAPIDummy) EthProtocolVersion(ctx context.Context) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthGasPrice(ctx context.Context) (types.EthBigInt, error) {
	return types.EthBigIntZero, ErrModuleDisabled
}

func (e *ethAPIDummy) EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error) {
	return 0, ErrModuleDisabled
}

func (e *ethAPIDummy) EthCall(ctx context.Context, tx types.EthCall, blkParam string) (types.EthBytes, error) {
	return nil, ErrModuleDisabled
}

func (e *ethAPIDummy) EthMaxPriorityFeePerGas(ctx context.Context) (types.EthBigInt, error) {
	return types.EthBigIntZero, ErrModuleDisabled
}

func (e *ethAPIDummy) EthSendRawTransaction(ctx context.Context, rawTx types.EthBytes) (types.EthHash, error) {
	return types.EthHash{}, ErrModuleDisabled
}

func (e *ethAPIDummy) Web3ClientVersion(ctx context.Context) (string, error) {
	return "", ErrModuleDisabled
}

func (e *ethAPIDummy) start(_ context.Context) error {
	return nil
}

func (e *ethAPIDummy) close() error {
	return nil
}

var _ v1.IETH = &ethAPIDummy{}
var _ ethAPIAdapter = &ethAPIDummy{}
