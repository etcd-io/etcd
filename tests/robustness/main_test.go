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

package robustness

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/framework"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
	"go.etcd.io/etcd/tests/v3/robustness/traffic"
	"go.etcd.io/etcd/tests/v3/robustness/validate"
)

var testRunner = framework.E2eTestRunner

func TestMain(m *testing.M) {
	testRunner.TestMain(m)
}

func TestRobustnessExploratory(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, scenario := range exploratoryScenarios(t) {
		t.Run(scenario.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			scenario.cluster.Logger = lg
			ctx := context.Background()
			testRobustness(ctx, t, lg, scenario)
		})
	}
}

func TestRobustnessRegression(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, scenario := range regressionScenarios(t) {
		t.Run(scenario.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			scenario.cluster.Logger = lg
			ctx := context.Background()
			testRobustness(ctx, t, lg, scenario)
		})
	}
}

func testRobustness(ctx context.Context, t *testing.T, lg *zap.Logger, s testScenario) {
	r := report.TestReport{Logger: lg}
	var err error
	r.Cluster, err = e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&s.cluster))
	if err != nil {
		t.Fatal(err)
	}
	defer forcestopCluster(r.Cluster)

	if s.failpoint == nil {
		s.failpoint, err = failpoint.PickRandom(r.Cluster)
		if err != nil {
			t.Fatal(err)
		}
	}

	// t.Failed() returns false during panicking. We need to forcibly
	// save data on panicking.
	// Refer to: https://github.com/golang/go/issues/49929
	panicked := true
	defer func() {
		r.Report(t, panicked)
	}()
	r.Client = s.run(ctx, t, lg, r.Cluster)
	persistedRequests, err := report.PersistedRequestsCluster(lg, r.Cluster)
	if err != nil {
		t.Fatal(err)
	}

	watchProgressNotifyEnabled := r.Cluster.Cfg.ServerConfig.ExperimentalWatchProgressNotifyInterval != 0
	validateGotAtLeastOneProgressNotify(t, r.Client, s.watch.requestProgress || watchProgressNotifyEnabled)
	validateConfig := validate.Config{ExpectRevisionUnique: s.traffic.ExpectUniqueRevision()}
	r.Visualize = validate.ValidateAndReturnVisualize(t, lg, validateConfig, r.Client, persistedRequests, 5*time.Minute)

	panicked = false
}

func (s testScenario) run(ctx context.Context, t *testing.T, lg *zap.Logger, clus *e2e.EtcdProcessCluster) (reports []report.ClientReport) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g := errgroup.Group{}
	var operationReport, watchReport, failpointClientReport []report.ClientReport
	failpointInjected := make(chan failpoint.Injection, 1)

	// using baseTime time-measuring operation to get monotonic clock reading
	// see https://github.com/golang/go/blob/master/src/time/time.go#L17
	baseTime := time.Now()
	ids := identity.NewIDProvider()
	g.Go(func() error {
		defer close(failpointInjected)
		// Give some time for traffic to reach qps target before injecting failpoint.
		time.Sleep(time.Second)
		fr, err := failpoint.Inject(ctx, t, lg, clus, s.failpoint, baseTime, ids)
		if err != nil {
			t.Error(err)
			cancel()
		}
		// Give some time for traffic to reach qps target after injecting failpoint.
		time.Sleep(time.Second)
		if fr != nil {
			failpointInjected <- fr.Injection
			failpointClientReport = fr.Client
		}
		return nil
	})
	maxRevisionChan := make(chan int64, 1)
	g.Go(func() error {
		defer close(maxRevisionChan)
		operationReport = traffic.SimulateTraffic(ctx, t, lg, clus, s.profile, s.traffic, failpointInjected, baseTime, ids)
		maxRevision := operationsMaxRevision(operationReport)
		maxRevisionChan <- maxRevision
		lg.Info("Finished simulating traffic", zap.Int64("max-revision", maxRevision))
		return nil
	})
	g.Go(func() error {
		watchReport = collectClusterWatchEvents(ctx, t, clus, maxRevisionChan, s.watch, baseTime, ids)
		return nil
	})
	g.Wait()
	return append(operationReport, append(failpointClientReport, watchReport...)...)
}

func operationsMaxRevision(reports []report.ClientReport) int64 {
	var maxRevision int64
	for _, r := range reports {
		for _, op := range r.KeyValue {
			resp := op.Output.(model.MaybeEtcdResponse)
			if resp.Revision > maxRevision {
				maxRevision = resp.Revision
			}
		}
	}
	return maxRevision
}

// forcestopCluster stops the etcd member with signal kill.
func forcestopCluster(clus *e2e.EtcdProcessCluster) error {
	for _, member := range clus.Procs {
		member.Kill()
	}
	return clus.ConcurrentStop()
}
