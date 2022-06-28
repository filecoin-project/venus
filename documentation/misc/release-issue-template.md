# Venus X.Y.Z Release

## 🚢 预计发布时间

<!-- 版本发布时间 -->

## 🤔 版本注意事项

<!-- 针对这个版本需要申明的注意事项 -->

## ✅ 常规检查项

准备:

  - [ ] 从上个稳定版本中`fork`出`release/vX.Y.Z`分支；按[分支管理规范](https://github.com/ipfs-force-community/dev-guidances/blob/master/%E8%B4%A8%E9%87%8F%E7%AE%A1%E7%90%86/%E4%BB%A3%E7%A0%81/git%E4%BD%BF%E7%94%A8/%E5%88%86%E6%94%AF%E7%AE%A1%E7%90%86%E8%A7%84%E8%8C%83.md)进行分支开发
  - [ ] 把`master`中需要的功能特性`PR`合并至`release/vX.Y.Z`分支
    
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
    - `Metrics`报告（待开发）
        - Block validation time
        - Memory / CPU usage
        - Number of goroutines
        - IPLD block read latency
        - Bandwidth usage
    - [ ] 如果有一项比原来有很大的差距，调查并修复
  - 确认以下工作流 (Z版本可选；X、Y版本必选)
    - [ ] 封装一个扇区
    - [ ] 发一个存储订单
    - [ ] 提交一个PoSt
    - [ ] 出块验证，出一个块
    - [ ] Snapdeal验证
    - [ ] 让一个扇区变成faulty，观察是否恢复
- [ ] **阶段 2 - 社区测试**
  - [ ] 社区[Venus Master](https://filecoinproject.slack.com/archives/C03B30M20N7)测试
  - [ ] 新功能特性，配置变化等等的文档撰写

- [ ] **阶段 3 - 社区生产测试**
  - [ ] 更新[CHANGELOG.md](https://github.com/filecoin-project/venus/blob/master/CHANGELOG.md)
  - [ ] 邀请更多社区成员参与测试
    
- [ ] **阶段 4 - 发版**
  - [ ] 最终准备
    - [ ] 确认`version.go`已更新
    - [ ] 准备changelog
    - [ ] `tag`为`vX.Y.Z`
    - [ ] 创建新版本的discussion讨论帖


- [ ] **发版后**
  - [ ] 按需更新[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-template.md)模版
  - [ ] 使用[release-issue-templat.md](https://github.com/filecoin-project/venus/blob/master/documentation/misc/release-issue-templat.md)模版创建下一个发版issue
