// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package oracles

import (
	"sync/atomic"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/pd/client"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/store/tikv/oracle"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var _ oracle.Oracle = &pdOracle{}

const slowDist = 30 * time.Millisecond

// pdOracle is an Oracle that uses a placement driver client as source.
type pdOracle struct {
	c      pd.Client
	lastTS uint64
	quit   chan struct{}
}

// NewPdOracle create an Oracle that uses a pd client source.
// Refer https://github.com/pingcap/pd/blob/master/client/client.go for more details.
// PdOracle mantains `lastTS` to store the last timestamp got from PD server. If
// `GetTimestamp()` is not called after `updateInterval`, it will be called by
// itself to keep up with the timestamp on PD server.
func NewPdOracle(pdClient pd.Client, updateInterval time.Duration) (oracle.Oracle, error) {
	o := &pdOracle{
		c:    pdClient,
		quit: make(chan struct{}),
	}
	ctx := context.TODO()
	go o.updateTS(ctx, updateInterval)
	// Initialize lastTS by Get.
	_, err := o.GetTimestamp(ctx)
	if err != nil {
		o.Close()
		return nil, errors.Trace(err)
	}
	return o, nil
}

// IsExpired returns whether lockTS+TTL is expired, both are ms. It uses `lastTS`
// to compare, may return false negative result temporarily.
func (o *pdOracle) IsExpired(lockTS, TTL uint64) bool {
	lastTS := atomic.LoadUint64(&o.lastTS)
	return oracle.ExtractPhysical(lastTS) >= oracle.ExtractPhysical(lockTS)+int64(TTL)
}

// GetTimestamp gets a new increasing time.
func (o *pdOracle) GetTimestamp(ctx context.Context) (uint64, error) {
	ts, err := o.getTimestamp(ctx)
	if err != nil {
		return 0, errors.Trace(err)
	}
	o.setLastTS(ts)
	return ts, nil
}

type tsFuture struct {
	pd.TSFuture
	o *pdOracle
}

// Wait implements the oracle.Future interface.
func (f *tsFuture) Wait() (uint64, error) {
	now := time.Now()
	physical, logical, err := f.TSFuture.Wait()
	metrics.TSFutureWaitDuration.Observe(time.Since(now).Seconds())
	if err != nil {
		return 0, errors.Trace(err)
	}
	ts := oracle.ComposeTS(physical, logical)
	f.o.setLastTS(ts)
	return ts, nil
}

func (o *pdOracle) GetTimestampAsync(ctx context.Context) oracle.Future {
	ts := o.c.GetTSAsync(ctx)
	return &tsFuture{ts, o}
}

func (o *pdOracle) getTimestamp(ctx context.Context) (uint64, error) {
	now := time.Now()
	physical, logical, err := o.c.GetTS(ctx)
	if err != nil {
		return 0, errors.Trace(err)
	}
	dist := time.Since(now)
	if dist > slowDist {
		log.Warnf("get timestamp too slow: %s", dist)
	}
	return oracle.ComposeTS(physical, logical), nil
}

func (o *pdOracle) setLastTS(ts uint64) {
	lastTS := atomic.LoadUint64(&o.lastTS)
	if ts > lastTS {
		atomic.CompareAndSwapUint64(&o.lastTS, lastTS, ts)
	}
}

func (o *pdOracle) updateTS(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			ts, err := o.getTimestamp(ctx)
			if err != nil {
				log.Errorf("updateTS error: %v", err)
				break
			}
			o.setLastTS(ts)
		case <-o.quit:
			ticker.Stop()
			return
		}
	}
}

func (o *pdOracle) Close() {
	close(o.quit)
}
