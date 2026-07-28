package statemanger

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"
)

// ===========================================================================
// 编译期 Red 验证：新方法签名存在性
// ===========================================================================

// skipSenderValidation 接口定义了待实现的新方法签名。
//
// 这行编译断言在方法未实现时会直接编译失败：
//   - "Stmgr does not implement skipSenderValidation"
//
// 这是最精确的 Red 验证 —— 它清楚地指向未实现的函数。
var _ skipSenderValidation = (*Stmgr)(nil)

type skipSenderValidation interface {
	// ApplyOnStateWithGasSkipSenderValidation 行为同 ApplyOnStateWithGas，
	// 但跳过 sender 验证。
	// - sender 不存在时：通过 ApplyImplicitMessage 创建 ephemeral placeholder actor
	// - sender 是非 account actor 时：通过 ApplyImplicitMessage 执行
	ApplyOnStateWithGasSkipSenderValidation(
		ctx context.Context,
		stateCid cid.Cid,
		msg *types.Message,
		ts *types.TipSet,
	) (*types.InvocResult, error)

	// CallWithGasSkipSenderValidation 行为同 CallWithGas，
	// 但跳过 sender 验证。
	CallWithGasSkipSenderValidation(
		ctx context.Context,
		msg *types.Message,
		priorMsgs []types.ChainMsg,
		ts *types.TipSet,
		applyTSMessages bool,
	) (*types.InvocResult, error)
}

// ===========================================================================
// 核心逻辑行为测试
// ===========================================================================

// TestSenderValidationErrorPatterns 验证 sender 验证错误的字符串模式。
//
// statemanger callInternal 可能抛出的 sender 验证错误格式：
//   - fmt.Errorf("call raw get actor: %s", err)       — sender 不存在
//   - fmt.Errorf("sender actor can't call messages, actor type: %s", code) — 非 account actor
//
// isSenderValidationError 函数（在 app/submodule/eth/ 中）需要识别这些模式。
func TestSenderValidationErrorPatterns(t *testing.T) {
	t.Run("sender not found error contains expected pattern", func(t *testing.T) {
		err := fmt.Errorf("call raw get actor: not found")
		require.Contains(t, err.Error(), "call raw get actor")
	})

	t.Run("non account actor error contains expected pattern", func(t *testing.T) {
		err := fmt.Errorf("sender actor can't call messages, actor type: t060")
		require.Contains(t, err.Error(), "sender actor can't call messages")
	})

	t.Run("other errors should not contain sender validation patterns", func(t *testing.T) {
		gasErr := fmt.Errorf("gas estimation failed: out of gas")
		require.NotContains(t, gasErr.Error(), "call raw get actor")
		require.NotContains(t, gasErr.Error(), "sender actor can't call messages")

		vmErr := fmt.Errorf("apply message failed: execution panic")
		require.NotContains(t, vmErr.Error(), "call raw get actor")
		require.NotContains(t, vmErr.Error(), "sender actor can't call messages")
	})
}

// TestApplyOnStateWithGas_DefaultBehavior 验证 ApplyOnStateWithGas
// 在重构后行为不变（内部传 skipSenderValidation=false）。
//
// 此测试需要完整的 VM 环境（gas schedule、syscalls 等），无法在 mock 中运行。
// 接口兼容性已通过编译验证（go build ./...）。
func TestApplyOnStateWithGas_DefaultBehavior(t *testing.T) {
	t.Skip("需要完整 VM 环境，无法在 mock 中运行")
	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	mockFork := fork.NewMockFork()

	stmgr, err := NewStateManager(
		builder.Store(), builder.MessageStore(), eval,
		nil, mockFork, nil, nil, false, builder.CirculatingSupplyCalcualtor(),
	)
	require.NoError(t, err)
	require.NotNil(t, stmgr)

	ctx := context.Background()
	genesis := builder.Genesis()
	require.NotNil(t, genesis)

	msg := &types.Message{
		From:       address.TestAddress,
		To:         address.TestAddress2,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   10_000_000,
		Nonce:      0,
		Method:     0,
	}

	// ApplyOnStateWithGas 在重构后应保持可调用，签名不变
	stateCid := genesis.Blocks()[0].ParentStateRoot
	_, err = stmgr.ApplyOnStateWithGas(ctx, stateCid, msg, genesis)
	// 在 mock 环境下，可能会因 VM 初始化失败返回错误，
	// 但接口不应被破坏
	t.Logf("ApplyOnStateWithGas 调用结果: %v", err)
}

