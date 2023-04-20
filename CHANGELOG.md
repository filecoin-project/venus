# venus changelog

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
