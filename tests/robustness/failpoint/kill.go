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
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
)

var KillFailpoint Failpoint = killFailpoint{}

type killFailpoint struct{}

func (f killFailpoint) Inject(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster, baseTime time.Time, ids identity.Provider) ([]report.ClientReport, error) {
	member := clus.Procs[rand.Int()%len(clus.Procs)]

	for member.IsRunning() {
		err := member.Kill()
		if err != nil {
			lg.Info("Sending kill signal failed", zap.Error(err))
		}
		err = member.Wait(ctx)
		if err != nil && !strings.Contains(err.Error(), "unexpected exit code") {
			lg.Info("Failed to kill the process", zap.Error(err))
			return nil, fmt.Errorf("failed to kill the process within %s, err: %w", triggerTimeout, err)
		}
	}
	if lazyfs := member.LazyFS(); lazyfs != nil {
		lg.Info("Removing data that was not fsynced")
		err := lazyfs.ClearCache(ctx)
		if err != nil {
			return nil, err
		}
	}
	err := member.Start(ctx)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (f killFailpoint) Name() string {
	return "Kill"
}

func (f killFailpoint) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess, traffic.Profile) bool {
	return true
}
