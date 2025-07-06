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

package rafthttp

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/httputil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	streamTypeMessage  streamType = "message"
	streamTypeMsgAppV2 streamType = "msgappv2"

	streamBufSize = 4096
)

var (
	errUnsupportedStreamType = fmt.Errorf("unsupported stream type")

	// the key is in string format "major.minor.patch"
	supportedStream = map[string][]streamType{
		"2.0.0": {},
		"2.1.0": {streamTypeMsgAppV2, streamTypeMessage},
		"2.2.0": {streamTypeMsgAppV2, streamTypeMessage},
		"2.3.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.0.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.1.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.2.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.3.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.4.0": {streamTypeMsgAppV2, streamTypeMessage},
		"3.5.0": {streamTypeMsgAppV2, streamTypeMessage},
	}
)

type streamType string

func (t streamType) endpoint(lg *zap.Logger) string {
	switch t {
	case streamTypeMsgAppV2:
		return path.Join(RaftStreamPrefix, "msgapp")
	case streamTypeMessage:
		return path.Join(RaftStreamPrefix, "message")
	default:
		if lg != nil {
			lg.Panic("unhandled stream type", zap.String("stream-type", t.String()))
		}
		return ""
	}
}

func (t streamType) String() string {
	switch t {
	case streamTypeMsgAppV2:
		return "stream MsgApp v2"
	case streamTypeMessage:
		return "stream Message"
	default:
		return "unknown stream"
	}
}

var (
	// linkHeartbeatMessage is a special message used as heartbeat message in
	// link layer. It never conflicts with messages from raft because raft
	// doesn't send out messages without From and To fields.
	linkHeartbeatMessage = raftpb.Message{Type: raftpb.MsgHeartbeat}
)

func isLinkHeartbeatMessage(m *raftpb.Message) bool {
	return m.Type == raftpb.MsgHeartbeat && m.From == 0 && m.To == 0
}

type outgoingConn struct {
	t streamType
	io.Writer
	http.Flusher
	io.Closer

	localID types.ID
	peerID  types.ID
}

// streamWriter writes messages to the attached outgoingConn.
// 向 Stream 消息通道写入消息
type streamWriter struct {
	lg *zap.Logger

	localID types.ID
	peerID  types.ID // 对端节点的 ID

	status *peerStatus
	fs     *stats.FollowerStats
	r      Raft

	mu      sync.Mutex // guard field working and closer
	closer  io.Closer  // 负责关闭底层的长连接。
	working bool       // 负责标识当 前的 streamWriter 是否可用( 底层是 否关联了 相应 的网络连接)。

	// Peer会将待发送的消息写入该通道， streamWriter则从该通道中读取消息并发送 出去。
	msgc chan raftpb.Message

	// 通过 该通道获取当前 streamWriter 实例关联的底层网络连接，
	//  outgoingConn 其实是对网络连接的一层封装，其中 记录了 当前连接使用 的协议版本，以及用于关闭连接的 Flusher和 Closer等信息。
	connc chan *outgoingConn
	stopc chan struct{}
	done  chan struct{}
}

// startStreamWriter creates a streamWrite and starts a long running go-routine that accepts
// messages and writes to the attached outgoing connection.
func startStreamWriter(lg *zap.Logger, local, id types.ID, status *peerStatus, fs *stats.FollowerStats, r Raft) *streamWriter {
	w := &streamWriter{
		lg: lg,

		localID: local,
		peerID:  id,

		status: status,
		fs:     fs,
		r:      r,
		msgc:   make(chan raftpb.Message, streamBufSize),
		connc:  make(chan *outgoingConn),
		stopc:  make(chan struct{}),
		done:   make(chan struct{}),
	}
	go w.run()
	return w
}

