// Copyright 2018 The etcd Authors
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

package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/functional/rpcpb"
	"github.com/coreos/etcd/pkg/debugutil"
	"github.com/coreos/etcd/pkg/fileutil"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

// Cluster defines tester cluster.
type Cluster struct {
	lg *zap.Logger

	agentConns    []*grpc.ClientConn
	agentClients  []rpcpb.TransportClient
	agentStreams  []rpcpb.Transport_TransportClient
	agentRequests []*rpcpb.Request

	testerHTTPServer *http.Server

	Members []*rpcpb.Member `yaml:"agent-configs"`
	Tester  *rpcpb.Tester   `yaml:"tester-config"`

	failures []Failure

	rateLimiter *rate.Limiter
	stresser    Stresser
	checker     Checker

	currentRevision int64
	rd              int
	cs              int
}

var dialOpts = []grpc.DialOption{
	grpc.WithInsecure(),
	grpc.WithTimeout(5 * time.Second),
	grpc.WithBlock(),
}

// NewCluster creates a client from a tester configuration.
func NewCluster(lg *zap.Logger, fpath string) (*Cluster, error) {
	clus, err := read(lg, fpath)
	if err != nil {
		return nil, err
	}

	clus.agentConns = make([]*grpc.ClientConn, len(clus.Members))
	clus.agentClients = make([]rpcpb.TransportClient, len(clus.Members))
	clus.agentStreams = make([]rpcpb.Transport_TransportClient, len(clus.Members))
	clus.agentRequests = make([]*rpcpb.Request, len(clus.Members))
	clus.failures = make([]Failure, 0)

	for i, ap := range clus.Members {
		var err error
		clus.agentConns[i], err = grpc.Dial(ap.AgentAddr, dialOpts...)
		if err != nil {
			return nil, err
		}
		clus.agentClients[i] = rpcpb.NewTransportClient(clus.agentConns[i])
		clus.lg.Info("connected", zap.String("agent-address", ap.AgentAddr))

		clus.agentStreams[i], err = clus.agentClients[i].Transport(context.Background())
		if err != nil {
			return nil, err
		}
		clus.lg.Info("created stream", zap.String("agent-address", ap.AgentAddr))
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	if clus.Tester.EnablePprof {
		for p, h := range debugutil.PProfHandlers() {
			mux.Handle(p, h)
		}
	}
	clus.testerHTTPServer = &http.Server{
		Addr:    clus.Tester.Addr,
		Handler: mux,
	}
	go clus.serveTesterServer()

	clus.updateFailures()

	clus.rateLimiter = rate.NewLimiter(
		rate.Limit(int(clus.Tester.StressQPS)),
		int(clus.Tester.StressQPS),
	)

	clus.updateStresserChecker()

	return clus, nil
}

func (clus *Cluster) serveTesterServer() {
	clus.lg.Info(
		"started tester HTTP server",
		zap.String("tester-address", clus.Tester.Addr),
	)
	err := clus.testerHTTPServer.ListenAndServe()
	clus.lg.Info(
		"tester HTTP server returned",
		zap.String("tester-address", clus.Tester.Addr),
		zap.Error(err),
	)
	if err != nil && err != http.ErrServerClosed {
		clus.lg.Fatal("tester HTTP errored", zap.Error(err))
	}
}

func (clus *Cluster) updateFailures() {
	for _, cs := range clus.Tester.FailureCases {
		switch cs {
		case "SIGTERM_ONE_FOLLOWER":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_ONE_FOLLOWER(clus))
		case "SIGTERM_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus))
		case "SIGTERM_LEADER":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_LEADER(clus))
		case "SIGTERM_LEADER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus))
		case "SIGTERM_QUORUM":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_QUORUM(clus))
		case "SIGTERM_ALL":
			clus.failures = append(clus.failures,
				new_FailureCase_SIGTERM_ALL(clus))

		case "BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER(clus))
		case "BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT())
		case "BLACKHOLE_PEER_PORT_TX_RX_LEADER":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER(clus))
		case "BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT())
		case "BLACKHOLE_PEER_PORT_TX_RX_QUORUM":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_QUORUM(clus))
		case "BLACKHOLE_PEER_PORT_TX_RX_ALL":
			clus.failures = append(clus.failures,
				new_FailureCase_BLACKHOLE_PEER_PORT_TX_RX_ALL(clus))

		case "DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER(clus, true))
		case "DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus, true))
		case "DELAY_PEER_PORT_TX_RX_LEADER":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_LEADER":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER(clus, true))
		case "DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus, true))
		case "DELAY_PEER_PORT_TX_RX_QUORUM":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_QUORUM(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_QUORUM":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_QUORUM(clus, true))
		case "DELAY_PEER_PORT_TX_RX_ALL":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ALL(clus, false))
		case "RANDOM_DELAY_PEER_PORT_TX_RX_ALL":
			clus.failures = append(clus.failures,
				new_FailureCase_DELAY_PEER_PORT_TX_RX_ALL(clus, true))

		case "NO_FAIL_WITH_STRESS":
			clus.failures = append(clus.failures,
				new_FailureCase_NO_FAIL_WITH_STRESS(clus))
		case "NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS":
			clus.failures = append(clus.failures,
				new_FailureCase_NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS(clus))

		case "EXTERNAL":
			clus.failures = append(clus.failures,
				new_FailureCase_EXTERNAL(clus.Tester.ExternalExecPath))
		case "FAILPOINTS":
			fpFailures, fperr := failpointFailures(clus)
			if len(fpFailures) == 0 {
				clus.lg.Info("no failpoints found!", zap.Error(fperr))
			}
			clus.failures = append(clus.failures,
				fpFailures...)
		}
	}
}

