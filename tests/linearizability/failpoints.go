// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	KillFailpoint           Failpoint = killFailpoint{}
	DefragBeforeCopyPanic   Failpoint = goFailpoint{"backend/defragBeforeCopy", "panic", triggerDefrag, AnyMember}
	DefragBeforeRenamePanic Failpoint = goFailpoint{"backend/defragBeforeRename", "panic", triggerDefrag, AnyMember}
	BeforeCommitPanic       Failpoint = goFailpoint{"backend/beforeCommit", "panic", nil, AnyMember}
	AfterCommitPanic        Failpoint = goFailpoint{"backend/afterCommit", "panic", nil, AnyMember}
	RaftBeforeSavePanic     Failpoint = goFailpoint{"etcdserver/raftBeforeSave", "panic", nil, AnyMember}
	RaftAfterSavePanic      Failpoint = goFailpoint{"etcdserver/raftAfterSave", "panic", nil, AnyMember}
	//BackendBeforePreCommitHookPanic          Failpoint = goFailpoint{"backend/commitBeforePreCommitHook", "panic", nil, AnyMember}
	//BackendAfterPreCommitHookPanic           Failpoint = goFailpoint{"backend/commitAfterPreCommitHook", "panic", nil, AnyMember}
	//BackendBeforeStartDBTxnPanic             Failpoint = goFailpoint{"backend/beforeStartDBTxn", "panic", nil, AnyMember}
	//BackendAfterStartDBTxnPanic              Failpoint = goFailpoint{"backend/afterStartDBTxn", "panic", nil, AnyMember}
	//BackendBeforeWritebackBufPanic           Failpoint = goFailpoint{"backend/beforeWritebackBuf", "panic", nil, AnyMember}
	//BackendAfterWritebackBufPanic            Failpoint = goFailpoint{"backend/afterWritebackBuf", "panic", nil, AnyMember}
	//CompactBeforeCommitScheduledCompactPanic Failpoint = goFailpoint{"mvcc/compactBeforeCommitScheduledCompact", "panic", triggerCompact, AnyMember}
	//CompactAfterCommitScheduledCompactPanic  Failpoint = goFailpoint{"mvcc/compactAfterCommitScheduledCompact", "panic", triggerCompact, AnyMember}
	//CompactBeforeSetFinishedCompactPanic     Failpoint = goFailpoint{"mvcc/compactBeforeSetFinishedCompact", "panic", triggerCompact, AnyMember}
	//CompactAfterSetFinishedCompactPanic      Failpoint = goFailpoint{"mvcc/compactAfterSetFinishedCompact", "panic", triggerCompact, AnyMember}
	//CompactBeforeCommitBatchPanic            Failpoint = goFailpoint{"mvcc/compactBeforeCommitBatch", "panic", triggerCompact, AnyMember}
	//CompactAfterCommitBatchPanic             Failpoint = goFailpoint{"mvcc/compactAfterCommitBatch", "panic", triggerCompact, AnyMember}
	//RaftBeforeLeaderSendPanic                Failpoint = goFailpoint{"etcdserver/raftBeforeLeaderSend", "panic", nil, Leader}
	RandomFailpoint Failpoint = randomFailpoint{[]Failpoint{
		//KillFailpoint,
		BeforeCommitPanic,
		AfterCommitPanic,
		RaftBeforeSavePanic,
		RaftAfterSavePanic,
		//DefragBeforeCopyPanic,
		//DefragBeforeRenamePanic,

		// tmp

		//BackendBeforePreCommitHookPanic, BackendAfterPreCommitHookPanic,
		//BackendBeforeStartDBTxnPanic, BackendAfterStartDBTxnPanic,
		//BackendBeforeWritebackBufPanic, BackendAfterWritebackBufPanic,
		//CompactBeforeCommitScheduledCompactPanic, CompactAfterCommitScheduledCompactPanic,
		//CompactBeforeSetFinishedCompactPanic, CompactAfterSetFinishedCompactPanic,
		//CompactBeforeCommitBatchPanic, CompactAfterCommitBatchPanic,
		//RaftBeforeLeaderSendPanic,
	}}
	// TODO: Figure out how to reliably trigger below failpoints and add them to RandomFailpoint
	raftBeforeApplySnapPanic    Failpoint = goFailpoint{"etcdserver/raftBeforeApplySnap", "panic", nil, AnyMember}
	raftAfterApplySnapPanic     Failpoint = goFailpoint{"etcdserver/raftAfterApplySnap", "panic", nil, AnyMember}
	raftAfterWALReleasePanic    Failpoint = goFailpoint{"etcdserver/raftAfterWALRelease", "panic", nil, AnyMember}
	raftBeforeFollowerSendPanic Failpoint = goFailpoint{"etcdserver/raftBeforeFollowerSend", "panic", nil, AnyMember}
	raftBeforeSaveSnapPanic     Failpoint = goFailpoint{"etcdserver/raftBeforeSaveSnap", "panic", nil, AnyMember}
	raftAfterSaveSnapPanic      Failpoint = goFailpoint{"etcdserver/raftAfterSaveSnap", "panic", nil, AnyMember}
)

