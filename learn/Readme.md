# ETCD

## 安装

etcd 官方文档：https://etcd.io/docs/

安装：https://github.com/etcd-io/etcd/releases/

```
ETCD_VER=v3.5.0

# choose either URL
GOOGLE_URL=https://storage.googleapis.com/etcd
GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
DOWNLOAD_URL=${GOOGLE_URL}

rm -f /tmp/etcd-${ETCD_VER}-darwin-amd64.zip
rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test

curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-darwin-amd64.zip -o /tmp/etcd-${ETCD_VER}-darwin-amd64.zip
unzip /tmp/etcd-${ETCD_VER}-darwin-amd64.zip -d /tmp && rm -f /tmp/etcd-${ETCD_VER}-darwin-amd64.zip
mv /tmp/etcd-${ETCD_VER}-darwin-amd64/* /tmp/etcd-download-test && rm -rf mv /tmp/etcd-${ETCD_VER}-darwin-amd64

/tmp/etcd-download-test/etcd --version
/tmp/etcd-download-test/etcdctl version
/tmp/etcd-download-test/etcdutl version
mv /tmp/etcd-download-test/etcd /usr/local/bin/
```

安装goreman：

```
go get github.com/mattn/goreman
go install github.com/mattn/goreman
export PATH=$HOME/go/bin:$PATH
```

启动测试集群：修改etcd 源码下的Procfile 文件，将bin/etcd 改为/usr/local/bin/ ; 然后运行 

```
goreman -f Procfile start
```

### 基础架构

<img src="./img/etcd架构图.png" alt="etcd架构图" style="zoom:45%;" />

- **Client 层**：Client 层包括 client v2 和 v3 两个大版本 API 客户端库，提供了简洁易用的 API，同时支持**负载均衡**、**节点间故障自动转移**，可极大降低业务使用 etcd 复杂度，提升开发效率、服务可用性。

- **API 网络层**：API 网络层主要包括 client 访问 server 和 server 节点之间的通信协议。一方面，client 访问 etcd server 的 API 分为 v2 和 v3 两个大版本。**v2 API 使用 HTTP/1.x 协议，v3 API 使用 gRPC 协议**。同时 v3 通过 etcd grpc-gateway 组件也支持 HTTP/1.x 协议，便于各种语言的服务调用。另一方面，**server 之间通信协议，是指节点间通过 Raft 算法实现数据复制和 Leader 选举等功能时使用的 HTTP 协议**。

- **Raft 算法层**：Raft 算法层**实现了 Leader 选举、日志复制、ReadIndex 等**核心算法特性，用于<font color='blue'>*保障 etcd 多个节点间的数据一致性、提升服务可用性等，是 etcd 的基石和亮点*</font>。

  - ReadIndex 线性读：确保最新的数据已经应用到状态机中(3.1 中引入)；

- **功能逻辑层**：**etcd 核心特性实现层**，如典型的 KVServer 模块、MVCC 模块、Auth 鉴权模块、Lease 租约模块、Compactor 压缩模块等，其中 MVCC 模块主要由 treeIndex 模块和 boltdb 模块组成。

  - Quota: (配额检查) 当 etcd server 收到 put/txn 等写请求的时候，会首先检查下当前 etcd db 大小加上你请求的 key-value 大小之和是否超过了配额（quota-backend-bytes）。如果超过了配额，它会产生一个告警（Alarm）请求，告警类型是 NO SPACE，并通过 Raft 日志同步给其它节点，告知 db 无空间了，并将告警持久化存储到 db 中。Apply 模块在执行每个命令的时候，都会去检查当前是否存在 NO SPACE 告警，如果有则拒绝写入.

  - KVServer 模块: 它需要将 put 写请求内容打包成一个提案消息，提交给 Raft 模块。不过 KVServer 模块在提交提案前，还有如下的一系列检查和限速。

    - Preflight Check: 首先，

      - 如果 Raft 模块已提交的日志索引（committed index）比已应用到状态机的日志索引（applied index）超过了 5000，那么它就返回一个"etcdserver: too many requests"错误给 client。
      - 然后它会尝试去获取请求中的鉴权信息，若使用了密码鉴权、请求中携带了 token，如果 token 无效，则返回"auth: invalid auth token"错误给 client。
      - 其次它会检查你写入的包大小是否超过默认的 1.5MB， 如果超过了会返回"etcdserver: request is too large"错误给给 client。

      <img src="./img/KvServer x限速.png" alt="KvServer x限速" style="zoom:35%;" />

    - Propose: 通过一系列检查之后，会生成一个唯一的 ID，将此请求关联到一个对应的消息通知 channel，然后向 Raft 模块发起（Propose）一个提案（Proposal）。向 Raft 模块发起提案后，KVServer 模块会等待此 put 请求，等待写入结果通过消息通知 channel 返回或者超时。etcd 默认超时时间是 7 秒（5 秒磁盘 IO 延时 +2*1 秒竞选超时时间），如果一个请求超时未返回结果，则可能会出现你熟悉的 etcdserver: request timed out 错误。

  - Apply 模块: 

    - 如何异常处理：提交给 Apply 模块执行的提案已获得多数节点确认、持久化，etcd 重启时，会从 WAL 中解析出 Raft 日志条目内容，追加到 Raft 日志的存储中，并重放已提交的日志提案给 Apply 模块执行。etcd 通过引入一个 consistent index 的字段，来存储系统当前已经执行过的日志条目索引，实现幂等性。
    - Apply 模块在执行提案内容前，首先会判断当前提案是否已经执行过了，如果执行了则直接返回，若未执行同时无 db 配额满告警，则进入到 MVCC 模块，开始与持久化存储模块打交道。

  - MVCC: 保存一个 key 的多个历史版本, 核心由内存树形索引模块 (treeIndex) 和嵌入式的 KV 持久化存储库 boltdb 组成. 从 treeIndex 中获取 key hello 的版本号，再以版本号作为 boltdb 的 key，从 boltdb 中获取其 value 信息. etcd 的最大版本号 currentRevision。

    - etcd 出于数据一致性、性能等考虑，在访问 boltdb 前，首先会从一个内存读事务 buffer 中，二分查找你要访问 key 是否在 buffer 里面，若命中则直接返回
    - boltdb 里每个 bucket 类似对应 MySQL 一个表，用户的 key 数据存放的 bucket 名字的是 key，etcd MVCC 元数据存放的 bucket 是 meta

  - 

