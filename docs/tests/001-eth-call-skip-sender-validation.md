# eth_call/eth_estimateGas 跳过 sender 验证 — 测试文档

## 概述

本文档记录 eth_call/eth_estimateGas 跳过 sender 验证功能的测试覆盖范围、测试文件结构和运行方式。

## 测试文件

| 文件 | 包 | 测试内容 |
|------|------|---------|
| `pkg/statemanger/call_test.go` | `statemanger` | statemanger 层 skip-sender 方法 |
| `app/submodule/eth/eth_call_fallback_test.go` | `eth` | ETH API 层 fallback 逻辑 |

## 测试覆盖范围

### 1. `pkg/statemanger/call_test.go`

#### 编译期验证（Red 验证）

| 测试 | 预期 | 当前状态 |
|------|------|---------|
| `skipSenderValidation` 接口断言 | `*Stmgr` 必须实现两个新方法 | 🔴 编译失败 — 方法未实现 |

编译断言行：
```go
var _ skipSenderValidation = (*Stmgr)(nil)
```
失败信息：
```
*Stmgr does not implement skipSenderValidation (missing method ApplyOnStateWithGasSkipSenderValidation)
```

#### 核心行为测试

| 测试函数 | 覆盖场景 | 状态 |
|---------|---------|------|
| `TestSenderValidationErrorPatterns` | sender 验证错误的字符串模式验证 | ✅ 可运行 |
| `TestApplyOnStateWithGas_DefaultBehavior` | 现有方法兼容性 | ✅ 可运行 |
| `TestCallWithGas_DefaultBehavior` | 现有方法兼容性 | ✅ 可运行 |
| `TestCallOnState_DefaultBehavior` | 现有方法兼容性 | ✅ 可运行 |
| `TestCall_DefaultBehavior` | 现有方法兼容性 | ✅ 可运行 |
| `TestCallAtStateAndVersion_DefaultBehavior` | 现有方法兼容性 | ✅ 可运行 |
| `TestInvocResultStructure` | 返回结果结构验证 | ✅ 可运行 |

#### 集成测试标记（`t.Skip`）

| 测试函数 | 所需环境 | 场景 |
|---------|---------|------|
| `TestCallInternalSkipSenderValidation_Integration` | 完整链环境 + 真实 state tree | 7 个测试场景（account/EVM 合约/不存在地址 × skip=true/false） |

### 2. `app/submodule/eth/eth_call_fallback_test.go`

#### 编译期验证（Red 验证）

| 测试 | 预期 | 当前状态 |
|------|------|---------|
| `isSenderValidationError` 存在性断言 | 函数必须存在 | 🔴 编译失败 — 函数未定义 |

编译断言行：
```go
var _ = isSenderValidationError
```
失败信息：
```
undefined: isSenderValidationError
```

#### 错误语义验证

| 测试函数 | 覆盖场景 | 状态 |
|---------|---------|------|
| `TestErrorSemantics` | 错误字符串模式、exit code 语义 | ✅ 可运行 |

#### 接口兼容性验证

| 测试函数 | 验证内容 | 状态 |
|---------|---------|------|
| `TestApplyMessageInterface` | `applyMessage` 签名未变 | ✅ 可运行 |
| `TestEthCallInterface` | `EthCall` 签名未变 | ✅ 可运行 |
| `TestEthEstimateGasInterface` | `EthEstimateGas` 签名未变 | ✅ 可运行 |

#### 集成测试标记（`t.Skip`）

| 测试函数 | 所需环境 | 场景 |
|---------|---------|------|
| `TestEthCallFallback_Integration` | mock chain + stmgr | 4 个 fallback 场景 |
| `TestEthEstimateGasFallback_Integration` | mock mpool + stmgr | 3 个 gas 估算 fallback 场景 |

## 测试用例矩阵

### statemanger 层 — `callInternal` 的 `skipSenderValidation` 参数

| # | sender 类型 | skipSenderValidation | 预期行为 | 测试层级 |
|---|-------------|---------------------|---------|---------|
| 1 | 正常 account 地址 | `false` | 正常执行（与原行为一致） | 单元/集成 |
| 2 | 正常 account 地址 | `true` | 正常执行（行为不变） | 集成 |
| 3 | EVM 合约地址 | `false` | 报错 `"sender actor can't call messages"` | 集成 |
| 4 | EVM 合约地址 | `true` | 通过 `ApplyImplicitMessage` 执行 | 集成 |
| 5 | 不存在的地址 | `false` | 报错 `"call raw get actor"` | 单元(错误模式) |
| 6 | 不存在的地址 | `true` | 创建 ephemeral placeholder 后执行 | 集成 |

### ETH API 层 — fallback 逻辑

