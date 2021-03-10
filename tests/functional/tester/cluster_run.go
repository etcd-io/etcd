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
	"fmt"
	"os"
	"time"

	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
)

// compactQPS is rough number of compact requests per second.
// Previous tests showed etcd can compact about 60,000 entries per second.
const compactQPS = 50000

// Run starts tester.
func (clus *Cluster) Run() {
	defer printReport()

	if err := fileutil.TouchDirAll(clus.Tester.DataDir); err != nil {
		clus.lg.Panic(
			"failed to create test data directory",
			zap.String("dir", clus.Tester.DataDir),
			zap.Error(err),
		)
	}

	var preModifiedKey int64
	for round := 0; round < int(clus.Tester.RoundLimit) || clus.Tester.RoundLimit == -1; round++ {
		roundTotalCounter.Inc()
		clus.rd = round

		if err := clus.doRound(); err != nil {
			clus.lg.Error(
				"round FAIL",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Int("case-total", len(clus.cases)),
				zap.Error(err),
			)
			if clus.cleanup(err) != nil {
				return
			}
			// reset preModifiedKey after clean up
			preModifiedKey = 0
			continue
		}

		// -1 so that logPrefix doesn't print out 'case'
		clus.cs = -1

		revToCompact := max(0, clus.currentRevision-10000)
		currentModifiedKey := clus.stresser.ModifiedKeys()
		modifiedKey := currentModifiedKey - preModifiedKey
		preModifiedKey = currentModifiedKey
		timeout := 10 * time.Second
		timeout += time.Duration(modifiedKey/compactQPS) * time.Second
		clus.lg.Info(
			"compact START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.Duration("timeout", timeout),
		)
		if err := clus.compact(revToCompact, timeout); err != nil {
			clus.lg.Warn(
				"compact FAIL",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Int("case-total", len(clus.cases)),
				zap.Error(err),
			)
			if err = clus.cleanup(err); err != nil {
				clus.lg.Warn(
					"cleanup FAIL",
					zap.Int("round", clus.rd),
					zap.Int("case", clus.cs),
					zap.Int("case-total", len(clus.cases)),
					zap.Error(err),
				)
				return
			}
			// reset preModifiedKey after clean up
			preModifiedKey = 0
		}
		if round > 0 && round%500 == 0 { // every 500 rounds
			if err := clus.defrag(); err != nil {
				clus.failed(err)
				return
			}
		}
	}

	clus.lg.Info(
		"functional-tester PASS",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.Int("case-total", len(clus.cases)),
	)
}

