// Copyright 2023 The etcd Authors
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

package failpoint

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	MemberReplace Failpoint = memberReplace{}
)

type memberReplace struct{}

func (f memberReplace) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) error {
	memberID := uint64(rand.Int() % len(clus.Procs))
	member := clus.Procs[memberID]
	var endpoints []string
	for i := 1; i < len(clus.Procs); i++ {
		endpoints = append(endpoints, clus.Procs[(int(memberID)+i)%len(clus.Procs)].EndpointsGRPC()...)
	}
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    50 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer cc.Close()
	memberID, found, err := getID(ctx, cc, member.Config().Name)
	if err != nil {
		return err
	}
	if !found {
		t.Fatal("Member not found")
	}
	// Need to wait health interval for cluster to accept member changes
	time.Sleep(etcdserver.HealthInterval)
	lg.Info("Removing member", zap.String("member", member.Config().Name))
	_, err = cc.MemberRemove(ctx, memberID)
	if err != nil {
		return err
	}
	_, found, err = getID(ctx, cc, member.Config().Name)
	if err != nil {
		return err
	}
	if found {
		t.Fatal("Expected member to be removed")
	}

	for member.IsRunning() {
		err = member.Kill()
		if err != nil {
			lg.Info("Sending kill signal failed", zap.Error(err))
		}
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Failed to kill the process", zap.Error(err))
			return fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}
	lg.Info("Removing member data", zap.String("member", member.Config().Name))
	err = os.RemoveAll(member.Config().DataDirPath)
	if err != nil {
		return err
	}

	lg.Info("Adding member back", zap.String("member", member.Config().Name))
	removedMemberPeerURL := member.Config().PeerURL.String()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		reqCtx, cancel := context.WithTimeout(ctx, time.Second)
		_, err = cc.MemberAdd(reqCtx, []string{removedMemberPeerURL})
		cancel()
		if err == nil {
			break
		}
	}
	err = patchArgs(member.Config().Args, "initial-cluster-state", "existing")
	if err != nil {
		return err
	}
	lg.Info("Starting member", zap.String("member", member.Config().Name))
	err = member.Start(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, found, err := getID(ctx, cc, member.Config().Name)
		if err != nil {
			continue
		}
		if found {
			break
		}
	}
	return nil
}

func (f memberReplace) Name() string {
	return "MemberReplace"
}

func (f memberReplace) Available(config e2e.EtcdProcessClusterConfig, _ e2e.EtcdProcess) bool {
	return config.ClusterSize > 1
}

func getID(ctx context.Context, cc *clientv3.Client, name string) (id uint64, found bool, err error) {
	resp, err := cc.MemberList(ctx)
	if err != nil {
		return 0, false, err
	}
	for _, member := range resp.Members {
		if name == member.Name {
			return member.ID, true, nil
		}
	}
	return 0, false, nil
}

func patchArgs(args []string, flag, newValue string) error {
	for i, arg := range args {
		if strings.Contains(arg, flag) {
			args[i] = fmt.Sprintf("--%s=%s", flag, newValue)
			return nil
		}
	}
	return fmt.Errorf("--%s flag not found", flag)
}
