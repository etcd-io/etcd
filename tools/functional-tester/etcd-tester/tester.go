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

package main

import (
	"fmt"
	"time"
)

type tester struct {
	failures []failure
	cluster  *cluster
	limit    int
	checker  Checker

	status          Status
	currentRevision int64
}

// compactQPS is rough number of compact requests per second.
// Previous tests showed etcd can compact about 60,000 entries per second.
const compactQPS = 50000

func (tt *tester) runLoop() {
	tt.status.Since = time.Now()
	tt.status.RoundLimit = tt.limit
	tt.status.cluster = tt.cluster
	for _, f := range tt.failures {
		tt.status.Failures = append(tt.status.Failures, f.Desc())
	}

	var prevCompactRev int64
	for round := 0; round < tt.limit || tt.limit == -1; round++ {
		tt.status.setRound(round)
		roundTotalCounter.Inc()

		if ok, err := tt.doRound(round); !ok {
			if err != nil {
				if tt.cleanup() != nil {
					return
				}
			}
			prevCompactRev = 0 // reset after clean up
			continue
		}
		// -1 so that logPrefix doesn't print out 'case'
		tt.status.setCase(-1)

		revToCompact := max(0, tt.currentRevision-10000)
		compactN := revToCompact - prevCompactRev
		timeout := 10 * time.Second
		if compactN > 0 {
			timeout += time.Duration(compactN/compactQPS) * time.Second
		}
		prevCompactRev = revToCompact

		plog.Printf("%s compacting %d entries (timeout %v)", tt.logPrefix(), compactN, timeout)
		if err := tt.compact(revToCompact, timeout); err != nil {
			plog.Warningf("%s functional-tester compact got error (%v)", tt.logPrefix(), err)
			if tt.cleanup() != nil {
				return
			}
			prevCompactRev = 0 // reset after clean up
		}
		if round > 0 && round%500 == 0 { // every 500 rounds
			if err := tt.defrag(); err != nil {
				plog.Warningf("%s functional-tester returning with error (%v)", tt.logPrefix(), err)
				return
			}
		}
	}

	plog.Printf("%s functional-tester is finished", tt.logPrefix())
}

func (tt *tester) doRound(round int) (bool, error) {
	for j, f := range tt.failures {
		caseTotalCounter.WithLabelValues(f.Desc()).Inc()
		tt.status.setCase(j)

		if err := tt.cluster.WaitHealth(); err != nil {
			plog.Printf("%s wait full health error: %v", tt.logPrefix(), err)
			return false, nil
		}

		plog.Printf("%s injecting failure %q", tt.logPrefix(), f.Desc())
		if err := f.Inject(tt.cluster, round); err != nil {
			plog.Printf("%s injection error: %v", tt.logPrefix(), err)
			return false, nil
		}
		plog.Printf("%s injected failure", tt.logPrefix())

		plog.Printf("%s recovering failure %q", tt.logPrefix(), f.Desc())
		if err := f.Recover(tt.cluster, round); err != nil {
			plog.Printf("%s recovery error: %v", tt.logPrefix(), err)
			return false, nil
		}
		plog.Printf("%s recovered failure", tt.logPrefix())

		if err := tt.checkConsistency(); err != nil {
			plog.Warningf("%s functional-tester returning with tt.checkConsistency error (%v)", tt.logPrefix(), err)
			return false, err
		}

		plog.Printf("%s succeed!", tt.logPrefix())
	}
	return true, nil
}

func (tt *tester) updateRevision() error {
	if tt.cluster.v2Only {
		return nil
	}

	revs, _, err := tt.cluster.getRevisionHash()
	for _, rev := range revs {
		tt.currentRevision = rev
		break // just need get one of the current revisions
	}

	plog.Printf("%s updated current revision to %d", tt.logPrefix(), tt.currentRevision)
	return err
}

func (tt *tester) checkConsistency() (err error) {
	tt.cancelStressers()
	defer func() {
		if err != nil {
			return
		}
		if err = tt.updateRevision(); err != nil {
			plog.Warningf("%s functional-tester returning with tt.updateRevision error (%v)", tt.logPrefix(), err)
			return
		}
		err = tt.startStressers()
	}()
	if err = tt.checker.Check(); err != nil {
		plog.Printf("%s %v", tt.logPrefix(), err)
	}
	return err
}

func (tt *tester) compact(rev int64, timeout time.Duration) (err error) {
	tt.cancelStressers()
	defer func() {
		if err == nil {
			err = tt.startStressers()
		}
	}()

	plog.Printf("%s compacting storage (current revision %d, compact revision %d)", tt.logPrefix(), tt.currentRevision, rev)
	if err = tt.cluster.compactKV(rev, timeout); err != nil {
		return err
	}
	plog.Printf("%s compacted storage (compact revision %d)", tt.logPrefix(), rev)

	plog.Printf("%s checking compaction (compact revision %d)", tt.logPrefix(), rev)
	if err = tt.cluster.checkCompact(rev); err != nil {
		plog.Warningf("%s checkCompact error (%v)", tt.logPrefix(), err)
		return err
	}

	plog.Printf("%s confirmed compaction (compact revision %d)", tt.logPrefix(), rev)
	return nil
}

func (tt *tester) defrag() error {
	plog.Printf("%s defragmenting...", tt.logPrefix())
	if err := tt.cluster.defrag(); err != nil {
		plog.Warningf("%s defrag error (%v)", tt.logPrefix(), err)
		if cerr := tt.cleanup(); cerr != nil {
			return fmt.Errorf("%s, %s", err, cerr)
		}
		return err
	}

	plog.Printf("%s defragmented...", tt.logPrefix())
	return nil
}

func (tt *tester) logPrefix() string {
	var (
		rd     = tt.status.getRound()
		cs     = tt.status.getCase()
		prefix = fmt.Sprintf("[round#%d case#%d]", rd, cs)
	)
	if cs == -1 {
		prefix = fmt.Sprintf("[round#%d]", rd)
	}
	return prefix
}

func (tt *tester) cleanup() error {
	roundFailedTotalCounter.Inc()
	desc := "compact/defrag"
	if tt.status.Case != -1 {
		desc = tt.failures[tt.status.Case].Desc()
	}
	caseFailedTotalCounter.WithLabelValues(desc).Inc()

	plog.Printf("%s cleaning up...", tt.logPrefix())
	if err := tt.cluster.Cleanup(); err != nil {
		plog.Warningf("%s cleanup error: %v", tt.logPrefix(), err)
		return err
	}

	if err := tt.cluster.Reset(); err != nil {
		plog.Warningf("%s cleanup Bootstrap error: %v", tt.logPrefix(), err)
		return err
	}

	return nil
}

func (tt *tester) cancelStressers() {
	plog.Printf("%s canceling the stressers...", tt.logPrefix())
	for _, s := range tt.cluster.Stressers {
		s.Cancel()
	}
	plog.Printf("%s canceled stressers", tt.logPrefix())
}

func (tt *tester) startStressers() error {
	plog.Printf("%s starting the stressers...", tt.logPrefix())
	for _, s := range tt.cluster.Stressers {
		if err := s.Stress(); err != nil {
			return err
		}
	}
	plog.Printf("%s started stressers", tt.logPrefix())
	return nil
}
