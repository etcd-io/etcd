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
	"sync"
	"time"

	"github.com/pingcap/tidb/store/tikv/oracle"
	"golang.org/x/net/context"
)

var _ oracle.Oracle = &localOracle{}

type localOracle struct {
	sync.Mutex
	lastTimeStampTS uint64
	n               uint64
}

// NewLocalOracle creates an Oracle that uses local time as data source.
func NewLocalOracle() oracle.Oracle {
	return &localOracle{}
}

func (l *localOracle) IsExpired(lockTS uint64, TTL uint64) bool {
	return oracle.GetPhysical(time.Now()) >= oracle.ExtractPhysical(lockTS)+int64(TTL)
}

func (l *localOracle) GetTimestamp(context.Context) (uint64, error) {
	l.Lock()
	defer l.Unlock()
	physical := oracle.GetPhysical(time.Now())
	ts := oracle.ComposeTS(physical, 0)
	if l.lastTimeStampTS == ts {
		l.n++
		return ts + l.n, nil
	}
	l.lastTimeStampTS = ts
	l.n = 0
	return ts, nil
}

func (l *localOracle) GetTimestampAsync(ctx context.Context) oracle.Future {
	return &future{
		ctx: ctx,
		l:   l,
	}
}

type future struct {
	ctx context.Context
	l   *localOracle
}

func (f *future) Wait() (uint64, error) {
	return f.l.GetTimestamp(f.ctx)
}

func (l *localOracle) Close() {
}
