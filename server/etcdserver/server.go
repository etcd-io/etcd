// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdserver

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/config"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/pkg/v3/runtime"
	"go.etcd.io/etcd/pkg/v3/schedule"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2discovery"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2http/httptypes"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3alarm"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3compactor"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/lease/leasehttp"
	"go.etcd.io/etcd/server/v3/mvcc"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/wal"
)

const (
	DefaultSnapshotCount = 100000

	// DefaultSnapshotCatchUpEntries is the number of entries for a slow follower
	// to catch-up after compacting the raft storage entries.
	// We expect the follower has a millisecond level latency with the leader.
	// The max throughput is around 10K. Keep a 5K entries is enough for helping
	// follower to catch up.
	DefaultSnapshotCatchUpEntries uint64 = 5000

	StoreClusterPrefix = "/0"
	StoreKeysPrefix    = "/1"

	// HealthInterval is the minimum time the cluster should be healthy
	// before accepting add member requests.
	HealthInterval = 5 * time.Second

	purgeFileInterval = 30 * time.Second

	// max number of in-flight snapshot messages etcdserver allows to have
	// This number is more than enough for most clusters with 5 machines.
	maxInFlightMsgSnap = 16

	releaseDelayAfterSnapshot = 30 * time.Second

	// maxPendingRevokes is the maximum number of outstanding expired lease revocations.
	maxPendingRevokes = 16

	recommendedMaxRequestBytes = 10 * 1024 * 1024

	readyPercent = 0.9

	DowngradeEnabledPath = "/downgrade/enabled"
)

var (
	// monitorVersionInterval should be smaller than the timeout
	// on the connection. Or we will not be able to reuse the connection
	// (since it will timeout).
	monitorVersionInterval = rafthttp.ConnWriteTimeout - time.Second

	recommendedMaxRequestBytesString = humanize.Bytes(uint64(recommendedMaxRequestBytes))
	storeMemberAttributeRegexp       = regexp.MustCompile(path.Join(membership.StoreMembersPrefix, "[[:xdigit:]]{1,16}", "attributes"))
)

func init() {
	rand.Seed(time.Now().UnixNano())

	expvar.Publish(
		"file_descriptor_limit",
		expvar.Func(
			func() interface{} {
				n, _ := runtime.FDLimit()
				return n
			},
		),
	)
}

type Response struct {
	Term    uint64
	Index   uint64
	Event   *v2store.Event
	Watcher v2store.Watcher
	Err     error
}

type ServerV2 interface {
	Server
	Leader() types.ID

	// Do takes a V2 request and attempts to fulfill it, returning a Response.
	Do(ctx context.Context, r pb.Request) (Response, error)
	stats.Stats
	ClientCertAuthEnabled() bool
}

type ServerV3 interface {
	Server
	RaftStatusGetter
}

func (s *EtcdServer) ClientCertAuthEnabled() bool { return s.Cfg.ClientCertAuthEnabled }

type Server interface {
	// AddMember attempts to add a member into the cluster. It will return
	// ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDExists if member ID exists in the cluster.
	AddMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error)
	// RemoveMember attempts to remove a member from the cluster. It will
	// return ErrIDRemoved if member ID is removed from the cluster, or return
	// ErrIDNotFound if member ID is not in the cluster.
	RemoveMember(ctx context.Context, id uint64) ([]*membership.Member, error)
	// UpdateMember attempts to update an existing member in the cluster. It will
	// return ErrIDNotFound if the member ID does not exist.
	UpdateMember(ctx context.Context, updateMemb membership.Member) ([]*membership.Member, error)
	// PromoteMember attempts to promote a non-voting node to a voting node. It will
	// return ErrIDNotFound if the member ID does not exist.
	// return ErrLearnerNotReady if the member are not ready.
	// return ErrMemberNotLearner if the member is not a learner.
	PromoteMember(ctx context.Context, id uint64) ([]*membership.Member, error)

	// ClusterVersion is the cluster-wide minimum major.minor version.
	// Cluster version is set to the min version that an etcd member is
	// compatible with when first bootstrap.
	//
	// ClusterVersion is nil until the cluster is bootstrapped (has a quorum).
	//
	// During a rolling upgrades, the ClusterVersion will be updated
	// automatically after a sync. (5 second by default)
	//
	// The API/raft component can utilize ClusterVersion to determine if
	// it can accept a client request or a raft RPC.
	// NOTE: ClusterVersion might be nil when etcd 2.1 works with etcd 2.0 and
	// the leader is etcd 2.0. etcd 2.0 leader will not update clusterVersion since
	// this feature is introduced post 2.0.
	ClusterVersion() *semver.Version
	Cluster() api.Cluster
	Alarms() []*pb.AlarmMember

	// LeaderChangedNotify returns a channel for application level code to be notified
	// when etcd leader changes, this function is intend to be used only in application
	// which embed etcd.
	// Caution:
	// 1. the returned channel is being closed when the leadership changes.
	// 2. so the new channel needs to be obtained for each raft term.
	// 3. user can loose some consecutive channel changes using this API.
	LeaderChangedNotify() <-chan struct{}
}

// EtcdServer is the production implementation of the Server interface
// 是 etcd-server 模块的核心，它会启动并协调多个后台 goroutine 的工作 。
// 在 EtcdServer初始化时，会根据配置项初始化 etcd-raft 模块 ，其中涉及 WAL 日志文件的初始化、etcd v2 存储和 etcd v3 存储的初始化，以及网络层 RoundTripper 实例的初始化。
// 初始化完成之后，EtcdServer 实例会注册集群通信使用的 Handler 实例，也会将当前节点的 URL 发布到集群 当中。
// 除此之外，EtcdServer 启动的多个后台 goroutine 会完成定期清理 WAL 日志文件，以及 快照文件、处理 LinearizableRead请求等功能。
type EtcdServer struct {
	// inflightSnapshots holds count the number of snapshots currently inflight.
	// 当前已发送出去但未收到响应的快照个数。
	inflightSnapshots int64 // must use atomic operations to access; keep 64-bit aligned.

	// 当前节点已应用的 Entry 记录的最大索引值。
	appliedIndex uint64 // must use atomic operations to access; keep 64-bit aligned.

	// 当前己提交的 Entry 记录的索引值。
	committedIndex uint64 // must use atomic operations to access; keep 64-bit aligned.
	term           uint64 // must use atomic operations to access; keep 64-bit aligned.
	lead           uint64 // must use atomic operations to access; keep 64-bit aligned.

	consistIndex cindex.ConsistentIndexer // consistIndex is used to get/set/save consistentIndex

	// 即前面介绍的 etcdserver.raftNode，它是 EtcdServer 实例与底层 etcd-raft 模块通信的桥梁。
	r raftNode // uses 64-bit atomics; keep 64-bit aligned.

	// 当前节点将自身的信息推送到集群中其他节点之后，会将该通道关闭，也作为当前 EtcdServer实例，可以对外提供服务的一个信号。
	readych chan struct{}
	Cfg     config.ServerConfig

	lgMu *sync.RWMutex
	lg   *zap.Logger

	// Wait 主要负责协调多个后台 goroutine 之间的执行。在 Wait 实例 中维护了一个 map (map[uint64]chan interface{}类型〉，我们可以通过 Wait.Register(id uint64)为指定的 ID创建一个对应的通道， ID与通道的映射关系会记录在上述 map 中。 之后，可以通过 Trigger(iduint64,xinterface{})方法将参数 x写入 id对应的通道中，其 他监听 该通道的 goroutine 就可以获取该参数 L
	w wait.Wait

	readMu sync.RWMutex

	// readwaitc, readNotifier 这两个字段主要用来 协调 LinearizableRead相关的 goroutine，
	// 后面介绍 EtcdServer.linearizableReadLoop()方法时，再详细介绍这两个字段的功能。
	// read routine notifies etcd server that it waits for reading by sending an empty struct to
	// readwaitC
	readwaitc chan struct{}
	// readNotifier is used to notify the read routine that it can process the request
	// when there is no error
	readNotifier *notifier

	// stop signals the run goroutine should shutdown.
	stop chan struct{}
	// stopping is closed by run goroutine on shutdown.
	stopping chan struct{}
	// done is closed when all goroutines from start() complete.
	done chan struct{}
	// leaderChanged is used to notify the linearizable read loop to drop the old read requests.
	leaderChanged   chan struct{}
	leaderChangedMu sync.RWMutex

	errorc chan error
	id     types.ID

	// 记录当前节点的名称及接收集群中其他节点请求的 URL地址。
	attributes membership.Attributes

	// 记录当前集群中全部节点的信息。
	cluster *membership.RaftCluster

	v2store v2store.Store

	// 用来读写快照文件
	snapshotter *snap.Snapshotter

	// 应用 v2 版本的 Entry 记录，其 底层封装了前面介绍的 v2 存储 。
	applyV2 ApplierV2

	// 应用 v3 版本的 Entry 记录，其底层封装了前面介绍的 v3 存储 。
	// applyV3 is the applier with auth and quotas
	applyV3 applierV3
	// applyV3Base is the core applier without auth or quotas
	applyV3Base applierV3
	// applyV3Internal is the applier for internal request
	applyV3Internal applierV3Internal
	applyWait       wait.WaitTime

	kv     mvcc.WatchableKV
	lessor lease.Lessor
	bemu   sync.Mutex

	// v3 版本的后端存储
	be         backend.Backend
	beHooks    *backendHooks
	authStore  auth.AuthStore
	alarmStore *v3alarm.AlarmStore

	stats  *stats.ServerStats
	lstats *stats.LeaderStats

	SyncTicker *time.Ticker
	// compactor is used to auto-compact the KV.
	compactor v3compactor.Compactor

	// peerRt used to send requests (version, lease) to peers.
	peerRt http.RoundTripper

	reqIDGen *idutil.Generator // 用于生成请求的唯一标识。

	// wgMu blocks concurrent waitgroup mutation while server stopping
	wgMu sync.RWMutex
	// wg is used to wait for the goroutines that depends on the server state
	// to exit when stopping the server.
	wg sync.WaitGroup

	// ctx is used for etcd-initiated requests that may need to be canceled
	// on etcd server shutdown.
	ctx    context.Context
	cancel context.CancelFunc

	leadTimeMu      sync.RWMutex
	leadElectedTime time.Time

	firstCommitInTermMu sync.RWMutex
	firstCommitInTermC  chan struct{}

	*AccessController
}

type backendHooks struct {
	indexer cindex.ConsistentIndexer
	lg      *zap.Logger

	// confState to be written in the next submitted backend transaction (if dirty)
	confState raftpb.ConfState
	// first write changes it to 'dirty'. false by default, so
	// not initialized `confState` is meaningless.
	confStateDirty bool
	confStateLock  sync.Mutex
}

func (bh *backendHooks) OnPreCommitUnsafe(tx backend.BatchTx) {
	bh.indexer.UnsafeSave(tx)
	bh.confStateLock.Lock()
	defer bh.confStateLock.Unlock()
	if bh.confStateDirty {
		membership.MustUnsafeSaveConfStateToBackend(bh.lg, tx, &bh.confState)
		// save bh.confState
		bh.confStateDirty = false
	}
}

func (bh *backendHooks) SetConfState(confState *raftpb.ConfState) {
	bh.confStateLock.Lock()
	defer bh.confStateLock.Unlock()
	bh.confState = *confState
	bh.confStateDirty = true
}