type Failpoint interface {
	Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error
	Name() string
}

type killFailpoint struct{}

func (f killFailpoint) Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	err := member.Kill()
	if err != nil {
		return err
	}
	err = member.Wait()
	if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
		return err
	}
	err = member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (f killFailpoint) Name() string {
	return "Kill"
}

type goFailpoint struct {
	failpoint string
	payload   string
	trigger   func(ctx context.Context, member e2e.EtcdProcess) error
	target    failpointTarget
}

type failpointTarget string

const (
	AnyMember failpointTarget = "AnyMember"
	Leader    failpointTarget = "Leader"
)

func (f goFailpoint) Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	var member e2e.EtcdProcess
	switch f.target {
	case AnyMember:
		member = clus.Procs[rand.Int()%len(clus.Procs)]
	case Leader:
		member = clus.Procs[clus.WaitLeader(t)]
	default:
		panic("unknown target")
	}
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().GoFailPort)

	failpoints := []string{"backend/beforeCommit", "backend/afterCommit", "etcdserver/raftBeforeSave", "etcdserver/raftAfterSave"}
	index1 := rand.Int() % len(failpoints)                                    // 0-3
	index2 := (rand.Int()%(len(failpoints)-1) + 1 + index1) % len(failpoints) // 0-2
	payload := failpoints[index1] + "=panic;" + failpoints[index2] + "=sleep(100);"
	t.Logf("Payload %q", payload)

	err := setupGoFailpoint(address, payload)
	if err != nil {
		return fmt.Errorf("gofailpoint setup failed: %w", err)
	}
	if f.trigger != nil {
		err = f.trigger(ctx, member)
		if err != nil {
			return fmt.Errorf("triggering gofailpoint failed: %w", err)
		}
	}
	err = member.Wait()
	if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
		return err
	}
	err = member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func setupGoFailpoint(host, payload string) error {
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   "failpoints",
	}

	r, err := http.NewRequest("PUT", failpointUrl.String(), bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

func (f goFailpoint) Name() string {
	return f.failpoint
}

func triggerDefrag(ctx context.Context, member e2e.EtcdProcess) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsV3(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	_, err = cc.Defragment(ctx, member.EndpointsV3()[0])
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func triggerCompact(ctx context.Context, member e2e.EtcdProcess) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsV3(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    1 * time.Millisecond,
		DialKeepAliveTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	resp, err := cc.Get(ctx, "/")
	if err != nil {
		return err
	}
	_, err = cc.Compact(ctx, resp.Header.Revision)
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

var httpClient = http.Client{
	Timeout: 10 * time.Millisecond,
}

type randomFailpoint struct {
	failpoints []Failpoint
}

func (f randomFailpoint) Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	failpoint := f.failpoints[rand.Int()%len(f.failpoints)]
	t.Logf("Triggering %v failpoint\n", failpoint.Name())
	return failpoint.Trigger(t, ctx, clus)
}

func (f randomFailpoint) Name() string {
	return "Random"
}
