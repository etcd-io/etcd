// Copyright 2019 The etcd Authors
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

package backend

// ReadTxRequest is a channel to send a requested ReadTx to when it becomes available.
type ReadTxRequest = chan ReadTx

// concurrentReadScheduler accumulates requests to begin concurrent ReadTxs and waits until
// BeginConcurrentReadTxs is called to begin them. It also limits the number of
// concurrent ReadTxs running at any point in time to the provided maxConcurrentReadTxs.
type concurrentReadScheduler struct {
	maxConcurrentReadTxs uint64
	readTxCh             chan ReadTxRequest
	b                    *backend
	pendingReadCounter   *GaugedCounter
	openReadCounter      *GaugedCounter
}

func newConcurrentReadScheduler(b *backend, maxConcurrentReadTxs uint64) *concurrentReadScheduler {
	return &concurrentReadScheduler{
		maxConcurrentReadTxs: maxConcurrentReadTxs,
		readTxCh:             make(chan ReadTxRequest),
		b:                    b,
		pendingReadCounter:   &GaugedCounter{0, pendingReadGauge},
		openReadCounter:      &GaugedCounter{0, openReadGauge},
	}
}

// RequestConcurrentReadTx requests a new ReadTx and blocks until it is available.
func (r *concurrentReadScheduler) RequestConcurrentReadTx() ReadTx {
	rch := make(chan ReadTx)
	r.pendingReadCounter.Inc()
	defer r.pendingReadCounter.Dec()
	r.readTxCh <- rch
	return <-rch
}

// BeginConcurrentReadTxs begins pending read transactions and sends them
// to the channels of all blocked RequestReadTx() callers.
// Ensures more than maxConcurrentReadTxns are running at the same time.
func (r *concurrentReadScheduler) BeginConcurrentReadTxs() {
	// TODO(jpbetz): This has the potential to backlog indefinitely under heavly load.
	// If we're going to impose a limit here. We might want to do more to ensure we're
	// managing context deadlines and cancelations also.

	concurrentReadTxs := r.openReadCounter.Value()
	for i := uint64(0); i < (r.maxConcurrentReadTxs - concurrentReadTxs); i++ {
		select {
		case rch := <-r.readTxCh:
			rtx, err := r.b.db.Begin(false)
			if err != nil {
				plog.Fatalf("cannot begin read tx (%s)", err)
			}
			rch <- &MonitoredReadTx{r.openReadCounter, &concurrentReadTx{tx: rtx}}
		default:
			// no more to create.
			return
		}
	}
}