// NewServer creates a new EtcdServer from the supplied configuration. The
// configuration is considered static for the lifetime of the EtcdServer.
func NewServer(cfg config.ServerConfig) (srv *EtcdServer, err error) {
	st := v2store.New(StoreClusterPrefix, StoreKeysPrefix)

	var (
		w  *wal.WAL
		n  raft.Node
		s  *raft.MemoryStorage
		id types.ID
		cl *membership.RaftCluster
	)

	if cfg.MaxRequestBytes > recommendedMaxRequestBytes {
		cfg.Logger.Warn(
			"exceeded recommended request limit",
			zap.Uint("max-request-bytes", cfg.MaxRequestBytes),
			zap.String("max-request-size", humanize.Bytes(uint64(cfg.MaxRequestBytes))),
			zap.Int("recommended-request-bytes", recommendedMaxRequestBytes),
			zap.String("recommended-request-size", recommendedMaxRequestBytesString),
		)
	}

	if terr := fileutil.TouchDirAll(cfg.DataDir); terr != nil {
		return nil, fmt.Errorf("cannot access data directory: %v", terr)
	}

	haveWAL := wal.Exist(cfg.WALDir())

	if err = fileutil.TouchDirAll(cfg.SnapDir()); err != nil {
		cfg.Logger.Fatal(
			"failed to create snapshot directory",
			zap.String("path", cfg.SnapDir()),
			zap.Error(err),
		)
	}

	if err = fileutil.RemoveMatchFile(cfg.Logger, cfg.SnapDir(), func(fileName string) bool {
		return strings.HasPrefix(fileName, "tmp")
	}); err != nil {
		cfg.Logger.Error(
			"failed to remove temp file(s) in snapshot directory",
			zap.String("path", cfg.SnapDir()),
			zap.Error(err),
		)
	}

	ss := snap.New(cfg.Logger, cfg.SnapDir())

	bepath := cfg.BackendPath()
	beExist := fileutil.Exist(bepath)

	ci := cindex.NewConsistentIndex(nil)
	beHooks := &backendHooks{lg: cfg.Logger, indexer: ci}
	be := openBackend(cfg, beHooks)
	ci.SetBackend(be)
	cindex.CreateMetaBucket(be.BatchTx())

	if cfg.ExperimentalBootstrapDefragThresholdMegabytes != 0 {
		err := maybeDefragBackend(cfg, be)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if err != nil {
			be.Close()
		}
	}()

	prt, err := rafthttp.NewRoundTripper(cfg.PeerTLSInfo, cfg.PeerDialTimeout())
	if err != nil {
		return nil, err
	}
	var (
		remotes  []*membership.Member
		snapshot *raftpb.Snapshot
	)

	switch {
	case !haveWAL && !cfg.NewCluster:
		if err = cfg.VerifyJoinExisting(); err != nil {
			return nil, err
		}
		cl, err = membership.NewClusterFromURLsMap(cfg.Logger, cfg.InitialClusterToken, cfg.InitialPeerURLsMap)
		if err != nil {
			return nil, err
		}
		existingCluster, gerr := GetClusterFromRemotePeers(cfg.Logger, getRemotePeerURLs(cl, cfg.Name), prt)
		if gerr != nil {
			return nil, fmt.Errorf("cannot fetch cluster info from peer urls: %v", gerr)
		}
		if err = membership.ValidateClusterAndAssignIDs(cfg.Logger, cl, existingCluster); err != nil {
			return nil, fmt.Errorf("error validating peerURLs %s: %v", existingCluster, err)
		}
		if !isCompatibleWithCluster(cfg.Logger, cl, cl.MemberByName(cfg.Name).ID, prt) {
			return nil, fmt.Errorf("incompatible with current running cluster")
		}

		remotes = existingCluster.Members()
		cl.SetID(types.ID(0), existingCluster.ID())
		cl.SetStore(st)
		cl.SetBackend(be)
		id, n, s, w = startNode(cfg, cl, nil)
		cl.SetID(id, existingCluster.ID())

	case !haveWAL && cfg.NewCluster:
		if err = cfg.VerifyBootstrap(); err != nil {
			return nil, err
		}
		cl, err = membership.NewClusterFromURLsMap(cfg.Logger, cfg.InitialClusterToken, cfg.InitialPeerURLsMap)
		if err != nil {
			return nil, err
		}
		m := cl.MemberByName(cfg.Name)
		if isMemberBootstrapped(cfg.Logger, cl, cfg.Name, prt, cfg.BootstrapTimeoutEffective()) {
			return nil, fmt.Errorf("member %s has already been bootstrapped", m.ID)
		}
		if cfg.ShouldDiscover() {
			var str string
			str, err = v2discovery.JoinCluster(cfg.Logger, cfg.DiscoveryURL, cfg.DiscoveryProxy, m.ID, cfg.InitialPeerURLsMap.String())
			if err != nil {
				return nil, &DiscoveryError{Op: "join", Err: err}
			}
			var urlsmap types.URLsMap
			urlsmap, err = types.NewURLsMap(str)
			if err != nil {
				return nil, err
			}
			if config.CheckDuplicateURL(urlsmap) {
				return nil, fmt.Errorf("discovery cluster %s has duplicate url", urlsmap)
			}
			if cl, err = membership.NewClusterFromURLsMap(cfg.Logger, cfg.InitialClusterToken, urlsmap); err != nil {
				return nil, err
			}
		}
		cl.SetStore(st)
		cl.SetBackend(be)
		id, n, s, w = startNode(cfg, cl, cl.MemberIDs())
		cl.SetID(id, cl.ID())

	case haveWAL:
		if err = fileutil.IsDirWriteable(cfg.MemberDir()); err != nil {
			return nil, fmt.Errorf("cannot write to member directory: %v", err)
		}

		if err = fileutil.IsDirWriteable(cfg.WALDir()); err != nil {
			return nil, fmt.Errorf("cannot write to WAL directory: %v", err)
		}

		if cfg.ShouldDiscover() {
			cfg.Logger.Warn(
				"discovery token is ignored since cluster already initialized; valid logs are found",
				zap.String("wal-dir", cfg.WALDir()),
			)
		}

		// Find a snapshot to start/restart a raft node
		walSnaps, err := wal.ValidSnapshotEntries(cfg.Logger, cfg.WALDir())
		if err != nil {
			return nil, err
		}
		// snapshot files can be orphaned if etcd crashes after writing them but before writing the corresponding
		// wal log entries
		snapshot, err := ss.LoadNewestAvailable(walSnaps)
		if err != nil && err != snap.ErrNoSnapshot {
			return nil, err
		}

		if snapshot != nil {
			if err = st.Recovery(snapshot.Data); err != nil {
				cfg.Logger.Panic("failed to recover from snapshot", zap.Error(err))
			}

			if err = assertNoV2StoreContent(cfg.Logger, st, cfg.V2Deprecation); err != nil {
				cfg.Logger.Error("illegal v2store content", zap.Error(err))
				return nil, err
			}

			cfg.Logger.Info(
				"recovered v2 store from snapshot",
				zap.Uint64("snapshot-index", snapshot.Metadata.Index),
				zap.String("snapshot-size", humanize.Bytes(uint64(snapshot.Size()))),
			)

			if be, err = recoverSnapshotBackend(cfg, be, *snapshot, beExist, beHooks); err != nil {
				cfg.Logger.Panic("failed to recover v3 backend from snapshot", zap.Error(err))
			}
			s1, s2 := be.Size(), be.SizeInUse()
			cfg.Logger.Info(
				"recovered v3 backend from snapshot",
				zap.Int64("backend-size-bytes", s1),
				zap.String("backend-size", humanize.Bytes(uint64(s1))),
				zap.Int64("backend-size-in-use-bytes", s2),
				zap.String("backend-size-in-use", humanize.Bytes(uint64(s2))),
			)
		} else {
			cfg.Logger.Info("No snapshot found. Recovering WAL from scratch!")
		}

		if !cfg.ForceNewCluster {
			id, cl, n, s, w = restartNode(cfg, snapshot)
		} else {
			id, cl, n, s, w = restartAsStandaloneNode(cfg, snapshot)
		}

		cl.SetStore(st)
		cl.SetBackend(be)
		cl.Recover(api.UpdateCapability)
		if cl.Version() != nil && !cl.Version().LessThan(semver.Version{Major: 3}) && !beExist {
			os.RemoveAll(bepath)
			return nil, fmt.Errorf("database file (%v) of the backend is missing", bepath)
		}

	default:
		return nil, fmt.Errorf("unsupported bootstrap config")
	}

	if terr := fileutil.TouchDirAll(cfg.MemberDir()); terr != nil {
		return nil, fmt.Errorf("cannot access member directory: %v", terr)
	}

	sstats := stats.NewServerStats(cfg.Name, id.String())
	lstats := stats.NewLeaderStats(cfg.Logger, id.String())

	heartbeat := time.Duration(cfg.TickMs) * time.Millisecond
	srv = &EtcdServer{
		readych:     make(chan struct{}),
		Cfg:         cfg,
		lgMu:        new(sync.RWMutex),
		lg:          cfg.Logger,
		errorc:      make(chan error, 1),
		v2store:     st,
		snapshotter: ss,
		r: *newRaftNode(
			raftNodeConfig{
				lg:          cfg.Logger,
				isIDRemoved: func(id uint64) bool { return cl.IsIDRemoved(types.ID(id)) },
				Node:        n,
				heartbeat:   heartbeat,
				raftStorage: s,
				storage:     NewStorage(w, ss),
			},
		),
		id:                 id,
		attributes:         membership.Attributes{Name: cfg.Name, ClientURLs: cfg.ClientURLs.StringSlice()},
		cluster:            cl,
		stats:              sstats,
		lstats:             lstats,
		SyncTicker:         time.NewTicker(500 * time.Millisecond),
		peerRt:             prt,
		reqIDGen:           idutil.NewGenerator(uint16(id), time.Now()),
		AccessController:   &AccessController{CORS: cfg.CORS, HostWhitelist: cfg.HostWhitelist},
		consistIndex:       ci,
		firstCommitInTermC: make(chan struct{}),
	}
	serverID.With(prometheus.Labels{"server_id": id.String()}).Set(1)

	srv.applyV2 = NewApplierV2(cfg.Logger, srv.v2store, srv.cluster)

	srv.be = be
	srv.beHooks = beHooks
	minTTL := time.Duration((3*cfg.ElectionTicks)/2) * heartbeat

	// always recover lessor before kv. When we recover the mvcc.KV it will reattach keys to its leases.
	// If we recover mvcc.KV first, it will attach the keys to the wrong lessor before it recovers.
	srv.lessor = lease.NewLessor(srv.Logger(), srv.be, lease.LessorConfig{
		MinLeaseTTL:                int64(math.Ceil(minTTL.Seconds())),
		CheckpointInterval:         cfg.LeaseCheckpointInterval,
		ExpiredLeasesRetryInterval: srv.Cfg.ReqTimeout(),
	})

	tp, err := auth.NewTokenProvider(cfg.Logger, cfg.AuthToken,
		func(index uint64) <-chan struct{} {
			return srv.applyWait.Wait(index)
		},
		time.Duration(cfg.TokenTTL)*time.Second,
	)
	if err != nil {
		cfg.Logger.Warn("failed to create token provider", zap.Error(err))
		return nil, err
	}

	// 创建服务端存储
	srv.kv = mvcc.New(srv.Logger(), srv.be, srv.lessor, mvcc.StoreConfig{CompactionBatchLimit: cfg.CompactionBatchLimit})

	kvindex := ci.ConsistentIndex()
	srv.lg.Debug("restore consistentIndex", zap.Uint64("index", kvindex))
	if beExist {
		// TODO: remove kvindex != 0 checking when we do not expect users to upgrade
		// etcd from pre-3.0 release.
		if snapshot != nil && kvindex < snapshot.Metadata.Index {
			if kvindex != 0 {
				return nil, fmt.Errorf("database file (%v index %d) does not match with snapshot (index %d)", bepath, kvindex, snapshot.Metadata.Index)
			}
			cfg.Logger.Warn(
				"consistent index was never saved",
				zap.Uint64("snapshot-index", snapshot.Metadata.Index),
			)
		}
	}

	srv.authStore = auth.NewAuthStore(srv.Logger(), srv.be, tp, int(cfg.BcryptCost))

	newSrv := srv // since srv == nil in defer if srv is returned as nil
	defer func() {
		// closing backend without first closing kv can cause
		// resumed compactions to fail with closed tx errors
		if err != nil {
			newSrv.kv.Close()
		}
	}()
	if num := cfg.AutoCompactionRetention; num != 0 {
		srv.compactor, err = v3compactor.New(cfg.Logger, cfg.AutoCompactionMode, num, srv.kv, srv)
		if err != nil {
			return nil, err
		}
		srv.compactor.Run()
	}

	srv.applyV3Base = srv.newApplierV3Backend()
	srv.applyV3Internal = srv.newApplierV3Internal()
	if err = srv.restoreAlarms(); err != nil {
		return nil, err
	}

	if srv.Cfg.EnableLeaseCheckpoint {
		// setting checkpointer enables lease checkpoint feature.
		srv.lessor.SetCheckpointer(func(ctx context.Context, cp *pb.LeaseCheckpointRequest) {
			srv.raftRequestOnce(ctx, pb.InternalRaftRequest{LeaseCheckpoint: cp})
		})
	}

	// TODO: move transport initialization near the definition of remote
	tr := &rafthttp.Transport{
		Logger:      cfg.Logger,
		TLSInfo:     cfg.PeerTLSInfo,
		DialTimeout: cfg.PeerDialTimeout(),
		ID:          id,
		URLs:        cfg.PeerURLs,
		ClusterID:   cl.ID(),
		Raft:        srv,
		Snapshotter: ss,
		ServerStats: sstats,
		LeaderStats: lstats,
		ErrorC:      srv.errorc,
	}
	if err = tr.Start(); err != nil {
		return nil, err
	}
	// add all remotes into transport
	for _, m := range remotes {
		if m.ID != id {
			tr.AddRemote(m.ID, m.PeerURLs)
		}
	}
	for _, m := range cl.Members() {
		if m.ID != id {
			tr.AddPeer(m.ID, m.PeerURLs)
		}
	}
	srv.r.transport = tr

	return srv, nil
}

// assertNoV2StoreContent -> depending on the deprecation stage, warns or report an error
// if the v2store contains custom content.
func assertNoV2StoreContent(lg *zap.Logger, st v2store.Store, deprecationStage config.V2DeprecationEnum) error {
	metaOnly, err := membership.IsMetaStoreOnly(st)
	if err != nil {
		return err
	}
	if metaOnly {
		return nil
	}
	if deprecationStage.IsAtLeast(config.V2_DEPR_1_WRITE_ONLY) {
		return fmt.Errorf("detected disallowed custom content in v2store for stage --v2-deprecation=%s", deprecationStage)
	}
	lg.Warn("detected custom v2store content. Etcd v3.5 is the last version allowing to access it using API v2. Please remove the content.")
	return nil
}