func (clus *Cluster) failureStrings() (fs []string) {
	fs = make([]string, len(clus.failures))
	for i := range clus.failures {
		fs[i] = clus.failures[i].Desc()
	}
	return fs
}

// UpdateDelayLatencyMs updates delay latency with random value
// within election timeout.
func (clus *Cluster) UpdateDelayLatencyMs() {
	rand.Seed(time.Now().UnixNano())
	clus.Tester.UpdatedDelayLatencyMs = uint32(rand.Int63n(clus.Members[0].Etcd.ElectionTimeoutMs))

	minLatRv := clus.Tester.DelayLatencyMsRv + clus.Tester.DelayLatencyMsRv/5
	if clus.Tester.UpdatedDelayLatencyMs <= minLatRv {
		clus.Tester.UpdatedDelayLatencyMs += minLatRv
	}
}

func (clus *Cluster) updateStresserChecker() {
	cs := &compositeStresser{}
	for _, m := range clus.Members {
		cs.stressers = append(cs.stressers, newStresser(clus, m))
	}
	clus.stresser = cs

	if clus.Tester.ConsistencyCheck {
		clus.checker = newHashChecker(clus.lg, hashAndRevGetter(clus))
		if schk := cs.Checker(); schk != nil {
			clus.checker = newCompositeChecker([]Checker{clus.checker, schk})
		}
	} else {
		clus.checker = newNoChecker()
	}

	clus.lg.Info("updated stressers")
}

func (clus *Cluster) checkConsistency() (err error) {
	defer func() {
		if err != nil {
			return
		}
		if err = clus.updateRevision(); err != nil {
			clus.lg.Warn(
				"updateRevision failed",
				zap.Error(err),
			)
			return
		}
	}()

	if err = clus.checker.Check(); err != nil {
		clus.lg.Warn(
			"consistency check FAIL",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Error(err),
		)
		return err
	}
	clus.lg.Info(
		"consistency check ALL PASS",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.String("desc", clus.failures[clus.cs].Desc()),
	)

	return err
}

// Send_INITIAL_START_ETCD bootstraps etcd cluster the very first time.
// After this, just continue to call kill/restart.
func (clus *Cluster) Send_INITIAL_START_ETCD() error {
	// this is the only time that creates request from scratch
	return clus.broadcast(rpcpb.Operation_INITIAL_START_ETCD)
}

// send_SIGQUIT_ETCD_AND_ARCHIVE_DATA sends "send_SIGQUIT_ETCD_AND_ARCHIVE_DATA" operation.
func (clus *Cluster) send_SIGQUIT_ETCD_AND_ARCHIVE_DATA() error {
	return clus.broadcast(rpcpb.Operation_SIGQUIT_ETCD_AND_ARCHIVE_DATA)
}

// send_RESTART_ETCD sends restart operation.
func (clus *Cluster) send_RESTART_ETCD() error {
	return clus.broadcast(rpcpb.Operation_RESTART_ETCD)
}

