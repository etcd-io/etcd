// Copyright 2018 PingCAP, Inc.
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

package memory

import (
	"sync"

	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	log "github.com/sirupsen/logrus"
)

// ActionOnExceed is the action taken when memory usage exceeds memory quota.
// NOTE: All the implementors should be thread-safe.
type ActionOnExceed interface {
	// Action will be called when memory usage exceeds memory quota by the
	// corresponding Tracker.
	Action(t *Tracker)
}

// LogOnExceed logs a warning only once when memory usage exceeds memory quota.
type LogOnExceed struct {
	mutex sync.Mutex // For synchronization.
	acted bool
}

// Action logs a warning only once when memory usage exceeds memory quota.
func (a *LogOnExceed) Action(t *Tracker) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if !a.acted {
		a.acted = true
		log.Warnf(errMemExceedThreshold.GenWithStackByArgs(t.label, t.BytesConsumed(), t.bytesLimit, t.String()).Error())
	}
}

// PanicOnExceed panics when memory usage exceeds memory quota.
type PanicOnExceed struct {
	mutex sync.Mutex // For synchronization.
	acted bool
}

// Action panics when memory usage exceeds memory quota.
func (a *PanicOnExceed) Action(t *Tracker) {
	a.mutex.Lock()
	if a.acted {
		a.mutex.Unlock()
		return
	}
	a.acted = true
	a.mutex.Unlock()
	panic(PanicMemoryExceed + t.String())
}

var (
	errMemExceedThreshold = terror.ClassExecutor.New(codeMemExceedThreshold, mysql.MySQLErrName[mysql.ErrMemExceedThreshold])
)

const (
	codeMemExceedThreshold terror.ErrCode = 8001

	// PanicMemoryExceed represents the panic message when out of memory quota.
	PanicMemoryExceed string = "Out Of Memory Quota!"
)