func (s *EtcdServer) Logger() *zap.Logger {
	s.lgMu.RLock()
	l := s.lg
	s.lgMu.RUnlock()
	return l
}

func tickToDur(ticks int, tickMs uint) string {
	return fmt.Sprintf("%v", time.Duration(ticks)*time.Duration(tickMs)*time.Millisecond)
}

func (s *EtcdServer) adjustTicks() {
	lg := s.Logger()
	clusterN := len(s.cluster.Members())

	// single-node fresh start, or single-node recovers from snapshot
	if clusterN == 1 {
		ticks := s.Cfg.ElectionTicks - 1
		lg.Info(
			"started as single-node; fast-forwarding election ticks",
			zap.String("local-member-id", s.ID().String()),
			zap.Int("forward-ticks", ticks),
			zap.String("forward-duration", tickToDur(ticks, s.Cfg.TickMs)),
			zap.Int("election-ticks", s.Cfg.ElectionTicks),
			zap.String("election-timeout", tickToDur(s.Cfg.ElectionTicks, s.Cfg.TickMs)),
		)
		s.r.advanceTicks(ticks)
		return
	}

	if !s.Cfg.InitialElectionTickAdvance {
		lg.Info("skipping initial election tick advance", zap.Int("election-ticks", s.Cfg.ElectionTicks))
		return
	}
	lg.Info("starting initial election tick advance", zap.Int("election-ticks", s.Cfg.ElectionTicks))

	// retry up to "rafthttp.ConnReadTimeout", which is 5-sec
	// until peer connection reports; otherwise:
	// 1. all connections failed, or
	// 2. no active peers, or
	// 3. restarted single-node with no snapshot
	// then, do nothing, because advancing ticks would have no effect
	waitTime := rafthttp.ConnReadTimeout
	itv := 50 * time.Millisecond
	for i := int64(0); i < int64(waitTime/itv); i++ {
		select {
		case <-time.After(itv):
		case <-s.stopping:
			return
		}

		peerN := s.r.transport.ActivePeers()
		if peerN > 1 {
			// multi-node received peer connection reports
			// adjust ticks, in case slow leader message receive
			ticks := s.Cfg.ElectionTicks - 2

			lg.Info(
				"initialized peer connections; fast-forwarding election ticks",
				zap.String("local-member-id", s.ID().String()),
				zap.Int("forward-ticks", ticks),
				zap.String("forward-duration", tickToDur(ticks, s.Cfg.TickMs)),
				zap.Int("election-ticks", s.Cfg.ElectionTicks),
				zap.String("election-timeout", tickToDur(s.Cfg.ElectionTicks, s.Cfg.TickMs)),
				zap.Int("active-remote-members", peerN),
			)

			s.r.advanceTicks(ticks)
			return
		}
	}
}

// Start performs any initialization of the Server necessary for it to
// begin serving requests. It must be called before Do or Process.
// Start must be non-blocking; any long-running server functionality
// should be implemented in goroutines.
func (s *EtcdServer) Start() {
	// 其中会启动一个后台 goroutine， 执行 EtcdServer.run()方法，是 EtcdServer 启 动的核心
	s.start()
	// 启动一个后台 goroutine，将 当前节点的相关信息发送到 集群其他节点
	s.GoAttach(func() { s.adjustTicks() })
	// TODO: Switch to publishV3 in 3.6.
	// Support for cluster_member_set_attr was added in 3.5.
	s.GoAttach(func() { s.publish(s.Cfg.ReqTimeout()) })

	// 启动一个后台 goroutine，定义清理 WAL 日志文件和快照文件
	s.GoAttach(s.purgeFile)
	// 启动一个后台 goroutine，实现一些监控相关的功能，这里不展开详述，对监控感兴趣的读者可以参考源码进行学习
	s.GoAttach(func() { monitorFileDescriptor(s.Logger(), s.stopping) })
	//启动一个后台 goroutine监控集群中其他节点的版本信息，主妾是在版本升级的时候使用，这里不展开详述 ， 对感兴趣的读者可以参考源码进行学 习
	s.GoAttach(s.monitorVersions)
	// 启动一个后台 goroutine ，用来实现 linearizable Read 的功能
	s.GoAttach(s.linearizableReadLoop)
	s.GoAttach(s.monitorKVHash)
	s.GoAttach(s.monitorDowngrade)
}

// start prepares and starts server in a new goroutine. It is no longer safe to
// modify a server's fields after it has been sent to Start.
// This function is just used for testing.
func (s *EtcdServer) start() {
	lg := s.Logger()

	if s.Cfg.SnapshotCount == 0 {
		lg.Info(
			"updating snapshot-count to default",
			zap.Uint64("given-snapshot-count", s.Cfg.SnapshotCount),
			zap.Uint64("updated-snapshot-count", DefaultSnapshotCount),
		)
		s.Cfg.SnapshotCount = DefaultSnapshotCount
	}
	if s.Cfg.SnapshotCatchUpEntries == 0 {
		lg.Info(
			"updating snapshot catch-up entries to default",
			zap.Uint64("given-snapshot-catchup-entries", s.Cfg.SnapshotCatchUpEntries),
			zap.Uint64("updated-snapshot-catchup-entries", DefaultSnapshotCatchUpEntries),
		)
		s.Cfg.SnapshotCatchUpEntries = DefaultSnapshotCatchUpEntries
	}

	s.w = wait.New()
	s.applyWait = wait.NewTimeList()
	s.done = make(chan struct{})
	s.stop = make(chan struct{})
	s.stopping = make(chan struct{}, 1)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.readwaitc = make(chan struct{}, 1)
	s.readNotifier = newNotifier()
	s.leaderChanged = make(chan struct{})
	if s.ClusterVersion() != nil {
		lg.Info(
			"starting etcd server",
			zap.String("local-member-id", s.ID().String()),
			zap.String("local-server-version", version.Version),
			zap.String("cluster-id", s.Cluster().ID().String()),
			zap.String("cluster-version", version.Cluster(s.ClusterVersion().String())),
		)
		membership.ClusterVersionMetrics.With(prometheus.Labels{"cluster_version": version.Cluster(s.ClusterVersion().String())}).Set(1)
	} else {
		lg.Info(
			"starting etcd server",
			zap.String("local-member-id", s.ID().String()),
			zap.String("local-server-version", version.Version),
			zap.String("cluster-version", "to_be_decided"),
		)
	}

	// TODO: if this is an empty log, writes all peer infos
	// into the first entry
	go s.run()
}

func (s *EtcdServer) purgeFile() {
	lg := s.Logger()
	var dberrc, serrc, werrc <-chan error
	var dbdonec, sdonec, wdonec <-chan struct{}
	if s.Cfg.MaxSnapFiles > 0 {
		dbdonec, dberrc = fileutil.PurgeFileWithDoneNotify(lg, s.Cfg.SnapDir(), "snap.db", s.Cfg.MaxSnapFiles, purgeFileInterval, s.stopping)
		sdonec, serrc = fileutil.PurgeFileWithDoneNotify(lg, s.Cfg.SnapDir(), "snap", s.Cfg.MaxSnapFiles, purgeFileInterval, s.stopping)
	}
	if s.Cfg.MaxWALFiles > 0 {
		wdonec, werrc = fileutil.PurgeFileWithDoneNotify(lg, s.Cfg.WALDir(), "wal", s.Cfg.MaxWALFiles, purgeFileInterval, s.stopping)
	}

	select {
	case e := <-dberrc:
		lg.Fatal("failed to purge snap db file", zap.Error(e))
	case e := <-serrc:
		lg.Fatal("failed to purge snap file", zap.Error(e))
	case e := <-werrc:
		lg.Fatal("failed to purge wal file", zap.Error(e))
	case <-s.stopping:
		if dbdonec != nil {
			<-dbdonec
		}
		if sdonec != nil {
			<-sdonec
		}
		if wdonec != nil {
			<-wdonec
		}
		return
	}
}

func (s *EtcdServer) Cluster() api.Cluster { return s.cluster }

func (s *EtcdServer) ApplyWait() <-chan struct{} { return s.applyWait.Wait(s.getCommittedIndex()) }

type ServerPeer interface {
	ServerV2
	RaftHandler() http.Handler
	LeaseHandler() http.Handler
}

func (s *EtcdServer) LeaseHandler() http.Handler {
	if s.lessor == nil {
		return nil
	}
	return leasehttp.NewHandler(s.lessor, s.ApplyWait)
}

func (s *EtcdServer) RaftHandler() http.Handler { return s.r.transport.Handler() }

type ServerPeerV2 interface {
	ServerPeer
	HashKVHandler() http.Handler
	DowngradeEnabledHandler() http.Handler
}

func (s *EtcdServer) DowngradeInfo() *membership.DowngradeInfo { return s.cluster.DowngradeInfo() }

type downgradeEnabledHandler struct {
	lg      *zap.Logger
	cluster api.Cluster
	server  *EtcdServer
}

func (s *EtcdServer) DowngradeEnabledHandler() http.Handler {
	return &downgradeEnabledHandler{
		lg:      s.Logger(),
		cluster: s.cluster,
		server:  s,
	}
}

func (h *downgradeEnabledHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("X-Etcd-Cluster-ID", h.cluster.ID().String())

	if r.URL.Path != DowngradeEnabledPath {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.server.Cfg.ReqTimeout())
	defer cancel()

	// serve with linearized downgrade info
	if err := h.server.linearizableReadNotify(ctx); err != nil {
		http.Error(w, fmt.Sprintf("failed linearized read: %v", err),
			http.StatusInternalServerError)
		return
	}
	enabled := h.server.DowngradeInfo().Enabled
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(strconv.FormatBool(enabled)))
}

// Process takes a raft message and applies it to the server's raft state
// machine, respecting any timeout of the given context.
func (s *EtcdServer) Process(ctx context.Context, m raftpb.Message) error {
	lg := s.Logger()
	if s.cluster.IsIDRemoved(types.ID(m.From)) {
		lg.Warn(
			"rejected Raft message from removed member",
			zap.String("local-member-id", s.ID().String()),
			zap.String("removed-member-id", types.ID(m.From).String()),
		)
		return httptypes.NewHTTPError(http.StatusForbidden, "cannot process message from removed member")
	}
	if m.Type == raftpb.MsgApp {
		s.stats.RecvAppendReq(types.ID(m.From).String(), m.Size())
	}
	return s.r.Step(ctx, m)
}

func (s *EtcdServer) IsIDRemoved(id uint64) bool { return s.cluster.IsIDRemoved(types.ID(id)) }

func (s *EtcdServer) ReportUnreachable(id uint64) { s.r.ReportUnreachable(id) }

// ReportSnapshot reports snapshot sent status to the raft state machine,
// and clears the used snapshot from the snapshot store.
func (s *EtcdServer) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	s.r.ReportSnapshot(id, status)
}

type etcdProgress struct {
	confState raftpb.ConfState
	snapi     uint64
	appliedt  uint64
	appliedi  uint64
}

// raftReadyHandler contains a set of EtcdServer operations to be called by raftNode,
// and helps decouple state machine logic from Raft algorithms.
// TODO: add a state machine interface to apply the commit entries and do snapshot/recover
type raftReadyHandler struct {
	getLead              func() (lead uint64)
	updateLead           func(lead uint64)
	updateLeadership     func(newLeader bool)
	updateCommittedIndex func(uint64)
}

