// Copyright 2016 CoreOS, Inc.
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

package tcpproxy

import (
	"io"
	"net"
	"sync"
	"time"
)

type remote struct {
	mu       sync.Mutex
	addr     string
	inactive bool
}

func (r *remote) inactivate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inactive = true
}

func (r *remote) tryReactivate() {
	conn, err := net.Dial("tcp", r.addr)
	if err != nil {
		return
	}
	conn.Close()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inactive = false
	return
}

func (r *remote) isActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.inactive
}

type TCPProxy struct {
	Listener        net.Listener
	Endpoints       []string
	MonitorInterval time.Duration

	donec chan struct{}

	mu         sync.Mutex // guards the following fields
	remotes    []*remote
	nextRemote int
}

func (tp *TCPProxy) Run() error {
	tp.donec = make(chan struct{})
	if tp.MonitorInterval == 0 {
		tp.MonitorInterval = 5 * time.Minute
	}
	for _, ep := range tp.Endpoints {
		tp.remotes = append(tp.remotes, &remote{addr: ep})
	}

	go tp.runMonitor()
	for {
		in, err := tp.Listener.Accept()
		if err != nil {
			return err
		}

		go tp.serve(in)
	}
}

func (tp *TCPProxy) numRemotes() int {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return len(tp.remotes)
}

func (tp *TCPProxy) serve(in net.Conn) {
	var (
		err error
		out net.Conn
	)

	for i := 0; i < tp.numRemotes(); i++ {
		remote := tp.pick()
		if !remote.isActive() {
			continue
		}
		// TODO: add timeout
		out, err = net.Dial("tcp", remote.addr)
		if err == nil {
			break
		}
		remote.inactivate()
	}

	if out == nil {
		in.Close()
		return
	}

	go func() {
		io.Copy(in, out)
		in.Close()
		out.Close()
	}()

	io.Copy(out, in)
	out.Close()
	in.Close()
}

// pick picks a remote in round-robin fashion
func (tp *TCPProxy) pick() *remote {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	picked := tp.remotes[tp.nextRemote]
	tp.nextRemote = (tp.nextRemote + 1) % len(tp.remotes)
	return picked
}

func (tp *TCPProxy) runMonitor() {
	for {
		select {
		case <-time.After(tp.MonitorInterval):
			tp.mu.Lock()
			for _, r := range tp.remotes {
				if !r.isActive() {
					go r.tryReactivate()
				}
			}
			tp.mu.Unlock()
		case <-tp.donec:
			return
		}
	}
}

func (tp *TCPProxy) Stop() {
	// graceful shutdown?
	// shutdown current connections?
	tp.Listener.Close()
	close(tp.donec)
}
