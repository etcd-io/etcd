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

package validate

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

const (
	historyLength = 2000
)

func TestValidateWithoutWatch(t *testing.T) {
	ctx := context.Background()
	ids := identity.NewIdProvider()
	baseTime := time.Now()
	state := model.NewEtcdFake()
	g := errgroup.Group{}
	l := &limiter{finish: make(chan struct{})}
	traffic := traffic.HighTraffic

	mux := sync.Mutex{}
	reports := []model.ClientReport{}
	for i := 0; i < traffic.ClientCount; i++ {
		c := model.NewFakeClient(state, ids, baseTime)
		g.Go(func() error {
			traffic.Traffic.Run(ctx, c, l, ids, nil, l.finish)
			mux.Lock()
			reports = append(reports, c.Report())
			mux.Unlock()
			return nil
		})
	}
	g.Wait()

	ValidateAndReturnVisualize(t, zaptest.NewLogger(t), Config{ExpectRevisionUnique: traffic.Traffic.ExpectUniqueRevision()}, reports)
}

type limiter struct {
	operationCount int64
	finish         chan struct{}
}

func (l *limiter) Wait(ctx context.Context) error {
	l.operationCount++
	if l.operationCount == historyLength {
		close(l.finish)
	}
	return nil
}
