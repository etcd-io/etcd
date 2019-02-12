// Copyright 2017 The etcd Authors
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

package mvcc

import (
	"go.etcd.io/etcd/lease"
	"time"
)

type metricsTxnWrite struct {
	TxnWrite
	ranges         uint
	puts           uint
	committedReads uint
	deletes        uint
	rangeStart     time.Time
	putStart       time.Time
}

func newMetricsTxnRead(tr TxnRead) TxnRead {
	txn := &metricsTxnWrite{}
	txn.TxnWrite = &txnReadWrite{tr}
	return txn
}

func newMetricsTxnWrite(tw TxnWrite) TxnWrite {
	txn := &metricsTxnWrite{}
	txn.TxnWrite = tw
	return txn
}

func (tw *metricsTxnWrite) Range(key, end []byte, ro RangeOptions) (*RangeResult, error) {
	tw.ranges++
	tw.rangeStart = time.Now()
	return tw.TxnWrite.Range(key, end, ro)
}

func (tw *metricsTxnWrite) DeleteRange(key, end []byte) (n, rev int64) {
	tw.deletes++
	return tw.TxnWrite.DeleteRange(key, end)
}

func (tw *metricsTxnWrite) Put(key, value []byte, lease lease.LeaseID) (rev int64) {
	tw.puts++
	tw.putStart = time.Now()
	return tw.TxnWrite.Put(key, value, lease)
}

func (tw *metricsTxnWrite) End() {
	defer tw.TxnWrite.End()
	if sum := tw.ranges + tw.puts + tw.deletes; sum > 1 {
		txnCounter.Inc()
	}
	deleteCounter.Add(float64(tw.deletes))
	if tw.puts > 0 {
		putCounter.Add(float64(tw.puts))
		putSec.Observe(time.Since(tw.putStart).Seconds())
	}
	if tw.ranges > 0 {
		// cannot determine if the read transaction is a committed read at the beginning of the transaction
		if tw.TxnWrite.IsCommittedRead() {
			committedReadSec.Observe(time.Since(tw.rangeStart).Seconds())
			committedReadCounter.Inc()
		} else {
			rangeSec.Observe(time.Since(tw.rangeStart).Seconds())
			rangeCounter.Add(float64(tw.ranges))
		}
	}
}
