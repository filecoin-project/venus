# venus changelog

## v1.14.0

* chore: update sophon-auth

## v1.14.0-rc6

* opt: migration: set premigration to 90 minute [[#6217](https://github.com/filecoin-project/venus/pull/6217)]
* fix: light-weight patch to fix calibrationnet again by removing move_partitions from built-in actors [[#6223](https://github.com/filecoin-project/venus/pull/6223)]

## v1.14.0-rc5

fix: nv21fix migration: correctly upgrade system actor [[#6215](https://github.com/filecoin-project/venus/pull/6215)]

## v1.14.0-rc4

chore: update to builtin-actors v12.0.0-rc.2 [[#6207](https://github.com/filecoin-project/venus/pull/6207)]

## v1.14.0-rc3

* chore: reset butterflynet [[#6191](https://github.com/filecoin-project/venus/pull/6191)]
* feat: add network info command [[#6193](https://github.com/filecoin-project/venus/pull/6193)]
* chore: update go-state-types v0.12.5 [[#6194](https://github.com/filecoin-project/venus/pull/6194)]

## v1.14.0-rc1

* fix duplicate actor events / 解决以太坊event 日志重复问题 [[#6104](https://github.com/filecoin-project/venus/pull/6104)]
* Fix/wallet balance cmd [[#6107](https://github.com/filecoin-project/venus/pull/6107)]
* add check before write badger / 添加检查已经存在，在写入之前 [[#6110](https://github.com/filecoin-project/venus/pull/6110)]
* set gcConfidence = 2 * constants.ForkLengthThreshold /  设置gcConfidence的值为2 * constants.ForkLengthThreshold [[#6113](https://github.com/filecoin-project/venus/pull/6113)]
* feat: Add EIP-1898 support needed for The Graph compatibility [[#6109](https://github.com/filecoin-project/venus/pull/6109)]
* fix: add existing nonce error [[#6111](https://github.com/filecoin-project/venus/pull/6111)]
* feat: api: add SyncIncomingBlocks [[#6115](https://github.com/filecoin-project/venus/pull/6115)]
* feat: Implement a tooling for slasher [[#6119](https://github.com/filecoin-project/venus/pull/6119)]
* chore: Output log when FVM_CONCURRENCY value is too large [[#6120](https://github.com/filecoin-project/venus/pull/6120)]
* fix: Print only live sectors by default [[#6123](https://github.com/filecoin-project/venus/pull/6123)]
* feat: migrate to boxo [[#6121](https://github.com/filecoin-project/venus/pull/6121)]
* Auto-resume interrupted snapshot imports / 自动重链快照导入失败时 [[#6117](https://github.com/filecoin-project/venus/pull/6117)]
* fix: DecodeRLP can panic [[#6125](https://github.com/filecoin-project/venus/pull/6125)]
* feat: vm: allow raw cbor in state and use the new go-multicodec [[#6126](https://github.com/filecoin-project/venus/pull/6126)]
* Feat/invert validation switch checks [[#6124](https://github.com/filecoin-project/venus/pull/6124)]
* optimize LoadTipSetMessage / 优化LoadTipSetMessage [[#6118](https://github.com/filecoin-project/venus/pull/6118)]
* Feat/nv skeleton [[#6134](https://github.com/filecoin-project/venus/pull/6134)]
* feat: include extra messages in ComputeState InvocResult [[#6135](https://github.com/filecoin-project/venus/pull/6135)]
* Feat/add eth syncing api [[#6136](https://github.com/filecoin-project/venus/pull/6136)]
* Feat/make it optional to apply msg from ts when call with gas [[#6138](https://github.com/filecoin-project/venus/pull/6138)]
* feat: Plumb through a proper Flush() method on all blockstores [[#6137](https://github.com/filecoin-project/venus/pull/6137)]
* chore: Use type proxies instead of versioned GST imports [[#6139](https://github.com/filecoin-project/venus/pull/6139)]
* feat: add PieceCID to StorageDealQueryParams [[#6143](https://github.com/filecoin-project/venus/pull/6143)]
* fix: chain: cancel long operations upon ctx cancelation [[#6144](https://github.com/filecoin-project/venus/pull/6144)]
* feat: vm: switch to the new exec trace format [[#6148](https://github.com/filecoin-project/venus/pull/6148)]
* feat: eth trace api [[#6149](https://github.com/filecoin-project/venus/pull/6149)]
* fix: typos [[#6146](https://github.com/filecoin-project/venus/pull/6146)]
* chore(deps): bump actions/checkout from 3 to 4 [[#6151](https://github.com/filecoin-project/venus/pull/6151)]
* refactor: verify and save block messages when receive new bock [[#6150](https://github.com/filecoin-project/venus/pull/6150)]
* opt: generate datacap types to venus-shared [[#6152](https://github.com/filecoin-project/venus/pull/6152)]
* feat: add id to MinerDeal [[#6158](https://github.com/filecoin-project/venus/pull/6158)]
* Feat/synthetic po rep [[#6161](https://github.com/filecoin-project/venus/pull/6161)]
* feat: add asc field to StorageDealQueryParams [[#6163](https://github.com/filecoin-project/venus/pull/6163)]
* Chore/updates [[#6169](https://github.com/filecoin-project/venus/pull/6169)]
* fix: support exchange protocols [[#6171](https://github.com/filecoin-project/venus/pull/6171)]
* chore: remove ioutil [[#6174](https://github.com/filecoin-project/venus/pull/6174)]
* feat: add bootstrap for mainnet [[#6173](https://github.com/filecoin-project/venus/pull/6173)]
* Chore/updates [[#6175](https://github.com/filecoin-project/venus/pull/6175)]
* chore: Remove PL's european bootstrap nodes from mainnet.pi [[#6177](https://github.com/filecoin-project/venus/pull/6177)]
* fix: update GenesisNetworkVersion to Version20 [[#6178](https://github.com/filecoin-project/venus/pull/6178)]
* fix: not use chain randomness [[#6179](https://github.com/filecoin-project/venus/pull/6179)]
* chore(deps): bump golang.org/x/net from 0.11.0 to 0.17.0 in /venus-devtool [[#6181](https://github.com/filecoin-project/venus/pull/6181)]
* chore(deps): bump golang.org/x/net from 0.11.0 to 0.17.0 [[#6180](https://github.com/filecoin-project/venus/pull/6180)]
* Opt/limit partitions [[#6182](https://github.com/filecoin-project/venus/pull/6182)]
* feat: migrate boostrap for config [[#6183](https://github.com/filecoin-project/venus/pull/6183)]
* feat: implement nv21 [[#6114](https://github.com/filecoin-project/venus/pull/6114)]
* fix: not save data to event table [[#6184](https://github.com/filecoin-project/venus/pull/6184)]
* chore: update deps [[#6185](https://github.com/filecoin-project/venus/pull/6185)]
* chore: Set calibration upgrade height [[#6186](https://github.com/filecoin-project/venus/pull/6186)]

## v1.13.0
## v1.13.0-rc1

### New Features

* Feat/add sign type to wallet types [[#6036](https://github.com/filecoin-project/venus/pull/6036)]
* feat: upgrade builtin actors to v1.23.1 [[#6039](https://github.com/filecoin-project/venus/pull/6039)]
* feat: Increase the environment variable to skip checking whether mine… [[#6055](https://github.com/filecoin-project/venus/pull/6055)]
* feat: modify query params for message and deal [[#6066](https://github.com/filecoin-project/venus/pull/6066)]
* feat: add bootstrap [[#6084](https://github.com/filecoin-project/venus/pull/6084)]
* feat: add proxy interface to gateway [[#6089](https://github.com/filecoin-project/venus/pull/6089)]
* feat(market): filter deals by sector lifetime [[#6093](https://github.com/filecoin-project/venus/pull/6093)]
* feat: set tipset to the given epoch by default [[#6099](https://github.com/filecoin-project/venus/pull/6099)]
* feat: remove MinPeerThreshold in bootstrap config because it is not used /删除MinPeerThreshold字段从bootstrap配置，没有用到 [[#6063](https://github.com/filecoin-project/venus/pull/6063)]
* feat: make execution trace configurable via env variable venus / 通过VENUS_EXEC_TRACE_CACHE环境变量谁知trace缓存大小 [[#6100](https://github.com/filecoin-project/venus/pull/6100)]

### Optimizations

* opt: Adjust size flag to string type [[#6102](https://github.com/filecoin-project/venus/pull/6102)]

### Bug Fixes

* fix: Unsubscribe required on exit [[#6103](https://github.com/filecoin-project/venus/pull/6103)]
* fix: ethtypes: handle length overflow case / 处理rlp长度越界问题 [[#6101](https://github.com/filecoin-project/venus/pull/6101)]

### Documentation and Chores

* doc: add config desc / 添加config注释 [[#6062](https://github.com/filecoin-project/venus/pull/6062)]
* doc: add design doc of sync/添加同步设计文档 [[#5989](https://github.com/filecoin-project/venus/pull/5989)]

* chore: correct god eye url and prioritize ftp [[#6031](https://github.com/filecoin-project/venus/pull/6031)]
* chore(deps): bump actions/setup-go from 3 to 4 [[#6029](https://github.com/filecoin-project/venus/pull/6029)]
* chore(deps): bump TheDoctor0/zip-release from 0.6.0 to 0.7.1 [[#6028](https://github.com/filecoin-project/venus/pull/6028)]
* chore: bump github.com/gin-gonic/gin from v1.9.0 to v1.9.1 [[#6037](https://github.com/filecoin-project/venus/pull/6037)]
* chore: update minimum Go version to 1.19 [[#6038](https://github.com/filecoin-project/venus/pull/6038)]
* chore: Update issue template enhancement.yml [[#6046](https://github.com/filecoin-project/venus/pull/6046)]
* chore: fix bug issue template [[#6047](https://github.com/filecoin-project/venus/pull/6047)]
* chore: pick 10971,10955,10934 from lotus [[#6078](https://github.com/filecoin-project/venus/pull/6078)]
* chore(deps): bump github.com/libp2p/go-libp2p from 0.27.5 to 0.27.8 in /venus-devtool [[#6091](https://github.com/filecoin-project/venus/pull/6091)]
* chore(deps): bump github.com/libp2p/go-libp2p from 0.27.5 to 0.27.8 [[#6090](https://github.com/filecoin-project/venus/pull/6090)]
* chore: transport code [[#6097](https://github.com/filecoin-project/venus/pull/6097)]

## v1.12.0

* fix: compatible with older versions [[#6024](https://github.com/filecoin-project/venus/pull/6024)]
* chore: update github actions by @galargh in [[#6027](https://github.com/filecoin-project/venus/pull/6027)]

## v1.12.0-rc1

* fix: update UpgradeHyggeHeight to -21 in forcenet [[#5912](https://github.com/filecoin-project/venus/pull/5912)]
* opt: push all tag to tianyan by [[#5934](https://github.com/filecoin-project/venus/pull/5934)]
* fix: supply release id [[#5938](https://github.com/filecoin-project/venus/pull/5938)]
* feat: change init logic [[#5941](https://github.com/filecoin-project/venus/pull/5941)]
* fix: fix import when less than 900 height [[#5950](https://github.com/filecoin-project/venus/pull/5950)]
* opt: remove perm adapt strategy [[#5953](https://github.com/filecoin-project/venus/pull/5953)]
* feat: message pool: change locks to RWMutexes for performance [[#5964](https://github.com/filecoin-project/venus/pull/5964)]
* feat: mpool: prune excess messages before selection [[#5967](https://github.com/filecoin-project/venus/pull/5967)]
* fix: Fix generating version in the current directory [[#5972](https://github.com/filecoin-project/venus/pull/5972)]
* Opt/remove init option [[#5974](https://github.com/filecoin-project/venus/pull/5974)]
* Fix/not store event [[#5977](https://github.com/filecoin-project/venus/pull/5977)]
* fix: fetch filecoin-ffi failed [[#5981](https://github.com/filecoin-project/venus/pull/5981)]
* feat: add unseal state / 增加了 unseal 的状态类型 [[#5990](https://github.com/filecoin-project/venus/pull/5990)]
* feat: chain: speed up calculation of genesis circ supply [[#5991](https://github.com/filecoin-project/venus/pull/5991)]
* fix: gas estimation: do not special case paych collects [[#5992](https://github.com/filecoin-project/venus/pull/5992)]
* feat: downgrade perm for eth from write to read [[#5995](https://github.com/filecoin-project/venus/pull/5995)]
* feat: verify eth filter's block index is in hex [[#5996](https://github.com/filecoin-project/venus/pull/5996)]
* feat: Network parameters add an Eip155ChainID field [[#5997](https://github.com/filecoin-project/venus/pull/5997)]
* opt: add codecov token [[#5998](https://github.com/filecoin-project/venus/pull/5998)]
* opt: MinerInfo adds the PendingOwnerAddress field [[#5999](https://github.com/filecoin-project/venus/pull/5999)]
* feat: add ExecutionTrace cache [[#6000](https://github.com/filecoin-project/venus/pull/6000)]
* feat: add batch deal for market [[#5829](https://github.com/filecoin-project/venus/pull/5829)]
* check if epoch is negative in GetTipsetByHeight() [[#6008](https://github.com/filecoin-project/venus/pull/6008)]
* chore: rename market to droplet [[#6005](https://github.com/filecoin-project/venus/pull/6005)]
* feat: add vm execution lanes [[#6011](https://github.com/filecoin-project/venus/pull/6011)]
* refactor: market: rename OfflineDealImport to DealsImport [[#6012](https://github.com/filecoin-project/venus/pull/6012)]

## v1.11.1

* fix: windowPoST verify fail [[#1750](https://github.com/filecoin-project/ref-fvm/pull/1750)]

## v1.11.0

This is the stable release of Venus v1.11.0 for the upcoming MANDATORY network upgrade at `2023-04-27T13:00:00Z`, epoch `2809800`. This release delivers the nv19 Lighting and nv20 Thunder network upgrade for mainnet.
The Lighting and Thunder upgrade introduces the following Filecoin Improvement Proposals (FIPs), delivered by builtin-actors v11 (see actors [v11.0.0](https://github.com/filecoin-project/builtin-actors/releases/tag/v11.0.0)):

- [FIP 0060](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0060.md) - Thirty day market deal maintenance interval
- [FIP 0061](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0061.md) - WindowPoSt grindability fix
- [FIP 0062](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0062.md) - Fallback method handler for multisig actor

### New Features

- add interface for health check /各组件增加状态检查接口 [[#4887](https://github.com/filecoin-project/venus/issues/4887)]
- ffi validation parallelism / ffi 验证平行计算 [[#5838](https://github.com/filecoin-project/venus/issues/5838)]
- chain fetch faster / chain fetch 优化 [[#5814](https://github.com/filecoin-project/venus/issues/5814)]

## v1.11.0-rc2

### Main Changes

* Revert add new-vm-exec-tracer [[#5901](https://github.com/filecoin-project/venus/pull/5901)]

## v1.11.0-rc1

### New Features
* feat: add bootstrap peers flag / 添加bootstrap节点的Flag [#5742](https://github.com/filecoin-project/venus/pull/5742)
* feat: add status api to detect api ready / 添加状态检测接口 [#5740](https://github.com/filecoin-project/venus/pull/5740)
* feat: update auth client with token /更新authClient的token授权 [[#5752](https://github.com/filecoin-project/venus/pull/5752)]
* feat: make chain tipset fetching 1000x faster / chain fetch 优化 [[#5824](https://github.com/filecoin-project/venus/pull/5824)]
* feat: get signer deal detail /增加获取单个存储订单和检索订单的接口 [[#5831](https://github.com/filecoin-project/venus/pull/5831)]
* feat: update cache to the new generic version /缓存库版本升级 [[#5857](https://github.com/filecoin-project/venus/pull/5857)]
* feat: add docker push by @hunjixin /增加推送到镜像仓库的功能 [[#5889](https://github.com/filecoin-project/venus/pull/5889)]

### Bug Fixes

* fix: Saving genesis blocks when importing snapshots by @simlecode / 修复删除 badger chain 后导入快照失败 [[#5892](https://github.com/filecoin-project/venus/pull/5892)]
* fix: use FIL pointer to unmarshaltext by @simlecode /设置UnmarshalText方法的接收器为FIL指针 [[#5869](https://github.com/filecoin-project/venus/pull/5869)]
* fix: not sync in 2k network by @hunjixin / 修复2k网络同步失败 [[#5748](https://github.com/filecoin-project/venus/pull/5748)]


## v1.10.1

* 修复 evm 命令部署合约失败 [[#5785](https://github.com/filecoin-project/venus/pull/5785)]
* 修复无法获取扇区的开始和过期时间 [[#5792](https://github.com/filecoin-project/venus/pull/5792)]
* market 增加 miner 管理接口 [[#5794](https://github.com/filecoin-project/venus/pull/5794)]
* 增加 StateCompute 接口 [[#5795](https://github.com/filecoin-project/venus/pull/5795)]
* 优化调用 newEthBlockFromFilecoinTipSet 函数的耗时 [[#5798](https://github.com/filecoin-project/venus/pull/5798)]

## v1.10.0

* 把实际的 vmTracing 传递给 VM [[#5757](https://github.com/filecoin-project/venus/pull/5757)]
* 增加接口 FilecoinAddressToEthAddress [[#5772]](https://github.com/filecoin-project/venus/pull/5772)
* 重构 market 接口 updatedealstatus [[#5778](https://github.com/filecoin-project/venus/pull/5778)]

## v1.10.0-rc4

* 修复保存 MessageReceipt 失败问题 [[#5743](https://github.com/filecoin-project/venus/pull/5743)]

## v1.10.0-rc3

* 调整 force 网络的 Hygge 升级高度
* 增加 bootstrap peers flag
* 增加 RPC接口：EthAddressToFilecoinAddress
* 升级 go-libipfs 版本到 v0.4.1
* 修复 EthGetTransactionCount 接口返回不正确的 nonce
* 调整预迁移开始高度和结束高度

## v1.10.0-rc1

这是 venus v1.10.0 版本（**强制性**）的第一个候选版本，此版本将提供 Hygge 网络升级，并引入 Filecoin 网络版本18。
本次升级的核心是引入[Filecoin Virtual Machine (FVM)’s Milestone 2.1](https://fvm.filecoin.io/)，这将允许EVM兼容合同部署在Filecoin网络上，此次升级首次为Filecoin网络提供了用户可编程性！

Hygge 升级引入了以下 FIPs（Filecoin Improvement Proposals）：

- [FIP-0048](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0048.md): f4 Address Class
- [FIP-0049](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0049.md): Actor events
- [FIP-0050](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0050.md): API between user-programmed actors and built-in actors
- [FIP-0054](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0054.md): Filecoin EVM runtime (FEVM)
- [FIP-0055](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0055.md): Supporting Ethereum Accounts, Addresses, and Transactions
- [FIP-0057](https://github.com/filecoin-project/FIPs/blob/master/FIPS/fip-0057.md): Update gas charging schedule and system limits for FEVM

### Filecoin Ethereum Virtual (FEVM)

Filecoin 以太坊虚拟机（FEVM）构建在 Skyr v16升级中引入的基于WASM的执行环境之上。主要功能是参与Filecoin网络的任何人都可以将自己的EVM兼容合约部署到区块链上，并调用它们。

### Calibration nv18 Hygge Upgrade

此候选版本将 calibration Hygge 升级高度设置为：**322354**，具体升级时间为：**2023-02-21T16:30:00Z**

### Ethereum JSON RPC API

Eth APIs 只在 venus v1 API提供支持，v0 API 不提供支持。有以下两种方法开启Eth APIs：

* 把节点配置文件（config.json）中 `fevm.EnableEthRPC` 设置为true
* 在启动节点前设置环境变量：export VENUS_FEVM_ENABLEETHRPC=1
