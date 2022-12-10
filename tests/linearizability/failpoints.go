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

const (
	triggerTimeout = time.Second
)

var (
	KillFailpoint                            Failpoint = killFailpoint{}
	DefragBeforeCopyPanic                    Failpoint = goPanicFailpoint{"defragBeforeCopy", triggerDefrag, AnyMember}
	DefragBeforeRenamePanic                  Failpoint = goPanicFailpoint{"defragBeforeRename", triggerDefrag, AnyMember}
	BeforeCommitPanic                        Failpoint = goPanicFailpoint{"beforeCommit", nil, AnyMember}
	AfterCommitPanic                         Failpoint = goPanicFailpoint{"afterCommit", nil, AnyMember}
	RaftBeforeSavePanic                      Failpoint = goPanicFailpoint{"raftBeforeSave", nil, AnyMember}
	RaftAfterSavePanic                       Failpoint = goPanicFailpoint{"raftAfterSave", nil, AnyMember}
	BackendBeforePreCommitHookPanic          Failpoint = goPanicFailpoint{"commitBeforePreCommitHook", nil, AnyMember}
	BackendAfterPreCommitHookPanic           Failpoint = goPanicFailpoint{"commitAfterPreCommitHook", nil, AnyMember}
	BackendBeforeStartDBTxnPanic             Failpoint = goPanicFailpoint{"beforeStartDBTxn", nil, AnyMember}
	BackendAfterStartDBTxnPanic              Failpoint = goPanicFailpoint{"afterStartDBTxn", nil, AnyMember}
	BackendBeforeWritebackBufPanic           Failpoint = goPanicFailpoint{"beforeWritebackBuf", nil, AnyMember}
	BackendAfterWritebackBufPanic            Failpoint = goPanicFailpoint{"afterWritebackBuf", nil, AnyMember}
	CompactBeforeCommitScheduledCompactPanic Failpoint = goPanicFailpoint{"compactBeforeCommitScheduledCompact", triggerCompact, AnyMember}
	CompactAfterCommitScheduledCompactPanic  Failpoint = goPanicFailpoint{"compactAfterCommitScheduledCompact", triggerCompact, AnyMember}
	CompactBeforeSetFinishedCompactPanic     Failpoint = goPanicFailpoint{"compactBeforeSetFinishedCompact", triggerCompact, AnyMember}
	CompactAfterSetFinishedCompactPanic      Failpoint = goPanicFailpoint{"compactAfterSetFinishedCompact", triggerCompact, AnyMember}
	CompactBeforeCommitBatchPanic            Failpoint = goPanicFailpoint{"compactBeforeCommitBatch", triggerCompact, AnyMember}
	CompactAfterCommitBatchPanic             Failpoint = goPanicFailpoint{"compactAfterCommitBatch", triggerCompact, AnyMember}
	RaftBeforeLeaderSendPanic                Failpoint = goPanicFailpoint{"raftBeforeLeaderSend", nil, Leader}
	RandomFailpoint                          Failpoint = randomFailpoint{[]Failpoint{
		KillFailpoint, BeforeCommitPanic, AfterCommitPanic, RaftBeforeSavePanic,
		RaftAfterSavePanic, DefragBeforeCopyPanic, DefragBeforeRenamePanic,
		BackendBeforePreCommitHookPanic, BackendAfterPreCommitHookPanic,
		BackendBeforeStartDBTxnPanic, BackendAfterStartDBTxnPanic,
		BackendBeforeWritebackBufPanic, BackendAfterWritebackBufPanic,
		CompactBeforeCommitScheduledCompactPanic, CompactAfterCommitScheduledCompactPanic,
		CompactBeforeSetFinishedCompactPanic, CompactAfterSetFinishedCompactPanic,
		CompactBeforeCommitBatchPanic, CompactAfterCommitBatchPanic,
		RaftBeforeLeaderSendPanic,
	}}
	// TODO: Figure out how to reliably trigger below failpoints and add them to RandomFailpoint
	raftBeforeApplySnapPanic    Failpoint = goPanicFailpoint{"raftBeforeApplySnap", nil, AnyMember}
	raftAfterApplySnapPanic     Failpoint = goPanicFailpoint{"raftAfterApplySnap", nil, AnyMember}
	raftAfterWALReleasePanic    Failpoint = goPanicFailpoint{"raftAfterWALRelease", nil, AnyMember}
	raftBeforeFollowerSendPanic Failpoint = goPanicFailpoint{"raftBeforeFollowerSend", nil, AnyMember}
	raftBeforeSaveSnapPanic     Failpoint = goPanicFailpoint{"raftBeforeSaveSnap", nil, AnyMember}
	raftAfterSaveSnapPanic      Failpoint = goPanicFailpoint{"raftAfterSaveSnap", nil, AnyMember}
)

type Failpoint interface {
	Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error
	Name() string
}

type killFailpoint struct{}

func (f killFailpoint) Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]

	killCtx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	for member.IsRunning() {
		err := member.Kill()
		if err != nil {
			t.Logf("sending kill signal failed: %v", err)
		}
		err = member.Wait(killCtx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			return fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}

	err := member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (f killFailpoint) Name() string {
	return "Kill"
}

type goPanicFailpoint struct {
	failpoint string
	trigger   func(ctx context.Context, member e2e.EtcdProcess) error
	target    failpointTarget
}

type failpointTarget string

const (
	AnyMember failpointTarget = "AnyMember"
	Leader    failpointTarget = "Leader"
)

func (f goPanicFailpoint) Trigger(t *testing.T, ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	member := f.pickMember(t, clus)
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().GoFailPort)

	triggerCtx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()

	for member.IsRunning() {
		err := setupGoFailpoint(triggerCtx, address, f.failpoint, "panic")
		if err != nil {
			t.Logf("gofailpoint setup failed: %v", err)
		}
		if f.trigger != nil {
			err = f.trigger(triggerCtx, member)
			if err != nil {
				t.Logf("triggering gofailpoint failed: %v", err)
			}
		}
		err = member.Wait(triggerCtx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			return fmt.Errorf("failed to trigger a process panic within %s, err: %w", triggerTimeout, err)
		}
	}

	err := member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (f goPanicFailpoint) pickMember(t *testing.T, clus *e2e.EtcdProcessCluster) e2e.EtcdProcess {
	switch f.target {
	case AnyMember:
		return clus.Procs[rand.Int()%len(clus.Procs)]
	case Leader:
		return clus.Procs[clus.WaitLeader(t)]
	default:
		panic("unknown target")
	}
}

func setupGoFailpoint(ctx context.Context, host, failpoint, payload string) error {
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequestWithContext(ctx, "PUT", failpointUrl.String(), bytes.NewBuffer([]byte(payload)))
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

func (f goPanicFailpoint) Name() string {
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