func (clus *Cluster) doRound() error {
	if clus.Tester.CaseShuffle {
		clus.shuffleCases()
	}

	roundNow := time.Now()
	clus.lg.Info(
		"round START",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.Int("case-total", len(clus.cases)),
		zap.Strings("cases", clus.listCases()),
	)
	for i, fa := range clus.cases {
		clus.cs = i

		caseTotal[fa.Desc()]++
		caseTotalCounter.WithLabelValues(fa.Desc()).Inc()

		caseNow := time.Now()
		clus.lg.Info(
			"case START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
		)

		clus.lg.Info("wait health before injecting failures")
		if err := clus.WaitHealth(); err != nil {
			return fmt.Errorf("wait full health error: %v", err)
		}

		stressStarted := false
		fcase := fa.TestCase()
		if fcase != rpcpb.Case_NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS {
			clus.lg.Info(
				"stress START",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Int("case-total", len(clus.cases)),
				zap.String("desc", fa.Desc()),
			)
			if err := clus.stresser.Stress(); err != nil {
				return fmt.Errorf("start stresser error: %v", err)
			}
			stressStarted = true
		}

		clus.lg.Info(
			"inject START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
		)
		if err := fa.Inject(clus); err != nil {
			return fmt.Errorf("injection error: %v", err)
		}

		// if run local, recovering server may conflict
		// with stressing client ports
		// TODO: use unix for local tests
		clus.lg.Info(
			"recover START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
		)
		if err := fa.Recover(clus); err != nil {
			return fmt.Errorf("recovery error: %v", err)
		}

		if stressStarted {
			clus.lg.Info(
				"stress PAUSE",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Int("case-total", len(clus.cases)),
				zap.String("desc", fa.Desc()),
			)
			ems := clus.stresser.Pause()
			if fcase == rpcpb.Case_NO_FAIL_WITH_STRESS && len(ems) > 0 {
				ess := make([]string, 0, len(ems))
				cnt := 0
				for k, v := range ems {
					ess = append(ess, fmt.Sprintf("%s (count: %d)", k, v))
					cnt += v
				}
				clus.lg.Warn(
					"expected no errors",
					zap.String("desc", fa.Desc()),
					zap.Strings("errors", ess),
				)

				// with network delay, some ongoing requests may fail
				// only return error, if more than 30% of QPS requests fail
				if cnt > int(float64(clus.Tester.StressQPS)*0.3) {
					return fmt.Errorf("expected no error in %q, got %q", fcase.String(), ess)
				}
			}
		}

		clus.lg.Info(
			"health check START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
		)
		if err := clus.WaitHealth(); err != nil {
			return fmt.Errorf("wait full health error: %v", err)
		}

		checkerFailExceptions := []rpcpb.Checker{}
		switch fcase {
		case rpcpb.Case_SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH:
			// TODO: restore from snapshot
			checkerFailExceptions = append(checkerFailExceptions, rpcpb.Checker_LEASE_EXPIRE)
		}

		clus.lg.Info(
			"consistency check START",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
		)
		if err := clus.runCheckers(checkerFailExceptions...); err != nil {
			return fmt.Errorf("consistency check error (%v)", err)
		}
		clus.lg.Info(
			"consistency check PASS",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.String("desc", fa.Desc()),
			zap.Duration("took", time.Since(caseNow)),
		)
	}

	clus.lg.Info(
		"round ALL PASS",
		zap.Int("round", clus.rd),
		zap.Strings("cases", clus.listCases()),
		zap.Int("case-total", len(clus.cases)),
		zap.Duration("took", time.Since(roundNow)),
	)
	return nil
}

func (clus *Cluster) updateRevision() error {
	revs, _, err := clus.getRevisionHash()
	for _, rev := range revs {
		clus.currentRevision = rev
		break // just need get one of the current revisions
	}

	clus.lg.Info(
		"updated current revision",
		zap.Int64("current-revision", clus.currentRevision),
	)
	return err
}

func (clus *Cluster) compact(rev int64, timeout time.Duration) (err error) {
	if err = clus.compactKV(rev, timeout); err != nil {
		clus.lg.Warn(
			"compact FAIL",
			zap.Int64("current-revision", clus.currentRevision),
			zap.Int64("compact-revision", rev),
			zap.Error(err),
		)
		return err
	}
	clus.lg.Info(
		"compact DONE",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)

	if err = clus.checkCompact(rev); err != nil {
		clus.lg.Warn(
			"check compact FAIL",
			zap.Int64("current-revision", clus.currentRevision),
			zap.Int64("compact-revision", rev),
			zap.Error(err),
		)
		return err
	}
	clus.lg.Info(
		"check compact DONE",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)

	return nil
}

func (clus *Cluster) failed(err error) {
	clus.lg.Error(
		"functional-tester FAIL",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.Int("case-total", len(clus.cases)),
		zap.Error(err),
	)
	clus.Send_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT()

	os.Exit(2)
}

func (clus *Cluster) cleanup(err error) error {
	if clus.Tester.ExitOnCaseFail {
		defer clus.failed(err)
	}

	roundFailedTotalCounter.Inc()
	desc := "compact/defrag"
	if clus.cs != -1 {
		desc = clus.cases[clus.cs].Desc()
	}
	caseFailedTotalCounter.WithLabelValues(desc).Inc()

	clus.lg.Info(
		"closing stressers before archiving failure data",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
		zap.Int("case-total", len(clus.cases)),
	)
	clus.stresser.Close()

	if err := clus.send_SIGQUIT_ETCD_AND_ARCHIVE_DATA(); err != nil {
		clus.lg.Warn(
			"cleanup FAIL",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.Error(err),
		)
		return err
	}
	if err := clus.send_RESTART_ETCD(); err != nil {
		clus.lg.Warn(
			"restart FAIL",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Int("case-total", len(clus.cases)),
			zap.Error(err),
		)
		return err
	}

	clus.setStresserChecker()
	return nil
}
