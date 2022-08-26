# Venus X.Y.Z Release

## 🚢 预计发布时间

<!-- 版本发布时间 -->

## 🤔 版本注意事项

<!-- 针对这个版本需要申明的注意事项 -->

<!-- 需要特别注意的issue，PR等等 -->

## ✅ 常规检查项

准备:

  - [ ] 从`master`拉出发布分支（3选1）
    - [ ] 正式版`release/vX.Y.Z`
    - [ ]`rc`版`release/vX.Y.Z-rc[x]`
    - [ ]`pre-rc`版`release/vX.Y.Z-pre-rc[x]`
  - [ ] 依照[发版规则](https://github.com/ipfs-force-community/dev-guidances/blob/master/%E9%A1%B9%E7%9B%AE%E7%AE%A1%E7%90%86/Venus/%E7%89%88%E6%9C%AC%E5%8F%91%E5%B8%83%E7%AE%A1%E7%90%86.md)递进`master`上的版本号，并更新发布分支中`version.go`的版本号
  - [ ] 依照[分支管理规范](https://github.com/ipfs-force-community/dev-guidances/blob/master/%E8%B4%A8%E9%87%8F%E7%AE%A1%E7%90%86/%E4%BB%A3%E7%A0%81/git%E4%BD%BF%E7%94%A8/%E5%88%86%E6%94%AF%E7%AE%A1%E7%90%86%E8%A7%84%E8%8C%83.md)进行分支开发；如有重大`bug`修复需要从`master`中并入分支，可以通过[backport](https://github.com/filecoin-project/lotus/pull/8847)的方式合并至`release/vX.Y.Z`分支

<!-- 
关于backport解释：

Lotus方面backport指master的pr合到`release/vX.Y.Z`, Venus基于master的话，backport的意义可能和lotus不一样。

@SimleCode补充backport：

1. 稳定版本(指vX.Y.Z)有bug，意味着master分支也会有相应的问题，可以考虑先把修复代码合到 release/vX.Y.Z，待测试及版本发布后通过backport方式合到master
2. rc 及 pre-rc 的bug，可以在rc的基础上发一个rc+1版本，若该rc已合到master，则rc+1需要合到master，反之则不需要

具体举例：当需要发版时，建立标题为，chore: backport: xxxx, xxxx... 的PR。用于把master上的一些bug修复的PR合并回release/vX.Y.Z分支。xxxx为bug修复的PR号码。参考：https://github.com/filecoin-project/lotus/pull/8847（注：参考中为一个feat非bug修复）
-->

测试:

- [ ] **阶段 0 - 自动化测试**
  - 自动化测试
    - [ ] CI: 通过所有CI

- [ ] **阶段 1 - 自测**
  - 升级dev测试环境
    - [ ] 检查节点同步情况
  - 升级预生产环境
    - [ ] （可选）检查节点同步情况
  - 确认以下工作流 (如是Z版本，此项可选；如是X、Y版本，此项为必选)
    - [ ] 封装一个扇区
    - [ ] 发一个存储订单
    - [ ] 提交一个PoSt
    - [ ] 出块验证，出一个块
    - [ ] Snapdeal验证
    - [ ] （可选）让一个扇区变成faulty，观察是否恢复
- [ ] **阶段 2 - 社区Beta测试**
  - [ ] （可选）社区[Venus Master](https://filecoinproject.slack.com/archives/C03B30M20N7)测试
  - [ ] 新功能特性，配置变化等等的文档撰写
    
- [ ] **阶段 3 - 发版**
  - [ ] 最终准备
    - [ ] 确认`version.go`已更新新版本号
    - [ ] 准备changelog
    - [ ] `tag`版本（3选1）
      - [ ]正式版`vX.Y.Z`
      - [ ]rc版`vX.Y.Z-rc[x]`，并标记为`pre-release`
      - [ ]pre-rc版`vX.Y.Z-pre-rc[x]`，并标记为`pre-release`
    - [ ] 版本发布至`releases` <!-- 注：[github](https://github.com/filecoin-project/venus/releases)有区分`tag`和`releases`）-->
    - [ ] （可选）检查是否有`PR`单独提交至`release/vX.Y.Z`分支，并提交`Release back to master`的`PR`
    - [ ] （可选）创建新版本的discussion讨论帖

<!-- 
关于Release back to master解释：

在开发release/vX.Y.Z分支的过程中，可能有些PR只提交了release/vX.Y.Z，但是没有合并至master，例如 升级epoch，bug修复，版本提升等等。

那么当发版结束时，需要提交题为，chore: releases back to master的PR。把只合并到release/vX.Y.Z分支的PR合回master。参考：https://github.com/filecoin-project/lotus/pull/8929
-->

- [ ] **发版后**
  - [ ] （可选）按需更新[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-template.md)模版
  - [ ] （可选）使用[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-templat.md)模版创建下一个发版issue