func (clus *Cluster) broadcast(op rpcpb.Operation) error {
	var wg sync.WaitGroup
	wg.Add(len(clus.agentStreams))

	errc := make(chan error, len(clus.agentStreams))
	for i := range clus.agentStreams {
		go func(idx int, o rpcpb.Operation) {
			defer wg.Done()
			errc <- clus.sendOp(idx, o)
		}(i, op)
	}
	wg.Wait()
	close(errc)

	errs := []string{}
	for err := range errc {
		if err == nil {
			continue
		}

		if err != nil {
			destroyed := false
			if op == rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT {
				if err == io.EOF {
					destroyed = true
				}
				if strings.Contains(err.Error(),
					"rpc error: code = Unavailable desc = transport is closing") {
					// agent server has already closed;
					// so this error is expected
					destroyed = true
				}
				if strings.Contains(err.Error(),
					"desc = os: process already finished") {
					destroyed = true
				}
			}
			if !destroyed {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, ", "))
}

func (clus *Cluster) sendOp(idx int, op rpcpb.Operation) error {
	if op == rpcpb.Operation_INITIAL_START_ETCD {
		clus.agentRequests[idx] = &rpcpb.Request{
			Operation: op,
			Member:    clus.Members[idx],
			Tester:    clus.Tester,
		}
	} else {
		clus.agentRequests[idx].Operation = op
	}

	err := clus.agentStreams[idx].Send(clus.agentRequests[idx])
	clus.lg.Info(
		"sent request",
		zap.String("operation", op.String()),
		zap.String("to", clus.Members[idx].EtcdClientEndpoint),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	resp, err := clus.agentStreams[idx].Recv()
	if resp != nil {
		clus.lg.Info(
			"received response",
			zap.String("operation", op.String()),
			zap.String("from", clus.Members[idx].EtcdClientEndpoint),
			zap.Bool("success", resp.Success),
			zap.String("status", resp.Status),
			zap.Error(err),
		)
	} else {
		clus.lg.Info(
			"received empty response",
			zap.String("operation", op.String()),
			zap.String("from", clus.Members[idx].EtcdClientEndpoint),
			zap.Error(err),
		)
	}
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Status)
	}

	m, secure := clus.Members[idx], false
	for _, cu := range m.Etcd.AdvertiseClientURLs {
		u, err := url.Parse(cu)
		if err != nil {
			return err
		}
		if u.Scheme == "https" { // TODO: handle unix
			secure = true
		}
	}

	// store TLS assets from agents/servers onto disk
	if secure && (op == rpcpb.Operation_INITIAL_START_ETCD || op == rpcpb.Operation_RESTART_ETCD) {
		dirClient := filepath.Join(
			clus.Tester.DataDir,
			clus.Members[idx].Etcd.Name,
			"fixtures",
			"client",
		)
		if err = fileutil.TouchDirAll(dirClient); err != nil {
			return err
		}

		clientCertData := []byte(resp.Member.ClientCertData)
		if len(clientCertData) == 0 {
			return fmt.Errorf("got empty client cert from %q", m.EtcdClientEndpoint)
		}
		clientCertPath := filepath.Join(dirClient, "cert.pem")
		if err = ioutil.WriteFile(clientCertPath, clientCertData, 0644); err != nil { // overwrite if exists
			return err
		}
		resp.Member.ClientCertPath = clientCertPath
		clus.lg.Info(
			"saved client cert file",
			zap.String("path", clientCertPath),
		)

		clientKeyData := []byte(resp.Member.ClientKeyData)
		if len(clientKeyData) == 0 {
			return fmt.Errorf("got empty client key from %q", m.EtcdClientEndpoint)
		}
		clientKeyPath := filepath.Join(dirClient, "key.pem")
		if err = ioutil.WriteFile(clientKeyPath, clientKeyData, 0644); err != nil { // overwrite if exists
			return err
		}
		resp.Member.ClientKeyPath = clientKeyPath
		clus.lg.Info(
			"saved client key file",
			zap.String("path", clientKeyPath),
		)

		clientTrustedCAData := []byte(resp.Member.ClientTrustedCAData)
		if len(clientTrustedCAData) != 0 {
			// TODO: disable this when auto TLS is deprecated
			clientTrustedCAPath := filepath.Join(dirClient, "ca.pem")
			if err = ioutil.WriteFile(clientTrustedCAPath, clientTrustedCAData, 0644); err != nil { // overwrite if exists
				return err
			}
			resp.Member.ClientTrustedCAPath = clientTrustedCAPath
			clus.lg.Info(
				"saved client trusted CA file",
				zap.String("path", clientTrustedCAPath),
			)
		}

		// no need to store peer certs for tester clients

		clus.Members[idx] = resp.Member
	}
	return nil
}

// Send_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT terminates all tester connections to agents and etcd servers.
func (clus *Cluster) Send_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT() {
	err := clus.broadcast(rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT)
	if err != nil {
		clus.lg.Warn("destroying etcd/agents FAIL", zap.Error(err))
	} else {
		clus.lg.Info("destroying etcd/agents PASS")
	}

	for i, conn := range clus.agentConns {
		err := conn.Close()
		clus.lg.Info("closed connection to agent", zap.String("agent-address", clus.Members[i].AgentAddr), zap.Error(err))
	}

	if clus.testerHTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := clus.testerHTTPServer.Shutdown(ctx)
		cancel()
		clus.lg.Info("closed tester HTTP server", zap.String("tester-address", clus.Tester.Addr), zap.Error(err))
	}
}

