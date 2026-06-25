# eth filter block range 限制增强

## 1. 目标和核心功能

### 目标

迁移 Lotus PR [#13561](https://github.com/filecoin-project/lotus/pull/13561)（fix(eth): tighten block range handling for filter APIs）到 Venus，为 `trace_filter` 和 `eth_getLogs` API 增加统一的块范围限制和标准 Ethereum JSON-RPC 错误码。

### 核心功能

- **新增 `EthTraceFilterMaxBlockRange` 配置** — 限制 `trace_filter` 的单次请求可遍历的最大区块数，默认 100
- **新增标准错误类型 `ErrBlockRangeExceeded`** — 使用标准 Ethereum JSON-RPC 错误码 `-32005`（limit exceeded）
- **`EthTraceFilter` 添加块范围检查** — 在遍历区块前检查范围是否超出配置上限，超出则立即返回 `ErrBlockRangeExceeded`
- **`parseBlockRange` 错误统一** — 将 `eth_getLogs` 中块范围超限的错误消息统一为 `ErrBlockRangeExceeded`

## 2. 技术方案

### 2.1 技术栈

- **语言**: Go
- **错误类型模式**: 沿用 Venus 现有的自定义错误模式（参考 `ErrNullRound`），不引入 Lotus 的 `jsonrpc.NewErrors` / `RPCErrors.Register` 机制
- **配置体系**: 沿用 `FevmConfig` 现有配置结构

### 2.2 技术选型与关键决策

#### 1. 错误类型设计

| 选项 | 说明 | 结论 |
|------|------|------|
| 完全对齐 Lotus（`FromJSONRPCError` / `ToJSONRPCError` + `RPCErrors.Register`） | 需要 Venus 引入 Lotus 的 jsonrpc 错误注册机制 | ❌ 不采用 — Venus 没有也不需要使用 Lotus 的 jsonrpc 错误注册体系 |
| 沿用 Venus 现有模式（纯 Go error 类型 + 编译时断言） | 参考 `ErrNullRound`，只实现 `Error()` 和 `Is()` 方法 | ✅ 采用 — 与项目现有风格一致，无额外依赖 |

**调研依据**: Venus 的 `venus-shared/types/api_types.go` 中已有 `ErrNullRound`、`ErrExecutionReverted` 等自定义错误类型，均使用纯 Go error 模式，未使用 Lotus 的 jsonrpc 注册机制。保持一致性。

#### 2. 块范围检查位置

| 选项 | 说明 | 结论 |
|------|------|------|
| Gateway 层检查（Lotus 做法） | Lotus 在 gateway proxy 层新增 `checkEthTraceFilterBlockRange` | ❌ 不适用 — Venus 没有 gateway proxy 架构 |
| 在 `EthTraceFilter` 实现中直接检查 | 在 `for blkNum := fromBlock; blkNum <= toBlock` 循环前插入检查 | ✅ 采用 — 直接有效，架构匹配 |

**调研依据**: Venus 的 `EthTraceFilter` 实现在 `app/submodule/eth/eth_api.go` 中，是一个单体方法，没有 gateway proxy 层。在遍历循环前直接检查 range 是最简洁的等效实现。

#### 3. `parseBlockRange` 错误统一

| 选项 | 说明 | 结论 |
|------|------|------|
| 全部改为 `ErrBlockRangeExceeded` | 完全对齐 Lotus，统一错误处理 | ✅ 采用 — 提升 API 一致性 |
| 不改动 `parseBlockRange` | 保留 `fmt.Errorf` 错误 | ❌ 不采用 — eth_getLogs 仍然返回非标准错误 |

**调研依据**: Lotus 统一了 `trace_filter` 和 `eth_getLogs` 的块范围超限错误类型。Venus 的 `parseBlockRange` 在 `eth_getLogs` 和 `eth_newFilter` 中均有使用，统一错误类型有助于 API 标准化。

#### 4. 默认值

| 环境 | Lotus 默认值 | Venus 方案 |
|------|:-----------:|:----------:|
| `EthTraceFilterMaxBlockRange` | 100（gateway 层） | **100**（节点层） |
| `MaxFilterHeightRange`（现有） | 2880 | 2880（不变） |

**理由**: trace_filter 代价远高于 eth_getLogs（需要重放整个区块的 trace），因此块范围上限更保守。100 是合理的默认值。

### 2.3 具体修改清单

#### 文件 1: `venus-shared/types/api_types.go` — 新增错误类型

**改动内容**: 在文件末尾（参考 `ErrNullRound` 的位置）新增 `ErrBlockRangeExceeded` 类型

```go
// ErrBlockRangeExceeded signals that a request's block range exceeds the configured
// maximum. Returned with the standard Ethereum JSON-RPC -32005 "limit exceeded" code.
type ErrBlockRangeExceeded struct {
	Message string
}

func NewErrBlockRangeExceeded(maxBlockRange, given uint64) *ErrBlockRangeExceeded {
	return &ErrBlockRangeExceeded{
		Message: fmt.Sprintf("block range exceeds maximum of %d (got %d)", maxBlockRange, given),
	}
}

func (e *ErrBlockRangeExceeded) Error() string {
	return e.Message
}

// Is performs a non-strict type check so errors.Is works regardless of field values.
func (e *ErrBlockRangeExceeded) Is(target error) bool {
	_, ok := target.(*ErrBlockRangeExceeded)
	return ok
}
```

同时在编译时断言区域添加：
```go
_ error = (*ErrBlockRangeExceeded)(nil)
```

**位置**:
- 错误类型定义：大约 `api_types.go:635`（`ErrNullRound` 之后）
- 编译时断言：大约 `api_types.go:571`（现有断言块中追加）

#### 文件 2: `pkg/config/config.go` — 新增配置项

**改动内容**: 在 `FevmConfig` 结构体 `EthTraceFilterMaxResults` 字段后添加：

```go
// EthTraceFilterMaxBlockRange sets the maximum block range allowed for EthTraceFilter
// requests. Trace replays are significantly more expensive than log lookups.
EthTraceFilterMaxBlockRange uint64 `json:"ethTraceFilterMaxBlockRange"`
```

在 `newFevmConfig()` 默认值中添加：

```go
EthTraceFilterMaxBlockRange: 100,
```

**位置**:
- 结构体字段：`config.go:503`（`EthTraceFilterMaxResults` 之后）
- 默认值：`config.go:531`（默认值块中）

#### 文件 3: `app/submodule/eth/eth_api.go` — EthTraceFilter 添加范围检查

**改动内容**: 在 `EthTraceFilter` 方法的 `fromBlock`/`toBlock` 解析完成后、遍历循环之前，插入块范围检查：

```go
// Validate block range
maxBlockRange := a.em.cfg.FevmConfig.EthTraceFilterMaxBlockRange
if maxBlockRange > 0 && toBlock > fromBlock && uint64(toBlock-fromBlock) > maxBlockRange {
    return nil, types.NewErrBlockRangeExceeded(maxBlockRange, uint64(toBlock-fromBlock))
}
```

**位置**: `eth_api.go:1461`（`traceCounter` 初始化之前，即 parseBlock 得到 fromBlock/toBlock 之后）

**注意**: 需要添加 `"github.com/filecoin-project/venus/venus-shared/types"` import（如已有则不需要）。

#### 文件 4: `app/submodule/eth/eth_event_api.go` — parseBlockRange 错误统一

**改动内容**: 修改 `parseBlockRange` 函数中的 3 处 `fmt.Errorf("invalid epoch range: ...")` 为 `NewErrBlockRangeExceeded`：

1. `"invalid epoch range: to block is too far in the future (maximum: %d)"`
   → `NewErrBlockRangeExceeded(maxRange, ...)`

2. `"invalid epoch range: from block is too far in the past (maximum: %d)"`
   → `NewErrBlockRangeExceeded(maxRange, ...)`

3. `"invalid epoch range: range between to and from blocks is too large (maximum: %d)"`
   → `NewErrBlockRangeExceeded(maxRange, ...)`

**注意**: 第 2 种情况（from 在 past 中太远）和第 3 种情况（range 太大）的 `given` 值需要计算正确。

**位置**: `eth_event_api.go:360-372`

#### 文件 5: `app/submodule/eth/eth_test.go` — 测试更新

**改动内容**: 更新 `parseBlockRange` 测试用例中的 `errStr` 匹配，从匹配子字符串改为匹配 `ErrBlockRangeExceeded`：

| 测试用例 | 当前 errStr | 改为 |
|----------|-----------|------|
| "fails when both are specified and range is greater than max allowed range" | `"too large"` | `"block range exceeds maximum"` |
| "fails when min is specified and range is greater than max allowed range" | `"too far in the past"` | `"block range exceeds maximum"` |
| "fails when max is specified and range is greater than max allowed range" | `"too large"` | `"block range exceeds maximum"` |

同时可以使用 `require.True(t, errors.Is(err, &types.ErrBlockRangeExceeded{}))` 来断言错误类型。

**位置**: `eth_test.go:40-76`

## 3. 验收方案

### 3.1 验收标准

| 验收项 | 标准 |
|--------|------|
| 编译通过 | `go build ./...` 无错误 |
| 现有测试通过 | `go test ./app/submodule/eth/...` 全部通过 |
| 新增错误类型正常 | `errors.Is(err, &types.ErrBlockRangeExceeded{})` 正确判断 |
| 默认值正确 | `FevmConfig.EthTraceFilterMaxBlockRange` 默认值为 100 |
| 范围检查生效 | trace_filter 请求超出 100 块范围时返回 `ErrBlockRangeExceeded` |
| eth_getLogs 错误统一 | 块范围超限时返回 `ErrBlockRangeExceeded` |

### 3.2 验收步骤

1. **编译验证**:
   ```bash
   cd /tmp/ghost-worktrees/fix/eth-filter-block-range
   go build ./venus-shared/types/...
   go build ./pkg/config/...
   go build ./app/submodule/eth/...
   ```

2. **运行测试**:
   ```bash
   go test ./app/submodule/eth/... -run TestParseBlockRange -v
   go test ./app/submodule/eth/...
   ```

3. **错误类型验证**（通过简单测试确认）:
   ```go
   err := types.NewErrBlockRangeExceeded(100, 200)
   errors.Is(err, &types.ErrBlockRangeExceeded{}) // → true
   err.Error() // → "block range exceeds maximum of 100 (got 200)"
   ```

## 4. 参考资料

- Lotus PR: [fix(eth): tighten block range handling for filter APIs #13561](https://github.com/filecoin-project/lotus/pull/13561)
- EIP-1474: [Ethereum JSON-RPC Error Codes](https://eips.ethereum.org/EIPS/eip-1474) — 定义 `-32005` 为 "limit exceeded"
- Venus 现有错误类型参考: `venus-shared/types/api_types.go` 中的 `ErrNullRound` 实现
- Lotus 变更文件清单:
  - `api/api_errors.go` — 新增 `ELimitExceeded(-32005)` + `ErrBlockRangeExceeded`
  - `gateway/eth_utils.go` — 新增 `checkEthTraceFilterBlockRange`
  - `gateway/node.go` — 新增 `DefaultEthTraceFilterMaxBlockRange` + 配置 option
  - `gateway/proxy_eth_v1.go` / `proxy_v2.go` — proxy 层调用范围检查
  - `node/impl/eth/trace.go` — trace_filter 错误统一
  - `node/impl/eth/events.go` — parseBlockRange 错误统一
  - `node/impl/eth/events_test.go` — 测试更新