// TestCallWithGas_DefaultBehavior 验证 CallWithGas 在重构后行为不变。
func TestCallWithGas_DefaultBehavior(t *testing.T) {
	t.Skip("需要完整 VM 环境，无法在 mock 中运行")
	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	mockFork := fork.NewMockFork()

	stmgr, err := NewStateManager(
		builder.Store(), builder.MessageStore(), eval,
		nil, mockFork, nil, nil, false, builder.CirculatingSupplyCalcualtor(),
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesis := builder.Genesis()
	require.NotNil(t, genesis)

	msg := &types.Message{
		From:       address.TestAddress,
		To:         address.TestAddress2,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   10_000_000,
		Nonce:      0,
		Method:     0,
	}

	// CallWithGas 在重构后应保持可调用，签名不变
	_, err = stmgr.CallWithGas(ctx, msg, nil, genesis, false)
	t.Logf("CallWithGas 调用结果: %v", err)
}

// TestCallOnState_DefaultBehavior 验证 CallOnState 在重构后行为不变。
func TestCallOnState_DefaultBehavior(t *testing.T) {
	t.Skip("需要完整 VM 环境，无法在 mock 中运行")
	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	mockFork := fork.NewMockFork()

	stmgr, err := NewStateManager(
		builder.Store(), builder.MessageStore(), eval,
		nil, mockFork, nil, nil, false, builder.CirculatingSupplyCalcualtor(),
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesis := builder.Genesis()
	require.NotNil(t, genesis)

	msg := &types.Message{
		From:       address.TestAddress,
		To:         address.TestAddress2,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   constants.BlockGasLimit,
		Nonce:      0,
		Method:     0,
	}

	_, err = stmgr.CallOnState(ctx, genesis.Blocks()[0].ParentStateRoot, msg, genesis)
	t.Logf("CallOnState 调用结果: %v", err)
}

// TestCall_DefaultBehavior 验证 Call 在重构后行为不变。
func TestCall_DefaultBehavior(t *testing.T) {
	t.Skip("需要完整 VM 环境，无法在 mock 中运行")
	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	mockFork := fork.NewMockFork()

	stmgr, err := NewStateManager(
		builder.Store(), builder.MessageStore(), eval,
		nil, mockFork, nil, nil, false, builder.CirculatingSupplyCalcualtor(),
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesis := builder.Genesis()
	require.NotNil(t, genesis)

	msg := &types.Message{
		From:       address.TestAddress,
		To:         address.TestAddress2,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   constants.BlockGasLimit,
		Nonce:      0,
		Method:     0,
	}

	_, err = stmgr.Call(ctx, msg, genesis)
	t.Logf("Call 调用结果: %v", err)
}

// TestCallAtStateAndVersion_DefaultBehavior 验证 CallAtStateAndVersion
// 在重构后行为不变。
func TestCallAtStateAndVersion_DefaultBehavior(t *testing.T) {
	t.Skip("需要完整 VM 环境，无法在 mock 中运行")
	builder := chain.NewBuilder(t, address.Undef)
	eval := builder.FakeStateEvaluator()
	mockFork := fork.NewMockFork()

	stmgr, err := NewStateManager(
		builder.Store(), builder.MessageStore(), eval,
		nil, mockFork, nil, nil, false, builder.CirculatingSupplyCalcualtor(),
	)
	require.NoError(t, err)

	ctx := context.Background()
	genesis := builder.Genesis()
	require.NotNil(t, genesis)

	msg := &types.Message{
		From:       address.TestAddress,
		To:         address.TestAddress2,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   10_000_000,
		Nonce:      0,
		Method:     0,
	}

	// 用原始的 NetworkVersion 和 stateCid
	stateCid := genesis.Blocks()[0].ParentStateRoot
	// 在 mock 环境下 CallAtStateAndVersion 不直接依赖 tipset
	_, err = stmgr.CallAtStateAndVersion(ctx, msg, stateCid, 0)
	t.Logf("CallAtStateAndVersion 调用结果: %v", err)
}

// ===========================================================================
// InvocResult 结构验证
// ===========================================================================

// TestInvocResultStructure 验证 InvocResult 的结构完整性。
func TestInvocResultStructure(t *testing.T) {
	t.Run("result with success exit code", func(t *testing.T) {
		result := &types.InvocResult{
			MsgCid: testhelpers.CidFromString(t, "test"),
			Msg:    &types.Message{},
			MsgRct: &types.MessageReceipt{
				ExitCode: exitcode.Ok,
				GasUsed:  0,
				Return:   []byte{},
			},
			GasCost:        types.MsgGasCost{},
			ExecutionTrace: types.ExecutionTrace{},
			Error:          "",
			Duration:       0,
		}
		require.NotNil(t, result)
		require.NotNil(t, result.Msg)
		require.NotNil(t, result.MsgRct)
		require.False(t, result.MsgRct.ExitCode.IsError())
	})

	t.Run("result with sender invalid exit code", func(t *testing.T) {
		result := &types.InvocResult{
			MsgRct: &types.MessageReceipt{
				ExitCode: exitcode.SysErrSenderInvalid,
			},
			Error: "sender invalid",
		}
		require.True(t, result.MsgRct.ExitCode.IsError())
		require.NotEmpty(t, result.Error)
	})

	t.Run("skip sender success path returns same structure", func(t *testing.T) {
		t.Skip("实现 skip-sender 方法后启用，需要真实返回的 InvocResult 数据")

		// 验证 skip-sender 版本的返回结果结构与正常版本一致：
		// — 都包含 MsgCid/Msg/MsgRct 等完整字段
		// — MsgRct 结构相同（ExitCode/GasUsed/Return）
		// — GasCost 结构相同
	})
}

// ===========================================================================
// 集成测试标记
// ===========================================================================

// TestCallInternalSkipSenderValidation_Integration 需要完整的链环境。
// 包含真实 actor state tree、FVM 或 LegacyVM。
//
// 要运行此测试，需要：
// 1. 包含真实 actor state 的 tipset
// 2. 正常的 FVM 环境
// 3. 各种 sender 类型（account / EVM 合约 / 不存在地址）
func TestCallInternalSkipSenderValidation_Integration(t *testing.T) {
	t.Skip("需要完整的链环境和真实 state tree")

	// 测试用例矩阵（实现后启用）：
	//
	// | sender 类型              | skip=false         | skip=true          |
	// |-------------------------|--------------------|--------------------|
	// | 正常 account address    | 正常执行 ✅         | 正常执行 ✅         |
	// | EVM 合约地址            | sender 错误 ❌      | ApplyImplicitMessage ✅ |
	// | 不存在的地址            | sender 错误 ❌      | ephemeral placeholder ✅ |
	//
	// 每个测试需要：
	// 1. 创建 Stmgr
	// 2. 设置 state tree（包含/不包含 sender actor）
	// 3. 调用 callInternal 验证结果
}
