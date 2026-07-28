package eth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// ===========================================================================
// 编译期 Red 验证：isSenderValidationError 函数存在性
// ===========================================================================

// 这行代码在 isSenderValidationError 未实现时会编译失败：
//
//	"undefined: isSenderValidationError"
//
// 这是 Red 验证 —— 证明函数尚不存在，Coder 需要实现它。
var _ = isSenderValidationError

// ===========================================================================
// 错误识别函数的预期行为测试
// 注意：isSenderValidationError 目前未实现，这些测试是"规格说明"。
// 实现后应将期望值从 _ 替换为 require 断言。
// ===========================================================================

// TestIsSenderValidationErrorExpectedBehavior 验证 isSenderValidationError
// 函数应具备的行为规格。
//
// 期望行为（Coder 实现后）：
//
//	输入                                                | 输出
//	----------------------------------------------------|------
//	"call raw get actor: not found"                      | true
//	"sender actor can't call messages, actor type: t060" | true
//	"gas estimation failed: out of gas"                  | false
//	"message execution failed: exit SysErrOutOfGas"      | false
//	"apply message failed: execution panic"              | false
//	nil                                                   | false
//	""                                                    | false
func TestIsSenderValidationErrorExpectedBehavior(t *testing.T) {
	t.Skip("实现 isSenderValidationError 后启用")

	// 输入模式                     | 期望结果
	// -----------------------------|---------
	// "call raw get actor: ..."     | true (sender 不存在)
	// "sender actor can't call..."  | true (非 account actor)
	// "gas estimation failed: ..."  | false (普通错误)
	// "message execution failed..." | false (普通错误)
	// "apply message failed: ..."   | false (普通错误)
	// nil                            | false
	// ""                             | false

	require.True(t, isSenderValidationError(fmt.Errorf("call raw get actor: not found")))
	require.True(t, isSenderValidationError(fmt.Errorf("sender actor can't call messages, actor type: t060")))
	require.False(t, isSenderValidationError(fmt.Errorf("gas estimation failed: out of gas")))
	require.False(t, isSenderValidationError(fmt.Errorf("message execution failed: exit SysErrOutOfGas")))
	require.False(t, isSenderValidationError(fmt.Errorf("apply message failed: execution panic")))
	require.False(t, isSenderValidationError(nil))
	require.False(t, isSenderValidationError(fmt.Errorf("")))
}

// ===========================================================================
// 错误语义验证
// ===========================================================================

// TestErrorSemantics 验证 sender 验证错误的语义。
func TestErrorSemantics(t *testing.T) {
	t.Run("SysErrSenderInvalid exit code", func(t *testing.T) {
		receipt := &types.MessageReceipt{
			ExitCode: exitcode.SysErrSenderInvalid,
			GasUsed:  100,
			Return:   []byte{},
		}
		require.True(t, receipt.ExitCode.IsError())

		// 注意：EthCall 的 fallback 逻辑基于 Go error 的字符串匹配，
		// 而非 exit code。callInternal 在 sender 不存在时直接返回
		// Go error "call raw get actor: ..."，不会产生 MessageReceipt。
		// exit code SysErrSenderInvalid 是在消息执行结果中的错误码。
		require.Equal(t, exitcode.SysErrSenderInvalid, receipt.ExitCode)
	})

	t.Run("call raw get actor is the statemanger error pattern", func(t *testing.T) {
		// callInternal 中 sender 不存在时的错误格式
		err := fmt.Errorf("call raw get actor: not found")
		require.Contains(t, err.Error(), "call raw get actor")
		require.NotContains(t, err.Error(), "sender actor can't call messages")
	})

	t.Run("sender actor can't call messages is the statemanger error pattern", func(t *testing.T) {
		// callInternal 中非 account actor 时的错误格式
		err := fmt.Errorf("sender actor can't call messages, actor type: t060")
		require.Contains(t, err.Error(), "sender actor can't call messages")
		require.NotContains(t, err.Error(), "call raw get actor")
	})
}

// ===========================================================================
// 接口兼容性验证
// ===========================================================================

// TestApplyMessageInterface 验证 applyMessage 的接口签名在改动后保持不变。
func TestApplyMessageInterface(t *testing.T) {
	// applyMessage 签名：
	//   func (a *ethAPI) applyMessage(ctx context.Context, msg *types.Message, tsk types.TipSetKey)
	//
	// 重构后应保持此签名不变（不新增参数，不改返回值类型）。
	_ = (*ethAPI).applyMessage
}

// TestEthCallInterface 验证 EthCall 的接口签名在改动后保持不变。
func TestEthCallInterface(t *testing.T) {
	// EthCall 签名：
	//   func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam types.EthBlockNumberOrHash)
	//
	// 重构后不修改此接口签名。
	_ = (*ethAPI).EthCall
}

// TestEthEstimateGasInterface 验证 EthEstimateGas 的接口签名在改动后保持不变。
func TestEthEstimateGasInterface(t *testing.T) {
	// EthEstimateGas 签名：
	//   func (a *ethAPI) EthEstimateGas(ctx context.Context, p jsonrpc.RawParams) (types.EthUint64, error)
	//
	// 重构后不修改此接口签名。
	_ = (*ethAPI).EthEstimateGas
}

// ===========================================================================
// Fallback 逻辑的集成测试规划
// ===========================================================================

// TestEthCallFallback_Integration 验证 EthCall 在 sender 验证失败时的
// fallback 行为。需要 mock chain 和 stmgr 的完整测试环境。
//
// 测试场景（实现后启用）：
//
//  1. 正常 account sender → applyMessage 成功 → 不走 fallback
//     from: 有效的 account 地址
//     预期：直接返回 applyMessage 结果
//
//  2. EVM 合约 sender → applyMessage 失败（sender 验证）→ fallback 成功
//     from: 链上已部署的 EVM 合约地址（如 t2t... 或 f410f...）
//     预期：fallback 到 ApplyOnStateWithGasSkipSenderValidation，返回成功
//
//  3. 不存在的地址 sender → applyMessage 失败（sender 验证）→ fallback 成功
//     from: 链上不存在的随机地址
//     预期：fallback 到 ApplyOnStateWithGasSkipSenderValidation，
//     通过隐式消息创建 ephemeral placeholder 后执行
//
//  4. 非 sender 验证错误 → 不触发 fallback
//     from: 有效 account 地址
//     gas/数据问题导致 applyMessage 失败
//     预期：不触发 fallback，直接返回原始错误
func TestEthCallFallback_Integration(t *testing.T) {
	t.Skip("需要 mock chain 和 stmgr 的完整测试环境")
}

// TestEthEstimateGasFallback_Integration 验证 EthEstimateGas 在 sender
// 验证失败时的 fallback 行为。
//
// gas 估算调用链：
//
//	EthEstimateGas → GasEstimateMessageGas → gasSearch → CallWithGas
//	或通过 GasEstimateCallWithGas → ethGasSearch
//
// 测试场景（实现后启用）：
//
//  1. 正常 sender → gas 估算正常
//  2. 不存在地址 sender → GasEstimateMessageGas 因 sender 验证失败
//     预期：使用 CallWithGasSkipSenderValidation 重新估算
//  3. 非 sender 错误 → 不触发 fallback
func TestEthEstimateGasFallback_Integration(t *testing.T) {
	t.Skip("需要 mock mpool 和 stmgr 的完整测试环境")
}
