## 1. 同步模块
整个同步子系统有三个主要模块组成。
1. 第一层`app/submodule/syncer`模块。
   > 模块主要有两个功能。一个是对外同步API逻辑实现。
    另一个是通过订阅 主题，获取区块，然后调用 `SendGossipBlock()`
    方法，驱动第二层`pkg/chainsync`模块进行数据的同步。
2. 第二层 `pkg/chainsync`模块。
    > 本模块主要有三个子模块: `types`, `dispatcher`, `syncer`。
    `types`模块主要是定义一个 `TargetTracker`类，是一个优先级队列类。
    `dispatcher`模块主要有两个goroutine在调度目标区块到 `TargetTracker`对象。
    一个goroutine主要由 `processIncoming()`方法处理目标区块到TargetTracker对象。
    另一个goroutine由 `syncWorker()`方法通过`selectTarget()`方法把目标区块选择出来，
    然后通过 `HandleNewTipSet()`进行同步。
    `syncer`是真正同步的模块。通过调用第三层模块，获取真正的区块信息。
    `syncOne`会调用区块执行器，来执行区块消息，进行状态更新。
3. 第三层 `pkg/net/exchange`模块。
   > 本模块实现了 `/fil/chain/xchg/0.0.1`协议。blocksync协议是request/response协议。
   协议规定了请求和响应的消息格式和错误码。
    Venus和Lotus都实现了此协议，所以才能两个客户端才能进行区块同步。
    

## 2. 同步流程
`venus/app/submodule/syncer/syncer_submodule.go`的`Start()`方法会启动 一个for循环，
不停的接受订阅`/fil/blocks/#{networkName}`的区块信息，
然后调用`handleIncomingBlocks()`方法处理收到的区块。处理方法逻辑也非常简单，
只有两个动作，一个是把收到的区块header存储到数据库。
另一动作是开启一个goroutine，在goroutine里面请求一下header对应的messages（但是不存储，奇怪，为什么不存储呐？），
然后把header通过调用`pkg/chainsync/dispatcher/dispatcher.go::SendGossipBlock()`方法把区块header广播出去。

不管是`SendGossipBlock()`还是`SendOwnBlock()`，两个方法都是把要发送出去的参数放进`imcomming`的一个channel，
然后`processingIncoming()`会通过for循环不停的读取`incomming`channel的信息，
然后把信息通过调用`pkc/chainsync/types/target_tracker.go::Add()`方法，
把`Target`对象放入`TargetTracker`对象，这个对象主要作用是一个优先级队列。

`pkg/chainsync/dispatcher/dispatcher.go::syncWoker()`方法是在系统启动时就开启的一个for循环goroutine，
方法会监听一个target的channel，每当有新的target通过`target_tracker::Add()`加入到队列时，就会触发channel，
`syncWorkder()`方法内部就会调用`selectTarget()`来获取优先级最高的target，
然后开启一个goroutine去调用`syncer.HandleNewTipSet()`方法来获取一系列tipset，
对tipset的block进行验证，计算里面的messsage，修改区块状态。

## 3. 同步事件
1. `venus-miner`通过调用`SyncSubmitBlock`API 出块时。
2. 订阅`/fil/blocks/{networkName}`主题收到区块时。
3. 新peer链接时，通过hello协议，会通知新peer的最新高度。

## 4. 同步模式
因为状态数据量太大，如果要从某个高度才开始进行执行区块同步的话，
需要链下从Filecoin官方下载快照，然后用导入命令的方式同步到指定块高。
链上现在只支持一种同步模式，就是从peer获取区块，然后执行消息，更改状态信息。
Venus支持多线程的同步多个Target，线程的最大数通过`maxCount`字段来控制。
当下默认值是1，可以通过`SetConcurrent`API进行修改。
现在Venus还不支持轻节点同步模式。
Target类有一个State状态字段，但本字段只是用于Target对象的选择。共识算法无法感知节点是否在同步状态。