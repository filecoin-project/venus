# eth_call/eth_estimateGas 跳过 sender 验证

## 1. 目标和核心功能

### 目标

将 Lotus PR [#13470](https://github.com/filecoin-project/lotus/pull/13470) / [#13724](https://github.com/filecoin-project/lotus/pull/13724) 中的「跳过 sender 验证」功能迁移到 Venus，使 `eth_call` 和 `eth_estimateGas` 支持从 EVM 合约地址或不存在的地址发起模拟调用。

### 核心功能

- **EVM 合约地址作为 sender**：允许 EVM 合约地址（非 account actor）作为 `from` 参数调用 `eth_call`/`eth_estimateGas`
- **不存在的地址作为 sender**：允许链上不存在的地址作为 `from` 参数进行 gas 估算
- **兼容 Safe SDK EIP-1271 签名校验**：Tenderly / Safe 等工具需要合约地址作为 caller 来模拟签名校验交易
- **保持原有行为的兼容性**：已有正常 account sender 的调用行为不变，不影响现有代码

## 2. 技术方案

### 2.1 涉及模块

| 模块 | 文件 | 改动类型 |
|------|------|---------|
| **statemanger** | `pkg/statemanger/call.go` | 核心逻辑变更 |
| **ETH API** | `app/submodule/eth/eth_api.go` | 调用方变更 |
| VM 接口 / FVM / LegacyVM | 无需修改 | — |

### 2.2 技术选型与关键决策

#### 2.2.1 方案选择：Fallback 策略

采用 **fallback 策略**（先试正常路径，sender 验证失败时 fallback 到 skip-sender 版本）而非直接修改 VM 接口或改为永远跳过验证。

**理由**：

1. **最小化回归风险**：正常路径已在生产环境长期验证，直接跳过 sender 验证会影响正常 account sender 的执行语义差异（隐式 vs 显式消息在 nonce 检查、gas 计费上的细微差异）
2. **性能优化**：正常 sender 场景（95%+ 调用）走原路径，零额外开销。skip 路径需创建 ephemeral placeholder actor，涉及附加 IO
3. **错误语义保持**：正常路径的错误语义精确，fallback 仅在识别到特定的 sender 验证失败时触发，不掩盖其他真正的错误
4. **对齐 Lotus 最终合并方案**：PR #13724（rvagg 精简版）采用相同的 fallback 设计

#### 2.2.2 Venus 特有简化

相比 Lotus 的改动，Venus 可以**省掉 VM 层的修改**（vmi.go / fvm.go / vm.go / execution.go），原因：

| 对比项 | Lotus | Venus |
|--------|-------|-------|
| VM 接口 `ApplyImplicitMessage` | 未直接暴露在公共接口 | ✅ **已有**（`vmcontext/types.go:146`） |
| FVM 实现 | 需新增包装方法 | ✅ FVM 已原生支持 |
| LegacyVM 实现 | 需新增存根返回错误 | ✅ LegacyVM 已有（`vmcontext/vmcontext.go:146`） |

这意味着 Venus 可以直接在 `statemanger/call.go` 层调用 `vmi.ApplyImplicitMessage`，无需修改 VM 层接口。

### 2.3 接口变更设计

#### 2.3.1 `callInternal` 新增参数

```go
// 变更前
func (s *Stmgr) callInternal(ctx context.Context,
    msg *types.Message,
    priorMsgs []types.ChainMsg,
    ts *types.TipSet,
    stateCid cid.Cid,
    nvGetter chain.NetworkVersionGetter,
    checkGas bool,
    strategy execMessageStrategy,
) (*types.InvocResult, error)

// 变更后 — 新增 skipSenderValidation bool 参数
func (s *Stmgr) callInternal(ctx context.Context,
    msg *types.Message,
    priorMsgs []types.ChainMsg,
    ts *types.TipSet,
    stateCid cid.Cid,
    nvGetter chain.NetworkVersionGetter,
    checkGas bool,
    strategy execMessageStrategy,
    skipSenderValidation bool,
) (*types.InvocResult, error)
```

#### 2.3.2 新增公开方法

```go
// ApplyOnStateWithGasSkipSenderValidation — 供 EthCall 使用
// 行为同 ApplyOnStateWithGas，但跳过 sender 验证
func (s *Stmgr) ApplyOnStateWithGasSkipSenderValidation(ctx context.Context,
    stateCid cid.Cid, msg *types.Message, ts *types.TipSet,
) (*types.InvocResult, error) {
    return s.callInternal(ctx, msg, nil, ts, stateCid, s.GetNetworkVersion, true, execNoMessages, true)
}

// CallWithGasSkipSenderValidation — 供 gas 估算使用
// 行为同 CallWithGas，但跳过 sender 验证
func (s *Stmgr) CallWithGasSkipSenderValidation(ctx context.Context,
    msg *types.Message, priorMsgs []types.ChainMsg, ts *types.TipSet, applyTSMessages bool,
) (*types.InvocResult, error) {
    var strategy execMessageStrategy
    if applyTSMessages {
        strategy = execAllMessages
    } else {
        strategy = execSameSenderMessages
    }
    return s.callInternal(ctx, msg, priorMsgs, ts, cid.Undef, s.GetNetworkVersion, true, strategy, true)
}
```

#### 2.3.3 现有方法保持兼容

`Call`、`CallOnState`、`ApplyOnStateWithGas`、`CallWithGas`、`CallAtStateAndVersion` 内部全部传 `skipSenderValidation=false`，行为不变。

### 2.4 核心逻辑变更

#### 2.4.1 sender 验证处修改（`callInternal` 中原 sender 检查位置）

```go
// 当前代码（行 210-215）
fromActor, found, err := st.GetActor(ctx, msg.From)
if err != nil || !found {
    return nil, fmt.Errorf("call raw get actor: %s", err)
}

// 变更后
fromActor, found, err := st.GetActor(ctx, msg.From)
if err != nil || !found {
    if !skipSenderValidation {
        return nil, fmt.Errorf("call raw get actor: %s", err)
    }
    // skipSenderValidation=true: 处理不存在的 sender
    // 创建 ephemeral placeholder actor 并填入 nonce=0
    // ... （详见 2.4.2）
}

// 检查 actor 类型（行 210 之后）
if !isAccountActor(fromActor) {
    if !skipSenderValidation {
        return nil, fmt.Errorf("sender actor can't call messages, actor type: %s", fromActor.Code)
    }
    // skipSenderValidation=true: 使用 ApplyImplicitMessage 路径
    // ...（详见 2.4.3）
}
```

#### 2.4.2 不存在的 sender：创建 ephemeral placeholder

当 sender actor 在 state tree 中不存在时：

1. 通过 `buffStore`（已有临时的 TieredBstore）构建隐式零值发送消息
2. 调用 `vmi.ApplyImplicitMessage` 执行隐式消息，在 VM 中创建 ephemeral placeholder actor
3. flush 获取更新后的 stateCid
4. 重新从更新后的 state tree 中获取 sender actor

#### 2.4.3 已存在的非 account sender：使用 ApplyImplicitMessage

当 sender 已存在但不是 account actor（如 EVM 合约地址）时：

1. 不修改 nonce（nonce 对合约地址无意义）
2. 调用 `vmi.ApplyImplicitMessage` 执行消息
3. FVM 原生支持隐式模式，跳过 sender 验证

#### 2.4.4 ETH API 层修改

**EthCall**（`eth_api.go:1203`）：

```go
// 当前
invokeResult, err := a.applyMessage(ctx, msg, ts.Key())

// 变更后
invokeResult, err := a.applyMessage(ctx, msg, ts.Key())
if err != nil && isSenderValidationError(err) {
    // fallback: 尝试跳过 sender 验证
    st, err2 := a.em.chainModule.ChainReader.GetTipSetStateRoot(ctx, ts)
    if err2 != nil {
        return nil, err2
    }
    invokeResult, err2 = a.em.chainModule.Stmgr.ApplyOnStateWithGasSkipSenderValidation(ctx, st, msg, ts)
    if err2 != nil {
        return nil, fmt.Errorf("message execution failed (skip sender): %w", err2)
    }
}
```

**EthEstimateGas**（`eth_api.go:1032`）：

gas 估算的 fallback 逻辑较为复杂，因为 `GasEstimateMessageGas` → `gasSearch` → `CallWithGas` 链路过长。采用以下方案：

```go
// 当前：gasSearch 内部调用 smgr.CallWithGas
// 变更：检测到 sender 验证失败时，重复 gas 搜索，但使用 CallWithGasSkipSenderValidation
```

或者更简洁的：在 `EthEstimateGas` 检测到 `GasEstimateMessageGas` 失败且错误为 sender 验证错误时，创建一个新的 gassedMsg，用 `applyMessageSkipSenderValidation`（新增辅助方法）重新估算。

#### 2.4.5 错误识别辅助函数

```go
// 判断错误是否是 sender 验证失败类型
func isSenderValidationError(err error) bool {
    return strings.Contains(err.Error(), "call raw get actor") ||
           strings.Contains(err.Error(), "sender actor can't call messages")
}
```

### 2.5 与 Lotus PR #13724 实现差异对比

| 对比项 | Lotus PR #13724 | Venus 实现 |
|--------|----------------|------------|
| 修改文件数 | ~9 个文件 | **~2 个文件** |
| VM 接口扩展 | 新增 `ApplyMessageSkipSenderValidation` | 无需改动，复用已有 `ApplyImplicitMessage` |
| FVM 层修改 | 新增包装方法 | 无需改动 |
| LegacyVM 修改 | 新增存根返回错误 | 无需改动 |
| gasutils 模块 | 新建 `node/impl/gasutils/gasutils.go` | 无需添加（Venus 无此模块） |
| fallback 策略 | ✅ 先试正常路径，失败后 fallback | ✅ 相同 |
| ephemeral placeholder | ✅ 通过隐式零值发送 | ✅ 相同 |
| 关键差异 | 需要在 VM 接口加一层 | Venus VM 接口已暴露 `ApplyImplicitMessage`，statemanger 直接调用 |

## 3. 迁移步骤

### Phase 1：statemanger 核心变更

1. `pkg/statemanger/call.go`：
   - `callInternal` 新增 `skipSenderValidation bool` 参数
   - sender 不存在时 → ephemeral placeholder（通过 `ApplyImplicitMessage` 创建零值 actor）
   - sender 存在但非 account actor 时 → 走 `ApplyImplicitMessage`
   - 新增 `ApplyOnStateWithGasSkipSenderValidation` 和 `CallWithGasSkipSenderValidation` 公开方法
   - 更新所有现有调用的签名（传 `false`）

### Phase 2：ETH API 层变更

2. `app/submodule/eth/eth_api.go`：
   - 新增 `applyMessageSkipSenderValidation` 辅助方法
   - `EthCall` 增加 fallback 逻辑
   - `EthEstimateGas` / `gasSearch` 增加 fallback 逻辑
   - 新增 `isSenderValidationError` 辅助函数

### Phase 3：测试

3. 编写测试覆盖以下场景：
   - 正常 account sender → 行为不变
   - EVM 合约地址作为 sender → fallback 生效
   - 不存在的地址作为 sender → fallback 生效
   - 普通错误（如 gas 不足）→ 不触发 fallback

### Phase 4：集成验证

4. 构建并本地运行，通过 `eth_call` RPC 测试合约地址调用

## 4. 验收方案

### 4.1 验收标准

1. **正常 sender 行为不变**：现有 account sender 的 `eth_call`/`eth_estimateGas` 返回结果与改动前一致
2. **EVM 合约 sender 可用**：`eth_call` 从 EVM 合约地址发起模拟调用能正常执行
3. **不存在地址 sender 可用**：`eth_estimateGas` 从不存在的地址发起能正常返回 gas 估算
4. **错误隔离 clean**：非 sender 验证错误的场景不会误触发 fallback
5. **编译通过**：`make build` 无错误

### 4.2 验收步骤

1. `make build` 编译通过
2. 运行 `make test` 全部通过（尤其是 statemanger 和 eth 相关测试）
3. 手动测试示例：
   ```bash
   # 正常 account sender — 应正常工作
   curl -X POST http://localhost:1234/rpc/v1 -d '{
     "jsonrpc":"2.0","method":"eth_call","params":[{"from":"0x...","to":"0x...","data":"0x..."},"latest"],"id":1
   }'
   
   # EVM 合约地址作为 sender — 应成功执行
   curl -X POST http://localhost:1234/rpc/v1 -d '{
     "jsonrpc":"2.0","method":"eth_call","params":[{"from":"0x<合约地址>","to":"0x...","data":"0x..."},"latest"],"id":1
   }'
   
   # 不存在的地址作为 sender — 应成功返回 gas 估算
   curl -X POST http://localhost:1234/rpc/v1 -d '{
     "jsonrpc":"2.0","method":"eth_estimateGas","params":[{"from":"0x<不存在地址>","to":"0x...","data":"0x..."},"latest"],"id":1
   }'
   ```

## 5. 参考资料

- [Lotus PR #13470 — feat: eth_call/eth_estimateGas skip sender validation (原始设计)](https://github.com/filecoin-project/lotus/pull/13470)
- [Lotus PR #13724 — chore: stmgr: eth_call skip sender validation refactor (最终合并版本)](https://github.com/filecoin-project/lotus/pull/13724)
- [Venus VM 接口定义 — `vmcontext/types.go`](./../../pkg/vmcontext/types.go)
- [Venus statemanger — `pkg/statemanger/call.go`](./../../pkg/statemanger/call.go)
- [Venus ETH API — `app/submodule/eth/eth_api.go`](./../../app/submodule/eth/eth_api.go)
