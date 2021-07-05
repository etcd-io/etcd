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

<img src="./img/etcd架构图.png" alt="etcd架构图" style="zoom:40%;" />

- Client 层：Client 层包括 client v2 和 v3 两个大版本 API 客户端库，提供了简洁易用的 API，同时支持**负载均衡**、**节点间故障自动转移**，可极大降低业务使用 etcd 复杂度，提升开发效率、服务可用性。
- API 网络层：API 网络层主要包括 client 访问 server 和 server 节点之间的通信协议。一方面，client 访问 etcd server 的 API 分为 v2 和 v3 两个大版本。**v2 API 使用 HTTP/1.x 协议，v3 API 使用 gRPC 协议**。同时 v3 通过 etcd grpc-gateway 组件也支持 HTTP/1.x 协议，便于各种语言的服务调用。另一方面，**server 之间通信协议，是指节点间通过 Raft 算法实现数据复制和 Leader 选举等功能时使用的 HTTP 协议**。
- Raft 算法层：Raft 算法层**实现了 Leader 选举、日志复制、ReadIndex 等**核心算法特性，用于<font color='blue'>*保障 etcd 多个节点间的数据一致性、提升服务可用性等，是 etcd 的基石和亮点*</font>。
- 功能逻辑层：**etcd 核心特性实现层**，如典型的 KVServer 模块、MVCC 模块、Auth 鉴权模块、Lease 租约模块、Compactor 压缩模块等，其中 MVCC 模块主要由 treeIndex 模块和 boltdb 模块组成。
- 存储层：存储层包含预写日志 (WAL) 模块、快照 (Snapshot) 模块、boltdb 模块。其中 WAL 可保障 etcd crash 后数据不丢失，boltdb 则保存了集群元数据和用户写入的数据。

etcd 是典型的读多写少存储; 



### Etcd 线性读流程：

<img src="./img/etcd线性读.png" alt="etcd线性读" style="zoom:40%;" />





