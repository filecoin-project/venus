# Venus X.Y.Z Release

## 🚢 预计发布时间

<!-- 版本发布时间 -->

## 🤔 版本注意事项

<!-- 针对这个版本需要申明的注意事项 -->

<!-- 需要特别注意的issue，PR等等 -->

## ✅ 常规检查项

准备:

  - [ ] 从上个稳定版本中`fork`出`release/vX.Y.Z`分支；按[分支管理规范](https://github.com/ipfs-force-community/dev-guidances/blob/master/%E8%B4%A8%E9%87%8F%E7%AE%A1%E7%90%86/%E4%BB%A3%E7%A0%81/git%E4%BD%BF%E7%94%A8/%E5%88%86%E6%94%AF%E7%AE%A1%E7%90%86%E8%A7%84%E8%8C%83.md)进行分支开发
  - [ ] 把`master`中需要的功能特性`PR`通过[backport](https://github.com/filecoin-project/lotus/pull/8847)的方式合并至`release/vX.Y.Z`分支

<!-- 

关于backport解释：

1. 研发团队首先通过feat/xxxx分支开发所需功能特性，并合并至master；参考：https://github.com/filecoin-project/lotus/pull/8838；

2. 当需要发版时，建立标题为，chore: backport: xxxx, xxxx... 的PR。用于把上述功能特性PR合并至release/vX.Y.Z分支。xxxx为功能特性PR的PR号码。参考：https://github.com/filecoin-project/lotus/pull/8847

-->
    
准备RC版本: (可选)

- [ ] `tag`为`vX.Y.Z-rc[x]`
- [ ] 标记为`pre-release`

测试:

- [ ] **阶段 0 - 自动化测试**
  - 自动化测试
    - [ ] CI: 通过所有CI

- [ ] **阶段 1 - 自测**
  - 升级dev测试环境
    - [ ] 检查节点同步情况
  - 升级预生产环境
    - [ ] 检查节点同步情况
  - 确认以下工作流 (Z版本可选；X、Y版本必选)
    - [ ] 封装一个扇区
    - [ ] 发一个存储订单
    - [ ] 提交一个PoSt
    - [ ] 出块验证，出一个块
    - [ ] Snapdeal验证
    - [ ] 让一个扇区变成faulty，观察是否恢复
- [ ] **阶段 2 - 社区Beta测试**
  - [ ] 社区[Venus Master](https://filecoinproject.slack.com/archives/C03B30M20N7)测试
  - [ ] 新功能特性，配置变化等等的文档撰写
    
- [ ] **阶段 3 - 发版**
  - [ ] 最终准备
    - [ ] 确认`version.go`已更新
    - [ ] 准备changelog
    - [ ] `tag`为`vX.Y.Z`
    - [ ] 版本发布至`releases`（注：[github](https://github.com/filecoin-project/venus/releases)有区分`tag`和`releases`）
    - [ ] 检查是否有`PR`单独提交至`release/vX.Y.Z`分支，并提交`Release back to master`的`PR`
    - [ ] 创建新版本的discussion讨论帖

<!-- 

关于Release back to master解释：

在开发release/vX.Y.Z分支的过程中，可能有些PR只提交了release/vX.Y.Z，但是没有合并至master，例如 升级epoch，bug修复，版本提升等等。

那么当发版结束时，需要提交题为，chore: releases back to master的PR。把只合并到release/vX.Y.Z分支的PR合回master。参考：https://github.com/filecoin-project/lotus/pull/8929

-->


- [ ] **发版后**
  - [ ] 按需更新[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-template.md)模版
  - [ ] 使用[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-templat.md)模版创建下一个发版issue