func (s *EtcdServer) run() {
	lg := s.Logger()

	sn, err := s.r.raftStorage.Snapshot()
	if err != nil {
		lg.Panic("failed to get snapshot from Raft storage", zap.Error(err))
	}

	// asynchronously accept apply packets, dispatch progress in-order
	sched := schedule.NewFIFOScheduler()

	var (
		smu   sync.RWMutex
		syncC <-chan time.Time
	)
	setSyncC := func(ch <-chan time.Time) {
		smu.Lock()
		syncC = ch
		smu.Unlock()
	}
	getSyncC := func() (ch <-chan time.Time) {
		smu.RLock()
		ch = syncC
		smu.RUnlock()
		return
	}
	rh := &raftReadyHandler{
		getLead:    func() (lead uint64) { return s.getLead() },
		updateLead: func(lead uint64) { s.setLead(lead) },
		updateLeadership: func(newLeader bool) {
			if !s.isLeader() {
				if s.lessor != nil {
					s.lessor.Demote()
				}
				if s.compactor != nil {
					s.compactor.Pause()
				}
				setSyncC(nil)
			} else {
				if newLeader {
					t := time.Now()
					s.leadTimeMu.Lock()
					s.leadElectedTime = t
					s.leadTimeMu.Unlock()
				}
				setSyncC(s.SyncTicker.C)
				if s.compactor != nil {
					s.compactor.Resume()
				}
			}
			if newLeader {
				s.leaderChangedMu.Lock()
				lc := s.leaderChanged
				s.leaderChanged = make(chan struct{})
				close(lc)
				s.leaderChangedMu.Unlock()
			}
			// TODO: remove the nil checking
			// current test utility does not provide the stats
			if s.stats != nil {
				s.stats.BecomeLeader()
			}
		},
		updateCommittedIndex: func(ci uint64) {
			cci := s.getCommittedIndex()
			fmt.Printf("updateCommittedIndex, 收到的已经提交位置参数=%d, raftNode的getCommittedIndex=%d \n", ci, cci)
			if ci > cci {
				s.setCommittedIndex(ci)
			}
		},
	}
	s.r.start(rh)

	ep := etcdProgress{
		confState: sn.Metadata.ConfState,
		snapi:     sn.Metadata.Index,
		appliedt:  sn.Metadata.Term,
		appliedi:  sn.Metadata.Index,
	}

	defer func() {
		s.wgMu.Lock() // block concurrent waitgroup adds in GoAttach while stopping
		close(s.stopping)
		s.wgMu.Unlock()
		s.cancel()
		sched.Stop()

		// wait for gouroutines before closing raft so wal stays open
		s.wg.Wait()

		s.SyncTicker.Stop()

		// must stop raft after scheduler-- etcdserver can leak rafthttp pipelines
		// by adding a peer after raft stops the transport
		s.r.stop()

		s.Cleanup()

		close(s.done)
	}()

	// 处理过期的lease
	var expiredLeaseC <-chan []*lease.Lease
	if s.lessor != nil {
		expiredLeaseC = s.lessor.ExpiredLeasesC()
	}

	for {
		select {
		// run goroutine会监听raftNode.applyc通道，并调用 EtcdServer.applyAll()方法处理从中读取到的 apply实例。
		// 在前面介绍 raftNode时提到， 在 apply实例中封装了待应用的 Entry记录、待应用的快照数据和notifyc通道
		case ap := <-s.r.apply():
			fmt.Println("server收到apply消息")
			for i, entry := range ap.entries {
				var rr2 pb.InternalRaftRequest
				err := rr2.Unmarshal(entry.Data)
				if err == nil {
					if rr2.Put != nil {
						fmt.Println("server收到apply消息, 在ap.entries解析出了Put请求,kv=", string(rr2.Put.Key), string(rr2.Put.Value))
						fmt.Println("len(ap.entries)=", len(ap.entries))
						fmt.Println("rr2.ID=", rr2.ID)
						fmt.Println("rr2.Header.ID=", rr2.Header.ID)
						fmt.Printf("当前请求数据在ap.entries位置是%d\n", i)
						break
					}
				}
			}
			f := func(context.Context) { s.applyAll(&ep, &ap) }
			sched.Schedule(f)
		case leases := <-expiredLeaseC:
			// 在 lessor 实例初始化的过程中会启动一个后台 goroutine，该后台 goroutine 会定时扫描 lessor.leaseMap 宇段， 并将过期的 Lease 实例写入 expireLeaseC 通道中等待处理。
			s.GoAttach(func() {
				// Increases throughput of expired leases deletion process through parallelization
				// 用于限流 ，上限是 16
				c := make(chan struct{}, maxPendingRevokes)
				for _, lease := range leases { //遍历过期的 Lease 实例
					select {
					case c <- struct{}{}: //向 c通过中添加一个空结构体
					case <-s.stopping:
						return
					}
					lid := lease.ID
					s.GoAttach(func() { //启动一个后台线程 ， 完成指定 Lease 的撤销
						ctx := s.authStore.WithRoot(s.ctx)
						// 会将 LeaseRevokeRequest请求中的信息封 装成 MsgProp消息，并发送到集群中的其他节点
						// 待上述LeaseRevokeRequest请求对应的Entry记录成功 经过Raft协议处理之后，
						// 会经过前面介绍的 applyEntryNormal()方法的处理，其中会调用 applierV3.LeaseRevoke()方法，
						// 该方法实际上是调用 EtcdServer.lessor.LeaseRevoke()方法，该方法会删除对应的 Lease 实例及其绑定的键值对
						_, lerr := s.LeaseRevoke(ctx, &pb.LeaseRevokeRequest{ID: int64(lid)})
						if lerr == nil {
							leaseExpired.Inc()
						} else {
							lg.Warn(
								"failed to revoke lease",
								zap.String("lease-id", fmt.Sprintf("%016x", lid)),
								zap.Error(lerr),
							)
						}

						<-c
					})
				}
			})
		case err := <-s.errorc:
			lg.Warn("server error", zap.Error(err))
			lg.Warn("data-dir used by this member must be removed")
			return
		case <-getSyncC():
			// SYNC 消息的主要作用是定义通知节点，清理过期 v2 存储中 的过期节点。
			// 在前面介绍 raftReadyHandler 时提到， 当节点成为 Leader 时，会调用 setSyncC()回调函数设置一个定时器，用来触发 SYNC 消息的定期发送。
			if s.v2store.HasTTLKeys() {
				s.sync(s.Cfg.ReqTimeout())
			}
		case <-s.stop:
			return
		}
	}
}

// Cleanup removes allocated objects by EtcdServer.NewServer in
// situation that EtcdServer::Start was not called (that takes care of cleanup).
func (s *EtcdServer) Cleanup() {
	// kv, lessor and backend can be nil if running without v3 enabled
	// or running unit tests.
	if s.lessor != nil {
		s.lessor.Stop()
	}
	if s.kv != nil {
		s.kv.Close()
	}
	if s.authStore != nil {
		s.authStore.Close()
	}
	if s.be != nil {
		s.be.Close()
	}
	if s.compactor != nil {
		s.compactor.Stop()
	}
}

// apply入口
func (s *EtcdServer) applyAll(ep *etcdProgress, apply *apply) {
	fmt.Println("applyAll")
	for _, entry := range apply.entries {
		var rr2 pb.InternalRaftRequest
		err := rr2.Unmarshal(entry.Data)
		if err == nil {
			if rr2.Put != nil {
				fmt.Println("applyAll的entries有put,kv=", string(rr2.Put.Key), string(rr2.Put.Value))
				break
			}
		}
	}
	// 处理 apply 实例 中的快照数据
	fmt.Println("开始applySnapshot")
	s.applySnapshot(ep, apply)
	fmt.Println("处理完applySnapshot")
	// 应用完快照数据之后 ， run goroutine 紧接着会调用 EtcdServer.applyEntries()方法处理待应用的 Entry记录，具体实现如下所示。
	fmt.Println("开始applyEntries")
	s.applyEntries(ep, apply)
	fmt.Println("处理完applyEntries")

	// 在前面的介绍中提到， etcdProgress.appliedi 记录了已应用 Entry 的索引值。 这里通过调用
	// WaitTime.Trigger ()方法 ， 将 id 小于 etcdProgress.appliedi 的 Entry 对应的通道全部关闭
	// 这样就可以通知其他监听通道的 goroutine，例如，前面介绍的 linearizableReadLoop goroutine
	proposalsApplied.Set(float64(ep.appliedi))

	// 作用于一致性读，这里结束后，一致性读取请求就返回结果了
	s.applyWait.Trigger(ep.appliedi)

	// 读者可以回顾前面对 raftNode 的分析， 当 Ready 处理基本完成时， 会向 notifyc 通道中 写入一个信号，
	// 通知当前 goroutine 去检测是否需妥生成快照
	// wait for the raft routine to finish the disk writes before triggering a
	// snapshot. or applied index might be greater than the last index in raft
	// storage, since the raft routine might be slower than apply routine.
	<-apply.notifyc

	// 根据当前状态决定是否触发快照的生成，后面展开介绍
	s.triggerSnapshot(ep)
	select {
	// 在 raftNode 中处理 Ready 实例时 ，如果并没有直接发送 MsgSnap 消息 ，而是将其 写入 msgSnapC 通道中，
	// 这里会读取 msgSnapC 通道 ，并完成快照数据的发送
	// snapshot requested via send()
	case m := <-s.r.msgSnapC:
		//将 v2 存储的快照 数据和 v3 存储 的数据合并成完整的快照数据
		merged := s.createMergedSnapshotMessage(m, ep.appliedt, ep.appliedi, ep.confState)

		//创建完 snap.Message 实例之后会调用 EtcdServer.sendMergedSnap()方法将其发送到指定节 点
		s.sendMergedSnap(merged) //发送快照数据
	default:
	}
}

// 会先等待 raftNode 将快照数据持久化到磁盘中， 之后根据快照元数据查找 BoltDB 数据库文件并重建 Backend 实例，
// 最后根据重建后的存储更新本地 RaftCluster实例
func (s *EtcdServer) applySnapshot(ep *etcdProgress, apply *apply) {
	if raft.IsEmptySnap(apply.snapshot) {
		// 检测待应用的快照数据是否为空，如果 为空 则直接返回(咯)
		return
	}
	applySnapshotInProgress.Inc()

	lg := s.Logger()
	lg.Info(
		"applying snapshot",
		zap.Uint64("current-snapshot-index", ep.snapi),
		zap.Uint64("current-applied-index", ep.appliedi),
		zap.Uint64("incoming-leader-snapshot-index", apply.snapshot.Metadata.Index),
		zap.Uint64("incoming-leader-snapshot-term", apply.snapshot.Metadata.Term),
	)
	defer func() {
		lg.Info(
			"applied snapshot",
			zap.Uint64("current-snapshot-index", ep.snapi),
			zap.Uint64("current-applied-index", ep.appliedi),
			zap.Uint64("incoming-leader-snapshot-index", apply.snapshot.Metadata.Index),
			zap.Uint64("incoming-leader-snapshot-term", apply.snapshot.Metadata.Term),
		)
		applySnapshotInProgress.Dec()
	}()

	if apply.snapshot.Metadata.Index <= ep.appliedi {
		// 如采该快照中最后 一条 Entry 的索引值小于当前节点己应用 Entry 索引值，则程序异常结束(咯)
		lg.Panic(
			"unexpected leader snapshot from outdated index",
			zap.Uint64("current-snapshot-index", ep.snapi),
			zap.Uint64("current-applied-index", ep.appliedi),
			zap.Uint64("incoming-leader-snapshot-index", apply.snapshot.Metadata.Index),
			zap.Uint64("incoming-leader-snapshot-term", apply.snapshot.Metadata.Term),
		)
	}

	// 读者 可 以回顾前面对 raftNode 的分析，raftNode 在将快照数据写入磁盘文件之后，会向 notifyc 通道中写入一个空结构体作为信号，这里会阻塞等待该信号
	// wait for raftNode to persist snapshot onto the disk
	<-apply.notifyc

	// 根据快照信息查找对应的 BoltDB 数据库 文件，并创建新的 Backend 实例，错误处理(略)
	newbe, err := openSnapshotBackend(s.Cfg, s.snapshotter, apply.snapshot, s.beHooks)
	if err != nil {
		lg.Panic("failed to open snapshot backend", zap.Error(err))
	}

	// 因为在 store.restore() 方法中除了恢复内存索引，还会重新绑定键值对与对应的 Lease,
	// 所以需要先恢复 EtcdServer.lessor，再恢复 EtcdServer.kv 字段
	// always recover lessor before kv. When we recover the mvcc.KV it will reattach keys to its leases.
	// If we recover mvcc.KV first, it will attach the keys to the wrong lessor before it recovers.
	if s.lessor != nil {
		lg.Info("restoring lease store")

		s.lessor.Recover(newbe, func() lease.TxnDelete { return s.kv.Write(traceutil.TODO()) })

		lg.Info("restored lease store")
	}

	lg.Info("restoring mvcc store")

	if err := s.kv.Restore(newbe); err != nil {
		lg.Panic("failed to restore mvcc store", zap.Error(err))
	}

	// 重置 EtcdServer.consistIndex 字段
	s.consistIndex.SetBackend(newbe)
	lg.Info("restored mvcc store", zap.Uint64("consistent-index", s.consistIndex.ConsistentIndex()))

	// Closing old backend might block until all the txns
	// on the backend are finished.
	// We do not want to wait on closing the old backend.
	s.bemu.Lock()
	oldbe := s.be
	go func() {
		lg.Info("closing old backend file")
		defer func() {
			lg.Info("closed old backend file")
		}()
		// 因为此时可能还有事务在执行，关闭旧 Backend 实例 可能会被阻塞，所以这里启动一个后台 goroutine 用来关闭 Backend 实例
		if err := oldbe.Close(); err != nil {
			lg.Panic("failed to close old backend", zap.Error(err))
		}
	}()

	s.be = newbe // 更新 EtcdServer 实例 中使用的 Backend 实例
	s.bemu.Unlock()

	lg.Info("restoring alarm store")

	// 恢复 EtcdServer 中的 alarmStore 和 authStore，它们分别对应 BoltDB 中的 alarm Bucket 和 auth Bucket ，
	// 后面会介 绍 alarmStore 和 authStore 的作用
	if err := s.restoreAlarms(); err != nil {
		lg.Panic("failed to restore alarm store", zap.Error(err))
	}

	lg.Info("restored alarm store")

	if s.authStore != nil {
		lg.Info("restoring auth store")

		s.authStore.Recover(newbe)

		lg.Info("restored auth store")
	}

	// 恢复 v2 版本存储，该恢复过程在前面已经介绍过 了，这里不再赘述
	lg.Info("restoring v2 store")
	if err := s.v2store.Recovery(apply.snapshot.Data); err != nil {
		lg.Panic("failed to restore v2 store", zap.Error(err))
	}

	if err := assertNoV2StoreContent(lg, s.v2store, s.Cfg.V2Deprecation); err != nil {
		lg.Panic("illegal v2store content", zap.Error(err))
	}

	lg.Info("restored v2 store")

	s.cluster.SetBackend(newbe) // 设立 RaftCluster.backend 实例

	lg.Info("restoring cluster configuration")

	s.cluster.Recover(api.UpdateCapability) // 恢复本地的集群信息，不再展开介绍

	lg.Info("restored cluster configuration")
	lg.Info("removing old peers from network")

	//清空 Transport 中所有 Peer 实例，并根据恢复后的 RaftCluster 实例重新添加
	// recover raft transport
	s.r.transport.RemoveAllPeers()

	lg.Info("removed old peers from network")
	lg.Info("adding peers from new cluster configuration")

	for _, m := range s.cluster.Members() {
		if m.ID == s.ID() {
			continue
		}
		s.r.transport.AddPeer(m.ID, m.PeerURLs)
	}

	lg.Info("added peers from new cluster configuration")

	// 更新 etcdProgress，其中涉及已应用 Entry记录的 Term值、 Index值和快照相关信息
	ep.appliedt = apply.snapshot.Metadata.Term
	ep.appliedi = apply.snapshot.Metadata.Index
	ep.snapi = ep.appliedi
	ep.confState = apply.snapshot.Metadata.ConfState
}

