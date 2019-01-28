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

package tikv

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/metrics"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	// NoJitter makes the backoff sequence strict exponential.
	NoJitter = 1 + iota
	// FullJitter applies random factors to strict exponential.
	FullJitter
	// EqualJitter is also randomized, but prevents very short sleeps.
	EqualJitter
	// DecorrJitter increases the maximum jitter based on the last random value.
	DecorrJitter
)

// NewBackoffFn creates a backoff func which implements exponential backoff with
// optional jitters.
// See http://www.awsarchitectureblog.com/2015/03/backoff.html
func NewBackoffFn(base, cap, jitter int) func(ctx context.Context) int {
	if base < 2 {
		// Top prevent panic in 'rand.Intn'.
		base = 2
	}
	attempts := 0
	lastSleep := base
	return func(ctx context.Context) int {
		var sleep int
		switch jitter {
		case NoJitter:
			sleep = expo(base, cap, attempts)
		case FullJitter:
			v := expo(base, cap, attempts)
			sleep = rand.Intn(v)
		case EqualJitter:
			v := expo(base, cap, attempts)
			sleep = v/2 + rand.Intn(v/2)
		case DecorrJitter:
			sleep = int(math.Min(float64(cap), float64(base+rand.Intn(lastSleep*3-base))))
		}
		log.Debugf("backoff base %d, sleep %d", base, sleep)
		select {
		case <-time.After(time.Duration(sleep) * time.Millisecond):
		case <-ctx.Done():
		}

		attempts++
		lastSleep = sleep
		return lastSleep
	}
}

func expo(base, cap, n int) int {
	return int(math.Min(float64(cap), float64(base)*math.Pow(2.0, float64(n))))
}

type backoffType int

// Back off types.
const (
	boTiKVRPC backoffType = iota
	BoTxnLock
	boTxnLockFast
	boPDRPC
	BoRegionMiss
	BoUpdateLeader
	boServerBusy
)

func (t backoffType) createFn(vars *kv.Variables) func(context.Context) int {
	if vars.Hook != nil {
		vars.Hook(t.String(), vars)
	}
	switch t {
	case boTiKVRPC:
		return NewBackoffFn(100, 2000, EqualJitter)
	case BoTxnLock:
		return NewBackoffFn(200, 3000, EqualJitter)
	case boTxnLockFast:
		return NewBackoffFn(vars.BackoffLockFast, 3000, EqualJitter)
	case boPDRPC:
		return NewBackoffFn(500, 3000, EqualJitter)
	case BoRegionMiss:
		return NewBackoffFn(100, 500, NoJitter)
	case BoUpdateLeader:
		return NewBackoffFn(1, 10, NoJitter)
	case boServerBusy:
		return NewBackoffFn(2000, 10000, EqualJitter)
	}
	return nil
}

func (t backoffType) String() string {
	switch t {
	case boTiKVRPC:
		return "tikvRPC"
	case BoTxnLock:
		return "txnLock"
	case boTxnLockFast:
		return "txnLockFast"
	case boPDRPC:
		return "pdRPC"
	case BoRegionMiss:
		return "regionMiss"
	case BoUpdateLeader:
		return "updateLeader"
	case boServerBusy:
		return "serverBusy"
	}
	return ""
}

func (t backoffType) TError() error {
	switch t {
	case boTiKVRPC:
		return ErrTiKVServerTimeout
	case BoTxnLock, boTxnLockFast:
		return ErrResolveLockTimeout
	case boPDRPC:
		return ErrPDServerTimeout.GenWithStackByArgs(txnRetryableMark)
	case BoRegionMiss, BoUpdateLeader:
		return ErrRegionUnavailable
	case boServerBusy:
		return ErrTiKVServerBusy
	}
	return terror.ClassTiKV.New(mysql.ErrUnknown, mysql.MySQLErrName[mysql.ErrUnknown])
}

// Maximum total sleep time(in ms) for kv/cop commands.
const (
	copBuildTaskMaxBackoff         = 5000
	tsoMaxBackoff                  = 15000
	scannerNextMaxBackoff          = 20000
	batchGetMaxBackoff             = 20000
	copNextMaxBackoff              = 20000
	getMaxBackoff                  = 20000
	prewriteMaxBackoff             = 20000
	cleanupMaxBackoff              = 20000
	GcOneRegionMaxBackoff          = 20000
	GcResolveLockMaxBackoff        = 100000
	deleteRangeOneRegionMaxBackoff = 100000
	rawkvMaxBackoff                = 20000
	splitRegionBackoff             = 20000
)