| # | 场景 | 入口 | 预期行为 | 测试层级 |
|---|------|------|---------|---------|
| 7 | 正常 account sender | `EthCall` | `applyMessage` 成功，直接返回 | 集成 |
| 8 | EVM 合约 sender | `EthCall` | fallback 到 `ApplyOnStateWithGasSkipSenderValidation` | 集成 |
| 9 | 不存在地址 sender | `EthCall` | fallback 到 `ApplyOnStateWithGasSkipSenderValidation` | 集成 |
| 10 | 非 sender 错误（gas 不足） | `EthCall` | 不触发 fallback，返回原始错误 | 集成 |
| 11 | 正常 account sender | `EthEstimateGas` | gas 估算正常，不走 fallback | 集成 |
| 12 | 不存在地址 sender | `EthEstimateGas` | fallback 到 `CallWithGasSkipSenderValidation` | 集成 |
| 13 | 非 sender 错误 | `EthEstimateGas` | 不触发 fallback | 集成 |

### 辅助函数

| # | 函数 | 输入 | 预期输出 | 状态 |
|---|------|------|---------|------|
| 14 | `isSenderValidationError` | `"call raw get actor: not found"` | `true` | 🔴 未实现 |
| 15 | `isSenderValidationError` | `"sender actor can't call messages, type: t060"` | `true` | 🔴 未实现 |
| 16 | `isSenderValidationError` | `"gas estimation failed: out of gas"` | `false` | 🔴 未实现 |
| 17 | `isSenderValidationError` | `"apply message failed: panic"` | `false` | 🔴 未实现 |
| 18 | `isSenderValidationError` | `nil` | `false` | 🔴 未实现 |

## 运行方式

### 当前状态（Red 验证 — 编译失败）

```bash
# statemanger — 编译失败：方法未实现
go test ./pkg/statemanger/ -v -count=1

# eth — 编译失败：函数未定义
go test ./app/submodule/eth/ -v -count=1
```

预期输出：
```
pkg/statemanger/call_test.go:31:30: cannot use (*Stmgr)(nil) as skipSenderValidation value:
  *Stmgr does not implement skipSenderValidation
  (missing method ApplyOnStateWithGasSkipSenderValidation)

app/submodule/eth/eth_call_fallback_test.go:21:9: undefined: isSenderValidationError
```

### 实现后（Green 验证）

当函数实现后，编译通过：

```bash
# 运行全部测试
go test ./pkg/statemanger/ -v -count=1
go test ./app/submodule/eth/ -run TestErrorSemantics -v -count=1

# 运行完整 eth 测试
go test ./app/submodule/eth/ -v -count=1
```

### 实现后集成测试（需完整链环境）

```bash
# 移除 t.Skip() 后运行
go test ./pkg/statemanger/ -run TestCallInternalSkipSenderValidation_Integration -v -count=1
go test ./app/submodule/eth/ -run TestEthCallFallback_Integration -v -count=1
```

## Red 验证日志

<!-- 记录每次 Red 验证的结果 -->

| 时间 | 文件 | 编译错误 | 状态 |
|------|------|---------|------|
| 首次 | `call_test.go` | `missing method ApplyOnStateWithGasSkipSenderValidation` | ✅ 有效 |
| 首次 | `eth_call_fallback_test.go` | `undefined: isSenderValidationError` | ✅ 有效 |

## Coder 实施指引

Coder 需按以下顺序实现，使测试从 Red 变为 Green：

### Phase 1: 实现 `isSenderValidationError`（纯函数）

**文件**: `app/submodule/eth/eth_api.go`

```go
func isSenderValidationError(err error) bool {
    if err == nil {
        return false
    }
    msg := err.Error()
    return strings.Contains(msg, "call raw get actor") ||
           strings.Contains(msg, "sender actor can't call messages")
}
```

验证：
```bash
go test ./app/submodule/eth/ -v -count=1  # 编译通过
```

### Phase 2: 实现 statemanger 新方法

**文件**: `pkg/statemanger/call.go`

1. `callInternal` 新增 `skipSenderValidation bool` 参数
2. 实现 `ApplyOnStateWithGasSkipSenderValidation` 和 `CallWithGasSkipSenderValidation`
3. 更新所有现有调用传 `skipSenderValidation=false`

验证：
```bash
go test ./pkg/statemanger/ -v -count=1  # 编译通过
```

### Phase 3: 实现 ETH API fallback 逻辑

**文件**: `app/submodule/eth/eth_api.go`

1. `EthCall` 中 `applyMessage` 失败时检查 `isSenderValidationError`
2. 是 → fallback 到 `ApplyOnStateWithGasSkipSenderValidation`
3. `EthEstimateGas` 中类似 fallback

验证：
```bash
go test ./app/submodule/eth/ -v -count=1       # 全部通过
go test ./pkg/statemanger/ -v -count=1           # 全部通过
```

### Phase 4: 集成测试

移除 `t.Skip()`，运行集成测试验证完整行为。
