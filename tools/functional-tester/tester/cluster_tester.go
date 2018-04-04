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

	"go.uber.org/zap"
)

// compactQPS is rough number of compact requests per second.
// Previous tests showed etcd can compact about 60,000 entries per second.
const compactQPS = 50000

// StartTester starts tester.
func (clus *Cluster) StartTester() {
	// TODO: upate status
	clus.startStresser()

	var preModifiedKey int64
	for round := 0; round < int(clus.Tester.RoundLimit) || clus.Tester.RoundLimit == -1; round++ {
		roundTotalCounter.Inc()
		clus.rd = round

		if err := clus.doRound(); err != nil {
			clus.lg.Warn(
				"doRound failed; returning",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Error(err),
			)
			if clus.cleanup() != nil {
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
			"compacting",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Duration("timeout", timeout),
		)
		if err := clus.compact(revToCompact, timeout); err != nil {
			clus.lg.Warn(
				"compact failed",
				zap.Int("round", clus.rd),
				zap.Int("case", clus.cs),
				zap.Error(err),
			)
			if err = clus.cleanup(); err != nil {
				clus.lg.Warn(
					"cleanup failed",
					zap.Int("round", clus.rd),
					zap.Int("case", clus.cs),
					zap.Error(err),
				)
				return
			}
			// reset preModifiedKey after clean up
			preModifiedKey = 0
		}
		if round > 0 && round%500 == 0 { // every 500 rounds
			if err := clus.defrag(); err != nil {
				clus.lg.Warn(
					"defrag failed; returning",
					zap.Int("round", clus.rd),
					zap.Int("case", clus.cs),
					zap.Error(err),
				)
				clus.failed()
				return
			}
		}
	}

	clus.lg.Info(
		"functional-tester passed",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
	)
}

func (clus *Cluster) doRound() error {
	if clus.Tester.FailureShuffle {
		clus.shuffleFailures()
	}

	clus.lg.Info(
		"starting round",
		zap.Int("round", clus.rd),
		zap.Strings("failures", clus.failureStrings()),
	)

	for i, fa := range clus.failures {
		clus.cs = i

		caseTotalCounter.WithLabelValues(fa.Desc()).Inc()

		clus.lg.Info("wait health before injecting failures")
		if err := clus.WaitHealth(); err != nil {
			return fmt.Errorf("wait full health error: %v", err)
		}

		// TODO: "NO_FAIL_WITH_STRESS"
		// TODO: "NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS"

		clus.lg.Info(
			"injecting failure",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.String("desc", fa.Desc()),
		)
		if err := fa.Inject(clus); err != nil {
			return fmt.Errorf("injection error: %v", err)
		}
		clus.lg.Info(
			"injected failure",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.String("desc", fa.Desc()),
		)

		// if run local, recovering server may conflict
		// with stressing client ports
		// TODO: use unix for local tests
		clus.lg.Info(
			"recovering failure",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.String("desc", fa.Desc()),
		)
		if err := fa.Recover(clus); err != nil {
			return fmt.Errorf("recovery error: %v", err)
		}
		clus.lg.Info(
			"recovered failure",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.String("desc", fa.Desc()),
		)

		clus.lg.Info("pausing stresser after failure recovery, before wait health")
		clus.pauseStresser()

		clus.lg.Info("wait health after recovering failures")
		if err := clus.WaitHealth(); err != nil {
			return fmt.Errorf("wait full health error: %v", err)
		}

		clus.lg.Info("check consistency after recovering failures")
		if err := clus.checkConsistency(); err != nil {
			return fmt.Errorf("tt.checkConsistency error (%v)", err)
		}

		clus.lg.Info(
			"failure case passed",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.String("desc", fa.Desc()),
		)
	}

	clus.lg.Info(
		"finished round",
		zap.Int("round", clus.rd),
		zap.Strings("failures", clus.failureStrings()),
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
	clus.lg.Info("pausing stresser before compact")
	clus.pauseStresser()
	defer func() {
		if err == nil {
			err = clus.startStresser()
		}
	}()

	clus.lg.Info(
		"compacting storage",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)
	if err = clus.compactKV(rev, timeout); err != nil {
		return err
	}
	clus.lg.Info(
		"compacted storage",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)

	clus.lg.Info(
		"checking compaction",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)
	if err = clus.checkCompact(rev); err != nil {
		clus.lg.Warn(
			"checkCompact failed",
			zap.Int64("current-revision", clus.currentRevision),
			zap.Int64("compact-revision", rev),
			zap.Error(err),
		)
		return err
	}
	clus.lg.Info(
		"confirmed compaction",
		zap.Int64("current-revision", clus.currentRevision),
		zap.Int64("compact-revision", rev),
	)

	return nil
}

func (clus *Cluster) failed() {
	if !clus.Tester.ExitOnFailure {
		return
	}

	clus.lg.Info(
		"exiting on failure",
		zap.Int("round", clus.rd),
		zap.Int("case", clus.cs),
	)
	clus.DestroyEtcdAgents()
	os.Exit(2)
}

func (clus *Cluster) cleanup() error {
	defer clus.failed()

	roundFailedTotalCounter.Inc()
	desc := "compact/defrag"
	if clus.cs != -1 {
		desc = clus.failures[clus.cs].Desc()
	}
	caseFailedTotalCounter.WithLabelValues(desc).Inc()

	clus.closeStresser()
	if err := clus.FailArchive(); err != nil {
		clus.lg.Warn(
			"cleanup failed",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Error(err),
		)
		return err
	}
	if err := clus.Restart(); err != nil {
		clus.lg.Warn(
			"restart failed",
			zap.Int("round", clus.rd),
			zap.Int("case", clus.cs),
			zap.Error(err),
		)
		return err
	}

	clus.updateStresserChecker()
	return nil
}
