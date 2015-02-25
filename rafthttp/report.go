// Copyright 2015 CoreOS, Inc.
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

package rafthttp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

const (
	errBackoffTime = 10 * time.Second
)

var (
	linkErrMapMu sync.Mutex
	linkErrMap   = make(map[linkName]linkErr)
)

type linkName struct {
	local, remote types.ID
	worker        string
}

type linkErr struct {
	err  *opError
	last time.Time
}

func reportOpError(local, remote types.ID, worker string, err *opError) {
	linkErrMapMu.Lock()
	defer linkErrMapMu.Unlock()
	name := linkName{local: local, remote: remote, worker: worker}
	now := time.Now()
	if !err.equal(linkErrMap[name].err) || now.Sub(linkErrMap[name].last) > errBackoffTime {
		linkErrMap[name] = linkErr{err: err, last: now}
		log.Printf("rafthttp: encountered error sending messages in %s from %s to %s: %v", worker, local, remote, err)
	}
}

// opError is the error type passed in the package that describing the
// operation, worker type, remote id and address of an error.
type opError struct {
	// op is the operation which caused the error, such as
	// "read" or "write".
	op string
	// addr is the network address on which this error occurred.
	addr string
	// err is the error that occurred during the operation.
	err error
}

func newOpError(op string, addr string, err error) *opError {
	return &opError{
		op:   op,
		addr: addr,
		err:  err,
	}
}

func (e *opError) Error() string {
	if netErr, ok := e.err.(*net.OpError); ok {
		return netErr.Error()
	}
	return fmt.Sprintf("%s %s: %s", e.op, e.addr, e.err)
}

func (e *opError) equal(ee *opError) bool {
	return ee != nil && e.op == ee.op && e.addr == ee.addr && e.err.Error() == ee.err.Error()
}