func (s *EtcdServer) applyEntries(ep *etcdProgress, apply *apply) {
	//检测是否存在待应用的 Entry记录，如果为空如l直接返回(略)
	if len(apply.entries) == 0 {
		return
	}
	// 检测待应用 的 第 一条 Entry 记录是否合法
	firsti := apply.entries[0].Index
	fmt.Println("待应用的第一条 Entry 记录的Index=", firsti)
	fmt.Println("ep.snapi=", ep.snapi)
	fmt.Println("ep.appliedt=", ep.appliedt)
	fmt.Println("ep.appliedi=", ep.appliedi)
	fmt.Println("uint64(len(apply.entries))=", uint64(len(apply.entries)))
	if firsti > ep.appliedi+1 {
		lg := s.Logger()
		lg.Panic(
			"unexpected committed entry index",
			zap.Uint64("current-applied-index", ep.appliedi),
			zap.Uint64("first-committed-entry-index", firsti),
		)
	}
	var ents []raftpb.Entry
	if ep.appliedi+1-firsti < uint64(len(apply.entries)) {
		// 忽略 已经应用的 Entry 记录，只保留未应用的 Entry 记录
		ents = apply.entries[ep.appliedi+1-firsti:]
	}
	if len(ents) == 0 {
		fmt.Println("ents == 0, applyEntries return")
		return
	}
	var shouldstop bool
	// 调用 apply ()方法应用 ents 中的 Entry 记录
	// 在 apply() 方法中会遍历 ents 中的全部 Entry记录，并根据 Entry 的类型进行不同的处理。
	fmt.Println("调用 apply ()方法应用 ents 中的 Entry 记录")
	if ep.appliedt, ep.appliedi, shouldstop = s.apply(ents, &ep.confState); shouldstop {
		go s.stopWithDelay(10*100*time.Millisecond, fmt.Errorf("the member has been permanently removed from the cluster"))
	}
}

func (s *EtcdServer) triggerSnapshot(ep *etcdProgress) {
	if ep.appliedi-ep.snapi <= s.Cfg.SnapshotCount {
		//连续应用一定量的 Entry 记录， 会触发快照的生成( snapCount 默认为 100000 条 )
		return
	}

	lg := s.Logger()
	lg.Info(
		"triggering snapshot",
		zap.String("local-member-id", s.ID().String()),
		zap.Uint64("local-member-applied-index", ep.appliedi),
		zap.Uint64("local-member-snapshot-index", ep.snapi),
		zap.Uint64("local-member-snapshot-count", s.Cfg.SnapshotCount),
	)
	//创建新的快照文件
	s.snapshot(ep.appliedi, ep.confState)
	//更新 etcdProgress.snapi
	ep.snapi = ep.appliedi
}

func (s *EtcdServer) hasMultipleVotingMembers() bool {
	return s.cluster != nil && len(s.cluster.VotingMemberIDs()) > 1
}

func (s *EtcdServer) isLeader() bool {
	return uint64(s.ID()) == s.Lead()
}

// MoveLeader transfers the leader to the given transferee.
func (s *EtcdServer) MoveLeader(ctx context.Context, lead, transferee uint64) error {
	if !s.cluster.IsMemberExist(types.ID(transferee)) || s.cluster.Member(types.ID(transferee)).IsLearner {
		return ErrBadLeaderTransferee
	}

	now := time.Now()
	interval := time.Duration(s.Cfg.TickMs) * time.Millisecond

	lg := s.Logger()
	lg.Info(
		"leadership transfer starting",
		zap.String("local-member-id", s.ID().String()),
		zap.String("current-leader-member-id", types.ID(lead).String()),
		zap.String("transferee-member-id", types.ID(transferee).String()),
	)

	s.r.TransferLeadership(ctx, lead, transferee)
	for s.Lead() != transferee {
		select {
		case <-ctx.Done(): // time out
			return ErrTimeoutLeaderTransfer
		case <-time.After(interval):
		}
	}

	// TODO: drain all requests, or drop all messages to the old leader
	lg.Info(
		"leadership transfer finished",
		zap.String("local-member-id", s.ID().String()),
		zap.String("old-leader-member-id", types.ID(lead).String()),
		zap.String("new-leader-member-id", types.ID(transferee).String()),
		zap.Duration("took", time.Since(now)),
	)
	return nil
}

// TransferLeadership transfers the leader to the chosen transferee.
func (s *EtcdServer) TransferLeadership() error {
	lg := s.Logger()
	if !s.isLeader() {
		lg.Info(
			"skipped leadership transfer; local server is not leader",
			zap.String("local-member-id", s.ID().String()),
			zap.String("current-leader-member-id", types.ID(s.Lead()).String()),
		)
		return nil
	}

	if !s.hasMultipleVotingMembers() {
		lg.Info(
			"skipped leadership transfer for single voting member cluster",
			zap.String("local-member-id", s.ID().String()),
			zap.String("current-leader-member-id", types.ID(s.Lead()).String()),
		)
		return nil
	}

	transferee, ok := longestConnected(s.r.transport, s.cluster.VotingMemberIDs())
	if !ok {
		return ErrUnhealthy
	}

	tm := s.Cfg.ReqTimeout()
	ctx, cancel := context.WithTimeout(s.ctx, tm)
	err := s.MoveLeader(ctx, s.Lead(), uint64(transferee))
	cancel()
	return err
}

// HardStop stops the server without coordination with other members in the cluster.
func (s *EtcdServer) HardStop() {
	select {
	case s.stop <- struct{}{}:
	case <-s.done:
		return
	}
	<-s.done
}

// Stop stops the server gracefully, and shuts down the running goroutine.
// Stop should be called after a Start(s), otherwise it will block forever.
// When stopping leader, Stop transfers its leadership to one of its peers
// before stopping the server.
// Stop terminates the Server and performs any necessary finalization.
// Do and Process cannot be called after Stop has been invoked.
func (s *EtcdServer) Stop() {
	lg := s.Logger()
	if err := s.TransferLeadership(); err != nil {
		lg.Warn("leadership transfer failed", zap.String("local-member-id", s.ID().String()), zap.Error(err))
	}
	s.HardStop()
}

// ReadyNotify returns a channel that will be closed when the server
// is ready to serve client requests
func (s *EtcdServer) ReadyNotify() <-chan struct{} { return s.readych }

func (s *EtcdServer) stopWithDelay(d time.Duration, err error) {
	select {
	case <-time.After(d):
	case <-s.done:
	}
	select {
	case s.errorc <- err:
	default:
	}
}

// StopNotify returns a channel that receives a empty struct
// when the server is stopped.
func (s *EtcdServer) StopNotify() <-chan struct{} { return s.done }

// StoppingNotify returns a channel that receives a empty struct
// when the server is being stopped.
func (s *EtcdServer) StoppingNotify() <-chan struct{} { return s.stopping }

func (s *EtcdServer) SelfStats() []byte { return s.stats.JSON() }

func (s *EtcdServer) LeaderStats() []byte {
	lead := s.getLead()
	if lead != uint64(s.id) {
		return nil
	}
	return s.lstats.JSON()
}

func (s *EtcdServer) StoreStats() []byte { return s.v2store.JsonStats() }

func (s *EtcdServer) checkMembershipOperationPermission(ctx context.Context) error {
	if s.authStore == nil {
		// In the context of ordinary etcd process, s.authStore will never be nil.
		// This branch is for handling cases in server_test.go
		return nil
	}

	// Note that this permission check is done in the API layer,
	// so TOCTOU problem can be caused potentially in a schedule like this:
	// update membership with user A -> revoke root role of A -> apply membership change
	// in the state machine layer
	// However, both of membership change and role management requires the root privilege.
	// So careful operation by admins can prevent the problem.
	authInfo, err := s.AuthInfoFromCtx(ctx)
	if err != nil {
		return err
	}

	return s.AuthStore().IsAdminPermitted(authInfo)
}

func (s *EtcdServer) AddMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error) {
	if err := s.checkMembershipOperationPermission(ctx); err != nil {
		return nil, err
	}

	// TODO: move Member to protobuf type
	b, err := json.Marshal(memb)
	if err != nil {
		return nil, err
	}

	// by default StrictReconfigCheck is enabled; reject new members if unhealthy.
	if err := s.mayAddMember(memb); err != nil {
		return nil, err
	}

	cc := raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  uint64(memb.ID),
		Context: b,
	}

	if memb.IsLearner {
		cc.Type = raftpb.ConfChangeAddLearnerNode
	}

	return s.configure(ctx, cc)
}

func (s *EtcdServer) mayAddMember(memb membership.Member) error {
	lg := s.Logger()
	if !s.Cfg.StrictReconfigCheck {
		return nil
	}

	// protect quorum when adding voting member
	if !memb.IsLearner && !s.cluster.IsReadyToAddVotingMember() {
		lg.Warn(
			"rejecting member add request; not enough healthy members",
			zap.String("local-member-id", s.ID().String()),
			zap.String("requested-member-add", fmt.Sprintf("%+v", memb)),
			zap.Error(ErrNotEnoughStartedMembers),
		)
		return ErrNotEnoughStartedMembers
	}

	if !isConnectedFullySince(s.r.transport, time.Now().Add(-HealthInterval), s.ID(), s.cluster.VotingMembers()) {
		lg.Warn(
			"rejecting member add request; local member has not been connected to all peers, reconfigure breaks active quorum",
			zap.String("local-member-id", s.ID().String()),
			zap.String("requested-member-add", fmt.Sprintf("%+v", memb)),
			zap.Error(ErrUnhealthy),
		)
		return ErrUnhealthy
	}

	return nil
}

func (s *EtcdServer) RemoveMember(ctx context.Context, id uint64) ([]*membership.Member, error) {
	if err := s.checkMembershipOperationPermission(ctx); err != nil {
		return nil, err
	}

	// by default StrictReconfigCheck is enabled; reject removal if leads to quorum loss
	if err := s.mayRemoveMember(types.ID(id)); err != nil {
		return nil, err
	}

	cc := raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: id,
	}
	return s.configure(ctx, cc)
}

// PromoteMember promotes a learner node to a voting node.
func (s *EtcdServer) PromoteMember(ctx context.Context, id uint64) ([]*membership.Member, error) {
	// only raft leader has information on whether the to-be-promoted learner node is ready. If promoteMember call
	// fails with ErrNotLeader, forward the request to leader node via HTTP. If promoteMember call fails with error
	// other than ErrNotLeader, return the error.
	resp, err := s.promoteMember(ctx, id)
	if err == nil {
		learnerPromoteSucceed.Inc()
		return resp, nil
	}
	if err != ErrNotLeader {
		learnerPromoteFailed.WithLabelValues(err.Error()).Inc()
		return resp, err
	}

	cctx, cancel := context.WithTimeout(ctx, s.Cfg.ReqTimeout())
	defer cancel()
	// forward to leader
	for cctx.Err() == nil {
		leader, err := s.waitLeader(cctx)
		if err != nil {
			return nil, err
		}
		for _, url := range leader.PeerURLs {
			resp, err := promoteMemberHTTP(cctx, url, id, s.peerRt)
			if err == nil {
				return resp, nil
			}
			// If member promotion failed, return early. Otherwise keep retry.
			if err == ErrLearnerNotReady || err == membership.ErrIDNotFound || err == membership.ErrMemberNotLearner {
				return nil, err
			}
		}
	}

	if cctx.Err() == context.DeadlineExceeded {
		return nil, ErrTimeout
	}
	return nil, ErrCanceled
}

