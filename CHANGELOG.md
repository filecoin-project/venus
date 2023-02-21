# venus changelog


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