// CommitMaxBackoff is max sleep time of the 'commit' command
var CommitMaxBackoff = 41000

// Backoffer is a utility for retrying queries.
type Backoffer struct {
	ctx context.Context

	fn         map[backoffType]func(context.Context) int
	maxSleep   int
	totalSleep int
	errors     []error
	types      []backoffType
	vars       *kv.Variables
}

// txnStartKey is a key for transaction start_ts info in context.Context.
const txnStartKey = "_txn_start_key"

// NewBackoffer creates a Backoffer with maximum sleep time(in ms).
func NewBackoffer(ctx context.Context, maxSleep int) *Backoffer {
	return &Backoffer{
		ctx:      ctx,
		maxSleep: maxSleep,
		vars:     kv.DefaultVars,
	}
}

// WithVars sets the kv.Variables to the Backoffer and return it.
func (b *Backoffer) WithVars(vars *kv.Variables) *Backoffer {
	b.vars = vars
	return b
}

// Backoff sleeps a while base on the backoffType and records the error message.
// It returns a retryable error if total sleep time exceeds maxSleep.
func (b *Backoffer) Backoff(typ backoffType, err error) error {
	if strings.Contains(err.Error(), mismatchClusterID) {
		log.Fatalf("critical error %v", err)
	}
	select {
	case <-b.ctx.Done():
		return errors.Trace(err)
	default:
	}

	metrics.TiKVBackoffCounter.WithLabelValues(typ.String()).Inc()
	// Lazy initialize.
	if b.fn == nil {
		b.fn = make(map[backoffType]func(context.Context) int)
	}
	f, ok := b.fn[typ]
	if !ok {
		f = typ.createFn(b.vars)
		b.fn[typ] = f
	}

	b.totalSleep += f(b.ctx)
	b.types = append(b.types, typ)

	var startTs interface{} = ""
	if ts := b.ctx.Value(txnStartKey); ts != nil {
		startTs = ts
	}
	log.Debugf("%v, retry later(totalsleep %dms, maxsleep %dms), type: %s, txn_start_ts: %v", err, b.totalSleep, b.maxSleep, typ.String(), startTs)

	b.errors = append(b.errors, errors.Errorf("%s at %s", err.Error(), time.Now().Format(time.RFC3339Nano)))
	if b.maxSleep > 0 && b.totalSleep >= b.maxSleep {
		errMsg := fmt.Sprintf("backoffer.maxSleep %dms is exceeded, errors:", b.maxSleep)
		for i, err := range b.errors {
			// Print only last 3 errors for non-DEBUG log levels.
			if log.GetLevel() == log.DebugLevel || i >= len(b.errors)-3 {
				errMsg += "\n" + err.Error()
			}
		}
		log.Warn(errMsg)
		// Use the first backoff type to generate a MySQL error.
		return b.types[0].TError()
	}
	return nil
}

func (b *Backoffer) String() string {
	if b.totalSleep == 0 {
		return ""
	}
	return fmt.Sprintf(" backoff(%dms %v)", b.totalSleep, b.types)
}

// Clone creates a new Backoffer which keeps current Backoffer's sleep time and errors, and shares
// current Backoffer's context.
func (b *Backoffer) Clone() *Backoffer {
	return &Backoffer{
		ctx:        b.ctx,
		maxSleep:   b.maxSleep,
		totalSleep: b.totalSleep,
		errors:     b.errors,
		vars:       b.vars,
	}
}

// Fork creates a new Backoffer which keeps current Backoffer's sleep time and errors, and holds
// a child context of current Backoffer's context.
func (b *Backoffer) Fork() (*Backoffer, context.CancelFunc) {
	ctx, cancel := context.WithCancel(b.ctx)
	return &Backoffer{
		ctx:        ctx,
		maxSleep:   b.maxSleep,
		totalSleep: b.totalSleep,
		errors:     b.errors,
		vars:       b.vars,
	}, cancel
}