// promoteMember checks whether the to-be-promoted learner node is ready before sending the promote
// request to raft.
// The function returns ErrNotLeader if the local node is not raft leader (therefore does not have
// enough information to determine if the learner node is ready), returns ErrLearnerNotReady if the
// local node is leader (therefore has enough information) but decided the learner node is not ready
// to be promoted.
func (s *EtcdServer) promoteMember(ctx context.Context, id uint64) ([]*membership.Member, error) {
	if err := s.checkMembershipOperationPermission(ctx); err != nil {
		return nil, err
	}

	// check if we can promote this learner.
	if err := s.mayPromoteMember(types.ID(id)); err != nil {
		return nil, err
	}

	// build the context for the promote confChange. mark IsLearner to false and IsPromote to true.
	promoteChangeContext := membership.ConfigChangeContext{
		Member: membership.Member{
			ID: types.ID(id),
		},
		IsPromote: true,
	}

	b, err := json.Marshal(promoteChangeContext)
	if err != nil {
		return nil, err
	}

	cc := raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  id,
		Context: b,
	}

	return s.configure(ctx, cc)
}

func (s *EtcdServer) mayPromoteMember(id types.ID) error {
	lg := s.Logger()
	err := s.isLearnerReady(uint64(id))
	if err != nil {
		return err
	}

	if !s.Cfg.StrictReconfigCheck {
		return nil
	}
	if !s.cluster.IsReadyToPromoteMember(uint64(id)) {
		lg.Warn(
			"rejecting member promote request; not enough healthy members",
			zap.String("local-member-id", s.ID().String()),
			zap.String("requested-member-remove-id", id.String()),
			zap.Error(ErrNotEnoughStartedMembers),
		)
		return ErrNotEnoughStartedMembers
	}

	return nil
}

// check whether the learner catches up with leader or not.
// Note: it will return nil if member is not found in cluster or if member is not learner.
// These two conditions will be checked before apply phase later.
func (s *EtcdServer) isLearnerReady(id uint64) error {
	rs := s.raftStatus()

	// leader's raftStatus.Progress is not nil
	if rs.Progress == nil {
		return ErrNotLeader
	}

	var learnerMatch uint64
	isFound := false
	leaderID := rs.ID
	for memberID, progress := range rs.Progress {
		if id == memberID {
			// check its status
			learnerMatch = progress.Match
			isFound = true
			break
		}
	}

	if isFound {
		leaderMatch := rs.Progress[leaderID].Match
		// the learner's Match not caught up with leader yet
		if float64(learnerMatch) < float64(leaderMatch)*readyPercent {
			return ErrLearnerNotReady
		}
	}

	return nil
}

func (s *EtcdServer) mayRemoveMember(id types.ID) error {
	if !s.Cfg.StrictReconfigCheck {
		return nil
	}

	lg := s.Logger()
	isLearner := s.cluster.IsMemberExist(id) && s.cluster.Member(id).IsLearner
	// no need to check quorum when removing non-voting member
	if isLearner {
		return nil
	}

	if !s.cluster.IsReadyToRemoveVotingMember(uint64(id)) {
		lg.Warn(
			"rejecting member remove request; not enough healthy members",
			zap.String("local-member-id", s.ID().String()),
			zap.String("requested-member-remove-id", id.String()),
			zap.Error(ErrNotEnoughStartedMembers),
		)
		return ErrNotEnoughStartedMembers
	}

	// downed member is safe to remove since it's not part of the active quorum
	if t := s.r.transport.ActiveSince(id); id != s.ID() && t.IsZero() {
		return nil
	}

	// protect quorum if some members are down
	m := s.cluster.VotingMembers()
	active := numConnectedSince(s.r.transport, time.Now().Add(-HealthInterval), s.ID(), m)
	if (active - 1) < 1+((len(m)-1)/2) {
		lg.Warn(
			"rejecting member remove request; local member has not been connected to all peers, reconfigure breaks active quorum",
			zap.String("local-member-id", s.ID().String()),
			zap.String("requested-member-remove", id.String()),
			zap.Int("active-peers", active),
			zap.Error(ErrUnhealthy),
		)
		return ErrUnhealthy
	}

	return nil
}

func (s *EtcdServer) UpdateMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error) {
	b, merr := json.Marshal(memb)
	if merr != nil {
		return nil, merr
	}

	if err := s.checkMembershipOperationPermission(ctx); err != nil {
		return nil, err
	}
	cc := raftpb.ConfChange{
		Type:    raftpb.ConfChangeUpdateNode,
		NodeID:  uint64(memb.ID),
		Context: b,
	}
	return s.configure(ctx, cc)
}

func (s *EtcdServer) setCommittedIndex(v uint64) {
	atomic.StoreUint64(&s.committedIndex, v)
}

func (s *EtcdServer) getCommittedIndex() uint64 {
	return atomic.LoadUint64(&s.committedIndex)
}

func (s *EtcdServer) setAppliedIndex(v uint64) {
	atomic.StoreUint64(&s.appliedIndex, v)
}

func (s *EtcdServer) getAppliedIndex() uint64 {
	return atomic.LoadUint64(&s.appliedIndex)
}

func (s *EtcdServer) setTerm(v uint64) {
	atomic.StoreUint64(&s.term, v)
}

func (s *EtcdServer) getTerm() uint64 {
	return atomic.LoadUint64(&s.term)
}

func (s *EtcdServer) setLead(v uint64) {
	atomic.StoreUint64(&s.lead, v)
}

func (s *EtcdServer) getLead() uint64 {
	return atomic.LoadUint64(&s.lead)
}

func (s *EtcdServer) LeaderChangedNotify() <-chan struct{} {
	s.leaderChangedMu.RLock()
	defer s.leaderChangedMu.RUnlock()
	return s.leaderChanged
}

// FirstCommitInTermNotify returns channel that will be unlocked on first
// entry committed in new term, which is necessary for new leader to answer
// read-only requests (leader is not able to respond any read-only requests
// as long as linearizable semantic is required)
func (s *EtcdServer) FirstCommitInTermNotify() <-chan struct{} {
	s.firstCommitInTermMu.RLock()
	defer s.firstCommitInTermMu.RUnlock()
	return s.firstCommitInTermC
}

// RaftStatusGetter represents etcd server and Raft progress.
type RaftStatusGetter interface {
	ID() types.ID
	Leader() types.ID
	CommittedIndex() uint64
	AppliedIndex() uint64
	Term() uint64
}

func (s *EtcdServer) ID() types.ID { return s.id }

func (s *EtcdServer) Leader() types.ID { return types.ID(s.getLead()) }

func (s *EtcdServer) Lead() uint64 { return s.getLead() }

func (s *EtcdServer) CommittedIndex() uint64 { return s.getCommittedIndex() }

func (s *EtcdServer) AppliedIndex() uint64 { return s.getAppliedIndex() }

func (s *EtcdServer) Term() uint64 { return s.getTerm() }

type confChangeResponse struct {
	membs []*membership.Member
	err   error
}

// configure sends a configuration change through consensus and
// then waits for it to be applied to the server. It
// will block until the change is performed or there is an error.
func (s *EtcdServer) configure(ctx context.Context, cc raftpb.ConfChange) ([]*membership.Member, error) {
	lg := s.Logger()
	cc.ID = s.reqIDGen.Next()
	ch := s.w.Register(cc.ID)

	start := time.Now()
	if err := s.r.ProposeConfChange(ctx, cc); err != nil {
		s.w.Trigger(cc.ID, nil)
		return nil, err
	}

	select {
	case x := <-ch:
		if x == nil {
			lg.Panic("failed to configure")
		}
		resp := x.(*confChangeResponse)
		lg.Info(
			"applied a configuration change through raft",
			zap.String("local-member-id", s.ID().String()),
			zap.String("raft-conf-change", cc.Type.String()),
			zap.String("raft-conf-change-node-id", types.ID(cc.NodeID).String()),
		)
		return resp.membs, resp.err

	case <-ctx.Done():
		s.w.Trigger(cc.ID, nil) // GC wait
		return nil, s.parseProposeCtxErr(ctx.Err(), start)

	case <-s.stopping:
		return nil, ErrStopped
	}
}

// sync proposes a SYNC request and is non-blocking.
// This makes no guarantee that the request will be proposed or performed.
// The request will be canceled after the given timeout.
func (s *EtcdServer) sync(timeout time.Duration) {
	// 创建Sync消息
	req := pb.Request{
		Method: "SYNC",
		ID:     s.reqIDGen.Next(),
		Time:   time.Now().UnixNano(),
	}
	// 序列化消息
	data := pbutil.MustMarshal(&req)
	// There is no promise that node has leader when do SYNC request,
	// so it uses goroutine to propose.
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	s.GoAttach(func() { // 启动一个后台goroutine 发送 SYNC (MsgProp)消息
		s.r.Propose(ctx, data)
		cancel()
	})
}

// publishV3 registers server information into the cluster using v3 request. The
// information is the JSON representation of this server's member struct, updated
// with the static clientURLs of the server.
// The function keeps attempting to register until it succeeds,
// or its server is stopped.
func (s *EtcdServer) publishV3(timeout time.Duration) {
	req := &membershippb.ClusterMemberAttrSetRequest{
		Member_ID: uint64(s.id),
		MemberAttributes: &membershippb.Attributes{
			Name:       s.attributes.Name,
			ClientUrls: s.attributes.ClientURLs,
		},
	}
	lg := s.Logger()
	for {
		select {
		case <-s.stopping:
			lg.Warn(
				"stopped publish because server is stopping",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.Duration("publish-timeout", timeout),
			)
			return

		default:
		}

		ctx, cancel := context.WithTimeout(s.ctx, timeout)
		_, err := s.raftRequest(ctx, pb.InternalRaftRequest{ClusterMemberAttrSet: req})
		cancel()
		switch err {
		case nil:
			close(s.readych)
			lg.Info(
				"published local member to cluster through raft",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.String("cluster-id", s.cluster.ID().String()),
				zap.Duration("publish-timeout", timeout),
			)
			return

		default:
			lg.Warn(
				"failed to publish local member to cluster through raft",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.Duration("publish-timeout", timeout),
				zap.Error(err),
			)
		}
	}
}

// publish registers server information into the cluster. The information
// is the JSON representation of this server's member struct, updated with the
// static clientURLs of the server.
// The function keeps attempting to register until it succeeds,
// or its server is stopped.
//
// Use v2 store to encode member attributes, and apply through Raft
// but does not go through v2 API endpoint, which means even with v2
// client handler disabled (e.g. --enable-v2=false), cluster can still
// process publish requests through rafthttp
// TODO: Remove in 3.6 (start using publishV3)
func (s *EtcdServer) publish(timeout time.Duration) {
	lg := s.Logger()
	b, err := json.Marshal(s.attributes)
	if err != nil {
		lg.Panic("failed to marshal JSON", zap.Error(err))
		return
	}
	req := pb.Request{
		Method: "PUT",
		Path:   membership.MemberAttributesStorePath(s.id),
		Val:    string(b),
	}

	for {
		ctx, cancel := context.WithTimeout(s.ctx, timeout)
		_, err := s.Do(ctx, req)
		cancel()
		switch err {
		case nil:
			close(s.readych)
			lg.Info(
				"published local member to cluster through raft",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.String("request-path", req.Path),
				zap.String("cluster-id", s.cluster.ID().String()),
				zap.Duration("publish-timeout", timeout),
			)
			return

		case ErrStopped:
			lg.Warn(
				"stopped publish because server is stopped",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.Duration("publish-timeout", timeout),
				zap.Error(err),
			)
			return

		default:
			lg.Warn(
				"failed to publish local member to cluster through raft",
				zap.String("local-member-id", s.ID().String()),
				zap.String("local-member-attributes", fmt.Sprintf("%+v", s.attributes)),
				zap.String("request-path", req.Path),
				zap.Duration("publish-timeout", timeout),
				zap.Error(err),
			)
		}
	}
}

func (s *EtcdServer) sendMergedSnap(merged snap.Message) {
	//递增 inflightSnapshots 字段， 它表示已发送但未收到响应的快照消息个数
	atomic.AddInt64(&s.inflightSnapshots, 1)

	lg := s.Logger()
	fields := []zap.Field{
		zap.String("from", s.ID().String()),
		zap.String("to", types.ID(merged.To).String()),
		zap.Int64("bytes", merged.TotalSize),
		zap.String("size", humanize.Bytes(uint64(merged.TotalSize))),
	}

	now := time.Now()
	//发送 snap.Message 消息， 底层会启动单独的后台 goroutine， 通过 snapshotSender 完成发送，
	s.r.transport.SendSnapshot(merged)
	lg.Info("sending merged snapshot", fields...)

	s.GoAttach(func() {
		select {
		case ok := <-merged.CloseNotify():
			// delay releasing inflight snapshot for another 30 seconds to
			// block log compaction.
			// If the follower still fails to catch up, it is probably just too slow
			// to catch up. We cannot avoid the snapshot cycle anyway.
			//启动一个后台 goroutine 监听该快照消息是否发送完成
			if ok {
				select {
				case <-time.After(releaseDelayAfterSnapshot): //默认阻塞等待 30 秒
				case <-s.stopping:
				}
			}

			// snap.Message，消息发送完成(或是超时)之后会递减 inflightSnapshots 值
			// 当 inflightSnapshots 递减 到 0 时 ， 前面对 MemoryStorage 的压缩才能执行
			atomic.AddInt64(&s.inflightSnapshots, -1)

			lg.Info("sent merged snapshot", append(fields, zap.Duration("took", time.Since(now)))...)

		case <-s.stopping:
			lg.Warn("canceled sending merged snapshot; server stopping", fields...)
			return
		}
	})
}

