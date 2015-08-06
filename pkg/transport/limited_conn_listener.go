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

package transport

import (
	"errors"
	"net"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/pkg/runtime"
)

var plog = capnslog.NewPackageLogger("github.com/coreos/etcd/pkg", "transport")

type LimitedConnListener struct {
	net.Listener
	RuntimeFDLimit uint64
}

func (l *LimitedConnListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	n, err := runtime.FDUsage()
	// Check whether fd number in use exceeds the set limit.
	if err == nil && n >= l.RuntimeFDLimit {
		conn.Close()
		plog.Errorf("accept error: closing connection, exceed file descriptor usage limitation (fd limit=%d)", l.RuntimeFDLimit)
		return nil, &acceptError{error: errors.New("exceed file descriptor usage limitation"), temporary: true}
	}
	return conn, nil
}

type acceptError struct {
	error
	temporary bool
}

func (e *acceptError) Timeout() bool { return false }

func (e *acceptError) Temporary() bool { return e.temporary }
