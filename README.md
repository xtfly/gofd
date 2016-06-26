GoFD
==========

## 简介

GoFD是一个使用Go语言开发，集中控制的文件分发系统，用于在一个大规模业务系统文件分发。
系统采用C/S结构，S端负责接收分发任务，把任务下发给C端，C端之间采用P2P技术节点之间共享加速下载。
P2P部分代码参考了[Taipei-Torrent](https://github.com/jackpal/Taipei-Torrent/)。
P2P是一个简化版本的BT协议实现，只使用了其中四个消息（HAVE，REQUEST，PIECE, BITFIELD），并不与BT协议兼容。

与BT软件不一样的是，GoFD是一个P2SP系统，固定存在S端，用于做文件下载源，S端也是集中控制端。
S端与BT的Tracker机制也不一样，它不会维护节点的已下载的文件信息。C端下载文件完成之后也不会再做为种子。
节点之间也没有BT的激励机制，所以没有CHOKE与UNCHOKE消息。

GoFD目前正在开发，功能沿未完成。


## 测试

curl -v -l -H "Content-type: application/json" -X POST -d '{"id":"1","dispatchFiles":["/Users/xiao/2.pic_hd.jpg"],"destIPs":["127.0.0.1"]}' http://127.0.0.1:45000/api/v1/server/tasks