func (cw *streamWriter) run() {
	var (
		msgc chan raftpb.Message

		// 有所不同，该心跳消息的主要目的是为了防止连接长时间不用断升的
		heartbeatc <-chan time.Time
		t          streamType   // 用来记录消息的版本信息
		enc        encoder      //编码器，负责将消息序列化并写入连接的缓冲区
		flusher    http.Flusher // 负责刷新底层连接，将数据真正发送 出去
		batched    int          // 当前未 Flush 的消息个数
	)
	tickc := time.NewTicker(ConnReadTimeout / 3)
	defer tickc.Stop()
	unflushed := 0

	if cw.lg != nil {
		cw.lg.Info(
			"started stream writer with remote peer",
			zap.String("local-member-id", cw.localID.String()),
			zap.String("remote-peer-id", cw.peerID.String()),
		)
	}

	for {
		select {
		case <-heartbeatc: // 定时器到期，触发心跳消息
			err := enc.encode(&linkHeartbeatMessage)
			unflushed += linkHeartbeatMessage.Size() // 增加未 Flush 出去的字节数
			if err == nil {
				// 若没有异常，则使用 flusher 把缓存的消息全部发送出去，并重置 batched 和 unflushed
				flusher.Flush()
				batched = 0
				sentBytes.WithLabelValues(cw.peerID.String()).Add(float64(unflushed))
				unflushed = 0
				continue
			}

			cw.status.deactivate(failureType{source: t.String(), action: "heartbeat"}, err.Error())

			sentFailures.WithLabelValues(cw.peerID.String()).Inc()
			cw.close()
			if cw.lg != nil {
				cw.lg.Warn(
					"lost TCP streaming connection with remote peer",
					zap.String("stream-writer-type", t.String()),
					zap.String("local-member-id", cw.localID.String()),
					zap.String("remote-peer-id", cw.peerID.String()),
				)
			}

			// 将 heartbeatc 和 msgc 两个通道清 空，后续就不会再发送心跳消息和其他类型的消息了
			heartbeatc, msgc = nil, nil

		case m := <-msgc:
			// 将消息序列化并写入底层连接
			err := enc.encode(&m)
			if err == nil {
				unflushed += m.Size()

				if len(msgc) == 0 || batched > streamBufSize/2 {
					// msgc 通道中的消息全部发送完成或是未 Flush 的消息较多，则触发 Flush, 否则只是递增 batched 变量
					flusher.Flush()
					sentBytes.WithLabelValues(cw.peerID.String()).Add(float64(unflushed))
					unflushed = 0
					batched = 0
				} else {
					batched++
				}

				continue
			}

			cw.status.deactivate(failureType{source: t.String(), action: "write"}, err.Error())
			cw.close()
			if cw.lg != nil {
				cw.lg.Warn(
					"lost TCP streaming connection with remote peer",
					zap.String("stream-writer-type", t.String()),
					zap.String("local-member-id", cw.localID.String()),
					zap.String("remote-peer-id", cw.peerID.String()),
				)
			}
			heartbeatc, msgc = nil, nil
			cw.r.ReportUnreachable(m.To)
			sentFailures.WithLabelValues(cw.peerID.String()).Inc()

		case conn := <-cw.connc: // 获取与当前 streamWriter 实例绑定的底层连接
			// 当其他节点主动与当前节点创建 Stream 消息通道时，会先通过 StreamHandler 的处理，
			// StreamHandler会通过 attach() 方法将连接写入对应peer.writer.connc通道
			// 而当前的 goroutine 会通过该通道获取连接，然后开始发送消息
			cw.mu.Lock()
			closed := cw.closeUnlocked()
			t = conn.t
			switch conn.t {
			case streamTypeMsgAppV2:
				enc = newMsgAppV2Encoder(conn.Writer, cw.fs)
			case streamTypeMessage:
				enc = &messageEncoder{w: conn.Writer}
			default:
				if cw.lg != nil {
					cw.lg.Panic("unhandled stream type", zap.String("stream-type", t.String()))
				}
			}
			if cw.lg != nil {
				cw.lg.Info(
					"set message encoder",
					zap.String("from", conn.localID.String()),
					zap.String("to", conn.peerID.String()),
					zap.String("stream-type", t.String()),
				)
			}
			flusher = conn.Flusher // 记录底层连接对应的 Flusher
			unflushed = 0          // 重直未 Flush 的字节数
			cw.status.activate()   // peerStatus.activate 设置为 true
			cw.closer = conn.Closer
			cw.working = true // 标识当前 streamWriter 正在运行
			cw.mu.Unlock()

			if closed {
				if cw.lg != nil {
					cw.lg.Warn(
						"closed TCP streaming connection with remote peer",
						zap.String("stream-writer-type", t.String()),
						zap.String("local-member-id", cw.localID.String()),
						zap.String("remote-peer-id", cw.peerID.String()),
					)
				}
			}
			if cw.lg != nil {
				cw.lg.Info(
					"established TCP streaming connection with remote peer",
					zap.String("stream-writer-type", t.String()),
					zap.String("local-member-id", cw.localID.String()),
					zap.String("remote-peer-id", cw.peerID.String()),
				)
			}

			// 更新 heartbeatc 和 msgc 两个远远，自此之后，才能发送消息
			heartbeatc, msgc = tickc.C, cw.msgc

		case <-cw.stopc:
			if cw.close() {
				if cw.lg != nil {
					cw.lg.Warn(
						"closed TCP streaming connection with remote peer",
						zap.String("stream-writer-type", t.String()),
						zap.String("remote-peer-id", cw.peerID.String()),
					)
				}
			}
			if cw.lg != nil {
				cw.lg.Info(
					"stopped TCP streaming connection with remote peer",
					zap.String("stream-writer-type", t.String()),
					zap.String("remote-peer-id", cw.peerID.String()),
				)
			}
			close(cw.done)
			return
		}
	}
}