// WaitHealth ensures all members are healthy
// by writing a test key to etcd cluster.
func (clus *Cluster) WaitHealth() error {
	var err error
	// wait 60s to check cluster health.
	// TODO: set it to a reasonable value. It is set that high because
	// follower may use long time to catch up the leader when reboot under
	// reasonable workload (https://github.com/coreos/etcd/issues/2698)
	for i := 0; i < 60; i++ {
		for _, m := range clus.Members {
			if err = m.WriteHealthKey(); err != nil {
				clus.lg.Warn(
					"health check FAIL",
					zap.Int("retries", i),
					zap.String("endpoint", m.EtcdClientEndpoint),
					zap.Error(err),
				)
				break
			}
			clus.lg.Info(
				"health check PASS",
				zap.Int("retries", i),
				zap.String("endpoint", m.EtcdClientEndpoint),
			)
		}
		if err == nil {
			clus.lg.Info(
				"health check ALL PASS",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
			)
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

// GetLeader returns the index of leader and error if any.
func (clus *Cluster) GetLeader() (int, error) {
	for i, m := range clus.Members {
		isLeader, err := m.IsLeader()
		if isLeader || err != nil {
			return i, err
		}
	}
	return 0, fmt.Errorf("no leader found")
}

// maxRev returns the maximum revision found on the cluster.
func (clus *Cluster) maxRev() (rev int64, err error) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	revc, errc := make(chan int64, len(clus.Members)), make(chan error, len(clus.Members))
	for i := range clus.Members {
		go func(m *rpcpb.Member) {
			mrev, merr := m.Rev(ctx)
			revc <- mrev
			errc <- merr
		}(clus.Members[i])
	}
	for i := 0; i < len(clus.Members); i++ {
		if merr := <-errc; merr != nil {
			err = merr
		}
		if mrev := <-revc; mrev > rev {
			rev = mrev
		}
	}
	return rev, err
}

func (clus *Cluster) getRevisionHash() (map[string]int64, map[string]int64, error) {
	revs := make(map[string]int64)
	hashes := make(map[string]int64)
	for _, m := range clus.Members {
		rev, hash, err := m.RevHash()
		if err != nil {
			return nil, nil, err
		}
		revs[m.EtcdClientEndpoint] = rev
		hashes[m.EtcdClientEndpoint] = hash
	}
	return revs, hashes, nil
}

func (clus *Cluster) compactKV(rev int64, timeout time.Duration) (err error) {
	if rev <= 0 {
		return nil
	}

	for i, m := range clus.Members {
		clus.lg.Info(
			"compact START",
			zap.String("endpoint", m.EtcdClientEndpoint),
			zap.Int64("compact-revision", rev),
			zap.Duration("timeout", timeout),
		)
		now := time.Now()
		cerr := m.Compact(rev, timeout)
		succeed := true
		if cerr != nil {
			if strings.Contains(cerr.Error(), "required revision has been compacted") && i > 0 {
				clus.lg.Info(
					"compact error is ignored",
					zap.String("endpoint", m.EtcdClientEndpoint),
					zap.Int64("compact-revision", rev),
					zap.Error(cerr),
				)
			} else {
				clus.lg.Warn(
					"compact FAIL",
					zap.String("endpoint", m.EtcdClientEndpoint),
					zap.Int64("compact-revision", rev),
					zap.Error(cerr),
				)
				err = cerr
				succeed = false
			}
		}

		if succeed {
			clus.lg.Info(
				"compact PASS",
				zap.String("endpoint", m.EtcdClientEndpoint),
				zap.Int64("compact-revision", rev),
				zap.Duration("timeout", timeout),
				zap.Duration("took", time.Since(now)),
			)
		}
	}
	return err
}

func (clus *Cluster) checkCompact(rev int64) error {
	if rev == 0 {
		return nil
	}
	for _, m := range clus.Members {
		if err := m.CheckCompact(rev); err != nil {
			return err
		}
	}
	return nil
}

func (clus *Cluster) defrag() error {
	for _, m := range clus.Members {
		if err := m.Defrag(); err != nil {
			clus.lg.Warn(
				"defrag FAIL",
				zap.String("endpoint", m.EtcdClientEndpoint),
				zap.Error(err),
			)
			return err
		}
		clus.lg.Info(
			"defrag PASS",
			zap.String("endpoint", m.EtcdClientEndpoint),
		)
	}
	clus.lg.Info(
		"defrag ALL PASS",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.Int("case-total", len(clus.failures)),
	)
	return nil
}

// GetFailureDelayDuration computes failure delay duration.
func (clus *Cluster) GetFailureDelayDuration() time.Duration {
	return time.Duration(clus.Tester.FailureDelayMs) * time.Millisecond
}

// Report reports the number of modified keys.
func (clus *Cluster) Report() int64 {
	return clus.stresser.ModifiedKeys()
}