// apply takes entries received from Raft (after it has been committed) and
// applies them to the current state of the EtcdServer.
// The given entries should not be empty.
func (s *EtcdServer) apply(
	es []raftpb.Entry,
	confState *raftpb.ConfState,
) (appliedt uint64, appliedi uint64, shouldStop bool) {
	s.lg.Debug("Applying entries", zap.Int("num-entries", len(es)))
	//遍历待应用的 Entry记录
	for i := range es {
		e := es[i]
		s.lg.Debug("Applying entry",
			zap.Uint64("index", e.Index),
			zap.Uint64("term", e.Term),
			zap.Stringer("type", e.Type))

		// 根据 Entry记录的不同类型，进行不同的处理
		switch e.Type {
		case raftpb.EntryNormal:
			s.applyEntryNormal(&e)
			s.setAppliedIndex(e.Index)
			s.setTerm(e.Term)

		case raftpb.EntryConfChange:
			// We need to apply all WAL entries on top of v2store
			// and only 'unapplied' (e.Index>backend.ConsistentIndex) on the backend.
			shouldApplyV3 := membership.ApplyV2storeOnly

			// 更新 EtcdServer.consistIndex，其中保存了当前节点应用的最后一条 Entry 记录的索引值

			// set the consistent index of current executing entry
			if e.Index > s.consistIndex.ConsistentIndex() {
				s.consistIndex.SetConsistentIndex(e.Index, e.Term)
				shouldApplyV3 = membership.ApplyBoth
			}

			var cc raftpb.ConfChange
			// 将 Entry.Data反序列化成 ConfChange 实例
			pbutil.MustUnmarshal(&cc, e.Data)
			// 调用 applyConfChange ()方法处理 ConfChange，注意返回值 removedSelf，当它为 true 时表示将当前节点从集群中移除
			removedSelf, err := s.applyConfChange(cc, confState, shouldApplyV3)

			// 更新raftNode的index字段和term字段返回值

			s.setAppliedIndex(e.Index) // 更新 EtcdServer.appliedIndex 字段
			s.setTerm(e.Term)
			// 在后面介绍 AddMember ()、 UpdateMember ()、 RemoveMember ()等方法时会看到它们都会阻塞监听 ConfChange 对应的通道，
			// Wait 的原理不再赘述 。这里在对应的 Entry 处理完成之后，会关闭 对应的通道，通知监听 的 goroutine
			shouldStop = shouldStop || removedSelf
			s.w.Trigger(cc.ID, &confChangeResponse{s.cluster.Members(), err})

		default:
			lg := s.Logger()
			lg.Panic(
				"unknown entry type; must be either EntryNormal or EntryConfChange",
				zap.String("type", e.Type.String()),
			)
		}
		appliedi, appliedt = e.Index, e.Term
	}
	// shouldStop 标识当前节点是否已经从集群中移除
	return appliedt, appliedi, shouldStop
}

// applyEntryNormal()方法首先会尝试将 Entry.Data反序列 化成 InternalRaftRequest实例 ，如果失败，
// 则将其反序列化成 etcdserverpb.Request 实例，之后根据反序列化的结果调用 EtcdServer 的相应方法进行处理，最后将处理结果写入 Entry对应的通道中
// applyEntryNormal apples an EntryNormal type raftpb request to the EtcdServer
func (s *EtcdServer) applyEntryNormal(e *raftpb.Entry) {
	shouldApplyV3 := membership.ApplyV2storeOnly
	index := s.consistIndex.ConsistentIndex()
	if e.Index > index {
		// set the consistent index of current executing entry
		// 更新 EtcdServer.consistIndex 记录 // 的索引位
		s.consistIndex.SetConsistentIndex(e.Index, e.Term)
		shouldApplyV3 = membership.ApplyBoth
	}
	s.lg.Debug("apply entry normal",
		zap.Uint64("consistent-index", index),
		zap.Uint64("entry-index", e.Index),
		zap.Bool("should-applyV3", bool(shouldApplyV3)))

	// raft state machine may generate noop entry when leader confirmation.
	// skip it in advance to avoid some potential bug in the future
	if len(e.Data) == 0 {
		s.notifyAboutFirstCommitInTerm()

		// 空的 Entry记录只会在 Leader 选举结束时出现 如果当前节点为 Leader，则晋升其 lessor 实例
		// promote lessor when the local member is leader and finished
		// applying all entries from the last term.
		if s.isLeader() {
			s.lessor.Promote(s.Cfg.ElectionTimeout())
		}
		return
	}

	// 尝试将 Entry.Data 反序列化成 InternalRaftRequest 实例 ， InternalRaftRequest 中封装 了所有类型的 Client 请求
	var raftReq pb.InternalRaftRequest
	if !pbutil.MaybeUnmarshal(&raftReq, e.Data) { // backward compatible
		var r pb.Request
		rp := &r
		// 兼容性处理 ， 如采上述序列化失败，则将 Entry.Data 反序列化成 pb.Request
		pbutil.MustUnmarshal(rp, e.Data)
		s.lg.Debug("applyEntryNormal", zap.Stringer("V2request", rp))
		// 调用 EtcdServer.applyV2Request ()方法进行处理，在 applyV2Request() 方法中，会根据请求的类型，
		// 调用不同的方法进行处理。处理结束后会将结果写入 Wait 中记录对应的通道中，然后关闭该通道
		s.w.Trigger(r.ID, s.applyV2Request((*RequestV2)(rp), shouldApplyV3))
		return
	}
	s.lg.Debug("applyEntryNormal", zap.Stringer("raftReq", &raftReq))

	if raftReq.V2 != nil {
		// 上述序列化成功，且是v2版本的请求，调用 applyV2Request()方法处理
		req := (*RequestV2)(raftReq.V2)
		s.w.Trigger(req.ID, s.applyV2Request(req, shouldApplyV3)) //关闭 Wait 中记录的该 Entry对应的通道
		return
	}

	// 下面是对v3版本请求的处理

	id := raftReq.ID //获取请求id
	if id == 0 {
		id = raftReq.Header.ID
	}

	var ar *applyResult
	needResult := s.w.IsRegistered(id) //检测该请求是否需要进行响应
	if needResult || !noSideEffect(&raftReq) {
		if !needResult && raftReq.Txn != nil {
			removeNeedlessRangeReqs(raftReq.Txn)
		}
		//调用 applyV3.Apply() 方法处理该 Entry，其中会根据请求的类型选择不同的方法进行处理
		ar = s.applyV3.Apply(&raftReq, shouldApplyV3)
	}

	// do not re-apply applied entries.
	if !shouldApplyV3 {
		return
	}

	if ar == nil {
		return
	}

	fmt.Println("applyEntryNormal, kv=", string(raftReq.Put.Key), string(raftReq.Put.Value))

	// 返回结果 ar(applyResult 类型)为 nil，直接返回(咯)
	// 如果返回了 ErrNoSpace 错误，则表示底层的 Backend 已经没有足够的空间，如采是第一次出现这种情况， 则在后面立即启动一个后台 goroutine，并调用 EtcdServer.raftRequest()方法发送
	// AlarmRequest 请求，当前其他节点收到该请求时，会停止后续的 PUT 操作，后面介绍 applyV3 及其实现时详细分析
	if ar.err != ErrNoSpace || len(s.alarmStore.Get(pb.AlarmType_NOSPACE)) > 0 {
		// 将上述处理结果写入对应的通道中，然后将对应通道关闭
		// 这个id是请求的ID，这里结束后，put请求就会返回了
		fmt.Println("ar.err != ErrNoSpace || len(s.alarmStore.Get(pb.AlarmType_NOSPACE)) > 0, id=", id)
		s.w.Trigger(id, ar)
		return
	}

	lg := s.Logger()
	lg.Warn(
		"message exceeded backend quota; raising alarm",
		zap.Int64("quota-size-bytes", s.Cfg.QuotaBackendBytes),
		zap.String("quota-size", humanize.Bytes(uint64(s.Cfg.QuotaBackendBytes))),
		zap.Error(ar.err),
	)

	s.GoAttach(func() {
		//第一次出现 ErrNoSpace错误
		a := &pb.AlarmRequest{
			MemberID: uint64(s.ID()),
			Action:   pb.AlarmRequest_ACTIVATE,
			Alarm:    pb.AlarmType_NOSPACE,
		}
		//将 AlarmRequest 请求封装成 MsgProp 消息，并友送到集群中，后面会详细介绍该方法的实现
		s.raftRequest(s.ctx, pb.InternalRaftRequest{Alarm: a})
		fmt.Println("第一次出现 ErrNoSpace错误, id=", id)
		s.w.Trigger(id, ar) // 将上述处理结果写入对应的通道中，然后将对应通过关闭
	})
}

func (s *EtcdServer) notifyAboutFirstCommitInTerm() {
	newNotifier := make(chan struct{})
	s.firstCommitInTermMu.Lock()
	notifierToClose := s.firstCommitInTermC
	s.firstCommitInTermC = newNotifier
	s.firstCommitInTermMu.Unlock()
	close(notifierToClose)
}

// applyConfChange applies a ConfChange to the server. It is only
// invoked with a ConfChange that has already passed through Raft
func (s *EtcdServer) applyConfChange(cc raftpb.ConfChange, confState *raftpb.ConfState, shouldApplyV3 membership.ShouldApplyV3) (bool, error) {
	// 在开始进行节点修改之前，先调用 ValidateConfigurationChange ()方法进行检测，后面会展开介绍该方法的具体实现，错误处理(咯)
	if err := s.cluster.ValidateConfigurationChange(cc); err != nil {
		cc.NodeID = raft.None
		s.r.ApplyConfChange(cc)
		return false, err
	}

	// 将 ConfChange 交给 etcd-raft 模块进行处理，其中会根据 ConfChange 的类型进行分类处理
	// etcd-raft 模块对 ConfChange 的处理在前 已经详细介绍过了，这里不 再展开介绍
	// 这里的返回值的 ConfState 中记录了集群中最新的节点id
	lg := s.Logger()
	*confState = *s.r.ApplyConfChange(cc)
	s.beHooks.SetConfState(confState)
	switch cc.Type {
	//根据请求的类型进行相应的处理
	case raftpb.ConfChangeAddNode, raftpb.ConfChangeAddLearnerNode:
		//将 ConfChange.Context 中的 数据反序列 化成 Member 实例 ，错误处理(咯)
		confChangeContext := new(membership.ConfigChangeContext)
		if err := json.Unmarshal(cc.Context, confChangeContext); err != nil {
			lg.Panic("failed to unmarshal member", zap.Error(err))
		}
		if cc.NodeID != uint64(confChangeContext.Member.ID) {
			lg.Panic(
				"got different member ID",
				zap.String("member-id-from-config-change-entry", types.ID(cc.NodeID).String()),
				zap.String("member-id-from-message", confChangeContext.Member.ID.String()),
			)
		}
		if confChangeContext.IsPromote {
			s.cluster.PromoteMember(confChangeContext.Member.ID, shouldApplyV3)
		} else {
			// 将新 Member 实例添加到本地 RaftCluster 中
			s.cluster.AddMember(&confChangeContext.Member, shouldApplyV3)

			// 如果添加的是远端节点，则需要在 Transport 中添加对应的 Peer 实例
			if confChangeContext.Member.ID != s.id {
				s.r.transport.AddPeer(confChangeContext.Member.ID, confChangeContext.PeerURLs)
			}
		}

		// update the isLearner metric when this server id is equal to the id in raft member confChange
		if confChangeContext.Member.ID == s.id {
			if cc.Type == raftpb.ConfChangeAddLearnerNode {
				isLearner.Set(1)
			} else {
				isLearner.Set(0)
			}
		}

	case raftpb.ConfChangeRemoveNode:
		id := types.ID(cc.NodeID)
		//从 RaftCluster 中删除指定的 Member 实伽
		s.cluster.RemoveMember(id, shouldApplyV3)
		if id == s.id {
			// 如果移除的是当前节点则返回 true
			return true, nil
		}
		// 如果移除远程节点，则需要从Transporter中删除对应的Peer实例
		s.r.transport.RemovePeer(id)

	case raftpb.ConfChangeUpdateNode:
		//将 ConfChange.Context 中的数据反序列化成 Member 实例
		m := new(membership.Member)
		if err := json.Unmarshal(cc.Context, m); err != nil {
			lg.Panic("failed to unmarshal member", zap.Error(err))
		}
		if cc.NodeID != uint64(m.ID) {
			lg.Panic(
				"got different member ID",
				zap.String("member-id-from-config-change-entry", types.ID(cc.NodeID).String()),
				zap.String("member-id-from-message", m.ID.String()),
			)
		}
		// 更新本地 RaftCluster 实例中相应的 Member 实例
		s.cluster.UpdateRaftAttributes(m.ID, m.RaftAttributes, shouldApplyV3)
		if m.ID != s.id {
			// 如果更新的是远端节点，则需要更新Transporter中对应的 Peer 实例
			s.r.transport.UpdatePeer(m.ID, m.PeerURLs)
		}
	}
	return false, nil
}