func (cw *streamWriter) writec() (chan<- raftpb.Message, bool) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.msgc, cw.working
}

func (cw *streamWriter) close() bool {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.closeUnlocked()
}

func (cw *streamWriter) closeUnlocked() bool {
	if !cw.working {
		return false
	}
	if err := cw.closer.Close(); err != nil {
		if cw.lg != nil {
			cw.lg.Warn(
				"failed to close connection with remote peer",
				zap.String("remote-peer-id", cw.peerID.String()),
				zap.Error(err),
			)
		}
	}
	if len(cw.msgc) > 0 {
		cw.r.ReportUnreachable(uint64(cw.peerID))
	}
	cw.msgc = make(chan raftpb.Message, streamBufSize)
	cw.working = false
	return true
}

func (cw *streamWriter) attach(conn *outgoingConn) bool {
	select {
	case cw.connc <- conn:
		return true
	case <-cw.done:
		return false
	}
}

func (cw *streamWriter) stop() {
	close(cw.stopc)
	<-cw.done
}

// streamReader is a long-running go-routine that dials to the remote stream
// endpoint and reads messages from the response body returned.
type streamReader struct {
	lg *zap.Logger

	peerID types.ID
	typ    streamType // 关联 的底 层连接使用的协议版本信息。

	tr     *Transport
	picker *urlPicker // 用于获取对端节点的可用的 URL
	status *peerStatus

	// 主要是读取底层网络连接数据，写入通道，然后再由peer的后台协程读取并调用底层的Raft
	// 在前面介绍的 peer.startPeer()方法中提 ，streamReader 实例时是使用 peer.recvc 通道初始化该宇段的，其中还会启动一个后台 goroutine 从 peer.recvc 通道中读取消息。在下面分析中会看到，从对端节点发送来的 非 MsgProp 类型 的消息会首先由 streamReader 写入 recvc 通道 中， 然后由 peer.start() 启动的后台 goroutine读取出来，交由底层的 etcd-raft模块进行处理。
	recvc chan<- raftpb.Message // 和peer的recvc共享
	propc chan<- raftpb.Message // 和peer的propc共享，接收的是 MsgProp 类型的消息

	rl *rate.Limiter // alters the frequency of dial retrial attempts

	errorc chan<- error

	mu     sync.Mutex
	paused bool // 是否暂停读取数据
	closer io.Closer

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func (cr *streamReader) start() {
	cr.done = make(chan struct{})
	if cr.errorc == nil {
		cr.errorc = cr.tr.ErrorC
	}
	if cr.ctx == nil {
		cr.ctx, cr.cancel = context.WithCancel(context.Background())
	}
	go cr.run()
}

func (cr *streamReader) run() {
	t := cr.typ

	if cr.lg != nil {
		cr.lg.Info(
			"started stream reader with remote peer",
			zap.String("stream-reader-type", t.String()),
			zap.String("local-member-id", cr.tr.ID.String()),
			zap.String("remote-peer-id", cr.peerID.String()),
		)
	}

	for {
		// 向对端节点发送一个 GET 请求 ， 然后获取并返回相应的 ReadCloser
		rc, err := cr.dial(t)
		if err != nil {
			if err != errUnsupportedStreamType {
				cr.status.deactivate(failureType{source: t.String(), action: "dial"}, err.Error())
			}
		} else {
			cr.status.activate()
			if cr.lg != nil {
				cr.lg.Info(
					"established TCP streaming connection with remote peer",
					zap.String("stream-reader-type", cr.typ.String()),
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
				)
			}
			// 如未出现异常，则开始读取对端返回的消息，并将读取到的消息写入 streamReader.recvc 通道 中
			err = cr.decodeLoop(rc, t)
			if cr.lg != nil {
				cr.lg.Warn(
					"lost TCP streaming connection with remote peer",
					zap.String("stream-reader-type", cr.typ.String()),
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
					zap.Error(err),
				)
			}
			switch {
			// all data is read out
			case err == io.EOF:
			// connection is closed by the remote
			case transport.IsClosedConnError(err):
			default:
				cr.status.deactivate(failureType{source: t.String(), action: "read"}, err.Error())
			}
		}
		// Wait for a while before new dial attempt
		err = cr.rl.Wait(cr.ctx)
		if cr.ctx.Err() != nil {
			if cr.lg != nil {
				cr.lg.Info(
					"stopped stream reader with remote peer",
					zap.String("stream-reader-type", t.String()),
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
				)
			}
			close(cr.done)
			return
		}
		if err != nil {
			if cr.lg != nil {
				cr.lg.Warn(
					"rate limit on stream reader with remote peer",
					zap.String("stream-reader-type", t.String()),
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
					zap.Error(err),
				)
			}
		}
	}
}

// 它会从底层的网络连接读取数据并进行反序列化，之后将得到的消息实例写入 recvc 通道(或 prope通道)中， 等待 Peer进行处理
func (cr *streamReader) decodeLoop(rc io.ReadCloser, t streamType) error {
	var dec decoder
	cr.mu.Lock()
	switch t {
	case streamTypeMsgAppV2:
		dec = newMsgAppV2Decoder(rc, cr.tr.ID, cr.peerID)
	case streamTypeMessage:
		dec = &messageDecoder{r: rc}
	default:
		if cr.lg != nil {
			cr.lg.Panic("unknown stream type", zap.String("type", t.String()))
		}
	}
	select {
	case <-cr.ctx.Done():
		cr.mu.Unlock()
		if err := rc.Close(); err != nil {
			return err
		}
		return io.EOF
	default:
		cr.closer = rc
	}
	cr.mu.Unlock()

	// gofail: labelRaftDropHeartbeat:
	for {
		// //从底层连接中读取数据，并反序列化成 raftpb . Message 实例
		m, err := dec.decode()
		if err != nil {
			cr.mu.Lock()
			cr.close()
			cr.mu.Unlock()
			return err
		}

		// gofail-go: var raftDropHeartbeat struct{}
		// continue labelRaftDropHeartbeat
		receivedBytes.WithLabelValues(types.ID(m.From).String()).Add(float64(m.Size()))

		cr.mu.Lock()
		paused := cr.paused
		cr.mu.Unlock()

		if paused {
			continue
		}

		if isLinkHeartbeatMessage(&m) {
			// /忽略连接层的心跳消息
			// raft is not interested in link layer
			// heartbeat message, so we should ignore
			// it.
			continue
		}

		// 根据读取到的消息类型，选择对应通道进行写入
		recvc := cr.recvc
		if m.Type == raftpb.MsgProp {
			recvc = cr.propc
		}

		select {
		case recvc <- m: // 把收到的消息赋值给recvc通道
		default:
			// recvc 通过满了之后，只能丢弃消息，并打印日志
			if cr.status.isActive() {
				if cr.lg != nil {
					cr.lg.Warn(
						"dropped internal Raft message since receiving buffer is full (overloaded network)",
						zap.String("message-type", m.Type.String()),
						zap.String("local-member-id", cr.tr.ID.String()),
						zap.String("from", types.ID(m.From).String()),
						zap.String("remote-peer-id", types.ID(m.To).String()),
						zap.Bool("remote-peer-active", cr.status.isActive()),
					)
				}
			} else {
				if cr.lg != nil {
					cr.lg.Warn(
						"dropped Raft message since receiving buffer is full (overloaded network)",
						zap.String("message-type", m.Type.String()),
						zap.String("local-member-id", cr.tr.ID.String()),
						zap.String("from", types.ID(m.From).String()),
						zap.String("remote-peer-id", types.ID(m.To).String()),
						zap.Bool("remote-peer-active", cr.status.isActive()),
					)
				}
			}
			recvFailures.WithLabelValues(types.ID(m.From).String()).Inc()
		}
	}
}

func (cr *streamReader) stop() {
	cr.mu.Lock()
	cr.cancel()
	cr.close()
	cr.mu.Unlock()
	<-cr.done
}

// 负责与对端节点建立连接
func (cr *streamReader) dial(t streamType) (io.ReadCloser, error) {
	u := cr.picker.pick() //  获取对端节点暴露的一个 URL
	uu := u
	// 根据使用协议的版本和节点 ID创建最终的 URL地址
	uu.Path = path.Join(t.endpoint(cr.lg), cr.tr.ID.String())

	if cr.lg != nil {
		cr.lg.Debug(
			"dial stream reader",
			zap.String("from", cr.tr.ID.String()),
			zap.String("to", cr.peerID.String()),
			zap.String("address", uu.String()),
		)
	}
	// 创建一个 GET请求
	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		cr.picker.unreachable(u)
		return nil, fmt.Errorf("failed to make http request to %v (%v)", u, err)
	}
	req.Header.Set("X-Server-From", cr.tr.ID.String())
	req.Header.Set("X-Server-Version", version.Version)
	req.Header.Set("X-Min-Cluster-Version", version.MinClusterVersion)
	req.Header.Set("X-Etcd-Cluster-ID", cr.tr.ClusterID.String())
	req.Header.Set("X-Raft-To", cr.peerID.String())

	// 将当前节点暴露的 URL 也一起发送给对端节点 检测当前 streamReader 是否被关闭，若已被关闭，则返回异常(略)
	setPeerURLsHeader(req, cr.tr.URLs)

	req = req.WithContext(cr.ctx)

	cr.mu.Lock()
	select {
	case <-cr.ctx.Done():
		cr.mu.Unlock()
		return nil, fmt.Errorf("stream reader is stopped")
	default:
	}
	cr.mu.Unlock()

	// 通过前面介绍的 rafthttp.Transport.streamRt 与对端建立连接
	resp, err := cr.tr.streamRt.RoundTrip(req)
	if err != nil {
		cr.picker.unreachable(u)
		return nil, err
	}

	rv := serverVersion(resp.Header)
	lv := semver.Must(semver.NewVersion(version.Version))
	if compareMajorMinorVersion(rv, lv) == -1 && !checkStreamSupport(rv, t) {
		httputil.GracefulClose(resp)
		cr.picker.unreachable(u)
		return nil, errUnsupportedStreamType
	}

	switch resp.StatusCode {
	case http.StatusGone:
		httputil.GracefulClose(resp)
		cr.picker.unreachable(u)
		reportCriticalError(errMemberRemoved, cr.errorc)
		return nil, errMemberRemoved

	case http.StatusOK:
		return resp.Body, nil

	case http.StatusNotFound:
		httputil.GracefulClose(resp)
		cr.picker.unreachable(u)
		return nil, fmt.Errorf("peer %s failed to find local node %s", cr.peerID, cr.tr.ID)

	case http.StatusPreconditionFailed:
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			cr.picker.unreachable(u)
			return nil, err
		}
		httputil.GracefulClose(resp)
		cr.picker.unreachable(u)

		switch strings.TrimSuffix(string(b), "\n") {
		case errIncompatibleVersion.Error():
			if cr.lg != nil {
				cr.lg.Warn(
					"request sent was ignored by remote peer due to server version incompatibility",
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
					zap.Error(errIncompatibleVersion),
				)
			}
			return nil, errIncompatibleVersion

		case errClusterIDMismatch.Error():
			if cr.lg != nil {
				cr.lg.Warn(
					"request sent was ignored by remote peer due to cluster ID mismatch",
					zap.String("remote-peer-id", cr.peerID.String()),
					zap.String("remote-peer-cluster-id", resp.Header.Get("X-Etcd-Cluster-ID")),
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("local-member-cluster-id", cr.tr.ClusterID.String()),
					zap.Error(errClusterIDMismatch),
				)
			}
			return nil, errClusterIDMismatch

		default:
			return nil, fmt.Errorf("unhandled error %q when precondition failed", string(b))
		}

	default:
		httputil.GracefulClose(resp)
		cr.picker.unreachable(u)
		return nil, fmt.Errorf("unhandled http status %d", resp.StatusCode)
	}
}

func (cr *streamReader) close() {
	if cr.closer != nil {
		if err := cr.closer.Close(); err != nil {
			if cr.lg != nil {
				cr.lg.Warn(
					"failed to close remote peer connection",
					zap.String("local-member-id", cr.tr.ID.String()),
					zap.String("remote-peer-id", cr.peerID.String()),
					zap.Error(err),
				)
			}
		}
	}
	cr.closer = nil
}

func (cr *streamReader) pause() {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.paused = true
}

func (cr *streamReader) resume() {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.paused = false
}

// checkStreamSupport checks whether the stream type is supported in the
// given version.
func checkStreamSupport(v *semver.Version, t streamType) bool {
	nv := &semver.Version{Major: v.Major, Minor: v.Minor}
	for _, s := range supportedStream[nv.String()] {
		if s == t {
			return true
		}
	}
	return false
}