- **存储层**：存储层包含预写日志 (WAL) 模块、快照 (Snapshot) 模块、boltdb 模块。其中 WAL 可保障 etcd crash 后数据不丢失，boltdb 则保存了集群元数据和用户写入的数据。

  - WAL 模块:

    - Raft 模块收到提案后，如果当前节点是 Follower，它会转发给 Leader，只有 Leader 才能处理写请求。Leader 收到提案后，通过 Raft 模块输出待转发给 Follower 节点的消息和待持久化的日志条目，日志条目则封装了我们上面所说的 put hello 提案内容。
    - etcdserver 从 Raft 模块获取到以上消息和日志条目后，作为 Leader，它会将 put 提案消息广播给集群各个节点，同时需要把*集群 Leader 任期号、投票信息、已提交索引、提案内容持久化到一个 WAL（Write Ahead Log）日志文件中*，用于保证集群的一致性、可恢复性.
    - WAL 结构，它由多种类型的 WAL 记录顺序追加写入组成，每个记录由类型、数据、循环冗余校验码组成。不同类型的记录通过 Type 字段区分，Data 为对应记录内容，CRC 为循环校验码信息。
      - WAL 记录类型目前支持 5 种，分别是文件元数据记录、日志条目记录、状态信息记录、CRC 记录、快照记录：
        1. **文件元数据记录**包含节点 ID、集群 ID 信息，它在 WAL 文件创建的时候写入；
        2. **日志条目记录**包含 Raft 日志信息，如 put 提案内容；
        3. **状态信息记录**，包含集群的任期号、节点投票信息等，一个日志文件中会有多条，以最后的记录为准；
        4. **CRC 记录**包含上一个 WAL 文件的最后的 CRC（循环冗余校验码）信息， 在创建、切割 WAL 文件时，作为第一条记录写入到新的 WAL 文件， 用于校验数据文件的完整性、准确性等；
        5. **快照记录**包含快照的任期号、日志索引信息，用于检查快照文件的准确性。
    - WAL 模块持久化 Raft 日志条目: 它首先先将 Raft 日志条目内容（含任期号、索引、提案内容）序列化后保存到 WAL 记录的 Data 字段， 然后计算 Data 的 CRC 值，设置 Type 为 Entry Type， 以上信息就组成了一个完整的 WAL 记录。

    <img src="./img/WAL 格式.png" alt="WAL 格式" style="zoom:30%;" />

  - 

- 

  

etcd 是典型的读多写少存储; 







### Etcd 写流程：

<img src="./img/etcd写流程.png" alt="etcd写流程" style="zoom:45%;" />

​	当 client 发起一个更新 hello 为 world 请求后，若 Leader 收到写请求，它会将此请求持久化到 WAL 日志，并广播给各个节点，若**一半以上**节点持久化成功，则该请求对应的日志条目被标识为已提交，etcdserver 模块异步从 Raft 模块获取已提交的日志条目，应用到**状态机** (boltdb 等)。

​	此时若 client 发起一个读取 hello 的请求，假设此请求直接从状态机中读取， 如果连接到的是 C 节点，若 C 节点磁盘 I/O 出现波动，可能导致它应用已提交的日志条目很慢，则会出现更新 hello 为 world 的写命令，在 client 读 hello 的时候还未被提交到状态机，因此就可能读取到旧数据; 所以会出现两种场景：

- **串行** (Serializable) 读，它具有低延时、高吞吐量的特点，适合对**数据一致性要求不高**的场景。(所以不会经过线性读中的3、4 两步骤)
- **线性**读(默认)：它需要经过 Raft 协议模块，反应的是集群共识，因此在延时和吞吐量上相比串行读略差一点，适用于对**数据一致性要求高**的场景。

### Etcd 线性读流程：

<img src="./img/etcd线性读.png" alt="etcd线性读" style="zoom:45%;" />