// TODO: non-blocking snapshot
// EtcdServer.snapshot()方法是真正生成快照文件的地方，其中会启动一个单独 的 后台 goroutine来完成新快照文件的生成，主要是序列化 v2存储中的数据并持久化到文件中，触发相应的压缩操作
func (s *EtcdServer) snapshot(snapi uint64, confState raftpb.ConfState) {
	clone := s.v2store.Clone() //复制v2存储
	// commit kv to write metadata (for example: consistent index) to disk.
	//
	// This guarantees that Backend's consistent_index is >= index of last snapshot.
	//
	// KV().commit() updates the consistent index in backend.
	// All operations that update consistent index must be called sequentially
	// from applyAll function.
	// So KV().Commit() cannot run in parallel with apply. It has to be called outside
	// the go routine created below.
	s.KV().Commit() //提交 v3 存储中当前等待读写事务

	s.GoAttach(func() {
		lg := s.Logger()

		// 将 v2 存储序列化成 JSON 数据，错误处理(咯)
		d, err := clone.SaveNoCopy()
		// TODO: current store will never fail to do a snapshot
		// what should we do if the store might fail?
		if err != nil {
			lg.Panic("failed to save v2 store", zap.Error(err))
		}
		// 将上述快照数据和元数据更新到 etcd-raft 模块中的 MemoryStorage 中
		// 并且返回 Snapshot 实例(即 MemoryStorage.snapshot 字段)，错误处理(咯)
		snap, err := s.r.raftStorage.CreateSnapshot(snapi, &confState, d)
		if err != nil {
			// the snapshot was done asynchronously with the progress of raft.
			// raft might have already got a newer snapshot.
			if err == raft.ErrSnapOutOfDate {
				return
			}
			lg.Panic("failed to create snapshot", zap.Error(err))
		}

		//将 v2 存储的快照数据记录到磁盘中，该过程涉及在 WAL 日志文件中记录快照元数据及写入 snap 文件等操作
		// SaveSnap saves the snapshot to file and appends the corresponding WAL entry.
		if err = s.r.storage.SaveSnap(snap); err != nil {
			lg.Panic("failed to save snapshot", zap.Error(err))
		}
		if err = s.r.storage.Release(snap); err != nil {
			lg.Panic("failed to release wal", zap.Error(err))
		}

		lg.Info(
			"saved snapshot",
			zap.Uint64("snapshot-index", snap.Metadata.Index),
		)

		// 如果当前还存在已发送但未响应的快照消息，则不能进行后续的压缩操作 ， 如果进行了后续的压缩
		// 则可能导致 Follower 节点再次无法追赶上 Leader 节点，从而需要再次发送快照、数据
		// When sending a snapshot, etcd will pause compaction.
		// After receives a snapshot, the slow follower needs to get all the entries right after
		// the snapshot sent to catch up. If we do not pause compaction, the log entries right after
		// the snapshot sent might already be compacted. It happens when the snapshot takes long time
		// to send and save. Pausing compaction avoids triggering a snapshot sending cycle.
		if atomic.LoadInt64(&s.inflightSnapshots) != 0 {
			lg.Info("skip compaction since there is an inflight snapshot")
			return
		}

		// 为了防止集群中存在比较慢的 Follower 节点，保留 5000 条 Entry 记录不 压 缩
		// keep some in memory log entries for slow followers.
		compacti := uint64(1)
		if snapi > s.Cfg.SnapshotCatchUpEntries {
			compacti = snapi - s.Cfg.SnapshotCatchUpEntries
		}

		//压缩 MemoryStorage 中的指定位置之前的全部 Entry 记录 ， 不再展开
		err = s.r.raftStorage.Compact(compacti)
		if err != nil {
			// the compaction was done asynchronously with the progress of raft.
			// raft log might already been compact.
			if err == raft.ErrCompacted {
				return
			}
			lg.Panic("failed to compact", zap.Error(err))
		}
		lg.Info(
			"compacted Raft logs",
			zap.Uint64("compact-index", compacti),
		)
	})
}

// CutPeer drops messages to the specified peer.
func (s *EtcdServer) CutPeer(id types.ID) {
	tr, ok := s.r.transport.(*rafthttp.Transport)
	if ok {
		tr.CutPeer(id)
	}
}

// MendPeer recovers the message dropping behavior of the given peer.
func (s *EtcdServer) MendPeer(id types.ID) {
	tr, ok := s.r.transport.(*rafthttp.Transport)
	if ok {
		tr.MendPeer(id)
	}
}

func (s *EtcdServer) PauseSending() { s.r.pauseSending() }

func (s *EtcdServer) ResumeSending() { s.r.resumeSending() }

func (s *EtcdServer) ClusterVersion() *semver.Version {
	if s.cluster == nil {
		return nil
	}
	return s.cluster.Version()
}

// monitorVersions checks the member's version every monitorVersionInterval.
// It updates the cluster version if all members agrees on a higher one.
// It prints out log if there is a member with a higher version than the
// local version.
// TODO switch to updateClusterVersionV3 in 3.6
func (s *EtcdServer) monitorVersions() {
	for {
		select {
		case <-s.FirstCommitInTermNotify():
		case <-time.After(monitorVersionInterval):
		case <-s.stopping:
			return
		}

		if s.Leader() != s.ID() {
			continue
		}

		v := decideClusterVersion(s.Logger(), getVersions(s.Logger(), s.cluster, s.id, s.peerRt))
		if v != nil {
			// only keep major.minor version for comparison
			v = &semver.Version{
				Major: v.Major,
				Minor: v.Minor,
			}
		}

		// if the current version is nil:
		// 1. use the decided version if possible
		// 2. or use the min cluster version
		if s.cluster.Version() == nil {
			verStr := version.MinClusterVersion
			if v != nil {
				verStr = v.String()
			}
			s.GoAttach(func() { s.updateClusterVersionV2(verStr) })
			continue
		}

		if v != nil && membership.IsValidVersionChange(s.cluster.Version(), v) {
			s.GoAttach(func() { s.updateClusterVersionV2(v.String()) })
		}
	}
}

func (s *EtcdServer) updateClusterVersionV2(ver string) {
	lg := s.Logger()

	if s.cluster.Version() == nil {
		lg.Info(
			"setting up initial cluster version using v2 API",
			zap.String("cluster-version", version.Cluster(ver)),
		)
	} else {
		lg.Info(
			"updating cluster version using v2 API",
			zap.String("from", version.Cluster(s.cluster.Version().String())),
			zap.String("to", version.Cluster(ver)),
		)
	}

	req := pb.Request{
		Method: "PUT",
		Path:   membership.StoreClusterVersionKey(),
		Val:    ver,
	}

	ctx, cancel := context.WithTimeout(s.ctx, s.Cfg.ReqTimeout())
	_, err := s.Do(ctx, req)
	cancel()

	switch err {
	case nil:
		lg.Info("cluster version is updated", zap.String("cluster-version", version.Cluster(ver)))
		return

	case ErrStopped:
		lg.Warn("aborting cluster version update; server is stopped", zap.Error(err))
		return

	default:
		lg.Warn("failed to update cluster version", zap.Error(err))
	}
}

func (s *EtcdServer) updateClusterVersionV3(ver string) {
	lg := s.Logger()

	if s.cluster.Version() == nil {
		lg.Info(
			"setting up initial cluster version using v3 API",
			zap.String("cluster-version", version.Cluster(ver)),
		)
	} else {
		lg.Info(
			"updating cluster version using v3 API",
			zap.String("from", version.Cluster(s.cluster.Version().String())),
			zap.String("to", version.Cluster(ver)),
		)
	}

	req := membershippb.ClusterVersionSetRequest{Ver: ver}

	ctx, cancel := context.WithTimeout(s.ctx, s.Cfg.ReqTimeout())
	_, err := s.raftRequest(ctx, pb.InternalRaftRequest{ClusterVersionSet: &req})
	cancel()

	switch err {
	case nil:
		lg.Info("cluster version is updated", zap.String("cluster-version", version.Cluster(ver)))
		return

	case ErrStopped:
		lg.Warn("aborting cluster version update; server is stopped", zap.Error(err))
		return

	default:
		lg.Warn("failed to update cluster version", zap.Error(err))
	}
}

func (s *EtcdServer) monitorDowngrade() {
	t := s.Cfg.DowngradeCheckTime
	if t == 0 {
		return
	}
	lg := s.Logger()
	for {
		select {
		case <-time.After(t):
		case <-s.stopping:
			return
		}

		if !s.isLeader() {
			continue
		}

		d := s.cluster.DowngradeInfo()
		if !d.Enabled {
			continue
		}

		targetVersion := d.TargetVersion
		v := semver.Must(semver.NewVersion(targetVersion))
		if isMatchedVersions(s.Logger(), v, getVersions(s.Logger(), s.cluster, s.id, s.peerRt)) {
			lg.Info("the cluster has been downgraded", zap.String("cluster-version", targetVersion))
			ctx, cancel := context.WithTimeout(context.Background(), s.Cfg.ReqTimeout())
			if _, err := s.downgradeCancel(ctx); err != nil {
				lg.Warn("failed to cancel downgrade", zap.Error(err))
			}
			cancel()
		}
	}
}

func (s *EtcdServer) parseProposeCtxErr(err error, start time.Time) error {
	switch err {
	case context.Canceled:
		return ErrCanceled

	case context.DeadlineExceeded:
		s.leadTimeMu.RLock()
		curLeadElected := s.leadElectedTime
		s.leadTimeMu.RUnlock()
		prevLeadLost := curLeadElected.Add(-2 * time.Duration(s.Cfg.ElectionTicks) * time.Duration(s.Cfg.TickMs) * time.Millisecond)
		if start.After(prevLeadLost) && start.Before(curLeadElected) {
			return ErrTimeoutDueToLeaderFail
		}
		lead := types.ID(s.getLead())
		switch lead {
		case types.ID(raft.None):
			// TODO: return error to specify it happens because the cluster does not have leader now
		case s.ID():
			if !isConnectedToQuorumSince(s.r.transport, start, s.ID(), s.cluster.Members()) {
				return ErrTimeoutDueToConnectionLost
			}
		default:
			if !isConnectedSince(s.r.transport, start, lead) {
				return ErrTimeoutDueToConnectionLost
			}
		}
		return ErrTimeout

	default:
		return err
	}
}

func (s *EtcdServer) KV() mvcc.WatchableKV { return s.kv }
func (s *EtcdServer) Backend() backend.Backend {
	s.bemu.Lock()
	defer s.bemu.Unlock()
	return s.be
}

func (s *EtcdServer) AuthStore() auth.AuthStore { return s.authStore }

func (s *EtcdServer) restoreAlarms() error {
	s.applyV3 = s.newApplierV3()
	as, err := v3alarm.NewAlarmStore(s.lg, s)
	if err != nil {
		return err
	}
	s.alarmStore = as
	if len(as.Get(pb.AlarmType_NOSPACE)) > 0 {
		s.applyV3 = newApplierV3Capped(s.applyV3)
	}
	if len(as.Get(pb.AlarmType_CORRUPT)) > 0 {
		s.applyV3 = newApplierV3Corrupt(s.applyV3)
	}
	return nil
}

// GoAttach creates a goroutine on a given function and tracks it using
// the etcdserver waitgroup.
// The passed function should interrupt on s.StoppingNotify().
func (s *EtcdServer) GoAttach(f func()) {
	s.wgMu.RLock() // this blocks with ongoing close(s.stopping)
	defer s.wgMu.RUnlock()
	select {
	case <-s.stopping:
		lg := s.Logger()
		lg.Warn("server has stopped; skipping GoAttach")
		return
	default:
	}

	// now safe to add since waitgroup wait has not started yet
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		f()
	}()
}

func (s *EtcdServer) Alarms() []*pb.AlarmMember {
	return s.alarmStore.Get(pb.AlarmType_NONE)
}

// IsLearner returns if the local member is raft learner
func (s *EtcdServer) IsLearner() bool {
	return s.cluster.IsLocalMemberLearner()
}

// IsMemberExist returns if the member with the given id exists in cluster.
func (s *EtcdServer) IsMemberExist(id types.ID) bool {
	return s.cluster.IsMemberExist(id)
}

// raftStatus returns the raft status of this etcd node.
func (s *EtcdServer) raftStatus() raft.Status {
	return s.r.Node.Status()
}

func maybeDefragBackend(cfg config.ServerConfig, be backend.Backend) error {
	size := be.Size()
	sizeInUse := be.SizeInUse()
	freeableMemory := uint(size - sizeInUse)
	thresholdBytes := cfg.ExperimentalBootstrapDefragThresholdMegabytes * 1024 * 1024
	if freeableMemory < thresholdBytes {
		cfg.Logger.Info("Skipping defragmentation",
			zap.Int64("current-db-size-bytes", size),
			zap.String("current-db-size", humanize.Bytes(uint64(size))),
			zap.Int64("current-db-size-in-use-bytes", sizeInUse),
			zap.String("current-db-size-in-use", humanize.Bytes(uint64(sizeInUse))),
			zap.Uint("experimental-bootstrap-defrag-threshold-bytes", thresholdBytes),
			zap.String("experimental-bootstrap-defrag-threshold", humanize.Bytes(uint64(thresholdBytes))),
		)
		return nil
	}
	return be.Defrag()
}
